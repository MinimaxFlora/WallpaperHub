package store

import (
	"context"
	"os"
)

// LocalStore serves images from a directory on disk.
type LocalStore struct {
	root string
}

// NewLocal returns a store rooted at dir.
func NewLocal(dir string) *LocalStore {
	return &LocalStore{root: dir}
}

// Open opens the file at relPath inside the root directory.
func (s *LocalStore) Open(_ context.Context, relPath string) (Opened, error) {
	full, err := safeJoin(s.root, relPath)
	if err != nil {
		return Opened{}, ErrNotFound
	}
	if _, err := os.Stat(full); err != nil {
		return Opened{}, ErrNotFound
	}
	// Local files are always a cache hit: no network is involved.
	return openFile(full, true)
}
