package main

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
)

// demoReader is one -demo-readers fixture: books on the system "read" shelf
// plus explicit ratings (book slug -> 1..5).
type demoReader struct {
	read    []string
	ratings map[string]int
}

// demoReaders are 12 verified dev/demo readers with deterministic,
// overlapping reading sets so the stage-2 similar-books job (co-reading,
// K = 3 shared readers, see docs/features/similar-books-stage2.md) has data
// locally. Readers 1-6 mostly read the Russian-classics cluster, readers
// 7-12 the 20th-century dystopia/war cluster; master-and-margarita and
// crime-and-punishment bridge the two. reader06 rates war-and-peace 1, which
// exercises the "disliked is not a signal" rule; reader11 rates
// fathers-and-sons 5 without shelving it (rating-only signal).
var demoReaders = []demoReader{
	{read: []string{"war-and-peace", "crime-and-punishment", "anna-karenina", "brothers-karamazov"}, ratings: map[string]int{"war-and-peace": 5, "anna-karenina": 4}},
	{read: []string{"war-and-peace", "crime-and-punishment", "anna-karenina", "the-idiot"}},
	{read: []string{"war-and-peace", "crime-and-punishment", "brothers-karamazov", "master-and-margarita"}, ratings: map[string]int{"master-and-margarita": 5}},
	{read: []string{"war-and-peace", "anna-karenina", "brothers-karamazov", "dead-souls"}},
	{read: []string{"crime-and-punishment", "brothers-karamazov", "the-idiot", "master-and-margarita"}, ratings: map[string]int{"the-idiot": 4}},
	{read: []string{"war-and-peace", "anna-karenina", "the-idiot", "yevgeny-onegin"}, ratings: map[string]int{"war-and-peace": 1}},
	{read: []string{"1984", "brave-new-world", "animal-farm", "master-and-margarita"}, ratings: map[string]int{"1984": 5}},
	{read: []string{"1984", "brave-new-world", "all-quiet-on-the-western-front", "master-and-margarita"}},
	{read: []string{"1984", "animal-farm", "all-quiet-on-the-western-front", "crime-and-punishment"}},
	{read: []string{"1984", "brave-new-world", "animal-farm", "all-quiet-on-the-western-front"}, ratings: map[string]int{"brave-new-world": 4}},
	{read: []string{"brave-new-world", "animal-farm", "all-quiet-on-the-western-front", "master-and-margarita"}, ratings: map[string]int{"fathers-and-sons": 5}},
	{read: []string{"1984", "animal-farm", "crime-and-punishment", "war-and-peace"}},
}

func demoReaderEmail(i int) string {
	return fmt.Sprintf("reader%02d@demo.bookhaven.local", i)
}

// seedDemoReaders idempotently creates the demo readers (verified, no
// password, so nobody can log in as them), their system shelves (same
// semantics as shelves.EnsureDefaultShelves), "read" shelf items and
// ratings, then recalculates books.rating/ratings_count for rated books.
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
		items, ratings, skipped := 0, 0, 0
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
		}

		for bookID := range rated {
			if err := tx.Exec(`
				UPDATE books
				SET rating = COALESCE(ROUND((SELECT AVG(value) FROM ratings WHERE book_id = ?)::numeric, 2), 0),
				    ratings_count = (SELECT COUNT(*) FROM ratings WHERE book_id = ?)
				WHERE id = ?`, bookID, bookID, bookID).Error; err != nil {
				return fmt.Errorf("recalc rating: %w", err)
			}
		}

		fmt.Printf("demo readers: %d users (%s..%s, no password), %d shelf items, %d ratings, %d missing books skipped\n",
			len(demoReaders), demoReaderEmail(1), demoReaderEmail(len(demoReaders)), items, ratings, skipped)
		return nil
	})
}
