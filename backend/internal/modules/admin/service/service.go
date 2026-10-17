package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/pkg/slug"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrBookNotFound          = errors.New("book not found")
	ErrAuthorNotFound        = errors.New("author not found")
	ErrGenreNotFound         = errors.New("genre not found")
	ErrReviewNotFound        = errors.New("review not found")
	ErrInvalidTitle          = errors.New("title is required")
	ErrTitleNotRussian       = errors.New("title must be in Russian")
	ErrDescriptionNotRussian = errors.New("description must be in Russian")
	ErrInvalidAuthorName     = errors.New("author name is required")
	ErrInvalidAuthorStatus   = errors.New("author status is invalid")
	ErrInvalidGenreName      = errors.New("genre name is required")
	ErrInvalidGenreStatus    = errors.New("genre status is invalid")
	ErrInvalidBookStatus     = errors.New("book status is invalid")
	ErrDuplicateGenreQID     = errors.New("genre wikidata qid already exists")
	ErrInvalidSlug           = errors.New("slug is not URL safe")
	ErrDuplicateSlug         = errors.New("slug already exists")
	ErrInvalidISBN           = errors.New("isbn is invalid")
	ErrInvalidYear           = errors.New("published year is invalid")
	ErrInvalidPageCount      = errors.New("page count is invalid")
	ErrInvalidLanguage       = errors.New("language is invalid")
	ErrInvalidDate           = errors.New("date is invalid")
	ErrInvalidAuthorIDs      = errors.New("one or more author IDs are invalid")
	ErrInvalidGenreIDs       = errors.New("one or more genre IDs are invalid")
	// ErrAutoHarvestRequiresQID: auto_harvest = true on an author without
	// a Wikidata QID (the scheduled harvest queries Wikidata by QID).
	ErrAutoHarvestRequiresQID = errors.New("author has no Wikidata QID")
)

// Service provides admin-only business logic.
type Service struct {
	repo repository.Repository
	// lookupClient backs LookupBook (see lookup.go). Nil unless configured
	// via WithLookupClient - LookupBook returns ErrLookupUnavailable in that
	// case, which is the correct default for every call site that has no
	// use for the lookup assistant (backend/cmd/worker, every existing
	// admin/service test's NewService(repo) call).
	lookupClient bookdata.Client
	// authorFetcher backs ImportWikidataAuthor (see author_import.go). Nil
	// unless configured via WithAuthorFetcher - the import then returns
	// ErrAuthorImportUnavailable whenever it needs Wikidata.
	authorFetcher AuthorFetcher
	// enrichmentSource backs Wikidata author enrichment (see
	// author_enrich.go). Nil unless configured via
	// WithAuthorEnrichmentSource.
	enrichmentSource AuthorEnrichmentSource
	// authorFilmSyncer syncs films about a freshly imported author (see
	// author_import.go). Nil unless configured via WithAuthorFilmSyncer.
	authorFilmSyncer AuthorFilmSyncer
	// collectionRecomputer refreshes automatic curated collections switched
	// by a genre merge (see merge_genre.go). Nil unless configured via
	// WithCollectionRecomputer.
	collectionRecomputer CollectionRecomputer
	// genreSource backs the Wikidata genre search and import (see
	// genre_import.go). Nil unless configured via WithGenreWikidataSource.
	genreSource GenreWikidataSource
	// similarInvalidator drops the cached similar-books lists after writes
	// that change book visibility or composition (see
	// similar_invalidate.go). Nil unless configured via
	// WithSimilarInvalidator.
	similarInvalidator SimilarInvalidator
	// genreClassifyTimeout bounds each genre classification call; zero
	// means defaultGenreClassifyTimeout (tests shorten it).
	genreClassifyTimeout time.Duration
	// log receives informational events (harvest tombstone skips, merge
	// QID drops). A no-op logger unless configured via WithLogger.
	log *zap.Logger
}

// Option configures an optional Service dependency via the functional-
// options pattern. This keeps NewService's required signature (repo) stable
// for existing callers instead of growing positional parameters.
type Option func(*Service)

// WithLookupClient wires the bookdata.Client used by LookupBook (e.g.
// bookdata.NewOpenLibraryClient(), as constructed in backend/cmd/api/main.go).
// Callers that don't pass this option (backend/cmd/worker/main.go, every
// existing admin/service test) get a Service whose LookupBook always
// returns ErrLookupUnavailable - they have no use for the lookup assistant.
func WithLookupClient(client bookdata.Client) Option {
	return func(s *Service) {
		s.lookupClient = client
	}
}

