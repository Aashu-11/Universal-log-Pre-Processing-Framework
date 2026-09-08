package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Local stores objects as plain files under a root directory, using the key
// (with "/" as the path separator) as the relative path. Used for local
// development and every test in this repo, so the Raw Vault's logic is
// fully exercised without Docker/MinIO.
type Local struct {
	root string
}

// NewLocal returns a Local store rooted at dir, creating it if needed.
func NewLocal(dir string) (*Local, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("store: create root %q: %w", dir, err)
	}
	return &Local{root: dir}, nil
}

func (l *Local) path(key string) string {
	clean := filepath.FromSlash(strings.TrimPrefix(key, "/"))
	return filepath.Join(l.root, clean)
}

func (l *Local) Put(_ context.Context, key string, data []byte) error {
	p := l.path(key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("store: mkdir for %q: %w", key, err)
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("store: write %q: %w", key, err)
	}
	if err := os.Rename(tmp, p); err != nil {
		return fmt.Errorf("store: finalize %q: %w", key, err)
	}
	return nil
}

func (l *Local) Get(_ context.Context, key string) ([]byte, error) {
	b, err := os.ReadFile(l.path(key))
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("store: get %q: %w", key, ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("store: get %q: %w", key, err)
	}
	return b, nil
}

func (l *Local) List(_ context.Context, prefix string) ([]string, error) {
	base := l.path(prefix)
	var keys []string

	root := base
	if info, err := os.Stat(base); err != nil || !info.IsDir() {
		root = filepath.Dir(base)
	}

	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(l.root, p)
		if err != nil {
			return err
		}
		key := filepath.ToSlash(rel)
		if strings.HasPrefix(key, strings.TrimPrefix(prefix, "/")) {
			keys = append(keys, key)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("store: list %q: %w", prefix, err)
	}
	sort.Strings(keys)
	return keys, nil
}
