package markdowner

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/archeopternix/markdowner/store"
)

// docstore.go is a single-file reference implementation that groups the following topics:
// - concrete DocumentStore implementation
// - List() using rwfs.ListAll
// - GetByID() reading front.md/root.md + rwfs.ListMedia
// - SaveOrUpdate() incl. allocate ID if empty; mandatory file enforcement
// - Parse() selecting importer, importing, persisting, calling update handlers
// - RegisterUpdateHandler() + invocation ordering/error handling
// - importer registry implementation returned by Importers()
// - canonical paths: <id>/front.md, <id>/root.md, <id>/media/<name>

// NewDocumentStore constructs a concrete DocumentStore backed by the provided ReaderWriterFS.
//
// This implementation expects:
// - rwfs.ListAll(".") (or "/") to return document IDs only
// - rwfs.ListMedia(docID) to return base filenames only
// - rwfs.Create(name, perm) to create/overwrite files
func NewDocumentStore(rwfs ReaderWriterFS) DocumentStore {
	return &documentStore{
		rwfs:      rwfs,
		imps:      &importerRegistry{},
		handlers:  nil,
		listPath:  ".",
		indexPath: ".", // unused here; reserved for callers
	}
}

type documentStore struct {
	rwfs ReaderWriterFS

	imps     *importerRegistry
	handlers []func(context.Context, string) error

	// listPath is passed to rwfs.ListAll. Keep configurable if some RWFS expects "/".
	listPath string

	// indexPath reserved.
	indexPath string
}

func (d documentStore) SaveMedia(ctx context.Context, docID string, mediaName string, content io.Reader) error {
	_ = ctx
	if err := validateDocID(docID); err != nil {
		return err
	}
	if err := validateMediaName(mediaName); err != nil {
		return err
	}

	wc, err := d.rwfs.Create(mediaPath(docID, mediaName), 0o644)
	if err != nil {
		return err
	}
	defer wc.Close()
	_, err = io.Copy(wc, content)
	if err != nil {
		return err
	}
	return err
}

// -----------------------------
// paths.go (canonical paths)
// -----------------------------

func docDir(docID string) string { return cleanJoin(docID) }

func frontPath(docID string) string { return cleanJoin(docID, "front.md") }

func rootPath(docID string) string { return cleanJoin(docID, "root.md") }

func mediaDir(docID string) string { return cleanJoin(docID, "media") }

func mediaPath(docID, name string) string { return cleanJoin(docID, "media", name) }

// cleanJoin joins with forward slashes and cleans. It must never return absolute paths.
func cleanJoin(parts ...string) string {
	p := path.Clean(path.Join(parts...))
	p = strings.TrimPrefix(p, "/")
	if p == "." {
		return ""
	}
	return p
}

// validateDocID is intentionally minimal here.
// In production, validate UUID format and reject traversal characters.
func validateDocID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("empty document id")
	}
	if strings.Contains(id, "/") || strings.Contains(id, "\\") || strings.Contains(id, "..") {
		return fmt.Errorf("invalid document id: %q", id)
	}
	return nil
}

func validateMediaName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("empty media name")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid media name: %q", name)
	}
	return nil
}

// -----------------------------
// importers.go (registry)
// -----------------------------

type importerRegistry struct {
	list []Importer
}

func (r *importerRegistry) Register(i Importer) {
	if i == nil {
		return
	}
	r.list = append(r.list, i)
}

func (r *importerRegistry) List() []Importer {
	out := make([]Importer, 0, len(r.list))
	out = append(out, r.list...)
	return out
}

// selectImporter returns the first importer that Accepts the source.
func (r *importerRegistry) selectImporter(ctx context.Context, src ImportSource) (Importer, error) {
	for _, imp := range r.list {
		if imp == nil {
			continue
		}
		if imp.Accept(ctx, src) {
			return imp, nil
		}
	}
	return nil, fmt.Errorf("no importer accepted source %q", src.Name)
}

// -----------------------------
// handlers.go (update handlers)
// -----------------------------

func (s *documentStore) RegisterUpdateHandler(fn func(ctx context.Context, docID string) error) {
	if fn == nil {
		return
	}
	s.handlers = append(s.handlers, fn)
}

func (s *documentStore) runUpdateHandlers(ctx context.Context, docID string) error {
	for _, h := range s.handlers {
		if h == nil {
			continue
		}
		if err := h(ctx, docID); err != nil {
			return err
		}
	}
	return nil
}

// -----------------------------
// list.go
// -----------------------------

func (s *documentStore) List(ctx context.Context) ([]string, error) {
	_ = ctx
	ids, err := s.rwfs.ListAll(s.listPath)
	if err != nil {
		return nil, err
	}
	// Optional: sort for determinism
	sort.Strings(ids)
	return ids, nil
}

// -----------------------------
// get.go
// -----------------------------

