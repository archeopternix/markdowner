package main

import (
	"context"
	"log/slog"
	"os"

	. "github.com/archeopternix/markdowner"

	expdocx "github.com/archeopternix/markdowner/exporters/docx"
	expmd "github.com/archeopternix/markdowner/exporters/markdown"
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
	store.Exporters().Register(expmd.New())
	store.Exporters().Register(expdocx.New())

	samplePath := "testdata/strategy.docx"

	doc, err := store.ParseFromPath(ctx, samplePath)
	if err != nil {
		slog.Error("parse strategy.docx:" + err.Error())
		os.Exit(1)
	}

	wc, err := os.Create("output.docx") // io.WriteCloser
	if err != nil {
		slog.Error("create output.docx:" + err.Error())
		os.Exit(1)
	}
	defer wc.Close()

	store.Export(ctx, doc, wc, "application/msword")

	//store.Delete(ctx, doc.ID)
}
