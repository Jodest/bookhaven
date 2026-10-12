package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ResolveOrCreateAuthorIDsByNames resolves each display name to an existing
// author (matched by slugify(name), the existing normalized-name key) or
// creates a new author when no match exists. Empty/whitespace-only names are
// skipped. Order is not guaranteed to match the input order; duplicates
// (by resolved ID) are collapsed.
//
// This is the admin JSON/CSV import's author-matching strategy: exact
// normalized-name match, autocreate on miss, no fuzzy matching (business
// analysis Decision 3.a). Import is admin-curated input, so missing authors
// are created as published, and a slug-matched author is linked whatever
// its status (see docs/features/author-status.md rule 5). Harvest uses
// ResolveOrCreateHarvestAuthors instead.
func (s *Service) ResolveOrCreateAuthorIDsByNames(ctx context.Context, names []string) ([]string, error) {
	ids, _, err := s.resolveOrCreateAuthorsByNames(ctx, names)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// resolveOrCreateAuthorsByNames is ResolveOrCreateAuthorIDsByNames that
// also returns the authors it created, in creation order. On a failure it
// returns the authors created so far together with the error; whether they
// stay in the catalog depends on the caller (the file import rolls them back
// with its row transaction).
func (s *Service) resolveOrCreateAuthorsByNames(ctx context.Context, names []string) ([]string, []*catalogDomain.Author, error) {
	seen := make(map[string]struct{}, len(names))
	ids := make([]string, 0, len(names))
	var createdAuthors []*catalogDomain.Author

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		slug := slugify(name)
		author, err := s.repo.GetAuthorBySlug(ctx, slug)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, createdAuthors, fmt.Errorf("lookup author %q: %w", name, err)
			}
			created, createErr := s.CreateAuthor(ctx, AuthorInput{Name: name})
			if createErr != nil {
				return nil, createdAuthors, fmt.Errorf("create author %q: %w", name, createErr)
			}
			author = created
			createdAuthors = append(createdAuthors, created)
		}

		id := author.ID.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids, createdAuthors, nil
}

// AuthorRef is one harvested author reference: a display name plus an
// optional bare Wikidata QID (empty when the source reported none).
type AuthorRef struct {
	Name string
	QID  string
}

// ResolveOrCreateHarvestAuthors resolves harvested author refs to author IDs
// per docs/features/author-status.md rule 4:
//
//  1. QID present → match by (wikidata, QID). A rejected match is a
//     tombstone: the ref is skipped (no link, nothing created). A
//     pending/published match is linked.
//  2. Otherwise / no QID match → match by slugify(name).
//     - rejected → skipped; if it has no QID and one was given, the QID is
//     backfilled so future harvests match the tombstone by QID.
//     - no QID on the author, QID given → QID backfilled (status
//     unchanged) and linked.
//     - same QID or no QID given → linked.
//     - a different QID (namesake) → a new pending author is created with
//     slug slugify(name)+"-"+lower(QID) and linked; the existing one is
//     left untouched.
//  3. No match → a new pending author (with the QID, if any) is created and
//     linked.
//
// Empty names are skipped; duplicates (by resolved ID) are collapsed. When
// every ref is skipped the result is empty (the caller still creates the
// hidden book) and an info line with the skipped QIDs is logged.
func (s *Service) ResolveOrCreateHarvestAuthors(ctx context.Context, refs []AuthorRef) ([]string, error) {
	res, err := s.ResolveHarvestAuthorRefs(ctx, refs)
	if err != nil {
		return nil, err
	}
	return res.IDs, nil
}

// HarvestAuthorsResolution is the result of ResolveHarvestAuthorRefs.
type HarvestAuthorsResolution struct {
	// IDs are the linked author IDs (see ResolveOrCreateHarvestAuthors).
	IDs []string
	// CreatedWithQID are the authors created by this call that carry a
	// Wikidata QID - the harvest enriches them at the end of the job
	// (docs/features/author-wikidata-enrichment.md rule 4b).
	CreatedWithQID []uuid.UUID
}

