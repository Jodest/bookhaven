package coverstore

import (
	"bytes"
	"context"
	"io"
	"sort"
	"sync"
	"time"
)

type memFile struct {
	data []byte
	mod  time.Time
}

// MemStore is an in-memory Store for tests.
type MemStore struct {
	mu     sync.Mutex
	files  map[string]memFile
	marker bool
	// Now returns the modification time of new files; defaults to time.Now.
	Now func() time.Time
}

func NewMem() *MemStore { return &MemStore{files: map[string]memFile{}} }

func (s *MemStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *MemStore) Put(_ context.Context, name string, data []byte) error {
	if !ValidName(name) {
		return ErrInvalidName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.files[name] = memFile{data: append([]byte(nil), data...), mod: s.now()}
	return nil
}

type nopSeekCloser struct{ *bytes.Reader }

func (nopSeekCloser) Close() error { return nil }

func (s *MemStore) Open(_ context.Context, name string) (io.ReadSeekCloser, Info, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.files[name]
	if !ok {
		return nil, Info{}, ErrNotFound
	}
	return nopSeekCloser{bytes.NewReader(f.data)}, Info{Size: int64(len(f.data)), ModTime: f.mod}, nil
}

func (s *MemStore) Delete(_ context.Context, name string) error {
	if !ValidName(name) {
		return ErrInvalidName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.files, name)
	return nil
}

func (s *MemStore) Exists(_ context.Context, name string) (bool, error) {
	if !ValidName(name) {
		return false, ErrInvalidName
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.files[name]
	return ok, nil
}

func (s *MemStore) List(_ context.Context, fn func(name string, modTime time.Time) error) error {
	s.mu.Lock()
	names := make([]string, 0, len(s.files))
	mods := map[string]time.Time{}
	for n, f := range s.files {
		names = append(names, n)
		mods[n] = f.mod
	}
	s.mu.Unlock()
	sort.Strings(names)
	for _, n := range names {
		if err := fn(n, mods[n]); err != nil {
			return err
		}
	}
	return nil
}

func (s *MemStore) EnsureMarker(_ context.Context) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.marker {
		return false, nil
	}
	s.marker = true
	return true, nil
}

// SetModTime overrides a file's modification time (tests).
func (s *MemStore) SetModTime(name string, t time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if f, ok := s.files[name]; ok {
		f.mod = t
		s.files[name] = f
	}
}
