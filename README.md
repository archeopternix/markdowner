# markdowner

`markdowner` is a Go library for importing source documents (for example Markdown, TXT, VTT, and DOCX) into a normalized `Document` model and storing them in a filesystem-backed document store.

## Requirements

- Go `1.25.5+`
- Optional: `pandoc` on `PATH` if you use DOCX import/export (`importers/docx`, `exporters/docx`)

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

	"github.com/archeopternix/markdowner/app/bootstrap"
	"github.com/archeopternix/markdowner/app/localstore"
)

func main() {
	ctx := context.Background()

	rwfs := localstore.NewLocalStoreFS("./.store")
	store := bootstrap.NewDocumentStore(rwfs)

	doc, err := store.ParseFromPath(ctx, "testdata/sample.md")
	if err != nil {
		panic(err)
	}

	fmt.Println(doc.ID)
	fmt.Println(doc.Frontmatter.Title)
}
```

## Public API

### Package `github.com/archeopternix/markdowner/app/bootstrap`

- `func NewDocumentStore(rwfs markdowner.ReaderWriterFS) markdowner.DocumentStore`

### Package `github.com/archeopternix/markdowner`

- `type Document`
  - Fields: `ID`, `Frontmatter`, `Markdown`, `Media`, `Path`
  - Method: `String() string`
- `type Frontmatter`
  - Methods: `EncodeFrontmatterToMap() map[string]string`, `String() string`
- `func DecodeFrontmatterFromMap(map[string]string) Frontmatter`
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
  - `Export(ctx, doc, writer, mimeType)`
  - `Delete(ctx, id)`
  - `Importers()`
  - `Exporters()`
  - `RegisterUpdateHandler(fn)`
- `type Importer`, `type ImporterRegistry`, `type Exporter`, `type ExporterRegistry`
- `type Search`, `type SearchResult`

### Package `github.com/archeopternix/markdowner/app/localstore`

- `func NewLocalStoreFS(root string) *LocalStoreFS`
- `LocalStoreFS` implements `markdowner.ReaderWriterFS`



## Usage Notes

- `bootstrap.NewDocumentStore` registers built-in importers (`markdown`, `docx`, `vtt`) and exporters (`markdown`, `docx`) by default.
- `Parse` closes `ImportSource.Reader` internally.
- Document data is stored in this layout under your store root:
  - `<docID>/front.md`
  - `<docID>/root.md`
  - `<docID>/media/*`

## Parse From Reader Example

```go
package main

import (
	"context"
	"os"

	"github.com/archeopternix/markdowner"
	"github.com/archeopternix/markdowner/app/bootstrap"
	"github.com/archeopternix/markdowner/app/localstore"
)

func main() {
	ctx := context.Background()
	store := bootstrap.NewDocumentStore(localstore.NewLocalStoreFS("./.store"))

f, err := os.Open("sample.md")
if err != nil {
	panic(err)
}
defer f.Close()

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
_ = doc
}
```