// ResolveHarvestAuthorRefs is ResolveOrCreateHarvestAuthors that also
// reports which authors it created with a QID.
func (s *Service) ResolveHarvestAuthorRefs(ctx context.Context, refs []AuthorRef) (HarvestAuthorsResolution, error) {
	seen := make(map[string]struct{}, len(refs))
	ids := make([]string, 0, len(refs))
	var created []uuid.UUID
	var skipped []string

	for _, ref := range refs {
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			continue
		}
		qid := strings.TrimSpace(ref.QID)

		res, err := s.resolveHarvestAuthor(ctx, name, qid)
		if err != nil {
			return HarvestAuthorsResolution{}, err
		}
		if res.Outcome == AuthorOutcomeRejected {
			skipped = append(skipped, name+" "+qid)
			continue
		}
		author := res.Author
		if res.Outcome == AuthorOutcomeCreated && authorQID(author) != "" {
			created = append(created, author.ID)
		}

		id := author.ID.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(skipped) > 0 {
		s.log.Info("harvest skipped rejected (tombstoned) authors",
			zap.Strings("authors", skipped),
			zap.Bool("all_skipped", len(ids) == 0))
	}
	return HarvestAuthorsResolution{IDs: ids, CreatedWithQID: created}, nil
}

// Author resolution outcomes shared by the harvest resolver and the direct
// Wikidata import (docs/features/author-direct-import.md).
const (
	AuthorOutcomeCreated  = "created"
	AuthorOutcomeExisting = "existing"
	AuthorOutcomeRejected = "rejected"
)

// Author resolution match kinds ("" for a created author).
const (
	AuthorMatchQID  = "qid"
	AuthorMatchSlug = "slug"
)

// AuthorResolution is the result of resolving one name/QID with the
// harvest rules (author-status.md rule 4).
type AuthorResolution struct {
	// Author is the matched or created author. For a rejected outcome it
	// is the rejected (tombstone) author.
	Author *catalogDomain.Author
	// Outcome is AuthorOutcomeCreated, AuthorOutcomeExisting or
	// AuthorOutcomeRejected.
	Outcome string
	// Match is AuthorMatchQID or AuthorMatchSlug for an existing/rejected
	// match, "" for a created author.
	Match string
	// QIDBackfilled reports that the QID was written to a slug-matched
	// author that had none.
	QIDBackfilled bool
	// NamesakeOf is the slug-matched author with a different QID when a
	// separate namesake author was created.
	NamesakeOf *catalogDomain.Author
}

// resolveHarvestAuthor applies rule 4 to a single (trimmed, non-empty)
// name/QID. A rejected outcome means "skip" (tombstone).
func (s *Service) resolveHarvestAuthor(ctx context.Context, name, qid string) (*AuthorResolution, error) {
	if qid != "" {
		res, err := s.resolveAuthorByQID(ctx, qid)
		if err != nil {
			return nil, err
		}
		if res != nil {
			return res, nil
		}
	}
	return s.resolveAuthorBySlug(ctx, name, qid)
}

// resolveAuthorByQID is rule 4 step 1: match by (wikidata, qid). It returns
// a nil resolution (and nil error) when no author carries the QID.
func (s *Service) resolveAuthorByQID(ctx context.Context, qid string) (*AuthorResolution, error) {
	author, err := s.repo.GetAuthorByExternalID(ctx, catalogDomain.AuthorExternalSourceWikidata, qid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("lookup author by qid %q: %w", qid, err)
	}
	outcome := AuthorOutcomeExisting
	if author.Status == catalogDomain.AuthorStatusRejected {
		outcome = AuthorOutcomeRejected
	}
	return &AuthorResolution{Author: author, Outcome: outcome, Match: AuthorMatchQID}, nil
}

