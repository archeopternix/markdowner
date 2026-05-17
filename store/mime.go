package store

import (
	"io"

	md "github.com/archeopternix/markdowner"
)

// MimeDetector abstracts MIME detection for ParseFromPath.
// The concrete adapter is injected at composition time.
type MimeDetector interface {
	Detect(path string, r io.Reader) (string, error)
}

// docstore.go is a single-file reference implementation that groups the following topics:
// - concrete md.DocumentStore implementation
// - List() using rwfs.ListAll
// - GetByID() reading front.md/root.md + rwfs.ListMedia
// - SaveOrUpdate() incl. allocate ID if empty; mandatory file enforcement
// - Parse() selecting importer, importing, persisting, calling update handlers
// - RegisterUpdateHandler() + invocation ordering/error handling
// - importer registry implementation returned by Importers()
// - canonical paths: <id>/front.md, <id>/root.md, <id>/media/<name>

// New constructs a concrete md.DocumentStore backed by the provided ReaderWriterFS.
//
// This implementation expects:
// - rwfs.ListAll(".") (or "/") to return document IDs only
// - rwfs.ListMedia(docID) to return base filenames only
// - rwfs.Create(name, perm) to create/overwrite files
func New(rwfs md.ReaderWriterFS, mimeDetector MimeDetector) md.DocumentStore {
	if mimeDetector == nil {
		mimeDetector = noopMimeDetector{}
	}
	return &documentStore{
		rwfs:         rwfs,
		mimeDetector: mimeDetector,
		imps:         &importerRegistry{},
		exps:         &exporterRegistry{},
		handlers:     nil,
		listPath:     ".",
		indexPath:    ".", // unused here; reserved for callers
	}
}

type noopMimeDetector struct{}

func (noopMimeDetector) Detect(path string, r io.Reader) (string, error) {
	_ = path
	_ = r
	return "", nil
}
