package bookdata

import "strings"

// normalizeISBNStructural trims whitespace and strips spaces/dashes from an
// ISBN string. It performs no checksum validation — full ISBN-10/13
// checksum validation is owned by admin/service.isValidISBN and applied
// later, when a record is fed into admin.Service.CreateBook, per this
// package's business rules (avoids duplicating that validation here).
func normalizeISBNStructural(isbn string) string {
	isbn = strings.TrimSpace(isbn)
	isbn = strings.ReplaceAll(isbn, "-", "")
	isbn = strings.ReplaceAll(isbn, " ", "")
	return isbn
}