// resolveAuthorBySlug is rule 4 from the slug step on (the QID, when given,
// is known not to match any author yet).
func (s *Service) resolveAuthorBySlug(ctx context.Context, name, qid string) (*AuthorResolution, error) {
	slug := slugify(name)
	author, err := s.repo.GetAuthorBySlug(ctx, slug)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("lookup author %q: %w", name, err)
		}
		created, err := s.createHarvestAuthor(ctx, name, "", qid)
		if err != nil {
			return nil, err
		}
		return &AuthorResolution{Author: created, Outcome: AuthorOutcomeCreated}, nil
	}

	res := &AuthorResolution{Author: author, Outcome: AuthorOutcomeExisting, Match: AuthorMatchSlug}
	hasQID := author.ExternalSource != nil && author.ExternalID != nil
	if qid != "" && !hasQID {
		if err := s.repo.SetAuthorExternalID(ctx, author.ID, catalogDomain.AuthorExternalSourceWikidata, qid); err != nil {
			return nil, fmt.Errorf("backfill author %q qid %q: %w", name, qid, err)
		}
		source := catalogDomain.AuthorExternalSourceWikidata
		backfilled := qid
		author.ExternalSource = &source
		author.ExternalID = &backfilled
		hasQID = true
		res.QIDBackfilled = true
	}

	if author.Status == catalogDomain.AuthorStatusRejected {
		res.Outcome = AuthorOutcomeRejected
		return res, nil
	}

	if qid != "" && hasQID && (*author.ExternalSource != catalogDomain.AuthorExternalSourceWikidata || *author.ExternalID != qid) {
		// Namesake: a different entity already owns this slug.
		created, err := s.createHarvestAuthor(ctx, name, slug+"-"+strings.ToLower(qid), qid)
		if err != nil {
			return nil, err
		}
		return &AuthorResolution{Author: created, Outcome: AuthorOutcomeCreated, NamesakeOf: author}, nil
	}
	return res, nil
}

// createHarvestAuthor creates a pending author (slug "" = derived from the
// name) carrying the Wikidata QID when one is given.
func (s *Service) createHarvestAuthor(ctx context.Context, name, slug, qid string) (*catalogDomain.Author, error) {
	identity := authorIdentity{status: catalogDomain.AuthorStatusPending}
	if qid != "" {
		source := catalogDomain.AuthorExternalSourceWikidata
		extID := qid
		identity.externalSource = &source
		identity.externalID = &extID
	}
	created, err := s.createAuthor(ctx, AuthorInput{Name: name, Slug: slug}, identity)
	if err != nil {
		return nil, fmt.Errorf("create author %q: %w", name, err)
	}
	return created, nil
}

// ResolveOrCreateGenreIDsByNames resolves each genre name to an existing
// genre (matched by slugify(name)) or creates a new genre when no match
// exists. Empty/whitespace-only names are skipped. Duplicates (by resolved
// ID) are collapsed. See ResolveOrCreateAuthorIDsByNames for the matching
// policy this mirrors.
//
// This is the admin JSON/CSV file import's genre strategy
// (docs/features/genre-status.md rule 6): slug-only matching, missing
// genres are created as pending (free-text labels need review), and a
// slug-matched genre is linked whatever its status, rejected included.
// A slug match on an alias row (a merged genre,
// docs/features/genre-moderation-queue.md rule 5) links its target instead,
// whatever the target's status. Harvest uses ResolveOrCreateHarvestGenres
// instead.
func (s *Service) ResolveOrCreateGenreIDsByNames(ctx context.Context, names []string) ([]string, error) {
	ids, _, err := s.resolveOrCreateGenresByNames(ctx, names)
	if err != nil {
		return nil, err
	}
	return ids, nil
}

// resolveOrCreateGenresByNames is ResolveOrCreateGenreIDsByNames that also
// returns the genres it created (only on the not-found branch; an existing
// or alias match is never counted), in creation order. On a failure it
// returns the genres created so far together with the error (see
// resolveOrCreateAuthorsByNames for what happens to them).
func (s *Service) resolveOrCreateGenresByNames(ctx context.Context, names []string) ([]string, []*catalogDomain.Genre, error) {
	seen := make(map[string]struct{}, len(names))
	ids := make([]string, 0, len(names))
	var createdGenres []*catalogDomain.Genre

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		slug := slugify(name)
		genre, err := s.repo.GetGenreBySlug(ctx, slug)
		if err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, createdGenres, fmt.Errorf("lookup genre %q: %w", name, err)
			}
			created, createErr := s.createGenre(ctx, GenreInput{Name: name}, genreIdentity{status: catalogDomain.GenreStatusPending})
			if createErr != nil {
				return nil, createdGenres, fmt.Errorf("create genre %q: %w", name, createErr)
			}
			genre = created
			createdGenres = append(createdGenres, created)
		} else {
			target, _, followErr := s.followGenreAlias(ctx, genre)
			if followErr != nil {
				return nil, createdGenres, followErr
			}
			genre = target
		}

		id := genre.ID.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	return ids, createdGenres, nil
}

