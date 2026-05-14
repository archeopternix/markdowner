package localstore

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// LocalStoreFS is a simple implementation of ReaderWriterFS that operates on the local filesystem under a specified root directory.
// It ensures all operations are confined to the root and provides basic file and directory management for document storage.
type LocalStoreFS struct {
	root string
}

func NewLocalStoreFS(root string) *LocalStoreFS {
	return &LocalStoreFS{root: root}
}

// ReaderWriterFS implementation for LocalStoreFS.

// Open opens a file for reading. The name is relative to the root directory.
// It returns an error if the file does not exist or if the path is invalid.
func (l *LocalStoreFS) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(l.root, filepath.Clean(name)))
}

func (l *LocalStoreFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(filepath.Join(l.root, filepath.Clean(path)), perm)
}

func (l *LocalStoreFS) RemoveAll(path string) error {
	return os.RemoveAll(filepath.Join(l.root, filepath.Clean(path)))
}

func (l *LocalStoreFS) Create(name string, perm fs.FileMode) (io.WriteCloser, error) {
	full := filepath.Join(l.root, filepath.Clean(name))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(full, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (l *LocalStoreFS) ListAll(path string) ([]string, error) {
	root := filepath.Join(l.root, filepath.Clean(path))
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	ids := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			ids = append(ids, e.Name())
		}
	}
	return ids, nil
}

func (l *LocalStoreFS) ListMedia(docID string) ([]string, error) {
	dir := filepath.Join(l.root, filepath.Clean(docID), "media")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
