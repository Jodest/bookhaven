package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Manual editing of screen works (openspec change add-screen-works-admin-editing).
// Books and authors tables are read only here.

const workListSelect = `w.id, w.title, w.original_title, w.kind, w.year, w.director, w.imdb_id, w.kinopoisk_id,
w.external_id AS qid, w.manually_edited_at,
(SELECT COUNT(*) FROM book_screen_works b WHERE b.screen_work_id = w.id) AS book_count,
(SELECT COUNT(*) FROM author_screen_works a WHERE a.screen_work_id = w.id) AS author_count`

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	msg := err.Error()
	return strings.Contains(msg, "UNIQUE constraint failed") || strings.Contains(msg, "duplicate key")
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "%", `\%`)
	return strings.ReplaceAll(s, "_", `\_`)
}

func (r *GormRepository) ListWorks(ctx context.Context, f domain.WorkFilter) ([]domain.WorkListItem, int64, error) {
	q := r.db.WithContext(ctx).Table("screen_works AS w")
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "LIKE"
		if r.db.Dialector.Name() == "postgres" {
			like = "ILIKE"
		}
		pat := "%" + escapeLike(s) + "%"
		q = q.Where(`(w.title `+like+` ? ESCAPE '\' OR w.original_title `+like+` ? ESCAPE '\'
OR w.external_id = ? OR w.imdb_id = ? OR w.kinopoisk_id = ?)`, pat, pat, strings.ToUpper(s), s, s)
	}
	if f.Unlinked {
		q = q.Where("NOT EXISTS (SELECT 1 FROM book_screen_works b WHERE b.screen_work_id = w.id) AND NOT EXISTS (SELECT 1 FROM author_screen_works a WHERE a.screen_work_id = w.id)")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var items []domain.WorkListItem
	err := q.Select(workListSelect).
		Order("w.title, CASE WHEN w.year IS NULL THEN 1 ELSE 0 END, w.year, w.id").
		Limit(f.Limit).Offset((f.Page - 1) * f.Limit).Scan(&items).Error
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *GormRepository) GetWork(ctx context.Context, id uuid.UUID) (*domain.WorkDetail, error) {
	var rows []domain.WorkListItem
	if err := r.db.WithContext(ctx).Table("screen_works AS w").Select(workListSelect).
		Where("w.id = ?", id).Limit(1).Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, domain.ErrWorkNotFound
	}
	d := &domain.WorkDetail{WorkListItem: rows[0]}
	if err := r.db.WithContext(ctx).Table("book_screen_works AS l").
		Joins("JOIN books b ON b.id = l.book_id").
		Select("b.id, b.title, b.status, l.hidden, l.source").
		Where("l.screen_work_id = ?", id).Order("b.title, b.id").Scan(&d.Books).Error; err != nil {
		return nil, err
	}
	if err := r.db.WithContext(ctx).Table("author_screen_works AS l").
		Joins("JOIN authors a ON a.id = l.author_id").
		Select("a.id, a.name, a.status, l.hidden, l.source, l.relation").
		Where("l.screen_work_id = ?", id).Order("a.name, a.id").Scan(&d.Authors).Error; err != nil {
		return nil, err
	}
	return d, nil
}

// UpdateWork replaces the editable fields and marks the work as edited by
// hand. The QID can be added to a work without one, never changed or cleared.
func (r *GormRepository) UpdateWork(ctx context.Context, id uuid.UUID, f domain.WorkFields) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cur []struct {
			ExternalID *string `gorm:"column:external_id"`
		}
		if err := tx.Table("screen_works").Select("external_id").Where("id = ?", id).Limit(1).Scan(&cur).Error; err != nil {
			return err
		}
		if len(cur) == 0 {
			return domain.ErrWorkNotFound
		}
		setQID := false
		if cur[0].ExternalID != nil {
			if f.QID != nil && *f.QID != *cur[0].ExternalID {
				return domain.ErrQIDImmutable
			}
		} else if f.QID != nil {
			var taken []struct {
				ID uuid.UUID `gorm:"column:id"`
			}
			if err := tx.Table("screen_works").Select("id").
				Where("external_source = ? AND external_id = ? AND id <> ?", domain.ExternalSourceWikidata, *f.QID, id).
				Limit(1).Scan(&taken).Error; err != nil {
				return err
			}
			if len(taken) > 0 {
				return domain.ErrQIDTaken
			}
			setQID = true
		}
		now := time.Now()
		sql := `UPDATE screen_works SET title = ?, original_title = ?, kind = ?, year = ?, director = ?, imdb_id = ?, kinopoisk_id = ?, manually_edited_at = ?, updated_at = ?`
		args := []any{f.Title, f.OriginalTitle, f.Kind, f.Year, f.Director, f.IMDbID, f.KinopoiskID, now, now}
		if setQID {
			sql += `, external_source = ?, external_id = ?`
			args = append(args, domain.ExternalSourceWikidata, *f.QID)
		}
		sql += ` WHERE id = ?`
		args = append(args, id)
		if err := tx.Exec(sql, args...).Error; err != nil {
			if isUniqueViolation(err) {
				return domain.ErrQIDTaken
			}
			return err
		}
		return nil
	})
}