// WithAuthorFetcher wires the Wikidata entity fetcher used by
// ImportWikidataAuthor. backend/cmd/api passes the same
// *bookdata.WikidataClient it uses for the author search, so both share one
// rate-limit budget. Callers without it (backend/cmd/worker, tests) get
// ErrAuthorImportUnavailable from the import.
func WithAuthorFetcher(fetcher AuthorFetcher) Option {
	return func(s *Service) {
		s.authorFetcher = fetcher
	}
}

// WithGenreWikidataSource wires the Wikidata client used by
// SearchWikidataGenres and ImportWikidataGenre. backend/cmd/api passes the
// same *bookdata.WikidataClient it uses for the author search/import, so all
// admin Wikidata traffic shares one rate-limit budget. Without it the
// search and the import (unless the QID is already in BookHaven) return
// ErrGenreImportUnavailable.
func WithGenreWikidataSource(source GenreWikidataSource) Option {
	return func(s *Service) {
		s.genreSource = source
	}
}

// WithLogger wires the logger used for informational events. A nil logger
// keeps the default no-op logger.
func WithLogger(log *zap.Logger) Option {
	return func(s *Service) {
		if log != nil {
			s.log = log
		}
	}
}

// NewService creates a new admin Service. opts configures optional
// dependencies (see Option) and defaults to none, matching every existing
// call site's NewService(repo) signature.
func NewService(repo repository.Repository, opts ...Option) *Service {
	s := &Service{repo: repo, log: zap.NewNop()}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// withRepo returns a shallow copy of s that uses repo (e.g. a repository
// bound to a transaction, see repository.Repository.WithinTransaction).
// Service holds no mutable per-call state, so the copy is safe to use.
func (s *Service) withRepo(repo repository.Repository) *Service {
	c := *s
	c.repo = repo
	// The copy runs inside a transaction: invalidating the similar-books
	// cache before commit would let a concurrent request cache a list built
	// from pre-commit data under the new generation. Callers invalidate
	// after the transaction commits.
	c.similarInvalidator = nil
	return &c
}

// DashboardCounts returns overview counts for the admin dashboard.
func (s *Service) DashboardCounts(ctx context.Context) (repository.DashboardStats, error) {
	return s.repo.DashboardCounts(ctx)
}

// ListBooks returns an admin paginated list of books of every status;
// filter.Status, when non-empty, must be a valid book status.
func (s *Service) ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error) {
	if filter.Status != "" && !catalogDomain.IsValidBookStatus(filter.Status) {
		return nil, 0, ErrInvalidBookStatus
	}
	return s.repo.ListBooks(ctx, filter)
}

// GetBook returns a book by ID including hidden records.
func (s *Service) GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	book, err := s.repo.GetBookByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, fmt.Errorf("get book: %w", err)
	}
	return book, nil
}

// FindBookByExternalID returns a book by its (external_source, external_id)
// idempotency key, including hidden records. Returns ErrBookNotFound when no
// book has been harvested from that source/id yet.
func (s *Service) FindBookByExternalID(ctx context.Context, source, id string) (*catalogDomain.Book, error) {
	book, err := s.repo.GetBookByExternalID(ctx, source, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, fmt.Errorf("get book by external id: %w", err)
	}
	return book, nil
}

// BookInput contains fields for creating or updating a book.
type BookInput struct {
	Title         string
	Slug          string
	Description   *string
	CoverURL      *string
	ISBN          *string
	PublishedYear *int
	PageCount     *int
	Language      string
	AuthorIDs     []string
	GenreIDs      []string
	// Status is a book status (catalogDomain.BookStatus*). On create, nil
	// means published (manual creation is vetted); on update, nil means
	// unchanged. Any non-nil invalid value is ErrInvalidBookStatus.
	Status *string
	// ExternalSource/ExternalID identify the origin record for books
	// created via an automated harvest (e.g. "wikidata" + a QID). Applied
	// on create only; UpdateBook ignores them and keeps the stored values.
	// Left nil by the regular admin create flow.
	ExternalSource *string
	ExternalID     *string
	// AllowNonRussianTitle lifts only the ErrTitleNotRussian check (for
	// official Latin-script titles such as «Empire V»). Every other
	// validation, including the description language check, still applies.
	// Set by the Wikidata harvest pipeline (always true) and by the admin
	// HTTP create/update handlers (from the request body). Never set by
	// bulk CSV/JSON import. Not persisted.
	// See docs/features/latin-book-titles.md.
	AllowNonRussianTitle bool
}

// preparedBook holds the values prepareNewBook derives from a BookInput.
type preparedBook struct {
	status string
	slug   string
}

