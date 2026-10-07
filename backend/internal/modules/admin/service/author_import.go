package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"go.uber.org/zap"
)

var (
	// ErrInvalidQID: the QID is not ^Q[0-9]+$ after trim/uppercase (400).
	ErrInvalidQID = errors.New("invalid qid")
	// ErrWikidataEntityNotFound: Wikidata has no such entity (404).
	ErrWikidataEntityNotFound = errors.New("wikidata entity not found")
	// ErrNoRussianLabel: the entity has no Russian label (422, no_ru_label).
	ErrNoRussianLabel = errors.New("wikidata entity has no russian label")
	// ErrNotAPerson: the entity's P31 has no allowed person type (422,
	// not_a_person).
	ErrNotAPerson = errors.New("wikidata entity is not a person")
	// ErrAuthorImportUnavailable: no AuthorFetcher was configured (503).
	ErrAuthorImportUnavailable = errors.New("author import source is not configured")
)

// AuthorFetcher fetches a single Wikidata author entity. Satisfied by
// *bookdata.WikidataClient.
type AuthorFetcher interface {
	FetchAuthor(ctx context.Context, qid string) (*bookdata.ExternalAuthorRecord, error)
}

// importQIDPattern validates a normalized (trimmed, uppercased) QID.
var importQIDPattern = regexp.MustCompile(`^Q[0-9]+$`)

// importPersonTypes is the P31 (instance of) allowlist for a direct author
// import: human, pseudonym, collective pseudonym (keeps authors like
// «Козьма Прутков» importable).
var importPersonTypes = map[string]struct{}{
	"Q5":        {},
	"Q61002":    {},
	"Q16017119": {},
}

// AuthorImportResult is the outcome of ImportWikidataAuthor.
type AuthorImportResult struct {
	// Outcome is AuthorOutcomeCreated, AuthorOutcomeExisting or
	// AuthorOutcomeRejected.
	Outcome string
	// Match is AuthorMatchQID / AuthorMatchSlug, "" for created.
	Match         string
	QIDBackfilled bool
	NamesakeOf    *catalogDomain.Author
	Author        *catalogDomain.Author
}

// ImportWikidataAuthor imports one author by Wikidata QID, per
// docs/features/author-direct-import.md rule 2:
//
//  1. normalize the QID (trim, uppercase; ErrInvalidQID otherwise);
//  2. DB first by QID (no Wikidata call when found);
//  3. fetch the entity (ErrWikidataEntityNotFound / bookdata errors);
//  4. on a redirect, re-check the DB with the canonical QID;
//  5. name = ru label (ErrNoRussianLabel when absent or not Russian);
//  6. P31 must hold an allowed person type (ErrNotAPerson);
//  7. resolve with the harvest rules from the slug step on;
//  8. on a duplicate create, re-run from step 2 once (ErrDuplicateSlug
//     after that).
//
// It never changes an existing author's status, name or slug; the only
// possible write to an existing row is the QID backfill.
func (s *Service) ImportWikidataAuthor(ctx context.Context, qid string) (*AuthorImportResult, error) {
	qid = strings.ToUpper(strings.TrimSpace(qid))
	if !importQIDPattern.MatchString(qid) {
		return nil, ErrInvalidQID
	}

	res, err := s.resolveAuthorByQID(ctx, qid)
	if err != nil {
		return nil, err
	}
	if res != nil {
		return s.logImport(qid, res), nil
	}

	if s.authorFetcher == nil {
		return nil, ErrAuthorImportUnavailable
	}
	record, err := s.authorFetcher.FetchAuthor(ctx, qid)
	if err != nil {
		switch {
		case errors.Is(err, bookdata.ErrNotFound):
			return nil, ErrWikidataEntityNotFound
		case errors.Is(err, bookdata.ErrInvalidQuery):
			return nil, ErrInvalidQID
		}
		return nil, err
	}

	canonical := strings.ToUpper(strings.TrimSpace(record.QID))
	if !importQIDPattern.MatchString(canonical) {
		canonical = qid
	}
	if canonical != qid {
		res, err := s.resolveAuthorByQID(ctx, canonical)
		if err != nil {
			return nil, err
		}
		if res != nil {
			return s.logImport(qid, res), nil
		}
	}

	name := strings.TrimSpace(record.Name)
	if name == "" || !IsRussianText(name) {
		return nil, ErrNoRussianLabel
	}
	if !isImportablePerson(record.InstanceOf) {
		return nil, ErrNotAPerson
	}

	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			res, err := s.resolveAuthorByQID(ctx, canonical)
			if err != nil {
				return nil, err
			}
			if res != nil {
				return s.logImport(qid, res), nil
			}
		}
		res, err := s.resolveAuthorBySlug(ctx, name, canonical)
		if err == nil {
			return s.logImport(qid, res), nil
		}
		if !errors.Is(err, ErrDuplicateSlug) || attempt >= 1 {
			return nil, err
		}
	}
}

// FindAuthorsByWikidataQIDs returns the authors (any status) carrying one of
// the given Wikidata QIDs, keyed by QID, in one repository query. Used to
// mark "already in BookHaven" candidates in the harvest author search.
func (s *Service) FindAuthorsByWikidataQIDs(ctx context.Context, qids []string) (map[string]catalogDomain.Author, error) {
	out := make(map[string]catalogDomain.Author, len(qids))
	if len(qids) == 0 {
		return out, nil
	}
	authors, err := s.repo.ListAuthorsByExternalIDs(ctx, catalogDomain.AuthorExternalSourceWikidata, qids)
	if err != nil {
		return nil, fmt.Errorf("list authors by wikidata qids: %w", err)
	}
	for _, a := range authors {
		if a.ExternalID != nil {
			out[*a.ExternalID] = a
		}
	}
	return out, nil
}

func isImportablePerson(instanceOf []string) bool {
	for _, t := range instanceOf {
		if _, ok := importPersonTypes[t]; ok {
			return true
		}
	}
	return false
}

func (s *Service) logImport(qid string, res *AuthorResolution) *AuthorImportResult {
	s.log.Info("wikidata author import",
		zap.String("qid", qid),
		zap.String("outcome", res.Outcome),
		zap.String("match", res.Match),
		zap.String("author_id", res.Author.ID.String()))
	return &AuthorImportResult{
		Outcome:       res.Outcome,
		Match:         res.Match,
		QIDBackfilled: res.QIDBackfilled,
		NamesakeOf:    res.NamesakeOf,
		Author:        res.Author,
	}
}
