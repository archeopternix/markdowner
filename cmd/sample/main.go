package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	. "github.com/archeopternix/markdowner"

	"github.com/archeopternix/markdowner/importers/docx"
	"github.com/archeopternix/markdowner/importers/markdown"
)

func main() {
	ctx := context.Background()
	storeRoot := "./.sample-store"

	rwfs := &localStoreFS{root: storeRoot}
	store := NewDocumentStore(rwfs)
	store.Importers().Register(markdown.New())
	store.Importers().Register(docx.New())

	samplePath := "testdata/strategy.docx"
	f, info, err := open(samplePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read %s: %v\n", samplePath, err)
		os.Exit(1)
	}
	defer f.Close()

	doc, err := store.Parse(ctx, ImportSource{
		Reader:   f,
		Name:     filepath.Base(samplePath),
		Size:     info.Size(),
		MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
		ModTime:  info.ModTime(),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse sample.md: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(doc.String()))
}

// -----------------------------

type localStoreFS struct {
	root string
}

func (l *localStoreFS) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(l.root, filepath.Clean(name)))
}

func (l *localStoreFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(filepath.Join(l.root, filepath.Clean(path)), perm)
}

func (l *localStoreFS) RemoveAll(path string) error {
	return os.RemoveAll(filepath.Join(l.root, filepath.Clean(path)))
}

func (l *localStoreFS) Create(name string, perm fs.FileMode) (io.WriteCloser, error) {
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

func (l *localStoreFS) ListAll(path string) ([]string, error) {
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

func (l *localStoreFS) ListMedia(docID string) ([]string, error) {
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

// open is a helper that opens a file and returns its handle and info, ensuring the handle is closed on error.
func open(p string) (*os.File, os.FileInfo, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, nil, err
	}
	return f, info, nil
}
