# Simplified markdowner concept (v3)

This document defines a simplified `markdowner` concept around the given types and interfaces, updated with the following decisions:

1. `ListAll` lists only document IDs; add a second method for media listing.
2. Split read/write APIs.
3. `front.md` and `root.md` are both mandatory.
4. Expose importer registry publicly.
5. Update handlers use: `RegisterUpdateHandler(func(context.Context, docID string) error)`.
6. Search uses: `Find(ctx, text, numResults) ([]SearchResult, error)`.

---

## 1) Filesystem abstraction

### 1.1 Reader FS

```go
type ReaderFS interface {
    fs.FS
}
```

### 1.2 Writer FS

```go
type WriterFS interface {
    MkdirAll(path string, perm fs.FileMode) error
    RemoveAll(path string) error

    // Create/overwrite file and return a writer.
    Create(name string, perm fs.FileMode) (io.WriteCloser, error)
}
```

### 1.3 Store FS (combined)

```go
type ReaderWriterFS interface {
    ReaderFS
    WriterFS

    // Lists only documents (IDs).
    ListAll(path string) ([]string, error)

    // Lists media names for a document.
    ListMedia(docID string) ([]string, error)
}
```

Semantics:
- `ListAll(path)` returns document IDs only.
- `ListMedia(docID)` returns media base names only (e.g. `[]string{"image1.png"}`), corresponding to files stored under `/<docID>/media/`.

---

## 2) Domain model

```go
type Document struct {
    ID          string // UUID
    Frontmatter Frontmatter
    Markdown    Markdown
    Media       []string // media base names
}
```

---

## 3) Disk layout

A document is stored under a directory named by its `Document.ID`.

```text
/<docID>/
  root.md       // markdown body only (mandatory)
  front.md      // YAML frontmatter only (mandatory)
  media/
    <name>      // extracted media files
```

Rules:
- Both `front.md` and `root.md` are mandatory.
- `root.md` MUST NOT contain a YAML frontmatter block.
- `front.md` MUST contain YAML frontmatter only.

---

## 4) Importers (public registry)

### 4.1 Importer interface

Importers convert a source into the store’s canonical layout (front/root + media extraction).

```go
type Importer interface {
    Name() string

    // Accept decides whether the importer can handle a given source.
    // Sniffing strategy is implementation-defined.
    Accept(ctx context.Context, src ImportSource) bool

    // Import imports a new document into the store.
    Import(ctx context.Context, store DocumentStore, src ImportSource) (*Document, error)
}
```

### 4.2 Importer registry

```go
type ImporterRegistry interface {
    Register(Importer)
    List() []Importer
}
```

The `DocumentStore` exposes the registry publicly.

---

## 5) DocumentStore

### 5.1 Interface

```go
type DocumentStore interface {
    GetByID(ctx context.Context, id string) (*Document, error)

    // If doc.ID is empty a new document will be created.
    SaveOrUpdate(ctx context.Context, doc *Document) error

    List(ctx context.Context) ([]string, error)

    // Parse imports a new document into the Store from different formats (e.g. docx).
    // During this process media files will be extracted from the source and saved on disk.
    Parse(ctx context.Context, src ImportSource) (*Document, error)

    // Public access to importers
    Importers() ImporterRegistry

    // Update callbacks (e.g. search indexing)
    RegisterUpdateHandler(func(context.Context, docID string) error)
}

func NewDocumentStore(rwfs ReaderWriterFS) DocumentStore {}
```

Notes:
- `Parse` now takes an `ImportSource` stream rather than `fs.FS`, so format-specific importers can operate on the original payload and extract media.

### 5.2 List

`List(ctx)` returns document IDs by delegating to `ReaderWriterFS.ListAll(...)`.

### 5.3 GetByID

`GetByID(ctx, id)` loads a stored document by reading:
- `/<id>/front.md` (mandatory)
- `/<id>/root.md` (mandatory)
- media list via `ReaderWriterFS.ListMedia(id)`

On missing `front.md` or `root.md`, `GetByID` must return an error.

### 5.4 SaveOrUpdate

`SaveOrUpdate(ctx, doc)` persists `doc` to the canonical layout.

Required behavior:
- If `doc.ID == ""`, generate a new UUID, assign it to `doc.ID`, create the document directory, and write the files.
- If `doc.ID != ""`, write updates to the existing directory.