// GenreRef is one harvested genre reference: a display name plus an
// optional bare Wikidata QID (empty when the source reported none).
type GenreRef struct {
	Name string
	QID  string
}

// Genre resolution outcomes shared by the harvest resolver and the
// Wikidata genre import (docs/features/genre-wikidata-import.md).
const (
	GenreOutcomeCreated  = "created"
	GenreOutcomeExisting = "existing"
	GenreOutcomeRejected = "rejected"
)

// Genre resolution match kinds ("" for a created genre).
const (
	GenreMatchQID  = "qid"
	GenreMatchSlug = "slug"
)

// GenreResolution is the result of resolving one name/QID with the harvest
// rules (genre-status.md rule 5).
type GenreResolution struct {
	// Genre is the matched or created genre. For a rejected outcome it is
	// the rejected (tombstone) genre.
	Genre *catalogDomain.Genre
	// Outcome is GenreOutcomeCreated, GenreOutcomeExisting or
	// GenreOutcomeRejected.
	Outcome string
	// Match is GenreMatchQID or GenreMatchSlug for an existing/rejected
	// match, "" for a created genre.
	Match string
	// QIDBackfilled reports that the QID was written to a slug-matched
	// genre that had none.
	QIDBackfilled bool
	// ViaAlias is the matched alias row (a merged genre) when the QID or
	// slug match was followed to its target; Genre is then the target.
	// Nil for a direct match.
	ViaAlias *catalogDomain.Genre
}

// ResolveOrCreateHarvestGenres resolves harvested genre refs to genre IDs
// per docs/features/genre-status.md rule 5:
//
//  1. QID present -> match by wikidata_qid. A rejected match is a
//     tombstone: the ref is skipped (no link, nothing created). A
//     pending/published match is linked (a pending genre is reused).
//  2. Otherwise / no QID match -> match by slugify(name).
//     - no QID on the genre, QID given -> QID backfilled (status
//     unchanged; a unique violation skips the backfill) and linked.
//     - rejected -> skipped (after the backfill above).
//     - a different QID -> the existing genre is linked untouched; no
//     namesake genre is created (an info line logs both QIDs).
//     - same QID or no QID given -> linked.
//  3. No match -> a new pending genre (with the QID, if any) is created
//     and linked.
//
// Alias rows (merged genres, docs/features/genre-moderation-queue.md rule
// 5): when the QID or slug match is an alias, it is followed one hop to
// its target before any status check or backfill, and the target gets the
// normal outcome (rejected target -> skipped, otherwise linked). Neither
// the alias nor the target is written (no QID backfill), and nothing is
// created. A target that is itself an alias or missing is an error.
//
// Empty names are skipped; duplicates (by resolved ID) are collapsed. When
// refs are skipped one info line lists them; the caller still creates the
// hidden book even when every genre was skipped.
func (s *Service) ResolveOrCreateHarvestGenres(ctx context.Context, refs []GenreRef) ([]string, error) {
	seen := make(map[string]struct{}, len(refs))
	ids := make([]string, 0, len(refs))
	var skipped []string

	for _, ref := range refs {
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			continue
		}
		qid := strings.TrimSpace(ref.QID)

		res, err := s.resolveHarvestGenre(ctx, name, qid)
		if err != nil {
			return nil, err
		}
		if res.Outcome == GenreOutcomeRejected {
			skipped = append(skipped, strings.TrimSpace(name+" "+qid))
			continue
		}

		id := res.Genre.ID.String()
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}

	if len(skipped) > 0 {
		s.log.Info("harvest skipped rejected (tombstoned) genres",
			zap.Strings("genres", skipped),
			zap.Bool("all_skipped", len(ids) == 0))
	}
	return ids, nil
}

// resolveHarvestGenre applies rule 5 to a single (trimmed, non-empty)
// name/QID. A rejected outcome means "skip" (tombstone).
func (s *Service) resolveHarvestGenre(ctx context.Context, name, qid string) (*GenreResolution, error) {
	if qid != "" {
		genre, err := s.repo.GetGenreByQID(ctx, qid)
		switch {
		case err == nil:
			// The alias check comes before the rejected check: an alias
			// is always rejected itself, its target decides.
			target, alias, err := s.followGenreAlias(ctx, genre)
			if err != nil {
				return nil, err
			}
			return &GenreResolution{Genre: target, Outcome: genreMatchOutcome(target), Match: GenreMatchQID, ViaAlias: alias}, nil
		case !errors.Is(err, gorm.ErrRecordNotFound):
			return nil, fmt.Errorf("lookup genre by qid %q: %w", qid, err)
		}
	}
	return s.resolveGenreBySlug(ctx, name, qid)
}

