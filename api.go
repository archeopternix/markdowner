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
}

type ImportSource struct {
	Reader   io.Reader
	Name     string
	Size     int64
	MimeType string
	ModTime  time.Time
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
}

// -----------------------------
// Document store
// -----------------------------

type DocumentStore interface {
	GetByID(ctx context.Context, id string) (*Document, error)

	// If doc.ID is empty a new document will be created.
	SaveOrUpdate(ctx context.Context, doc *Document) error

	List(ctx context.Context) ([]string, error)

	// Parse imports a new document into the Store from different formats.
	Parse(ctx context.Context, src ImportSource) (*Document, error)

	// Public access to importers.
	Importers() ImporterRegistry

	// Update callbacks (e.g. search indexing).
	RegisterUpdateHandler(func(ctx context.Context, docID string) error)
}

// -----------------------------
// Importers
// -----------------------------

type Importer interface {
	Name() string
	Accept(ctx context.Context, src ImportSource) bool
	Import(ctx context.Context, store DocumentStore, src ImportSource) (*Document, error)
}

type ImporterRegistry interface {
	Register(Importer)
	List() []Importer
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
