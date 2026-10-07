package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Author enrichment from Wikidata (docs/features/author-wikidata-enrichment.md).

// AuthorEnrichmentSource fetches Wikidata entities and Commons images in
// batches. Satisfied by *bookdata.WikidataClient.
type AuthorEnrichmentSource interface {
	FetchAuthors(ctx context.Context, qids []string) (map[string]*bookdata.ExternalAuthorRecord, error)
	FetchCommonsImages(ctx context.Context, filenames []string) (map[string]*bookdata.CommonsImage, error)
}

// WithAuthorEnrichmentSource wires the enrichment source. cmd/api and
// cmd/worker pass the same *bookdata.WikidataClient they use elsewhere so
// every Wikidata/Commons call shares one rate limiter. Without it the
// admin action returns ErrAuthorEnrichmentUnavailable, harvest skips
// enrichment and a direct import applies only the dates.
func WithAuthorEnrichmentSource(src AuthorEnrichmentSource) Option {
	return func(s *Service) {
		s.enrichmentSource = src
	}
}

const (
	// authorEnrichmentCap bounds one admin enrichment request (one queue
	// page) and is also the per-chunk QID count (wbgetentities limit).
	authorEnrichmentCap = bookdata.MaxEntitiesPerRequest
)

var (
	// ErrAuthorEnrichmentUnavailable: no enrichment source configured (503).
	ErrAuthorEnrichmentUnavailable = errors.New("author enrichment source is not configured")
	// ErrAuthorEnrichmentTooLarge: more than authorEnrichmentCap distinct
	// ids in one admin request (400).
	ErrAuthorEnrichmentTooLarge = fmt.Errorf("too many author ids: at most %d per request", authorEnrichmentCap)
	// ErrAuthorEnrichmentEmpty: no ids (400).
	ErrAuthorEnrichmentEmpty = errors.New("ids must not be empty")
)

// Enrichment item outcomes.
const (
	EnrichOutcomeEnriched  = "enriched"
	EnrichOutcomeUnchanged = "unchanged"
	EnrichOutcomeSkipped   = "skipped"
	EnrichOutcomeError     = "error"
)

// Filled field names.
const (
	EnrichFieldBirthDate = "birth_date"
	EnrichFieldDeathDate = "death_date"
	EnrichFieldPhoto     = "photo"
)

// Enrichment notes.
const (
	EnrichNoteNoQID                = "no_qid"
	EnrichNoteEntityNotFound       = "entity_not_found"
	EnrichNoteBirthDateUnusable    = "birth_date_unusable"
	EnrichNoteDeathDateUnusable    = "death_date_unusable"
	EnrichNotePhotoUnavailable     = "photo_unavailable"
	EnrichNotePhotoLicenseNotFree  = "photo_license_not_free"
	EnrichNotePhotoAttributionMiss = "photo_attribution_missing"
	EnrichNoteWikidataUnavailable  = "wikidata_unavailable"
	EnrichNoteCommonsUnavailable   = "commons_unavailable"
)

// photoURLMaxLen guards against an absurd thumbnail URL.
const photoURLMaxLen = 2048

// AuthorEnrichmentReport is the JSON report of
// POST /admin/authors/enrich-wikidata (HTTP 200 regardless of per-item
// outcomes).
type AuthorEnrichmentReport struct {
	Processed int                    `json:"processed"`
	Enriched  int                    `json:"enriched"`
	Items     []AuthorEnrichmentItem `json:"items"`
}

// AuthorEnrichmentItem is the outcome for one requested author.
type AuthorEnrichmentItem struct {
	AuthorID string   `json:"author_id"`
	Name     string   `json:"name"`
	Outcome  string   `json:"outcome"`
	Filled   []string `json:"filled"`
	Notes    []string `json:"notes"`
	Message  string   `json:"message"`
}

// AuthorEnrichmentSummary is the enrichment part of a direct import result.
type AuthorEnrichmentSummary struct {
	Filled []string `json:"filled"`
	Notes  []string `json:"notes"`
	Error  string   `json:"error"`
}

