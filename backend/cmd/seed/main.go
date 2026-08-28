package main

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
)

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Println("DATABASE_URL is required")
		os.Exit(1)
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		fmt.Printf("failed to connect to database: %v\n", err)
		os.Exit(1)
	}

	if err := seed(db); err != nil {
		fmt.Printf("failed to seed database: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("seed data applied successfully")
}

func seed(db *gorm.DB) error {
	// Create admin user
	adminID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	admin := usersDomain.User{
		ID:          adminID,
		Email:       "admin@bookhaven.local",
		Name:        ptr("Admin"),
		Role:        usersDomain.RoleAdmin,
		ProfileType: usersDomain.ProfileTypeReader,
	}
	if err := db.FirstOrCreate(&admin, usersDomain.User{Email: admin.Email}).Error; err != nil {
		return fmt.Errorf("create admin: %w", err)
	}

	// Create genres
	genres := []catalogDomain.Genre{
		{ID: uuid.New(), Name: "Классика", Slug: "classics"},
		{ID: uuid.New(), Name: "Роман", Slug: "novel"},
		{ID: uuid.New(), Name: "Фантастика", Slug: "fiction"},
		{ID: uuid.New(), Name: "Фэнтези", Slug: "fantasy"},
		{ID: uuid.New(), Name: "Детектив", Slug: "detective"},
	}
	for _, g := range genres {
		if err := db.FirstOrCreate(&g, catalogDomain.Genre{Slug: g.Slug}).Error; err != nil {
			return fmt.Errorf("create genre %s: %w", g.Slug, err)
		}
	}

	// Create authors
	tolstoy := catalogDomain.Author{
		ID:        uuid.New(),
		Name:      "Лев Толстой",
		Slug:      "lev-tolstoy",
		Biography: ptr("Русский писатель, автор романов «Война и мир», «Анна Каренина», «Воскресение»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL26783A-L.jpg"),
		BirthDate: ptr("1828-09-09"),
	}
	dostoevsky := catalogDomain.Author{
		ID:        uuid.New(),
		Name:      "Фёдор Достоевский",
		Slug:      "fyodor-dostoevsky",
		Biography: ptr("Русский писатель, автор романов «Преступление и наказание», «Идиот», «Братья Карамазовы»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL22266A-L.jpg"),
		BirthDate: ptr("1821-11-11"),
	}
	orwell := catalogDomain.Author{
		ID:        uuid.New(),
		Name:      "Джордж Оруэлл",
		Slug:      "george-orwell",
		Biography: ptr("Английский писатель и публицист, автор «1984» и «Скотный двор»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL31827A-L.jpg"),
		BirthDate: ptr("1903-06-25"),
	}
	bulgakov := catalogDomain.Author{
		ID:        uuid.New(),
		Name:      "Михаил Булгаков",
		Slug:      "mikhail-bulgakov",
		Biography: ptr("Русский писатель, автор романа «Мастер и Маргарита»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL22149A-L.jpg"),
		BirthDate: ptr("1891-05-15"),
	}

	authors := []catalogDomain.Author{tolstoy, dostoevsky, orwell, bulgakov}
	for _, a := range authors {
		if err := db.FirstOrCreate(&a, catalogDomain.Author{Slug: a.Slug}).Error; err != nil {
			return fmt.Errorf("create author %s: %w", a.Slug, err)
		}
	}

	// Create books
	books := []struct {
		book   catalogDomain.Book
		author catalogDomain.Author
		genres []string
	}{
		{
			book: catalogDomain.Book{
				ID:            uuid.New(),
				Title:         "Война и мир",
				Slug:          "war-and-peace",
				Description:   ptr("Эпический роман о России в эпоху Наполеоновских войн."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/12621906-L.jpg"),
				ISBN:          ptr("9780140447934"),
				PublishedYear: ptrInt(1869),
				PageCount:     ptrInt(1225),
				Language:      "ru",
			},
			author: tolstoy,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				ID:            uuid.New(),
				Title:         "Преступление и наказание",
				Slug:          "crime-and-punishment",
				Description:   ptr("Роман о моральных терзаниях бывшего студента Родиона Раскольникова."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264381-L.jpg"),
				ISBN:          ptr("9780140449136"),
				PublishedYear: ptrInt(1866),
				PageCount:     ptrInt(671),
				Language:      "ru",
			},
			author: dostoevsky,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				ID:            uuid.New(),
				Title:         "1984",
				Slug:          "1984",
				Description:   ptr("Антиутопический роман о тоталитарном обществе."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8579161-L.jpg"),
				ISBN:          ptr("9780451524935"),
				PublishedYear: ptrInt(1949),
				PageCount:     ptrInt(328),
				Language:      "ru",
			},
			author: orwell,
			genres: []string{"fiction", "novel"},
		},
		{
			book: catalogDomain.Book{
				ID:            uuid.New(),
				Title:         "Мастер и Маргарита",
				Slug:          "master-and-margarita",
				Description:   ptr("Роман о любви, дьяволе и Москве 1930-х годов."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8373864-L.jpg"),
				ISBN:          ptr("9780141188285"),
				PublishedYear: ptrInt(1967),
				PageCount:     ptrInt(445),
				Language:      "ru",
			},
			author: bulgakov,
			genres: []string{"classics", "fiction", "fantasy"},
		},
	}

	for _, item := range books {
		if err := db.FirstOrCreate(&item.book, catalogDomain.Book{Slug: item.book.Slug}).Error; err != nil {
			return fmt.Errorf("create book %s: %w", item.book.Slug, err)
		}

		// Link author
		if err := db.Exec(
			"INSERT INTO book_authors (book_id, author_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
			item.book.ID, item.author.ID,
		).Error; err != nil {
			return fmt.Errorf("link author to book %s: %w", item.book.Slug, err)
		}

		// Link genres
		for _, slug := range item.genres {
			var genre catalogDomain.Genre
			if err := db.First(&genre, "slug = ?", slug).Error; err != nil {
				return fmt.Errorf("find genre %s: %w", slug, err)
			}
			if err := db.Exec(
				"INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
				item.book.ID, genre.ID,
			).Error; err != nil {
				return fmt.Errorf("link genre to book %s: %w", item.book.Slug, err)
			}
		}
	}

	return nil
}

func ptr(s string) *string {
	return &s
}

func ptrInt(n int) *int {
	return &n
}
