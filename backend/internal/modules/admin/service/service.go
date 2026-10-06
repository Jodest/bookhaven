package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
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
	ErrInvalidSlug           = errors.New("slug is not URL safe")
	ErrDuplicateSlug         = errors.New("slug already exists")
	ErrInvalidISBN           = errors.New("isbn is invalid")
	ErrInvalidYear           = errors.New("published year is invalid")
	ErrInvalidPageCount      = errors.New("page count is invalid")
	ErrInvalidLanguage       = errors.New("language is invalid")
	ErrInvalidDate           = errors.New("date is invalid")
	ErrInvalidAuthorIDs      = errors.New("one or more author IDs are invalid")
	ErrInvalidGenreIDs       = errors.New("one or more genre IDs are invalid")
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

// DashboardCounts returns overview counts for the admin dashboard.
func (s *Service) DashboardCounts(ctx context.Context) (repository.DashboardStats, error) {
	return s.repo.DashboardCounts(ctx)
}

// ListBooks returns an admin paginated list of books including hidden ones.
func (s *Service) ListBooks(ctx context.Context, filter repository.BookFilter) ([]catalogDomain.Book, int64, error) {
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
	IsHidden      bool
	// ExternalSource/ExternalID identify the origin record for books
	// created via an automated harvest (e.g. "wikidata" + a QID). Left nil
	// by existing callers (regular admin create/update flows).
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

// CreateBook creates a new book with relationships.
func (s *Service) CreateBook(ctx context.Context, input BookInput) (*catalogDomain.Book, error) {
	if err := validateBookInput(input, input.AllowNonRussianTitle); err != nil {
		return nil, err
	}

	slug := strings.TrimSpace(input.Slug)
	if slug == "" {
		slug = slugify(input.Title)
	}
	if !isURLSafeSlug(slug) {
		return nil, ErrInvalidSlug
	}

	exists, err := s.repo.BookSlugExists(ctx, slug, nil)
	if err != nil {
		return nil, fmt.Errorf("check slug: %w", err)
	}
	if exists {
		return nil, ErrDuplicateSlug
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
		IsHidden:       input.IsHidden,
		ExternalSource: normalizeOptionalString(input.ExternalSource),
		ExternalID:     normalizeOptionalString(input.ExternalID),
	}

	if err := s.repo.CreateBook(ctx, book, authorIDs, genreIDs); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("create book: %w", err)
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
	book.IsHidden = input.IsHidden
	book.ExternalSource = normalizeOptionalString(input.ExternalSource)
	book.ExternalID = normalizeOptionalString(input.ExternalID)

	if err := s.repo.UpdateBook(ctx, book, authorIDs, genreIDs); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("update book: %w", err)
	}

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
	author.PhotoURL = normalizeOptionalURL(input.PhotoURL)
	if input.Status != nil {
		author.Status = *input.Status
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

	return author, nil
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
}

// CreateGenre creates a new genre.
func (s *Service) CreateGenre(ctx context.Context, input GenreInput) (*catalogDomain.Genre, error) {
	if err := validateGenreInput(input); err != nil {
		return nil, err
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

	genre := &catalogDomain.Genre{
		ID:   uuid.New(),
		Name: lowercaseFirstRune(strings.TrimSpace(input.Name)),
		Slug: slug,
	}

	if err := s.repo.CreateGenre(ctx, genre); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("create genre: %w", err)
	}

	return genre, nil
}

// UpdateGenre updates an existing genre.
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

	genre.Name = strings.TrimSpace(input.Name)
	genre.Slug = slug

	if err := s.repo.UpdateGenre(ctx, genre); err != nil {
		if isDuplicateKeyError(err) {
			return nil, ErrDuplicateSlug
		}
		return nil, fmt.Errorf("update genre: %w", err)
	}

	return genre, nil
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

// ListGenres returns a paginated list of genres for admin.
func (s *Service) ListGenres(ctx context.Context, search string, page, limit int) ([]catalogDomain.Genre, int64, error) {
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
	return s.repo.ListGenres(ctx, search, limit, offset)
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

var slugUnsafeChars = regexp.MustCompile(`[^a-z0-9\-]+`)
var multipleDashes = regexp.MustCompile(`-+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Basic transliteration for common Cyrillic characters.
	replacements := map[rune]string{
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo",
		'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
		'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
		'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
		'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
	}
	var b strings.Builder
	for _, r := range s {
		if repl, ok := replacements[r]; ok {
			b.WriteString(repl)
		} else if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteRune('-')
		}
	}
	s = b.String()
	s = slugUnsafeChars.ReplaceAllString(s, "-")
	s = multipleDashes.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

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