// prepareNewBook runs, in order, the checks CreateBook performs before
// touching author/genre IDs: the status, validateBookInput, the slug
// derivation and URL-safety check, and the slug-existence check
// (ErrDuplicateSlug). It writes nothing. It is the single source of truth
// for those rules, shared by CreateBook and the bulk import's pre-creation
// check (import.go), so both always agree on a row's outcome.
func (s *Service) prepareNewBook(ctx context.Context, input BookInput) (preparedBook, error) {
	status := catalogDomain.BookStatusPublished
	if input.Status != nil {
		if !catalogDomain.IsValidBookStatus(*input.Status) {
			return preparedBook{}, ErrInvalidBookStatus
		}
		status = *input.Status
	}
	if err := validateBookInput(input, input.AllowNonRussianTitle); err != nil {
		return preparedBook{}, err
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(input.Title)
	}
	if !isURLSafeSlug(slug) {
		return preparedBook{}, ErrInvalidSlug
	}

	exists, err := s.repo.BookSlugExists(ctx, slug, nil)
	if err != nil {
		return preparedBook{}, fmt.Errorf("check slug: %w", err)
	}
	if exists {
		return preparedBook{}, ErrDuplicateSlug
	}
	return preparedBook{status: status, slug: slug}, nil
}

// normalizeNewBookISBN trims the optional ISBN, maps blank to nil and
// rejects a non-blank value with an invalid checksum (ErrInvalidISBN). It
// is CreateBook's last check (after the author/genre ID checks) and is also
// run by the bulk import's pre-creation check.
func normalizeNewBookISBN(raw *string) (*string, error) {
	isbn := normalizeOptionalString(raw)
	if isbn != nil && *isbn != "" {
		if !isValidISBN(*isbn) {
			return nil, ErrInvalidISBN
		}
		return isbn, nil
	}
	return nil, nil
}

// CreateBook creates a new book with relationships.
func (s *Service) CreateBook(ctx context.Context, input BookInput) (*catalogDomain.Book, error) {
	prepared, err := s.prepareNewBook(ctx, input)
	if err != nil {
		return nil, err
	}
	status, slug := prepared.status, prepared.slug

	authorIDs, err := parseUUIDsUnique(input.AuthorIDs)
	if err != nil {
		return nil, ErrInvalidAuthorIDs
	}
	genreIDs, err := parseUUIDsUnique(input.GenreIDs)
	if err != nil {
		return nil, ErrInvalidGenreIDs
	}

	if ok, err := s.repo.AuthorsExist(ctx, authorIDs); err != nil {
		return nil, fmt.Errorf("check authors: %w", err)
	} else if !ok {
		return nil, ErrInvalidAuthorIDs
	}

	if ok, err := s.repo.GenresExist(ctx, genreIDs); err != nil {
		return nil, fmt.Errorf("check genres: %w", err)
	} else if !ok {
		return nil, ErrInvalidGenreIDs
	}

	isbn, err := normalizeNewBookISBN(input.ISBN)
	if err != nil {
		return nil, err
	}

	book := &catalogDomain.Book{
		ID:             uuid.New(),
		Title:          strings.TrimSpace(input.Title),
		Slug:           slug,
		Description:    normalizeOptionalString(input.Description),
		CoverURL:       normalizeOptionalURL(input.CoverURL),
		ISBN:           isbn,
		PublishedYear:  input.PublishedYear,
		PageCount:      input.PageCount,
		Language:       strings.TrimSpace(input.Language),
		Status:         status,
		ExternalSource: normalizeOptionalString(input.ExternalSource),
		ExternalID:     normalizeOptionalString(input.ExternalID),
	}
	// A book created published (manual default, published import batch)
	// gets its first-publication time now; pending books stay nil (#245).
	book.MarkPublished(time.Now())

	if err := s.repo.CreateBook(ctx, book, authorIDs, genreIDs); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("create book: %w", err)
	}
	if status == catalogDomain.BookStatusPublished {
		s.invalidateSimilar(ctx)
	}

	return s.repo.GetBookByID(ctx, book.ID)
}