func (s *documentStore) GetByID(ctx context.Context, id string) (*Document, error) {
	if err := validateDocID(id); err != nil {
		return nil, err
	}

	fmBytes, err := readAllFS(ctx, s.rwfs, frontPath(id))
	if err != nil {
		return nil, fmt.Errorf("read front.md: %w", err)
	}
	bodyBytes, err := readAllFS(ctx, s.rwfs, rootPath(id))
	if err != nil {
		return nil, fmt.Errorf("read root.md: %w", err)
	}

	doc := &Document{ID: id}
	err = doc.Frontmatter.DecodeYAML(fmBytes)
	if err != nil {
		return nil, fmt.Errorf("decode front.md: %w", err)
	}

	doc.Markdown = string(bodyBytes)

	media, err := s.rwfs.ListMedia(id)
	if err != nil {
		return nil, fmt.Errorf("list media: %w", err)
	}
	for _, m := range media {
		if err := validateMediaName(m); err != nil {
			return nil, err
		}
	}
	sort.Strings(media)

	doc.Media = media

	return doc, nil
}

// -----------------------------
// save.go
// -----------------------------

func (s *documentStore) SaveOrUpdate(ctx context.Context, doc *Document) error {
	if doc == nil {
		return errors.New("nil document")
	}

	if strings.TrimSpace(doc.ID) == "" {
		doc.ID = newUUIDLike()
	}
	if err := validateDocID(doc.ID); err != nil {
		return err
	}

	// Ensure doc directory exists
	if err := s.rwfs.MkdirAll(docDir(doc.ID), 0o755); err != nil {
		return err
	}
	if err := s.rwfs.MkdirAll(mediaDir(doc.ID), 0o755); err != nil {
		return err
	}

	front, err := doc.Frontmatter.EncodeYAML()
	if err != nil {
		return err
	}
	root := []byte(doc.Markdown)

	// Both mandatory: always write both.
	if err := writeFile(ctx, s.rwfs, frontPath(doc.ID), 0o644, front); err != nil {
		return fmt.Errorf("write front.md: %w", err)
	}
	if err := writeFile(ctx, s.rwfs, rootPath(doc.ID), 0o644, root); err != nil {
		return fmt.Errorf("write root.md: %w", err)
	}

	// Invoke update handlers
	if err := s.runUpdateHandlers(ctx, doc.ID); err != nil {
		return err
	}
	return nil
}

// Delete removes all files for the document. It does not return an error if the document does not exist.
func (s *documentStore) Delete(ctx context.Context, id string) error {
	if err := validateDocID(id); err != nil {
		return err
	}
	return s.rwfs.RemoveAll(docDir(id))
}

// -----------------------------
// parse.go
// -----------------------------

func (s *documentStore) ParseFromPath(ctx context.Context, p string) (*Document, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	// file stats
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	// MIME type detection
	mime, err := store.DetectMime(p, f) // best effort; importers can also guess based on content
	if err != nil {
		_ = f.Close()
		return nil, err
	}

	f.Close()

	// new file reader for importers, since DetectMime may have read some bytes
	fzero, err := os.Open(p)
	if err != nil {
		return nil, err
	}

	src := ImportSource{
		Reader:   fzero,
		Name:     path.Base(p),
		Size:     info.Size(),
		MimeType: mime.MimeType, // optional; importers can guess based on name or content
		ModTime:  info.ModTime(),
	}
	return s.Parse(ctx, src)
}

func (s *documentStore) Parse(ctx context.Context, src ImportSource) (*Document, error) {
	if src.Reader == nil {
		return nil, errors.New("source reader is nil")
	}
	defer src.Reader.Close()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	imp, err := s.imps.selectImporter(ctx, src)
	if err != nil {
		return nil, err
	}

	slog.Info("Parse file", "source", src.Name, "mime", src.MimeType, "importer", imp.Name())

	doc, err := imp.Import(ctx, s, src)
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errors.New("importer returned nil document")
	}
	if strings.TrimSpace(doc.ID) == "" {
		return nil, errors.New("importer returned empty document ID")
	}

	// Ensure mandatory files exist by performing a GetByID read.
	// This also normalizes Media listing.
	stored, err := s.GetByID(ctx, doc.ID)
	if err != nil {
		return nil, err
	}

	// Update handlers
	if err := s.runUpdateHandlers(ctx, stored.ID); err != nil {
		return nil, err
	}
	return stored, nil
}

// Importers returns the registry for registering external importers.
func (s *documentStore) Importers() ImporterRegistry { return s.imps }

// -----------------------------
// helpers: IO
// -----------------------------

func readAllFS(ctx context.Context, rfs fs.FS, name string) ([]byte, error) {
	_ = ctx
	f, err := rfs.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

func writeFile(ctx context.Context, wfs WriterFS, name string, perm fs.FileMode, b []byte) error {
	_ = ctx
	w, err := wfs.Create(name, perm)
	if err != nil {
		return err
	}
	bw := bufio.NewWriter(w)
	_, werr := bw.Write(b)
	errFlush := bw.Flush()
	errClose := w.Close()
	if werr != nil {
		return werr
	}
	if errFlush != nil {
		return errFlush
	}
	return errClose
}

// newUUIDLike generates a random 16-byte hex string.
// This is not a canonical UUID string but is sufficient as a unique doc ID placeholder.
// Replace with a real UUID generator if you want canonical UUID formatting.
func newUUIDLike() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