// DeleteWorkIfUnlinked deletes a work that has no book and no author link;
// domain.ErrWorkNotFound or domain.ErrWorkHasLinks otherwise.
//
// A single "DELETE ... WHERE NOT EXISTS (links)" is not race-free under
// Postgres READ COMMITTED: a concurrent manual-link INSERT holds FOR KEY
// SHARE on the work row, the DELETE does not see the uncommitted link, waits
// for the row lock, and once the insert commits the row is unchanged so the
// subquery is not re-evaluated; the DELETE then proceeds and ON DELETE
// CASCADE silently drops the new link. So the work row is locked FOR UPDATE
// first (it conflicts with the link insert's FOR KEY SHARE, which serialises
// the two), and the links are checked in a separate statement that takes a
// fresh snapshot.
func (r *GormRepository) DeleteWorkIfUnlinked(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		q := tx.Table("screen_works").Select("id").Where("id = ?", id)
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE"})
		}
		var rows []struct {
			ID uuid.UUID `gorm:"column:id"`
		}
		if err := q.Scan(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return domain.ErrWorkNotFound
		}
		var links int64
		if err := tx.Raw(`SELECT (SELECT COUNT(*) FROM book_screen_works WHERE screen_work_id = ?)
+ (SELECT COUNT(*) FROM author_screen_works WHERE screen_work_id = ?)`, id, id).Scan(&links).Error; err != nil {
			return err
		}
		if links > 0 {
			return domain.ErrWorkHasLinks
		}
		return tx.Exec("DELETE FROM screen_works WHERE id = ?", id).Error
	})
}

// resolveWork returns the id of the referenced work, creating it when needed.
// created is true only for a newly inserted row.
func resolveWork(tx *gorm.DB, ref domain.WorkRef) (id uuid.UUID, created bool, err error) {
	if ref.WorkID != nil {
		var n int64
		if err := tx.Table("screen_works").Where("id = ?", *ref.WorkID).Count(&n).Error; err != nil {
			return uuid.Nil, false, err
		}
		if n == 0 {
			return uuid.Nil, false, domain.ErrWorkNotFound
		}
		return *ref.WorkID, false, nil
	}
	if ref.Fields == nil {
		return uuid.Nil, false, domain.ErrLinkRefRequired
	}
	f := *ref.Fields
	findByQID := func() (uuid.UUID, bool, error) {
		var rows []struct {
			ID uuid.UUID `gorm:"column:id"`
		}
		if err := tx.Table("screen_works").Select("id").
			Where("external_source = ? AND external_id = ?", domain.ExternalSourceWikidata, *f.QID).
			Limit(1).Scan(&rows).Error; err != nil {
			return uuid.Nil, false, err
		}
		if len(rows) == 0 {
			return uuid.Nil, false, nil
		}
		return rows[0].ID, true, nil
	}
	var extSource, extID any
	if f.QID != nil {
		if id, ok, err := findByQID(); err != nil || ok {
			return id, false, err
		}
		extSource, extID = domain.ExternalSourceWikidata, *f.QID
	}
	for _, d := range []struct {
		field, col string
		val        *string
	}{{"imdb id", "imdb_id", f.IMDbID}, {"kinopoisk id", "kinopoisk_id", f.KinopoiskID}} {
		if d.val == nil {
			continue
		}
		var rows []struct {
			ID uuid.UUID `gorm:"column:id"`
		}
		if err := tx.Table("screen_works").Select("id").Where(d.col+" = ?", *d.val).Limit(1).Scan(&rows).Error; err != nil {
			return uuid.Nil, false, err
		}
		if len(rows) > 0 {
			return uuid.Nil, false, &domain.ErrDuplicateExternalID{Field: d.field, WorkID: rows[0].ID}
		}
	}
	now := time.Now()
	newID := uuid.New()
	res := tx.Exec(`INSERT INTO screen_works (id, title, original_title, kind, year, director, imdb_id, kinopoisk_id, external_source, external_id, manually_edited_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (external_source, external_id) WHERE external_id IS NOT NULL DO NOTHING`,
		newID, f.Title, f.OriginalTitle, f.Kind, f.Year, f.Director, f.IMDbID, f.KinopoiskID, extSource, extID, now, now, now)
	if res.Error != nil {
		return uuid.Nil, false, res.Error
	}
	if res.RowsAffected > 0 {
		return newID, true, nil
	}
	// A racing create with the same QID won: converge on its row.
	if f.QID != nil {
		if id, ok, err := findByQID(); err != nil || ok {
			return id, false, err
		}
	}
	return uuid.Nil, false, errors.New("screen work vanished after insert")
}