// UpdateBook updates an existing book and replaces its relationships.
func (s *Service) UpdateBook(ctx context.Context, id uuid.UUID, input BookInput) (*catalogDomain.Book, error) {
	book, err := s.repo.GetBookByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, fmt.Errorf("get book: %w", err)
	}
	if input.Status != nil && !catalogDomain.IsValidBookStatus(*input.Status) {
		return nil, ErrInvalidBookStatus
	}

	// Grandfathering: the title-language check guards against *new*
	// non-Russian titles entering the catalog. A title equal to the one
	// already stored is not new text (it got in via the explicit bypass or
	// predates the russian-only rule), so editing other fields or
	// publishing (single-row or bulk) does not require the opt-in flag.
	// See docs/features/latin-book-titles.md, Design decision 2.
	skipTitleLangCheck := input.AllowNonRussianTitle || strings.TrimSpace(input.Title) == book.Title
	if err := validateBookInput(input, skipTitleLangCheck); err != nil {
		return nil, err
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(input.Title)
	}
	if !isURLSafeSlug(slug) {
		return nil, ErrInvalidSlug
	}

	if slug != book.Slug {
		exists, err := s.repo.BookSlugExists(ctx, slug, &id)
		if err != nil {
			return nil, fmt.Errorf("check slug: %w", err)
		}
		if exists {
			return nil, ErrDuplicateSlug
		}
	}

	authorIDs, err := parseUUIDsUnique(input.AuthorIDs)
	if err != nil {
		return nil, ErrInvalidAuthorIDs
	}
	genreIDs, err := parseUUIDsUnique(input.GenreIDs)
	if err != nil {
		return nil, ErrInvalidGenreIDs
	}

	if ok, err := s.repo.AuthorsExist(ctx, authorIDs); err != nil {
		return nil, fmt.Errorf("check authors: %w", err)
	} else if !ok {
		return nil, ErrInvalidAuthorIDs
	}

	if ok, err := s.repo.GenresExist(ctx, genreIDs); err != nil {
		return nil, fmt.Errorf("check genres: %w", err)
	} else if !ok {
		return nil, ErrInvalidGenreIDs
	}

	isbn := normalizeOptionalString(input.ISBN)
	if isbn != nil && *isbn != "" {
		if !isValidISBN(*isbn) {
			return nil, ErrInvalidISBN
		}
	} else {
		isbn = nil
	}

	book.Title = strings.TrimSpace(input.Title)
	book.Slug = slug
	book.Description = normalizeOptionalString(input.Description)
	book.CoverURL = normalizeOptionalURL(input.CoverURL)
	book.ISBN = isbn
	book.PublishedYear = input.PublishedYear
	book.PageCount = input.PageCount
	book.Language = strings.TrimSpace(input.Language)
	if input.Status != nil {
		book.Status = *input.Status
	}
	// ExternalSource/ExternalID are deliberately not touched: they are set
	// once at creation by the harvest and must survive admin edits.

	if err := s.repo.UpdateBook(ctx, book, authorIDs, genreIDs); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("update book: %w", err)
	}
	s.invalidateSimilar(ctx)

	return s.repo.GetBookByID(ctx, id)
}

var supportedLanguageCodes = map[string]struct{}{
	"ru": {},
	"en": {},
	"uk": {},
	"be": {},
	"kk": {},
	"uz": {},
	"hy": {},
	"ka": {},
	"az": {},
	"lt": {},
	"lv": {},
	"et": {},
	"mo": {},
	"tg": {},
	"tk": {},
	"ky": {},
	// The following match every code bookdata.wikidataLanguageCodes can
	// produce from Wikidata's P407 (language of work) property, beyond the
	// ones already listed above (ru/en/uk). Without these, a harvested book
	// whose original-language lookup resolves to one of these six
	// languages fails validation with ErrInvalidLanguage even though the
	// harvester correctly identified a real, known language - see
	// backend/internal/pkg/bookdata/wikidata.go's wikidataLanguageCodes map.
	"fr": {}, // French
	"de": {}, // German
	"es": {}, // Spanish
	"pl": {}, // Polish
	"it": {}, // Italian
	"pt": {}, // Portuguese
}

// validateBookInput validates book fields. skipTitleLangCheck (decided by
// the caller) lifts only the ErrTitleNotRussian check; the empty-title and
// description-language checks always apply.
func validateBookInput(input BookInput, skipTitleLangCheck bool) error {
	if strings.TrimSpace(input.Title) == "" {
		return ErrInvalidTitle
	}
	if !skipTitleLangCheck && !IsRussianText(strings.TrimSpace(input.Title)) {
		return ErrTitleNotRussian
	}
	if desc := normalizeOptionalString(input.Description); desc != nil && !IsRussianText(*desc) {
		return ErrDescriptionNotRussian
	}
	lang := strings.TrimSpace(input.Language)
	if lang == "" {
		return ErrInvalidLanguage
	}
	if _, ok := supportedLanguageCodes[lang]; !ok {
		return ErrInvalidLanguage
	}
	if input.PublishedYear != nil && (*input.PublishedYear < 1000 || *input.PublishedYear > 2100) {
		return ErrInvalidYear
	}
	if input.PageCount != nil && *input.PageCount <= 0 {
		return ErrInvalidPageCount
	}
	return nil
}