Write mapping:
- `/<docID>/front.md` is written from `doc.Frontmatter` (YAML only).
- `/<docID>/root.md` is written from `doc.Markdown` (body only).

Update handlers:
- After a successful `SaveOrUpdate`, invoke all registered update handlers with `(ctx, doc.ID)`.
- If a handler returns an error, `SaveOrUpdate` must return an error.

### 5.5 Parse (import)

`Parse(ctx, src)` imports a new document into the store.

Required semantics:
- Select an importer from the public importer registry.
- Importer must:
  - create a new document ID (or call into store to allocate one)
  - extract and write `front.md` and `root.md`
  - extract and write media files under `/<docID>/media/`

Return value:
- Returns the stored `*Document` (with its assigned `ID` and populated `Media` list).

Update handlers:
- After a successful `Parse`, invoke all registered update handlers with `(ctx, doc.ID)`.
- If a handler returns an error, `Parse` must return an error.

---

## 6) Search

### 6.1 Types

```go
type SearchResult struct {
    ID      string // Document ID
    Snippet string
    Link    string // Anchor link into the document
}
```

### 6.2 Interface

```go
type Search interface {
    Find(ctx context.Context, text string, numResults int) ([]SearchResult, error)

    // Search provides a callback to be registered in DocumentStore.
    UpdateHandler() func(context.Context, docID string) error
}

func NewSearch(rwfs ReaderWriterFS) Search {}
```

### 6.3 Registration

The search callback is registered via:

```go
store.RegisterUpdateHandler(search.UpdateHandler())
```

The handler is invoked by `DocumentStore` after successful:
- `SaveOrUpdate`
- `Parse`

---

## 7) Construction / wiring

Typical wiring:

1. Create store:
   - `store := NewDocumentStore(rwfs)`

2. Register importers:
   - `store.Importers().Register(...)`

3. Create search:
   - `search := NewSearch(rwfs)`

4. Register search update handler:
   - `store.RegisterUpdateHandler(search.UpdateHandler())`

---

## 8) Invariants

- Document directories are keyed by `Document.ID`.
- `front.md` and `root.md` are both mandatory.
- `front.md` contains YAML only.
- `root.md` contains Markdown body only.
- `ReaderWriterFS.ListAll` lists only document IDs.
- `ReaderWriterFS.ListMedia` lists media base names.
- `DocumentStore` invokes registered update handlers after successful store mutations.


---

## 9) Go project structure (lean public API)

This section proposes a Go project structure for this v2 concept with a **lean public API** and minimal root-level files.

### 9.1 Project layout

```text
markdowner/
  go.mod

  markdowner.go              // package markdowner: NewDocumentStore, minimal entrypoints
  api.go                  // package markdowner: ALL exported types + interfaces (merged)

  internal/
    docstore/
      store.go            // concrete DocumentStore implementation
      list.go             // List() uses rwfs.ListAll
      get.go              // GetByID() reads front.md/root.md + rwfs.ListMedia
      save.go             // SaveOrUpdate() incl. allocate ID if empty; mandatory file enforcement
      parse.go            // Parse() selects importer, runs import, persists, calls update handlers
      handlers.go         // RegisterUpdateHandler + invocation ordering/error handling
      importers.go        // importer registry implementation returned by Importers()
      paths.go            // canonical paths: <id>/front.md, <id>/root.md, <id>/media/<name>

    format/
      frontmatter_yaml.go // encode/decode Frontmatter <-> YAML (front.md)
      markdown_body.go    // read/write Markdown body only (root.md)

    fsutil/
      validate.go         // docID validation, media-name validation, traversal guards
      io.go               // readfile/writefile helpers using split read/write APIs
      tempwrite.go        // safe write patterns using WriterFS.Create

  importers/              // PUBLIC: built-in importers (plugin-friendly)
    registry/             // optional helper registry implementation
      registry.go
    markdown/
      importer.go
    docx/
      importer.go
      pandoc.go
    pptx/
      importer.go
      pptx2md.go
    zip/
      importer.go
      zipread.go

  search/                 // PUBLIC: search API + reference implementation
    api.go                // Search interface
    basic/
      search.go
      index.go
      handler.go
      storage.go
```

### 9.2 Public API design notes

- `markdowner/api.go` merges the exported type and interface definitions into a single file to keep the root package small.
- Built-in importers are in `importers/*` so end users can register them explicitly or replace them.
- Search is pluggable; the reference implementation lives under `search/basic`, and can be replaced by any other implementation that provides a compatible update handler.
