package coverstore

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FSStore stores covers on the local filesystem under root.
type FSStore struct {
	root string
}

// NewFS returns a filesystem store. root is created on demand by Put and
// EnsureMarker; reading never creates anything.
func NewFS(root string) *FSStore { return &FSStore{root: root} }

func (s *FSStore) path(name string) (string, error) {
	if !ValidName(name) {
		return "", ErrInvalidName
	}
	return filepath.Join(s.root, filepath.FromSlash(name)), nil
}

func (s *FSStore) Put(_ context.Context, name string, data []byte) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, p); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	return nil
}

func (s *FSStore) Open(_ context.Context, name string) (io.ReadSeekCloser, Info, error) {
	p, err := s.path(name)
	if err != nil {
		return nil, Info{}, ErrNotFound
	}
	f, err := os.Open(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, Info{}, ErrNotFound
		}
		return nil, Info{}, err
	}
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		_ = f.Close()
		return nil, Info{}, ErrNotFound
	}
	return f, Info{Size: st.Size(), ModTime: st.ModTime()}, nil
}

func (s *FSStore) Delete(_ context.Context, name string) error {
	p, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (s *FSStore) Exists(_ context.Context, name string) (bool, error) {
	p, err := s.path(name)
	if err != nil {
		return false, err
	}
	st, err := os.Stat(p)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	return !st.IsDir(), nil
}

func (s *FSStore) List(ctx context.Context, fn func(name string, modTime time.Time) error) error {
	shards, err := os.ReadDir(s.root)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, sh := range shards {
		if !sh.IsDir() || len(sh.Name()) != 2 {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(s.root, sh.Name()))
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := ctx.Err(); err != nil {
				return err
			}
			name := sh.Name() + "/" + e.Name()
			if e.IsDir() || strings.HasPrefix(e.Name(), ".tmp-") || !ValidName(name) {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			if err := fn(name, info.ModTime()); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *FSStore) EnsureMarker(_ context.Context) (bool, error) {
	p := filepath.Join(s.root, MarkerName)
	if _, err := os.Stat(p); err == nil {
		return false, nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return false, err
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return false, err
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return false, nil
		}
		return false, err
	}
	return true, f.Close()
}
