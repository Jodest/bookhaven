package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
)

// demoReader is one -demo-readers fixture: books on the system "read" shelf
// plus explicit ratings (book slug -> 1..5) and reviews (book slug ->
// content; the review's rating is the reader's rating of that book, as the
// reviews module keeps them in sync).
type demoReader struct {
	read    []string
	ratings map[string]int
	reviews map[string]string
}

// demoReaders are 12 verified dev/demo readers with deterministic,
// overlapping reading sets so the stage-2 similar-books job (co-reading,
// K = 3 shared readers, see docs/features/similar-books-stage2.md) has data
// locally. Readers 1-6 mostly read the Russian-classics cluster, readers
// 7-12 the 20th-century dystopia/war cluster; master-and-margarita and
// crime-and-punishment bridge the two. reader06 rates war-and-peace 1, which
// exercises the "disliked is not a signal" rule; reader11 rates
// fathers-and-sons 5 without shelving it (rating-only signal).
//
// The 4-5 ratings of read books also give at least 6 books
// ratings_count >= 3 and rating >= 4 (the home page "Высоко оценённые" row
// and the strict book-of-the-day pool), and four readers review different
// books with 40+ characters (the home page "Свежие рецензии"). See
// docs/features/home-page-content.md (Dev data).
var demoReaders = []demoReader{
	{
		read:    []string{"war-and-peace", "crime-and-punishment", "anna-karenina", "brothers-karamazov"},
		ratings: map[string]int{"war-and-peace": 5, "anna-karenina": 4, "crime-and-punishment": 5, "brothers-karamazov": 5},
		reviews: map[string]string{"war-and-peace": "Читал всё лето и ни разу не пожалел: батальные сцены и семейные главы держат одинаково крепко, а Наташа Ростова - один из самых живых героев русской прозы."},
	},
	{read: []string{"war-and-peace", "crime-and-punishment", "anna-karenina", "the-idiot"}, ratings: map[string]int{"war-and-peace": 5, "crime-and-punishment": 4, "anna-karenina": 5}},
	{
		read:    []string{"war-and-peace", "crime-and-punishment", "brothers-karamazov", "master-and-margarita"},
		ratings: map[string]int{"master-and-margarita": 5, "war-and-peace": 5, "brothers-karamazov": 4},
		reviews: map[string]string{"master-and-margarita": "Перечитываю раз в несколько лет и каждый раз нахожу новое: московская сатира смешит, а ершалаимские главы звучат как отдельная великая повесть."},
	},
	{read: []string{"war-and-peace", "anna-karenina", "brothers-karamazov", "dead-souls"}, ratings: map[string]int{"war-and-peace": 5, "anna-karenina": 4}},
	{
		read:    []string{"crime-and-punishment", "brothers-karamazov", "the-idiot", "master-and-margarita"},
		ratings: map[string]int{"the-idiot": 4, "crime-and-punishment": 5, "brothers-karamazov": 5, "master-and-margarita": 5},
		reviews: map[string]string{"crime-and-punishment": "Тяжёлая, душная и при этом невероятно затягивающая книга. Сцены с Порфирием Петровичем читаются как лучший психологический детектив."},
	},
	{read: []string{"war-and-peace", "anna-karenina", "the-idiot", "yevgeny-onegin"}, ratings: map[string]int{"war-and-peace": 1}},
	{
		read:    []string{"1984", "brave-new-world", "animal-farm", "master-and-margarita"},
		ratings: map[string]int{"1984": 5, "brave-new-world": 4},
		reviews: map[string]string{"1984": "Страшно, насколько точно Оруэлл описал механику пропаганды. После финала долго не мог взяться за следующую книгу, настолько сильное послевкусие."},
	},
	{read: []string{"1984", "brave-new-world", "all-quiet-on-the-western-front", "master-and-margarita"}, ratings: map[string]int{"1984": 5, "brave-new-world": 5, "master-and-margarita": 4}},
	{read: []string{"1984", "animal-farm", "all-quiet-on-the-western-front", "crime-and-punishment"}, ratings: map[string]int{"1984": 4, "animal-farm": 4, "crime-and-punishment": 4}},
	{read: []string{"1984", "brave-new-world", "animal-farm", "all-quiet-on-the-western-front"}, ratings: map[string]int{"brave-new-world": 4, "1984": 5, "animal-farm": 5}},
	{read: []string{"brave-new-world", "animal-farm", "all-quiet-on-the-western-front", "master-and-margarita"}, ratings: map[string]int{"fathers-and-sons": 5, "master-and-margarita": 5}},
	{read: []string{"1984", "animal-farm", "crime-and-punishment", "war-and-peace"}, ratings: map[string]int{"animal-farm": 4, "war-and-peace": 4}},
}

func demoReaderEmail(i int) string {
	return fmt.Sprintf("reader%02d@demo.bookhaven.local", i)
}

