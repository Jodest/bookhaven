// Package coverstore keeps self-hosted book cover files (#489) behind a small
// interface so the filesystem can later be swapped for object storage.
package coverstore

import (
	"context"
	"errors"
	"io"
	"regexp"
	"time"
)

// MarkerName is a hidden file that tells the sweep a storage volume is the
// same one it wrote to before (a new or lost volume has no marker).
const MarkerName = ".bookhaven-covers"

// ErrNotFound is returned by Open for a missing file.
var ErrNotFound = errors.New("coverstore: not found")

// ErrInvalidName is returned for names that fail ValidName.
var ErrInvalidName = errors.New("coverstore: invalid name")

var nameRE = regexp.MustCompile(`^[0-9a-f]{2}/[0-9a-f]{64}\.(jpg|png|webp)$`)

// ValidName reports whether name is a well-formed cover file name
// "{sha256[0:2]}/{sha256}.{ext}". The store and the HTTP handler share it, so
// path traversal is impossible by construction.
func ValidName(name string) bool {
	return nameRE.MatchString(name) && name[:2] == name[3:5]
}

// Info describes an opened file.
type Info struct {
	Size    int64
	ModTime time.Time
}

// Store is the cover file storage.
type Store interface {
	// Put writes a file atomically.
	Put(ctx context.Context, name string, data []byte) error
	// Open returns the file or ErrNotFound.
	Open(ctx context.Context, name string) (io.ReadSeekCloser, Info, error)
	// Delete removes a file; a missing file is not an error.
	Delete(ctx context.Context, name string) error
	Exists(ctx context.Context, name string) (bool, error)
	// List calls fn for every stored cover file (not the marker).
	List(ctx context.Context, fn func(name string, modTime time.Time) error) error
	// EnsureMarker creates the marker file when missing and reports whether it
	// had to be created.
	EnsureMarker(ctx context.Context) (created bool, err error)
}