// AuthorInput contains fields for creating or updating an author.
type AuthorInput struct {
	Name      string
	Slug      string
	Biography *string
	PhotoURL  *string
	BirthDate *string // YYYY-MM-DD
	DeathDate *string // YYYY-MM-DD
	// Status is applied by UpdateAuthor only (nil = unchanged); CreateAuthor
	// ignores it and always creates a published author.
	Status *string
	// AutoHarvest is applied by UpdateAuthor only (nil = unchanged);
	// CreateAuthor ignores it (new authors are never flagged). Switching it
	// on (true for an unflagged author) needs a Wikidata QID
	// (ErrAutoHarvestRequiresQID).
	AutoHarvest *bool
}

// authorIdentity is the creation-time status and optional external
// identity used by the shared createAuthor helper.
type authorIdentity struct {
	status         string
	externalSource *string
	externalID     *string
}

// CreateAuthor creates a new published author (admin-curated input). Any
// input.Status is ignored.
func (s *Service) CreateAuthor(ctx context.Context, input AuthorInput) (*catalogDomain.Author, error) {
	return s.createAuthor(ctx, input, authorIdentity{status: catalogDomain.AuthorStatusPublished})
}

// createAuthor is the shared create path (validation, slug derivation and
// uniqueness) for every author creation: admin manual/import (published)
// and harvest (pending, with a Wikidata QID).
func (s *Service) createAuthor(ctx context.Context, input AuthorInput, identity authorIdentity) (*catalogDomain.Author, error) {
	if err := validateAuthorInput(input); err != nil {
		return nil, err
	}
	if !catalogDomain.IsValidAuthorStatus(identity.status) {
		return nil, ErrInvalidAuthorStatus
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(input.Name)
	}
	if !isURLSafeSlug(slug) {
		return nil, ErrInvalidSlug
	}

	exists, err := s.repo.AuthorSlugExists(ctx, slug, nil)
	if err != nil {
		return nil, fmt.Errorf("check slug: %w", err)
	}
	if exists {
		return nil, ErrDuplicateSlug
	}

	author := &catalogDomain.Author{
		ID:             uuid.New(),
		Name:           strings.TrimSpace(input.Name),
		Slug:           slug,
		Biography:      normalizeOptionalString(input.Biography),
		PhotoURL:       normalizeOptionalURL(input.PhotoURL),
		Status:         identity.status,
		ExternalSource: identity.externalSource,
		ExternalID:     identity.externalID,
	}
	if input.BirthDate != nil {
		t, err := parseDate(*input.BirthDate)
		if err != nil {
			return nil, fmt.Errorf("birth_date: %w", err)
		}
		author.BirthDate = t
	} else {
		author.BirthDate = nil
	}
	if input.DeathDate != nil {
		t, err := parseDate(*input.DeathDate)
		if err != nil {
			return nil, fmt.Errorf("death_date: %w", err)
		}
		author.DeathDate = t
	} else {
		author.DeathDate = nil
	}

	if err := s.repo.CreateAuthor(ctx, author); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("create author: %w", err)
	}

	return author, nil
}

// UpdateAuthor updates an existing author.
func (s *Service) UpdateAuthor(ctx context.Context, id uuid.UUID, input AuthorInput) (*catalogDomain.Author, error) {
	author, err := s.repo.GetAuthorByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuthorNotFound
		}
		return nil, fmt.Errorf("get author: %w", err)
	}

	if err := validateAuthorInput(input); err != nil {
		return nil, err
	}
	if input.Status != nil && !catalogDomain.IsValidAuthorStatus(*input.Status) {
		return nil, ErrInvalidAuthorStatus
	}
	// Only switching the flag on needs a QID: re-sending true for an already
	// flagged author (e.g. one whose QID was removed by hand) must not fail.
	if input.AutoHarvest != nil && *input.AutoHarvest && !author.AutoHarvest && !hasWikidataQID(*author) {
		return nil, ErrAutoHarvestRequiresQID
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(input.Name)
	}
	if !isURLSafeSlug(slug) {
		return nil, ErrInvalidSlug
	}

	if slug != author.Slug {
		exists, err := s.repo.AuthorSlugExists(ctx, slug, &id)
		if err != nil {
			return nil, fmt.Errorf("check slug: %w", err)
		}
		if exists {
			return nil, ErrDuplicateSlug
		}
	}

	author.Name = strings.TrimSpace(input.Name)
	author.Slug = slug
	author.Biography = normalizeOptionalString(input.Biography)
	newPhoto := normalizeOptionalURL(input.PhotoURL)
	if !sameOptionalString(author.PhotoURL, newPhoto) {
		// A manual photo change invalidates the Commons attribution
		// (docs/features/author-wikidata-enrichment.md rule 7).
		author.PhotoAuthor = nil
		author.PhotoLicense = nil
		author.PhotoLicenseURL = nil
		author.PhotoSourceURL = nil
	}
	author.PhotoURL = newPhoto
	if input.Status != nil {
		author.Status = *input.Status
	}
	if input.AutoHarvest != nil {
		author.AutoHarvest = *input.AutoHarvest
	}
	if input.BirthDate != nil {
		t, err := parseDate(*input.BirthDate)
		if err != nil {
			return nil, fmt.Errorf("birth_date: %w", err)
		}
		author.BirthDate = t
	} else {
		author.BirthDate = nil
	}
	if input.DeathDate != nil {
		t, err := parseDate(*input.DeathDate)
		if err != nil {
			return nil, fmt.Errorf("death_date: %w", err)
		}
		author.DeathDate = t
	} else {
		author.DeathDate = nil
	}

	if err := s.repo.UpdateAuthor(ctx, author); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("update author: %w", err)
	}
	s.invalidateSimilar(ctx)

	return author, nil
}