// EnrichAuthorsFromWikidata is the admin action: 1..authorEnrichmentCap
// author ids (deduped), any status. See enrichAuthors.
func (s *Service) EnrichAuthorsFromWikidata(ctx context.Context, ids []uuid.UUID) (*AuthorEnrichmentReport, error) {
	if s.enrichmentSource == nil {
		return nil, ErrAuthorEnrichmentUnavailable
	}
	ids = dedupeUUIDs(ids)
	if len(ids) == 0 {
		return nil, ErrAuthorEnrichmentEmpty
	}
	if len(ids) > authorEnrichmentCap {
		return nil, ErrAuthorEnrichmentTooLarge
	}
	return s.enrichAuthors(ctx, ids)
}

// EnrichHarvestAuthors enriches the authors a harvest job created (no cap;
// chunked by authorEnrichmentCap QIDs). Same semantics as
// EnrichAuthorsFromWikidata.
func (s *Service) EnrichHarvestAuthors(ctx context.Context, ids []uuid.UUID) (*AuthorEnrichmentReport, error) {
	if s.enrichmentSource == nil {
		return nil, ErrAuthorEnrichmentUnavailable
	}
	ids = dedupeUUIDs(ids)
	if len(ids) == 0 {
		return &AuthorEnrichmentReport{Items: []AuthorEnrichmentItem{}}, nil
	}
	return s.enrichAuthors(ctx, ids)
}

// HasAuthorEnrichmentSource reports whether enrichment is configured.
func (s *Service) HasAuthorEnrichmentSource() bool {
	return s.enrichmentSource != nil
}