// resolveGenreBySlug is rule 5 from the slug step on (the QID, when given,
// is known not to match any genre yet).
func (s *Service) resolveGenreBySlug(ctx context.Context, name, qid string) (*GenreResolution, error) {
	genre, err := s.repo.GetGenreBySlug(ctx, slugify(name))
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("lookup genre %q: %w", name, err)
		}
		identity := genreIdentity{status: catalogDomain.GenreStatusPending}
		if qid != "" {
			q := qid
			identity.qid = &q
		}
		created, err := s.createGenre(ctx, GenreInput{Name: name}, identity)
		if err != nil {
			return nil, fmt.Errorf("create genre %q: %w", name, err)
		}
		return &GenreResolution{Genre: created, Outcome: GenreOutcomeCreated}, nil
	}

	if genre.IsAlias() {
		// Follow the alias before the backfill and the status checks; no
		// QID is written to the alias or the target.
		target, alias, err := s.followGenreAlias(ctx, genre)
		if err != nil {
			return nil, err
		}
		return &GenreResolution{Genre: target, Outcome: genreMatchOutcome(target), Match: GenreMatchSlug, ViaAlias: alias}, nil
	}

	res := &GenreResolution{Genre: genre, Outcome: GenreOutcomeExisting, Match: GenreMatchSlug}
	if qid != "" && genre.WikidataQID == nil {
		if err := s.repo.SetGenreQID(ctx, genre.ID, qid); err != nil {
			if !isDuplicateKeyError(err) {
				return nil, fmt.Errorf("backfill genre %q qid %q: %w", name, qid, err)
			}
			// Another genre took the QID concurrently: skip the backfill,
			// still link (rule 5.2).
			s.log.Info("harvest genre qid backfill skipped: qid already owned",
				zap.String("genre", genre.Slug), zap.String("qid", qid))
		} else {
			backfilled := qid
			genre.WikidataQID = &backfilled
			res.QIDBackfilled = true
		}
	}

	if genre.Status == catalogDomain.GenreStatusRejected {
		res.Outcome = GenreOutcomeRejected
		return res, nil
	}

	if qid != "" && genre.WikidataQID != nil && *genre.WikidataQID != qid {
		// Same ru label, different Wikidata item: link the existing genre
		// untouched (no namesake twin; #302 merge resolves near-duplicates).
		s.log.Info("harvest genre slug match with a different qid",
			zap.String("genre", genre.Slug),
			zap.String("genre_qid", *genre.WikidataQID),
			zap.String("ref_qid", qid))
	}
	return res, nil
}

// genreMatchOutcome is the outcome of an existing (matched) genre: a
// rejected genre is a tombstone (skip), anything else is linked.
func genreMatchOutcome(g *catalogDomain.Genre) string {
	if g.Status == catalogDomain.GenreStatusRejected {
		return GenreOutcomeRejected
	}
	return GenreOutcomeExisting
}

// followGenreAlias follows an alias row (a merged genre,
// docs/features/genre-moderation-queue.md rule 5) at most one hop. For a
// non-alias genre it returns (g, nil). For an alias it returns the target
// (loaded with GetGenreByID) and the alias itself. Merges keep alias
// chains one hop deep, so a target that is missing or itself an alias is
// an invariant breach: it is logged with both IDs and returned as an
// error.
func (s *Service) followGenreAlias(ctx context.Context, g *catalogDomain.Genre) (target, alias *catalogDomain.Genre, err error) {
	if !g.IsAlias() {
		return g, nil, nil
	}
	targetID := *g.MergedIntoID
	target, err = s.repo.GetGenreByID(ctx, targetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.log.Error("genre alias target is missing or itself an alias",
				zap.String("alias_id", g.ID.String()),
				zap.String("target_id", targetID.String()))
			return nil, nil, fmt.Errorf("genre alias %s: target %s is missing or itself an alias", g.ID, targetID)
		}
		return nil, nil, fmt.Errorf("load target %s of genre alias %s: %w", targetID, g.ID, err)
	}
	return target, g, nil
}