// sameOptionalString compares two optional strings (nil and "" differ
// only in representation: both mean "no value").
func sameOptionalString(a, b *string) bool {
	av, bv := "", ""
	if a != nil {
		av = strings.TrimSpace(*a)
	}
	if b != nil {
		bv = strings.TrimSpace(*b)
	}
	return av == bv
}

// hasWikidataQID reports whether the author carries a Wikidata QID (the
// precondition for auto_harvest).
func hasWikidataQID(a catalogDomain.Author) bool {
	return a.ExternalSource != nil && *a.ExternalSource == catalogDomain.AuthorExternalSourceWikidata &&
		a.ExternalID != nil && strings.TrimSpace(*a.ExternalID) != ""
}

func validateAuthorInput(input AuthorInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return ErrInvalidAuthorName
	}
	return nil
}

// GenreInput contains fields for creating or updating a genre.
type GenreInput struct {
	Name string
	Slug string
	// Status is applied by UpdateGenre only (nil = unchanged); CreateGenre
	// ignores it and always creates a published genre.
	Status *string
	// WikidataQID: nil = unchanged (on create: none), "" = clear, otherwise
	// a QID normalized by trim+uppercase that must match ^Q[0-9]+$
	// (ErrInvalidQID) and be unused by another genre (ErrDuplicateGenreQID).
	WikidataQID *string
}

// genreIdentity is the creation-time status and optional Wikidata QID used
// by the shared createGenre helper.
type genreIdentity struct {
	status string
	qid    *string
}

// CreateGenre creates a new published genre (admin manual input). Any
// input.Status is ignored; input.WikidataQID is validated and checked for
// uniqueness.
func (s *Service) CreateGenre(ctx context.Context, input GenreInput) (*catalogDomain.Genre, error) {
	qid, _, err := normalizeGenreQIDInput(input.WikidataQID)
	if err != nil {
		return nil, err
	}
	return s.createGenre(ctx, input, genreIdentity{status: catalogDomain.GenreStatusPublished, qid: qid})
}

// createGenre is the shared create path (validation, slug derivation,
// name casing and uniqueness) for every genre creation: admin manual
// (published) and file import / harvest (pending, harvest with a QID).
// identity.qid must already be normalized.
func (s *Service) createGenre(ctx context.Context, input GenreInput, identity genreIdentity) (*catalogDomain.Genre, error) {
	if err := validateGenreInput(input); err != nil {
		return nil, err
	}
	if !catalogDomain.IsValidGenreStatus(identity.status) {
		return nil, ErrInvalidGenreStatus
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(input.Name)
	}
	if !isURLSafeSlug(slug) {
		return nil, ErrInvalidSlug
	}

	exists, err := s.repo.GenreSlugExists(ctx, slug, nil)
	if err != nil {
		return nil, fmt.Errorf("check slug: %w", err)
	}
	if exists {
		return nil, ErrDuplicateSlug
	}
	if identity.qid != nil {
		if err := s.ensureGenreQIDFree(ctx, *identity.qid, nil); err != nil {
			return nil, err
		}
	}

	genre := &catalogDomain.Genre{
		ID:          uuid.New(),
		Name:        lowercaseFirstRune(strings.TrimSpace(input.Name)),
		Slug:        slug,
		Status:      identity.status,
		WikidataQID: identity.qid,
	}

	if err := s.repo.CreateGenre(ctx, genre); err != nil {
		if isDuplicateKeyError(err) {
			return nil, genreDuplicateKeyError(err)
		}
		return nil, fmt.Errorf("create genre: %w", err)
	}

	return genre, nil
}

