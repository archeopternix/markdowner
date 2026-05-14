package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

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

	doc, err := store.ParseFromPath(ctx, samplePath)
	if err != nil {
		slog.Error("parse sample.md:" + err.Error())
		os.Exit(1)
	}

	fmt.Println(string(doc.String()))
}
