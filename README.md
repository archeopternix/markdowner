# markdowner

`markdowner` is a Go library for importing source documents (for example Markdown and DOCX) into a normalized `Document` model and storing them in a filesystem-backed document store.

## Requirements

- Go `1.25.5+`
- Optional: `pandoc` on `PATH` if you use the DOCX importer (`importers/docx`)

## Install

```bash
go get github.com/archeopternix/markdowner
```

## Quick Start

```go
package main

import (
	"context"
	"fmt"

	. "github.com/archeopternix/markdowner"
	"github.com/archeopternix/markdowner/importers/docx"
	"github.com/archeopternix/markdowner/importers/markdown"
	"github.com/archeopternix/markdowner/store/localstore"
)

func main() {
	ctx := context.Background()

	rwfs := localstore.NewLocalStoreFS("./.store")
	store := NewDocumentStore(rwfs)

	store.Importers().Register(markdown.New())
	store.Importers().Register(docx.New()) // requires pandoc

	doc, err := store.ParseFromPath(ctx, "testdata/sample.md")
	if err != nil {
		panic(err)
	}

	fmt.Println(doc.ID)
	fmt.Println(doc.String())
}
```

## Public API

### Package `github.com/archeopternix/markdowner`

- `type Document`
  - Fields: `ID`, `Frontmatter`, `Markdown`, `Media`
  - Method: `String() string`
- `type Frontmatter`
  - Methods: `EncodeYAML() ([]byte, error)`, `DecodeYAML([]byte) error`
- `type ImportSource`
  - Fields: `Reader io.ReadCloser`, `Name`, `Size`, `MimeType`, `ModTime`
- `type ReaderFS`, `WriterFS`, `ReaderWriterFS`
- `type DocumentStore`
  - `GetByID(ctx, id)`
  - `SaveOrUpdate(ctx, doc)`
  - `SaveMedia(ctx, docID, mediaName, content)`
  - `List(ctx)`
  - `ParseFromPath(ctx, path)`
  - `Parse(ctx, src)`
  - `Importers()`
  - `RegisterUpdateHandler(fn)`
- `type Importer`, `type ImporterRegistry`
- `func NewDocumentStore(rwfs ReaderWriterFS) DocumentStore`

### Package `github.com/archeopternix/markdowner/store/localstore`

- `func NewLocalStoreFS(root string) *LocalStoreFS`
- `LocalStoreFS` implements `markdowner.ReaderWriterFS`

### Package `github.com/archeopternix/markdowner/importers/markdown`

- `func New() *MarkdownImporter`

### Package `github.com/archeopternix/markdowner/importers/docx`

- `func New() *DOCXImporter`

## Usage Notes

- Register at least one importer before calling `Parse` or `ParseFromPath`.
- `Parse` closes `ImportSource.Reader` internally.
- Document data is stored in this layout under your store root:
  - `<docID>/front.md`
  - `<docID>/root.md`
  - `<docID>/media/*`

## Parse From Reader Example

```go
f, err := os.Open("sample.md")
if err != nil {
	panic(err)
}

info, err := f.Stat()
if err != nil {
	panic(err)
}

doc, err := store.Parse(ctx, markdowner.ImportSource{
	Reader:   f,
	Name:     "sample.md",
	Size:     info.Size(),
	MimeType: "text/markdown",
	ModTime:  info.ModTime(),
})
if err != nil {
	panic(err)
}
```