// UpdateGenre updates an existing genre. input.Status (nil = unchanged)
// may move the genre to any status; input.WikidataQID follows GenreInput.
func (s *Service) UpdateGenre(ctx context.Context, id uuid.UUID, input GenreInput) (*catalogDomain.Genre, error) {
	genre, err := s.repo.GetGenreByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGenreNotFound
		}
		return nil, fmt.Errorf("get genre: %w", err)
	}

	if err := validateGenreInput(input); err != nil {
		return nil, err
	}
	if input.Status != nil && !catalogDomain.IsValidGenreStatus(*input.Status) {
		return nil, ErrInvalidGenreStatus
	}
	qid, qidSet, err := normalizeGenreQIDInput(input.WikidataQID)
	if err != nil {
		return nil, err
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(input.Name)
	}
	if !isURLSafeSlug(slug) {
		return nil, ErrInvalidSlug
	}

	if slug != genre.Slug {
		exists, err := s.repo.GenreSlugExists(ctx, slug, &id)
		if err != nil {
			return nil, fmt.Errorf("check slug: %w", err)
		}
		if exists {
			return nil, ErrDuplicateSlug
		}
	}
	if qidSet && qid != nil {
		if err := s.ensureGenreQIDFree(ctx, *qid, &id); err != nil {
			return nil, err
		}
	}

	genre.Name = strings.TrimSpace(input.Name)
	genre.Slug = slug
	if input.Status != nil {
		genre.Status = *input.Status
	}
	if qidSet {
		genre.WikidataQID = qid
	}

	if err := s.repo.UpdateGenre(ctx, genre); err != nil {
		// The row became an alias (or vanished) after the read above.
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGenreNotFound
		}
		if isDuplicateKeyError(err) {
			return nil, genreDuplicateKeyError(err)
		}
		return nil, fmt.Errorf("update genre: %w", err)
	}
	s.invalidateSimilar(ctx)

	return genre, nil
}

// normalizeGenreQIDInput interprets an optional QID input: nil = not set
// (set=false); "" (after trim) = clear (set=true, qid=nil); otherwise the
// trimmed, uppercased QID, which must match ^Q[0-9]+$ (ErrInvalidQID).
func normalizeGenreQIDInput(in *string) (qid *string, set bool, err error) {
	if in == nil {
		return nil, false, nil
	}
	v := strings.ToUpper(strings.TrimSpace(*in))
	if v == "" {
		return nil, true, nil
	}
	if !importQIDPattern.MatchString(v) {
		return nil, true, ErrInvalidQID
	}
	return &v, true, nil
}

// ensureGenreQIDFree returns ErrDuplicateGenreQID when a genre other than
// excludeID already owns qid.
func (s *Service) ensureGenreQIDFree(ctx context.Context, qid string, excludeID *uuid.UUID) error {
	owner, err := s.repo.GetGenreByQID(ctx, qid)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		return fmt.Errorf("check genre qid: %w", err)
	}
	if excludeID != nil && owner.ID == *excludeID {
		return nil
	}
	return ErrDuplicateGenreQID
}

// genreDuplicateKeyError maps a unique violation on genres to the right
// sentinel: the wikidata_qid partial index means ErrDuplicateGenreQID,
// anything else (the slug index) ErrDuplicateSlug.
func genreDuplicateKeyError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && strings.Contains(pgErr.ConstraintName, "wikidata_qid") {
		return ErrDuplicateGenreQID
	}
	return ErrDuplicateSlug
}

func validateGenreInput(input GenreInput) error {
	if strings.TrimSpace(input.Name) == "" {
		return ErrInvalidGenreName
	}
	return nil
}

