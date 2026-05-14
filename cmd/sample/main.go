package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/archeopternix/markdowner"

	"github.com/archeopternix/markdowner/importers/docx"
	"github.com/archeopternix/markdowner/importers/markdown"
	"github.com/archeopternix/markdowner/store/localstore"
)

func main() {
	ctx := context.Background()
	storeRoot := "./.sample-store"

	rwfs := localstore.NewLocalStoreFS(storeRoot)
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
