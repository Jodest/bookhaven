package service

import (
	"context"
	"errors"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/tmdb"
	"go.uber.org/zap"
)

// posterTMDBLookupCap bounds the TMDB lookups of one sync chunk.
const posterTMDBLookupCap = 100

// CommonsSource resolves Wikimedia Commons files. Satisfied by
// *bookdata.WikidataClient.
type CommonsSource interface {
	FetchCommonsImages(ctx context.Context, filenames []string) (map[string]*bookdata.CommonsImage, error)
}

// PosterLookup resolves a TMDB poster URL; kind is "movie" or "tv". It
// returns "" when TMDB has no poster. Satisfied by *tmdb.Client.
type PosterLookup interface {
	Poster(ctx context.Context, kind, id string) (string, error)
}

// resolvePosters finds posters for the works of one sync chunk that the sync
// may still fill: Commons first (P3383, then P18), then TMDB. It is
// best-effort: every failure is logged and swallowed, and the result is
// keyed by work QID.
func (s *Service) resolvePosters(ctx context.Context, found map[string][]bookdata.ExternalScreenWork) map[string]*domain.PosterInput {
	if s.commons == nil && s.tmdb == nil {
		return nil
	}
	byQID := map[string]bookdata.ExternalScreenWork{}
	var qids []string
	for _, works := range found {
		for _, w := range works {
			if _, dup := byQID[w.QID]; !dup {
				byQID[w.QID] = w
				qids = append(qids, w.QID)
			}
		}
	}
	if len(qids) == 0 {
		return nil
	}
	candidates, err := s.repo.PosterCandidates(ctx, qids)
	if err != nil {
		s.log.Warn("wikidata adaptation sync: poster candidates failed", zap.Error(err))
		return nil
	}
	var todo []bookdata.ExternalScreenWork
	for _, q := range qids {
		if candidates[q] {
			todo = append(todo, byQID[q])
		}
	}
	if len(todo) == 0 {
		return nil
	}

	out := map[string]*domain.PosterInput{}
	s.commonsPosters(ctx, todo, out)

	if s.tmdb != nil {
		s.tmdbPosters(ctx, todo, out)
	}
	return out
}

func (s *Service) commonsPosters(ctx context.Context, todo []bookdata.ExternalScreenWork, out map[string]*domain.PosterInput) {
	if s.commons == nil {
		return
	}
	var names []string
	seen := map[string]struct{}{}
	for _, w := range todo {
		for _, n := range []string{w.PosterFile, w.ImageFile} {
			if n == "" {
				continue
			}
			if _, dup := seen[n]; !dup {
				seen[n] = struct{}{}
				names = append(names, n)
			}
		}
	}
	images := map[string]*bookdata.CommonsImage{}
	for start := 0; start < len(names); start += bookdata.MaxEntitiesPerRequest {
		batch := names[start:min(start+bookdata.MaxEntitiesPerRequest, len(names))]
		got, err := s.commons.FetchCommonsImages(ctx, batch)
		if err != nil {
			s.log.Warn("wikidata adaptation sync: commons poster lookup failed", zap.Error(err))
			continue
		}
		for k, v := range got {
			images[k] = v
		}
	}
	for _, w := range todo {
		for _, n := range []string{w.PosterFile, w.ImageFile} {
			if img := images[n]; n != "" && usableCommonsImage(img) {
				out[w.QID] = &domain.PosterInput{
					Source:     domain.PosterSourceCommons,
					URL:        img.ThumbURL,
					Author:     strPtr(img.Author),
					License:    strPtr(img.License),
					LicenseURL: strPtr(img.LicenseURL),
					SourceURL:  strPtr(img.PageURL),
				}
				break
			}
		}
	}
}

// usableCommonsImage: a free licence, a thumbnail and, when the licence needs
// attribution, a known author.
func usableCommonsImage(img *bookdata.CommonsImage) bool {
	if img == nil || !img.Free || img.ThumbURL == "" {
		return false
	}
	return !img.AttributionRequired || img.Author != ""
}

func (s *Service) tmdbPosters(ctx context.Context, todo []bookdata.ExternalScreenWork, out map[string]*domain.PosterInput) {
	lookups := 0
	for _, w := range todo {
		if out[w.QID] != nil {
			continue
		}
		type try struct{ kind, id string }
		movie, tv := try{tmdb.KindMovie, w.TMDBMovieID}, try{tmdb.KindTV, w.TMDBTVID}
		order := []try{movie, tv}
		if w.Kind == domain.KindSeries || w.Kind == domain.KindAnimatedSeries {
			order = []try{tv, movie}
		}
		for _, t := range order {
			if t.id == "" {
				continue
			}
			if lookups >= posterTMDBLookupCap {
				return
			}
			lookups++
			u, err := s.tmdb.Poster(ctx, t.kind, t.id)
			if errors.Is(err, tmdb.ErrUnauthorized) {
				s.log.Warn("wikidata adaptation sync: tmdb rejected the api key; skipping tmdb for this chunk")
				return
			}
			if err != nil {
				s.log.Warn("wikidata adaptation sync: tmdb poster lookup failed", zap.String("kind", t.kind), zap.Error(err))
				continue
			}
			if u == "" {
				continue
			}
			out[w.QID] = &domain.PosterInput{
				Source: domain.PosterSourceTMDB,
				URL:    u,
			}
			break
		}
	}
}
