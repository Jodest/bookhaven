// Command seed populates the database with a fixed set of dev/demo data
// (a demo admin/test user plus ~19 hardcoded classic-literature books). It
// is dev/demo-only tooling and must NOT be used to populate a real catalog:
// its output is a small, hand-picked fixture set, not real production
// content, and it carries none of the provenance/review metadata
// (external_source/external_id, pending-review flow) that harvested books
// get. See docs/features/harvest-review-queue.md for the production
// guardrail this file enforces below.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"

	"github.com/bookhaven/backend/internal/config"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
)

func main() {
	force := flag.Bool("force", false, "allow running against APP_ENV=production (dangerous, dev/demo data only)")
	flag.Parse()

	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Printf("failed to load config: %v\n", err)
		os.Exit(1)
	}

	if shouldRefuseToRun(cfg.Env, *force) {
		fmt.Println("refusing to run cmd/seed against APP_ENV=production without -force; this seeds fixed dev/demo data, not real catalog content")
		os.Exit(1)
	}

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
	fmt.Println("test account: test@bookhaven.local / test1234")
}

// shouldRefuseToRun reports whether cmd/seed must abort given the loaded
// environment name and whether -force was passed. Mirrors the production
// guardrail precedent already used for JWT_SECRET in config.Load().
func shouldRefuseToRun(env string, force bool) bool {
	return env == "production" && !force
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

	// Create test user
	testPassword := "test1234"
	testHash, err := bcrypt.GenerateFromPassword([]byte(testPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash test password: %w", err)
	}
	testHashStr := string(testHash)
	testUser := usersDomain.User{
		ID:           uuid.MustParse("22222222-2222-2222-2222-222222222222"),
		Email:        "test@bookhaven.local",
		PasswordHash: &testHashStr,
		Name:         ptr("Тестовый пользователь"),
		Role:         usersDomain.RoleUser,
		ProfileType:  usersDomain.ProfileTypeReader,
	}
	if err := db.FirstOrCreate(&testUser, usersDomain.User{Email: testUser.Email}).Error; err != nil {
		return fmt.Errorf("create test user: %w", err)
	}

	// Create genres
	genres := []catalogDomain.Genre{
		{Name: "Классика", Slug: "classics"},
		{Name: "Роман", Slug: "novel"},
		{Name: "Фантастика", Slug: "fiction"},
		{Name: "Фэнтези", Slug: "fantasy"},
		{Name: "Детектив", Slug: "detective"},
	}
	for _, g := range genres {
		if err := db.FirstOrCreate(&g, catalogDomain.Genre{Slug: g.Slug}).Error; err != nil {
			return fmt.Errorf("create genre %s: %w", g.Slug, err)
		}
	}

	// Create authors
	tolstoy := catalogDomain.Author{
		Name:      "Лев Толстой",
		Slug:      "lev-tolstoy",
		Biography: ptr("Русский писатель, автор романов «Война и мир», «Анна Каренина», «Воскресение»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL26783A-L.jpg"),
		BirthDate: ptrTime("1828-09-09"),
	}
	dostoevsky := catalogDomain.Author{
		Name:      "Фёдор Достоевский",
		Slug:      "fyodor-dostoevsky",
		Biography: ptr("Русский писатель, автор романов «Преступление и наказание», «Идиот», «Братья Карамазовы»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL22266A-L.jpg"),
		BirthDate: ptrTime("1821-11-11"),
	}
	orwell := catalogDomain.Author{
		Name:      "Джордж Оруэлл",
		Slug:      "george-orwell",
		Biography: ptr("Английский писатель и публицист, автор «1984» и «Скотный двор»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL31827A-L.jpg"),
		BirthDate: ptrTime("1903-06-25"),
	}
	bulgakov := catalogDomain.Author{
		Name:      "Михаил Булгаков",
		Slug:      "mikhail-bulgakov",
		Biography: ptr("Русский писатель, автор романа «Мастер и Маргарита»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL22149A-L.jpg"),
		BirthDate: ptrTime("1891-05-15"),
	}
	gogol := catalogDomain.Author{
		Name:      "Николай Гоголь",
		Slug:      "nikolai-gogol",
		Biography: ptr("Русский писатель, автор «Мёртвые души», «Ревизор»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL25067A-L.jpg"),
		BirthDate: ptrTime("1809-04-01"),
	}
	turgenev := catalogDomain.Author{
		Name:      "Иван Тургенев",
		Slug:      "ivan-turgenev",
		Biography: ptr("Русский писатель, автор «Отцы и дети»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL27049A-L.jpg"),
		BirthDate: ptrTime("1818-11-09"),
	}
	chekhov := catalogDomain.Author{
		Name:      "Антон Чехов",
		Slug:      "anton-chekhov",
		Biography: ptr("Русский писатель и драматург, автор «Вишнёвый сад», «Чайка»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL22753A-L.jpg"),
		BirthDate: ptrTime("1860-01-29"),
	}
	lermontov := catalogDomain.Author{
		Name:      "Михаил Лермонтов",
		Slug:      "mikhail-lermontov",
		Biography: ptr("Русский поэт и писатель, автор «Герой нашего времени»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL21554A-L.jpg"),
		BirthDate: ptrTime("1814-10-15"),
	}
	pushkin := catalogDomain.Author{
		Name:      "Александр Пушкин",
		Slug:      "aleksandr-pushkin",
		Biography: ptr("Русский поэт, автор «Евгений Онегин», «Капитанская дочка»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL23919A-L.jpg"),
		BirthDate: ptrTime("1799-06-06"),
	}
	huxley := catalogDomain.Author{
		Name:      "Олдос Хаксли",
		Slug:      "aldous-huxley",
		Biography: ptr("Английский писатель, автор «Дивный новый мир»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL18053A-L.jpg"),
		BirthDate: ptrTime("1894-07-26"),
	}
	remarque := catalogDomain.Author{
		Name:      "Эрих Мария Ремарк",
		Slug:      "erich-maria-remarque",
		Biography: ptr("Немецкий писатель, автор «На Западе без перемен»."),
		PhotoURL:  ptr("https://covers.openlibrary.org/a/olid/OL27074A-L.jpg"),
		BirthDate: ptrTime("1898-06-22"),
	}

	// Authors are created via pointers so that FirstOrCreate fills in the
	// actual DB IDs, which are used below to link books to authors.
	authors := []*catalogDomain.Author{&tolstoy, &dostoevsky, &orwell, &bulgakov, &gogol, &turgenev, &chekhov, &lermontov, &pushkin, &huxley, &remarque}
	for _, a := range authors {
		if err := db.FirstOrCreate(a, catalogDomain.Author{Slug: a.Slug}).Error; err != nil {
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
		{
			book: catalogDomain.Book{
				Title:         "Анна Каренина",
				Slug:          "anna-karenina",
				Description:   ptr("Роман о трагической любви замужней женщины и молодого офицера."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264187-L.jpg"),
				ISBN:          ptr("9780143035008"),
				PublishedYear: ptrInt(1877),
				PageCount:     ptrInt(864),
				Language:      "ru",
			},
			author: tolstoy,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Воскресение",
				Slug:          "resurrection",
				Description:   ptr("Роман о духовном перерождении помещика Нехлюдова."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264175-L.jpg"),
				ISBN:          ptr("9780140448092"),
				PublishedYear: ptrInt(1899),
				PageCount:     ptrInt(552),
				Language:      "ru",
			},
			author: tolstoy,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Братья Карамазовы",
				Slug:          "brothers-karamazov",
				Description:   ptr("Философский роман о семейных конфликтах и нравственных исканиях."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264393-L.jpg"),
				ISBN:          ptr("9780140449136"),
				PublishedYear: ptrInt(1880),
				PageCount:     ptrInt(796),
				Language:      "ru",
			},
			author: dostoevsky,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Идиот",
				Slug:          "the-idiot",
				Description:   ptr("Роман о «идеальном человеке» князе Мышкине."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264389-L.jpg"),
				ISBN:          ptr("9780140447927"),
				PublishedYear: ptrInt(1869),
				PageCount:     ptrInt(608),
				Language:      "ru",
			},
			author: dostoevsky,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Мёртвые души",
				Slug:          "dead-souls",
				Description:   ptr("Поэма о приключениях Чичикова, скупающего мёртвых крепостных."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264015-L.jpg"),
				ISBN:          ptr("9780140448078"),
				PublishedYear: ptrInt(1842),
				PageCount:     ptrInt(352),
				Language:      "ru",
			},
			author: gogol,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Ревизор",
				Slug:          "the-government-inspector",
				Description:   ptr("Комедия о чиновниках provincialного города, принятивших бродягу за ревизора."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264021-L.jpg"),
				ISBN:          ptr("9780140447941"),
				PublishedYear: ptrInt(1836),
				PageCount:     ptrInt(128),
				Language:      "ru",
			},
			author: gogol,
			genres: []string{"classics"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Отцы и дети",
				Slug:          "fathers-and-sons",
				Description:   ptr("Роман о конфликте поколений и спорах о нигилизме."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264031-L.jpg"),
				ISBN:          ptr("9780140448108"),
				PublishedYear: ptrInt(1862),
				PageCount:     ptrInt(288),
				Language:      "ru",
			},
			author: turgenev,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Вишнёвый сад",
				Slug:          "the-cherry-orchard",
				Description:   ptr("Пьеса о старой дворянской семье, теряющей имение."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264041-L.jpg"),
				ISBN:          ptr("9780142437872"),
				PublishedYear: ptrInt(1904),
				PageCount:     ptrInt(128),
				Language:      "ru",
			},
			author: chekhov,
			genres: []string{"classics"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Чайка",
				Slug:          "the-seagull",
				Description:   ptr("Пьеса о любви, литературных амбициях и творческом поиске."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264047-L.jpg"),
				ISBN:          ptr("9780142437865"),
				PublishedYear: ptrInt(1896),
				PageCount:     ptrInt(128),
				Language:      "ru",
			},
			author: chekhov,
			genres: []string{"classics"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Герой нашего времени",
				Slug:          "a-hero-of-our-time",
				Description:   ptr("Роман о молодом офицере Печорине — «лишнем человеке»."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264051-L.jpg"),
				ISBN:          ptr("9780140448115"),
				PublishedYear: ptrInt(1840),
				PageCount:     ptrInt(224),
				Language:      "ru",
			},
			author: lermontov,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Евгений Онегин",
				Slug:          "yevgeny-onegin",
				Description:   ptr("Роман в стихах о судьбе молодого петербургского дворянина."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264057-L.jpg"),
				ISBN:          ptr("9780140448122"),
				PublishedYear: ptrInt(1833),
				PageCount:     ptrInt(240),
				Language:      "ru",
			},
			author: pushkin,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Капитанская дочка",
				Slug:          "the-captain's-daughter",
				Description:   ptr("Исторический роман о восстании Пугачёва."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264061-L.jpg"),
				ISBN:          ptr("9780140448139"),
				PublishedYear: ptrInt(1836),
				PageCount:     ptrInt(208),
				Language:      "ru",
			},
			author: pushkin,
			genres: []string{"classics", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Дивный новый мир",
				Slug:          "brave-new-world",
				Description:   ptr("Антиутопия о будущем обществе, контролируемом с помощью генетики и наркотиков."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8579167-L.jpg"),
				ISBN:          ptr("9780060850524"),
				PublishedYear: ptrInt(1932),
				PageCount:     ptrInt(288),
				Language:      "en",
			},
			author: huxley,
			genres: []string{"fiction", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "На Западе без перемен",
				Slug:          "all-quiet-on-the-western-front",
				Description:   ptr("Антивоенный роман о судьбе немецких солдат в Первой мировой войне."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8264401-L.jpg"),
				ISBN:          ptr("9780449212387"),
				PublishedYear: ptrInt(1929),
				PageCount:     ptrInt(200),
				Language:      "en",
			},
			author: remarque,
			genres: []string{"fiction", "novel"},
		},
		{
			book: catalogDomain.Book{
				Title:         "Скотный двор",
				Slug:          "animal-farm",
				Description:   ptr("Аллегорическая сказка о револении животных на ферме."),
				CoverURL:      ptr("https://covers.openlibrary.org/b/id/8579157-L.jpg"),
				ISBN:          ptr("9780451526342"),
				PublishedYear: ptrInt(1945),
				PageCount:     ptrInt(112),
				Language:      "en",
			},
			author: orwell,
			genres: []string{"fiction"},
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

func ptrTime(s string) *time.Time {
	t, _ := time.Parse("2006-01-02", s)
	return &t
}
