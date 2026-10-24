package http

// Schema for POST /admin/books/import (documented here per
// docs/features/catalog-manual-import.md's "pick one place, don't
// duplicate" instruction - see that file's API section for the response
// shape):
//
// CSV: header row, columns matched case-insensitively, order-independent.
//   - title (required; enforced by admin/service.CreateBook - a row
//     without one is reported as a row error, not a whole-file failure)
//   - authors (";"-separated author names; a row that resolves to zero
//     author names is rejected as a row error - see admin/service.ImportBooks)
//   - genres (optional, ";"-separated genre names)
//   - isbn, description, cover_url, published_year, page_count, language,
//     slug (all optional)
//
// A column that is entirely absent from the header behaves the same as an
// empty value on every row (not a whole-file error) - only a header/data
// line that cannot be parsed as CSV at all rejects the whole file with 400
// (E-01). Non-numeric published_year/page_count values are treated as "not
// provided" rather than failing the row, since both fields are optional and
// still bounds-checked by CreateBook when present.
//
// JSON: an array of objects with the same field names in snake_case;
// authors/genres are string arrays instead of ";"-joined strings.

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
)

const (
	// maxImportRows bounds the number of data rows accepted by one
	// POST /admin/books/import request (NFR-002). Rows beyond this are
	// rejected wholesale (422, E-06) before any row is processed. Chosen on
	// the lower end of the business analysis' "order of hundreds of rows"
	// guidance so the synchronous request stays comfortably inside a normal
	// HTTP timeout budget; larger loads should use cmd/harvest or cmd/seed.
	maxImportRows = 500
)

// ImportMaxRequestBodyBytes bounds the raw request body accepted by
// POST /admin/books/import (NFR-002). The 413 is applied before any parsing
// by the global middleware.BodyLimit through the override table
// (httptransport.BodyLimitOverrides), which is the only way to raise the
// default limit for a route.
const ImportMaxRequestBodyBytes int64 = 2 << 20

// sniffImportFormat maps a filename extension to "csv"/"json", or "" if
// undetectable. Used when the request omits the optional "format" field.
func sniffImportFormat(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		return "csv"
	case ".json":
		return "json"
	default:
		return ""
	}
}

// splitImportNames splits a ";"-separated CSV cell into trimmed, non-empty
// names.
func splitImportNames(raw string) []string {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ";")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			names = append(names, p)
		}
	}
	return names
}

// parseImportCSV parses a CSV file into ImportRowInput rows. It returns an
// error only for a structurally unreadable file (E-01): an unparseable
// header or data row.
func parseImportCSV(r io.Reader) ([]adminService.ImportRowInput, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}
	colIndex := make(map[string]int, len(header))
	for i, col := range header {
		colIndex[strings.ToLower(strings.TrimSpace(col))] = i
	}

	get := func(record []string, col string) string {
		idx, ok := colIndex[col]
		if !ok || idx >= len(record) {
			return ""
		}
		return strings.TrimSpace(record[idx])
	}

	var rows []adminService.ImportRowInput
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}

		row := adminService.ImportRowInput{
			Title:    get(record, "title"),
			Authors:  splitImportNames(get(record, "authors")),
			Genres:   splitImportNames(get(record, "genres")),
			Language: get(record, "language"),
			Slug:     get(record, "slug"),
		}
		if v := get(record, "isbn"); v != "" {
			row.ISBN = &v
		}
		if v := get(record, "description"); v != "" {
			row.Description = &v
		}
		if v := get(record, "cover_url"); v != "" {
			row.CoverURL = &v
		}
		if v := get(record, "published_year"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				row.PublishedYear = &n
			}
		}
		if v := get(record, "page_count"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				row.PageCount = &n
			}
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// jsonImportRow is the JSON shape for one row of POST /admin/books/import.
type jsonImportRow struct {
	Title         string   `json:"title"`
	Authors       []string `json:"authors"`
	Genres        []string `json:"genres"`
	ISBN          *string  `json:"isbn"`
	Description   *string  `json:"description"`
	CoverURL      *string  `json:"cover_url"`
	PublishedYear *int     `json:"published_year"`
	PageCount     *int     `json:"page_count"`
	Language      string   `json:"language"`
	Slug          string   `json:"slug"`
}

// parseImportJSON parses a JSON array of rows into ImportRowInput rows. It
// returns an error only for structurally invalid JSON (E-01): malformed
// syntax, or a top-level value that isn't an array of objects.
func parseImportJSON(r io.Reader) ([]adminService.ImportRowInput, error) {
	var raw []jsonImportRow
	if err := json.NewDecoder(r).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	rows := make([]adminService.ImportRowInput, 0, len(raw))
	for _, jr := range raw {
		rows = append(rows, adminService.ImportRowInput{
			Title:         jr.Title,
			Authors:       jr.Authors,
			Genres:        jr.Genres,
			ISBN:          jr.ISBN,
			Description:   jr.Description,
			CoverURL:      jr.CoverURL,
			PublishedYear: jr.PublishedYear,
			PageCount:     jr.PageCount,
			Language:      jr.Language,
			Slug:          jr.Slug,
		})
	}
	return rows, nil
}
