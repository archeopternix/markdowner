package markdowner

import (
	"context"
	"io"
	"io/fs"
	"time"
)

// Document is the normalized document model stored by markdowner.
type Document struct {
	ID          string // UUID
	Frontmatter Frontmatter
	Markdown    string
	Media       []string // media base names
	Path        string   // full path of the document including the folder name (ID)
}

// Frontmatter represents metadata fields serialized to front.md.
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
}

// ReaderFS is the read-only filesystem abstraction.
type ReaderFS interface {
	fs.FS
}

// WriterFS is the write-capable filesystem abstraction.
type WriterFS interface {
	MkdirAll(path string, perm fs.FileMode) error
	RemoveAll(path string) error
	Create(name string, perm fs.FileMode) (io.WriteCloser, error)
}

// ReaderWriterFS combines read/write access plus document-oriented listing behavior.
type ReaderWriterFS interface {
	ReaderFS
	WriterFS

	// ListAll lists only documents (IDs).
	ListAll(path string) ([]string, error)

	// ListMedia lists media names for a document.
	ListMedia(docID string) ([]string, error)

	// Root returns the root path of the storage.
	Root() string
}

// ImportSource describes an incoming document source to parse/import.
type ImportSource struct {
	Reader   io.ReadCloser
	Name     string
	Size     int64
	MimeType string
	ModTime  time.Time
}

// DocumentStore is the service contract for document persistence and conversion.
type DocumentStore interface {
	GetByID(ctx context.Context, id string) (*Document, error)
	SaveOrUpdate(ctx context.Context, doc *Document) error
	SaveMedia(ctx context.Context, docID string, mediaName string, content io.Reader) error
	List(ctx context.Context) ([]string, error)
	ParseFromPath(ctx context.Context, p string) (*Document, error)
	Parse(ctx context.Context, src ImportSource) (*Document, error)
	Export(ctx context.Context, doc *Document, writer io.WriteCloser, mimeType string) error
	Delete(ctx context.Context, id string) error
	Importers() ImporterRegistry
	Exporters() ExporterRegistry
	RegisterUpdateHandler(func(ctx context.Context, docID string) error)
}

// Importer imports source content into a normalized Document.
type Importer interface {
	Name() string
	Accept(ctx context.Context, src ImportSource) bool
	Import(ctx context.Context, store DocumentStore, src ImportSource) (*Document, error)
}

// Exporter exports a normalized Document into a target format.
type Exporter interface {
	Name() string
	Accept(ctx context.Context, mimeType string) bool
	Export(ctx context.Context, doc *Document, writer io.WriteCloser) error
}

// ImporterRegistry manages available importers.
type ImporterRegistry interface {
	Register(Importer)
	List() []Importer
}

// ExporterRegistry manages available exporters.
type ExporterRegistry interface {
	Register(Exporter)
	List() []Exporter
}

// SearchResult is a single search hit in a document.
type SearchResult struct {
	ID      string // Document ID
	Snippet string
	Link    string // Anchor link into the document
}

// Search defines a text search capability over stored documents.
type Search interface {
	Find(ctx context.Context, text string, numResults int) ([]SearchResult, error)
	UpdateHandler() func(ctx context.Context, docID string) error
}
