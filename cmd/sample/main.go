package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/archeopternix/markdowner/app/bootstrap"
	"github.com/archeopternix/markdowner/store/localstore"
)

func main() {
	ctx := context.Background()
	storeRoot := "./.sample-store"

	rwfs := localstore.NewLocalStoreFS(storeRoot)
	store := bootstrap.NewDocumentStore(rwfs)

	samplePath := "testdata/sap.md"

	doc, err := store.ParseFromPath(ctx, samplePath)
	if err != nil {
		slog.Error("Parse", "file", samplePath, "error", err.Error())
		os.Exit(1)
	}

	outfile := "test.md"
	wc, err := os.Create(outfile) // io.WriteCloser
	if err != nil {
		slog.Error("Output", "file", outfile, "error", err.Error())
		os.Exit(1)
	}
	defer wc.Close()

	store.Export(ctx, doc, wc, "markdown")
	slog.Info("Exported", "file", outfile, "format", "markdown")
	//store.Delete(ctx, doc.ID)
}
