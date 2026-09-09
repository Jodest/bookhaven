package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/admin/repository"
	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

var (
	ErrBookNotFound      = errors.New("book not found")
	ErrAuthorNotFound    = errors.New("author not found")
	ErrGenreNotFound     = errors.New("genre not found")
	ErrReviewNotFound    = errors.New("review not found")
	ErrInvalidTitle      = errors.New("title is required")
	ErrInvalidAuthorName = errors.New("author name is required")
	ErrInvalidGenreName  = errors.New("genre name is required")
	ErrInvalidSlug       = errors.New("slug is not URL safe")
	ErrDuplicateSlug     = errors.New("slug already exists")
	ErrInvalidISBN       = errors.New("isbn is invalid")
	ErrInvalidYear       = errors.New("published year is invalid")
	ErrInvalidPageCount  = errors.New("page count is invalid")
	ErrInvalidLanguage   = errors.New("language is invalid")
	ErrInvalidDate       = errors.New("date is invalid")
	ErrInvalidAuthorIDs  = errors.New("one or more author IDs are invalid")
	ErrInvalidGenreIDs   = errors.New("one or more genre IDs are invalid")
)

// Service provides admin-only business logic.
type Service struct {
	repo repository.Repository
}

// NewService creates a new admin Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// DashboardCounts returns overview counts for the admin dashboard.
func (s *Service) DashboardCounts(ctx context.Context) (books, hiddenBooks, authors, genres, pendingReviews int64, err error) {
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
}

// CreateBook creates a new book with relationships.
func (s *Service) CreateBook(ctx context.Context, input BookInput) (*catalogDomain.Book, error) {
	if err := validateBookInput(input); err != nil {
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
		ID:            uuid.New(),
		Title:         strings.TrimSpace(input.Title),
		Slug:          slug,
		Description:   normalizeOptionalString(input.Description),
		CoverURL:      normalizeOptionalURL(input.CoverURL),
		ISBN:          isbn,
		PublishedYear: input.PublishedYear,
		PageCount:     input.PageCount,
		Language:      strings.TrimSpace(input.Language),
		IsHidden:      input.IsHidden,
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

	if err := validateBookInput(input); err != nil {
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
}

func validateBookInput(input BookInput) error {
	if strings.TrimSpace(input.Title) == "" {
		return ErrInvalidTitle
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
}

// CreateAuthor creates a new author.
func (s *Service) CreateAuthor(ctx context.Context, input AuthorInput) (*catalogDomain.Author, error) {
	if err := validateAuthorInput(input); err != nil {
		return nil, err
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
		ID:        uuid.New(),
		Name:      strings.TrimSpace(input.Name),
		Slug:      slug,
		Biography: normalizeOptionalString(input.Biography),
		PhotoURL:  normalizeOptionalURL(input.PhotoURL),
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
		Name: strings.TrimSpace(input.Name),
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

// ListAuthors returns a paginated list of authors for admin.
func (s *Service) ListAuthors(ctx context.Context, search string, page, limit int) ([]catalogDomain.Author, int64, error) {
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
	return s.repo.ListAuthors(ctx, search, limit, offset)
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

