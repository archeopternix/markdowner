package markdowner

import (
	"context"
	"io"
	"io/fs"
	"time"
)

// -----------------------------
// Core domain types
// -----------------------------

type Document struct {
	ID          string // UUID
	Frontmatter Frontmatter
	Markdown    string
	Media       []string // media base names
	Path        string   //full path of the document including the folder name (ID)
}

type Frontmatter struct {
	Author           string
	Title            string
	Subtitle         string
	Date             string
	ChangedDate      string
	OriginalDocument string
	OriginalFormat   string
	Version          string
	Language         string
	Abstract         string
	Keywords         []string
}

// -----------------------------
// Storage FS abstractions
// -----------------------------

type ReaderFS interface {
	fs.FS
}

type WriterFS interface {
	MkdirAll(path string, perm fs.FileMode) error
	RemoveAll(path string) error
	Create(name string, perm fs.FileMode) (io.WriteCloser, error)
}

type ReaderWriterFS interface {
	ReaderFS
	WriterFS

	// Lists only documents (IDs).
	ListAll(path string) ([]string, error)

	// Lists media names for a document.
	ListMedia(docID string) ([]string, error)

	// Root returns the root path of the RWFS. This can be used by importers and exporters to construct media paths.
	Root() string
}

// -----------------------------
// Document store
// -----------------------------

type DocumentStore interface {
	GetByID(ctx context.Context, id string) (*Document, error)

	// If doc.ID is empty a new document will be created.
	SaveOrUpdate(ctx context.Context, doc *Document) error

	// SaveMedia saves media content for a document. The mediaName is a base name (e.g. "image.png") and should be unique within the document.
	SaveMedia(ctx context.Context, docID string, mediaName string, content io.Reader) error

	// List returns a list of all document IDs.
	List(ctx context.Context) ([]string, error)

	// ParseFromPath parses a document from a file path. The path can be a local file or a URL.
	ParseFromPath(ctx context.Context, p string) (*Document, error)

	// Parse imports a new document into the Store from different formats.
	Parse(ctx context.Context, src ImportSource) (*Document, error)

	// If doc.ID is empty a new document will be created.
	Export(ctx context.Context, doc *Document, writer io.WriteCloser, mimeType string) error

	// Delete removes a document and all its media.
	Delete(ctx context.Context, id string) error

	// Public access to importers and exporters.
	Importers() ImporterRegistry

	// Public access to importers and exporters.
	Exporters() ExporterRegistry

	// Update callbacks (e.g. search indexing).
	RegisterUpdateHandler(func(ctx context.Context, docID string) error)
}

// -----------------------------
// Importers / Exporters
// -----------------------------

type ImportSource struct {
	Reader   io.ReadCloser
	Name     string
	Size     int64
	MimeType string
	ModTime  time.Time
}

type Importer interface {
	Name() string
	Accept(ctx context.Context, src ImportSource) bool
	Import(ctx context.Context, store DocumentStore, src ImportSource) (*Document, error)
}

type Exporter interface {
	Name() string
	Accept(ctx context.Context, mimeType string) bool
	Export(ctx context.Context, doc *Document, writer io.WriteCloser) error
}

type ImporterRegistry interface {
	Register(Importer)
	List() []Importer
}

type ExporterRegistry interface {
	Register(Exporter)
	List() []Exporter
}

// -----------------------------
// Search
// -----------------------------

type SearchResult struct {
	ID      string // Document ID
	Snippet string
	Link    string // Anchor link into the document
}

type Search interface {
	Find(ctx context.Context, text string, numResults int) ([]SearchResult, error)
	UpdateHandler() func(ctx context.Context, docID string) error
}