// ListAuthors returns a paginated list of authors (every status) for
// admin. status, when non-empty, must be a valid author status
// (ErrInvalidAuthorStatus otherwise) and filters the list to it.
func (s *Service) ListAuthors(ctx context.Context, search, status string, page, limit int) ([]catalogDomain.Author, int64, error) {
	if status != "" && !catalogDomain.IsValidAuthorStatus(status) {
		return nil, 0, ErrInvalidAuthorStatus
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit
	return s.repo.ListAuthors(ctx, search, status, limit, offset)
}

// ListGenres returns a paginated list of genres (every status) for admin.
// status, when non-empty, must be a valid genre status
// (ErrInvalidGenreStatus otherwise) and filters the list to it.
func (s *Service) ListGenres(ctx context.Context, search, status string, page, limit int) ([]catalogDomain.Genre, int64, error) {
	if status != "" && !catalogDomain.IsValidGenreStatus(status) {
		return nil, 0, ErrInvalidGenreStatus
	}
	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit
	return s.repo.ListGenres(ctx, search, status, limit, offset)
}

// GetAuthor returns an author by ID in any status (admin view). Returns
// ErrAuthorNotFound when no such author exists (merged duplicates are
// deleted, so their IDs are not found either).
func (s *Service) GetAuthor(ctx context.Context, id uuid.UUID) (*catalogDomain.Author, error) {
	author, err := s.repo.GetAuthorByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuthorNotFound
		}
		return nil, fmt.Errorf("get author: %w", err)
	}
	return author, nil
}

// GetGenre returns a genre by ID in any status (admin view). Returns
// ErrGenreNotFound when no such genre exists or the ID is a merge alias
// (the repository hides alias rows).
func (s *Service) GetGenre(ctx context.Context, id uuid.UUID) (*catalogDomain.Genre, error) {
	genre, err := s.repo.GetGenreByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrGenreNotFound
		}
		return nil, fmt.Errorf("get genre: %w", err)
	}
	return genre, nil
}

// ListReviews returns paginated reviews for moderation.
func (s *Service) ListReviews(ctx context.Context, filter repository.ReviewFilter) ([]reviewsDomain.Review, int64, error) {
	return s.repo.ListReviews(ctx, filter)
}

// ModerateReview hides or restores a review and keeps books.reviews_count consistent.
func (s *Service) ModerateReview(ctx context.Context, id uuid.UUID, isHidden bool) (*reviewsDomain.Review, error) {
	review, err := s.repo.ModerateReview(ctx, id, isHidden)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrReviewNotFound
		}
		return nil, fmt.Errorf("moderate review: %w", err)
	}
	return review, nil
}

// Helpers.

func slugify(s string) string { return slug.Make(s) }

// lowercaseFirstRune lowercases only the first rune of s, leaving the rest of
// the string untouched. Used to normalize genre name casing on creation
// (e.g. "Роман" -> "роман") without affecting any internal capitalization.
func lowercaseFirstRune(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size <= 1 {
		return s
	}
	return string(unicode.ToLower(r)) + s[size:]
}

func isURLSafeSlug(slug string) bool {
	if slug == "" {
		return false
	}
	for _, r := range slug {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-') {
			return false
		}
	}
	return true
}

func normalizeOptionalString(s *string) *string {
	if s == nil {
		return nil
	}
	v := strings.TrimSpace(*s)
	if v == "" {
		return nil
	}
	return &v
}

func normalizeOptionalURL(s *string) *string {
	v := normalizeOptionalString(s)
	if v == nil {
		return nil
	}
	if !strings.HasPrefix(*v, "http://") && !strings.HasPrefix(*v, "https://") {
		return nil
	}
	return v
}

func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func parseUUIDsUnique(ids []string) ([]uuid.UUID, error) {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	result := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		uid, err := uuid.Parse(id)
		if err != nil {
			return nil, err
		}
		if _, ok := seen[uid]; ok {
			continue
		}
		seen[uid] = struct{}{}
		result = append(result, uid)
	}
	return result, nil
}

func isValidISBN(isbn string) bool {
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	if len(isbn) == 10 {
		return isValidISBN10(isbn)
	}
	if len(isbn) == 13 {
		return isValidISBN13(isbn)
	}
	return false
}

func isValidISBN10(isbn string) bool {
	if len(isbn) != 10 {
		return false
	}
	sum := 0
	for i := 0; i < 9; i++ {
		c := isbn[i]
		if c < '0' || c > '9' {
			return false
		}
		sum += int(c-'0') * (10 - i)
	}
	last := isbn[9]
	if last == 'X' || last == 'x' {
		sum += 10
	} else if last >= '0' && last <= '9' {
		sum += int(last - '0')
	} else {
		return false
	}
	return sum%11 == 0
}

func isValidISBN13(isbn string) bool {
	if len(isbn) != 13 {
		return false
	}
	sum := 0
	for i := 0; i < 13; i++ {
		c := isbn[i]
		if c < '0' || c > '9' {
			return false
		}
		digit := int(c - '0')
		if i%2 == 0 {
			sum += digit
		} else {
			sum += digit * 3
		}
	}
	return sum%10 == 0
}

func parseDate(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil, ErrInvalidDate
	}
	return &t, nil
}