func dedupeUUIDs(ids []uuid.UUID) []uuid.UUID {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// enrichAuthors loads the authors, skips those without a QID, and per chunk
// of authorEnrichmentCap QIDs makes one wbgetentities call and at most one
// Commons call, then applies one fill update per author. A chunk-level
// fetch failure marks that chunk's items as errors and the other chunks
// continue. When every attempted wbgetentities call failed, the source
// error is returned together with the report.
func (s *Service) enrichAuthors(ctx context.Context, ids []uuid.UUID) (*AuthorEnrichmentReport, error) {
	authors := make(map[uuid.UUID]*catalogDomain.Author, len(ids))
	for start := 0; start < len(ids); start += authorEnrichmentCap {
		end := min(start+authorEnrichmentCap, len(ids))
		loaded, err := s.repo.GetAuthorsByIDs(ctx, ids[start:end])
		if err != nil {
			return nil, fmt.Errorf("load authors: %w", err)
		}
		for i := range loaded {
			authors[loaded[i].ID] = &loaded[i]
		}
	}

	items := make([]AuthorEnrichmentItem, len(ids))
	var withQID []int // indexes into ids/items
	for i, id := range ids {
		items[i] = AuthorEnrichmentItem{AuthorID: id.String(), Filled: []string{}, Notes: []string{}}
		author, ok := authors[id]
		if !ok {
			items[i].Outcome = EnrichOutcomeError
			items[i].Message = ErrAuthorNotFound.Error()
			continue
		}
		items[i].Name = author.Name
		if authorQID(author) == "" {
			items[i].Outcome = EnrichOutcomeSkipped
			items[i].Notes = append(items[i].Notes, EnrichNoteNoQID)
			continue
		}
		withQID = append(withQID, i)
	}

	attempted, failed := 0, 0
	var lastErr error
	for start := 0; start < len(withQID); start += authorEnrichmentCap {
		chunk := withQID[start:min(start+authorEnrichmentCap, len(withQID))]
		attempted++
		if err := s.enrichChunk(ctx, chunk, ids, authors, items); err != nil {
			failed++
			lastErr = err
		}
	}

	report := &AuthorEnrichmentReport{Processed: len(items), Items: items}
	for _, item := range items {
		if item.Outcome == EnrichOutcomeEnriched {
			report.Enriched++
		}
	}
	if attempted > 0 && failed == attempted {
		return report, lastErr
	}
	return report, nil
}

// enrichChunk processes up to authorEnrichmentCap QID authors. It returns
// the wbgetentities error (items are then marked as errors).
func (s *Service) enrichChunk(ctx context.Context, chunk []int, ids []uuid.UUID, authors map[uuid.UUID]*catalogDomain.Author, items []AuthorEnrichmentItem) error {
	qids := make([]string, 0, len(chunk))
	for _, i := range chunk {
		qids = append(qids, authorQID(authors[ids[i]]))
	}

	records, err := s.enrichmentSource.FetchAuthors(ctx, qids)
	if err != nil {
		s.log.Warn("wikidata author enrichment: entity fetch failed",
			zap.Strings("qids", qids), zap.Error(err))
		for _, i := range chunk {
			items[i].Outcome = EnrichOutcomeError
			items[i].Notes = append(items[i].Notes, EnrichNoteWikidataUnavailable)
			items[i].Message = "wikidata unavailable"
		}
		return err
	}

	// Photo files for authors whose photo is still empty.
	var files []string
	seenFile := map[string]struct{}{}
	for _, i := range chunk {
		author := authors[ids[i]]
		rec := records[authorQID(author)]
		if rec == nil || rec.PhotoFile == "" || !isEmptyPhoto(author.PhotoURL) {
			continue
		}
		if _, ok := seenFile[rec.PhotoFile]; !ok {
			seenFile[rec.PhotoFile] = struct{}{}
			files = append(files, rec.PhotoFile)
		}
	}
	images, commonsErr := s.fetchCommonsImages(ctx, files)

	for _, i := range chunk {
		author := authors[ids[i]]
		rec := records[authorQID(author)]
		if rec == nil {
			items[i].Outcome = EnrichOutcomeSkipped
			items[i].Notes = append(items[i].Notes, EnrichNoteEntityNotFound)
			continue
		}
		res := s.enrichFromRecord(ctx, author, rec, images, commonsErr)
		items[i].Filled = res.Filled
		items[i].Notes = append(items[i].Notes, res.Notes...)
		switch {
		case res.Error != "":
			items[i].Outcome = EnrichOutcomeError
			items[i].Message = res.Error
		case len(res.Filled) > 0:
			items[i].Outcome = EnrichOutcomeEnriched
		default:
			items[i].Outcome = EnrichOutcomeUnchanged
		}
	}
	return nil
}

// fetchCommonsImages resolves files (≤ authorEnrichmentCap) in one call; no
// files or no source → nothing fetched.
func (s *Service) fetchCommonsImages(ctx context.Context, files []string) (map[string]*bookdata.CommonsImage, error) {
	if len(files) == 0 || s.enrichmentSource == nil {
		return nil, nil
	}
	images, err := s.enrichmentSource.FetchCommonsImages(ctx, files)
	if err != nil {
		s.log.Warn("wikidata author enrichment: commons fetch failed",
			zap.Strings("files", files), zap.Error(err))
		return nil, err
	}
	if images == nil {
		images = map[string]*bookdata.CommonsImage{}
	}
	return images, nil
}

// enrichFromRecord computes the fill set for author against its pre-read
// values, applies it with one fill-empty-only update, mirrors the filled
// values onto author and logs the outcome. Shared by the batch path and
// the direct import. images/commonsErr are the Commons lookup for the
// chunk (nil images without an error = photo not looked up).
func (s *Service) enrichFromRecord(ctx context.Context, author *catalogDomain.Author, rec *bookdata.ExternalAuthorRecord, images map[string]*bookdata.CommonsImage, commonsErr error) AuthorEnrichmentSummary {
	res := AuthorEnrichmentSummary{Filled: []string{}, Notes: []string{}}
	var fill repository.AuthorEnrichmentFill

	if rec.BirthDateUnusable {
		res.Notes = append(res.Notes, EnrichNoteBirthDateUnusable)
	}
	if rec.DeathDateUnusable {
		res.Notes = append(res.Notes, EnrichNoteDeathDateUnusable)
	}
	if author.BirthDate == nil && rec.BirthDate != nil {
		fill.BirthDate = rec.BirthDate
	}
	if author.DeathDate == nil && rec.DeathDate != nil {
		birth := author.BirthDate
		if birth == nil {
			birth = fill.BirthDate
		}
		if birth != nil && rec.DeathDate.Before(*birth) {
			res.Notes = append(res.Notes, EnrichNoteDeathDateUnusable)
		} else {
			fill.DeathDate = rec.DeathDate
		}
	}

	if rec.PhotoFile != "" && isEmptyPhoto(author.PhotoURL) {
		photo, note := photoFillFor(images[rec.PhotoFile], commonsErr, images != nil)
		if note != "" {
			res.Notes = append(res.Notes, note)
		}
		fill.Photo = photo
	}

	if !fill.IsEmpty() {
		if err := s.repo.FillAuthorEnrichment(ctx, author.ID, fill); err != nil {
			s.log.Warn("wikidata author enrichment: fill update failed",
				zap.String("author_id", author.ID.String()), zap.Error(err))
			res.Error = "failed to save enrichment"
			return res
		}
		if fill.BirthDate != nil {
			author.BirthDate = fill.BirthDate
			res.Filled = append(res.Filled, EnrichFieldBirthDate)
		}
		if fill.DeathDate != nil {
			author.DeathDate = fill.DeathDate
			res.Filled = append(res.Filled, EnrichFieldDeathDate)
		}
		if p := fill.Photo; p != nil {
			url, license, source := p.URL, p.License, p.SourceURL
			author.PhotoURL = &url
			author.PhotoAuthor = p.Author
			author.PhotoLicense = &license
			author.PhotoLicenseURL = p.LicenseURL
			author.PhotoSourceURL = &source
			res.Filled = append(res.Filled, EnrichFieldPhoto)
		}
	}

	s.log.Info("wikidata author enrichment",
		zap.String("author_id", author.ID.String()),
		zap.String("qid", authorQID(author)),
		zap.Strings("filled", res.Filled),
		zap.Strings("notes", res.Notes))
	return res
}

// photoFillFor applies rule 3 to a resolved Commons image. looked reports
// that a Commons lookup ran (false: no source configured → silently no
// photo).
func photoFillFor(img *bookdata.CommonsImage, commonsErr error, looked bool) (*repository.PhotoFill, string) {
	switch {
	case commonsErr != nil:
		return nil, EnrichNoteCommonsUnavailable
	case !looked:
		return nil, ""
	case img == nil || img.ThumbURL == "" || img.PageURL == "" || len(img.ThumbURL) > photoURLMaxLen:
		return nil, EnrichNotePhotoUnavailable
	case !img.Free:
		return nil, EnrichNotePhotoLicenseNotFree
	case img.AttributionRequired && img.Author == "":
		return nil, EnrichNotePhotoAttributionMiss
	}
	fill := &repository.PhotoFill{
		URL:       img.ThumbURL,
		License:   img.License,
		SourceURL: img.PageURL,
	}
	if img.Author != "" {
		author := img.Author
		fill.Author = &author
	}
	if img.LicenseURL != "" {
		licenseURL := img.LicenseURL
		fill.LicenseURL = &licenseURL
	}
	return fill, ""
}

// enrichImportedAuthor enriches an author just created by a direct import
// from the record FetchAuthor already returned (plus one Commons call for
// the photo when a source is configured). Never fails the import.
func (s *Service) enrichImportedAuthor(ctx context.Context, author *catalogDomain.Author, rec *bookdata.ExternalAuthorRecord) *AuthorEnrichmentSummary {
	var images map[string]*bookdata.CommonsImage
	var commonsErr error
	if rec.PhotoFile != "" && isEmptyPhoto(author.PhotoURL) && s.enrichmentSource != nil {
		images, commonsErr = s.fetchCommonsImages(ctx, []string{rec.PhotoFile})
	}
	res := s.enrichFromRecord(ctx, author, rec, images, commonsErr)
	return &res
}

func authorQID(a *catalogDomain.Author) string {
	if a == nil || a.ExternalSource == nil || a.ExternalID == nil ||
		*a.ExternalSource != catalogDomain.AuthorExternalSourceWikidata {
		return ""
	}
	return strings.TrimSpace(*a.ExternalID)
}

func isEmptyPhoto(url *string) bool {
	return url == nil || strings.TrimSpace(*url) == ""
}