func (r *GormRepository) AddManualBookLink(ctx context.Context, bookID uuid.UUID, ref domain.WorkRef) (uuid.UUID, bool, error) {
	var workID uuid.UUID
	var created bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if workID, created, err = resolveWork(tx, ref); err != nil {
			return err
		}
		now := time.Now()
		res := tx.Exec(`INSERT INTO book_screen_works (book_id, screen_work_id, hidden, source, created_at, updated_at)
VALUES (?, ?, FALSE, ?, ?, ?) ON CONFLICT (book_id, screen_work_id) DO NOTHING`,
			bookID, workID, domain.SourceManual, now, now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrAlreadyLinked
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, false, err
	}
	return workID, created, nil
}

func (r *GormRepository) AddManualAuthorLink(ctx context.Context, authorID uuid.UUID, ref domain.WorkRef, relation string) (uuid.UUID, bool, error) {
	var workID uuid.UUID
	var created bool
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		if workID, created, err = resolveWork(tx, ref); err != nil {
			return err
		}
		now := time.Now()
		res := tx.Exec(`INSERT INTO author_screen_works (author_id, screen_work_id, relation, hidden, source, created_at, updated_at)
VALUES (?, ?, ?, FALSE, ?, ?, ?) ON CONFLICT (author_id, screen_work_id) DO NOTHING`,
			authorID, workID, relation, domain.SourceManual, now, now)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return domain.ErrAlreadyLinked
		}
		return nil
	})
	if err != nil {
		return uuid.Nil, false, err
	}
	return workID, created, nil
}

// deleteManualLink deletes a manual link of the given table; notFound is
// returned for a missing link and ErrLinkNotManual for a Wikidata one.
func (r *GormRepository) deleteManualLink(ctx context.Context, table, ownerCol string, ownerID, workID uuid.UUID, notFound error) error {
	res := r.db.WithContext(ctx).Exec(
		"DELETE FROM "+table+" WHERE "+ownerCol+" = ? AND screen_work_id = ? AND source = ?",
		ownerID, workID, domain.SourceManual)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	var n int64
	if err := r.db.WithContext(ctx).Table(table).
		Where(ownerCol+" = ? AND screen_work_id = ?", ownerID, workID).Count(&n).Error; err != nil {
		return err
	}
	if n == 0 {
		return notFound
	}
	return domain.ErrLinkNotManual
}

func (r *GormRepository) DeleteManualBookLink(ctx context.Context, bookID, workID uuid.UUID) error {
	return r.deleteManualLink(ctx, "book_screen_works", "book_id", bookID, workID, domain.ErrLinkNotFound)
}

func (r *GormRepository) DeleteManualAuthorLink(ctx context.Context, authorID, workID uuid.UUID) error {
	return r.deleteManualLink(ctx, "author_screen_works", "author_id", authorID, workID, domain.ErrAuthorLinkNotFound)
}

// UpdateAuthorLink sets the hidden flag, the relation or both.
func (r *GormRepository) UpdateAuthorLink(ctx context.Context, authorID, workID uuid.UUID, hidden *bool, relation *string) error {
	sets := []string{"updated_at = ?"}
	args := []any{time.Now()}
	if hidden != nil {
		sets = append(sets, "hidden = ?")
		args = append(args, *hidden)
	}
	if relation != nil {
		sets = append(sets, "relation = ?")
		args = append(args, *relation)
	}
	args = append(args, authorID, workID)
	res := r.db.WithContext(ctx).Exec(
		"UPDATE author_screen_works SET "+strings.Join(sets, ", ")+" WHERE author_id = ? AND screen_work_id = ?", args...)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrAuthorLinkNotFound
	}
	return nil
}