// seedDemoReaders idempotently creates the demo readers (verified, no
// password, so nobody can log in as them), their system shelves (same
// semantics as shelves.EnsureDefaultShelves), "read" shelf items and
// ratings and reviews (an existing review of the same reader and book is
// left untouched), then recalculates books.rating/ratings_count/
// reviews_count for rated books.
// Books are resolved by seed slug; missing ones are skipped.
func seedDemoReaders(db *gorm.DB) error {
	return db.Transaction(func(tx *gorm.DB) error {
		var books []struct {
			ID   uuid.UUID
			Slug string
		}
		if err := tx.Raw("SELECT id, slug FROM books").Scan(&books).Error; err != nil {
			return fmt.Errorf("load books: %w", err)
		}
		bySlug := make(map[string]uuid.UUID, len(books))
		for _, b := range books {
			bySlug[b.Slug] = b.ID
		}

		verifiedAt := time.Now()
		rated := make(map[uuid.UUID]struct{})
		items, ratings, reviews, skipped := 0, 0, 0, 0
		for i, r := range demoReaders {
			n := i + 1
			user := usersDomain.User{
				Email:           demoReaderEmail(n),
				Name:            ptr(fmt.Sprintf("Демо-читатель %02d", n)),
				Role:            usersDomain.RoleUser,
				ProfileType:     usersDomain.ProfileTypeReader,
				EmailVerifiedAt: &verifiedAt,
			}
			if err := tx.FirstOrCreate(&user, usersDomain.User{Email: user.Email}).Error; err != nil {
				return fmt.Errorf("create demo reader %d: %w", n, err)
			}
			if err := tx.Exec(
				"UPDATE users SET email_verified_at = now() WHERE id = ? AND email_verified_at IS NULL", user.ID,
			).Error; err != nil {
				return fmt.Errorf("verify demo reader %d: %w", n, err)
			}

			if err := tx.Exec(`
				INSERT INTO shelves (id, user_id, name, type)
				VALUES
					(gen_random_uuid(), ?, 'Хочу прочитать', 'want_to_read'),
					(gen_random_uuid(), ?, 'Читаю', 'reading'),
					(gen_random_uuid(), ?, 'Прочитано', 'read')
				ON CONFLICT (user_id, type) WHERE type <> 'custom' DO NOTHING`,
				user.ID, user.ID, user.ID).Error; err != nil {
				return fmt.Errorf("create shelves for demo reader %d: %w", n, err)
			}
			var shelfIDs []uuid.UUID
			if err := tx.Table("shelves").
				Where("user_id = ? AND type = 'read'", user.ID).
				Pluck("id", &shelfIDs).Error; err != nil {
				return fmt.Errorf("load read shelf of demo reader %d: %w", n, err)
			}
			if len(shelfIDs) != 1 {
				return fmt.Errorf("demo reader %d: expected 1 read shelf, got %d", n, len(shelfIDs))
			}
			shelfID := shelfIDs[0]

			for _, slug := range r.read {
				bookID, ok := bySlug[slug]
				if !ok {
					skipped++
					continue
				}
				if err := tx.Exec(
					"INSERT INTO shelf_items (shelf_id, book_id) VALUES (?, ?) ON CONFLICT DO NOTHING",
					shelfID, bookID,
				).Error; err != nil {
					return fmt.Errorf("shelve %s for demo reader %d: %w", slug, n, err)
				}
				items++
			}
			for slug, value := range r.ratings {
				bookID, ok := bySlug[slug]
				if !ok {
					skipped++
					continue
				}
				if err := tx.Exec(`
					INSERT INTO ratings (id, book_id, user_id, value, created_at, updated_at)
					VALUES (gen_random_uuid(), ?, ?, ?, now(), now())
					ON CONFLICT (book_id, user_id) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
					bookID, user.ID, value).Error; err != nil {
					return fmt.Errorf("rate %s for demo reader %d: %w", slug, n, err)
				}
				rated[bookID] = struct{}{}
				ratings++
			}
			for slug, content := range r.reviews {
				bookID, ok := bySlug[slug]
				if !ok {
					skipped++
					continue
				}
				value, ok := r.ratings[slug]
				if !ok {
					return fmt.Errorf("demo reader %d: review of %s has no rating", n, slug)
				}
				if err := tx.Exec(`
					INSERT INTO reviews (id, book_id, user_id, rating, content, created_at, updated_at)
					VALUES (gen_random_uuid(), ?, ?, ?, ?, now(), now())
					ON CONFLICT (book_id, user_id) DO UPDATE SET rating = EXCLUDED.rating`,
					bookID, user.ID, value, content).Error; err != nil {
					return fmt.Errorf("review %s for demo reader %d: %w", slug, n, err)
				}
				reviews++
			}
		}

		for bookID := range rated {
			if err := tx.Exec(`
				UPDATE books
				SET rating = COALESCE(ROUND((SELECT AVG(value) FROM ratings WHERE book_id = ?)::numeric, 2), 0),
				    ratings_count = (SELECT COUNT(*) FROM ratings WHERE book_id = ?),
				    reviews_count = (SELECT COUNT(*) FROM reviews WHERE book_id = ? AND is_hidden = false)
				WHERE id = ?`, bookID, bookID, bookID, bookID).Error; err != nil {
				return fmt.Errorf("recalc rating: %w", err)
			}
		}

		fmt.Printf("demo readers: %d users (%s..%s, no password), %d shelf items, %d ratings, %d reviews, %d missing books skipped\n",
			len(demoReaders), demoReaderEmail(1), demoReaderEmail(len(demoReaders)), items, ratings, reviews, skipped)
		return nil
	})
}
