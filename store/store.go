package store

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
	"path/filepath"
	"sort"
	"strings"
	"sync"

	md "github.com/archeopternix/markdowner"
	"github.com/archeopternix/markdowner/internal/format"
	fsutils "github.com/archeopternix/markdowner/internal/fsutils"
)

type documentStore struct {
	rwfs         md.ReaderWriterFS
	mimeDetector MimeDetector

	// Mutex to prevent parallel calls to the same func
	opLocks keyMutex

	imps     *importerRegistry
	exps     *exporterRegistry
	handlers []func(context.Context, string) error

	// listPath is passed to rwfs.ListAll. Keep configurable if some RWFS expects "/".
	listPath string

	// indexPath reserved.
	indexPath string
}

// lock to prevent the same func is called twice in parallel
type keyMutex struct {
	mu sync.Mutex
	m  map[string]*sync.Mutex
}

func (k *keyMutex) Lock(key string) func() {
	k.mu.Lock()
	if k.m == nil {
		k.m = make(map[string]*sync.Mutex)
	}
	mu, ok := k.m[key]
	if !ok {
		mu = &sync.Mutex{}
		k.m[key] = mu
	}
	k.mu.Unlock()

	mu.Lock()
	return func() { mu.Unlock() }
}

// -----------------------------
// Public functions
// -----------------------------

// List returns all document IDs in the store. It relies on rwfs.ListAll returning document IDs only.
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

// GetByID returns the document with the given ID. It reads the front.md and root.md files and lists the media files.
func (s *documentStore) GetByID(ctx context.Context, id string) (*md.Document, error) {
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

	doc := &md.Document{ID: id}

	fmap, err := format.DecodeYAML(fmBytes)
	if err != nil {
		return nil, fmt.Errorf("decode front.md YAML: %w", err)
	}
	doc.Frontmatter = md.DecodeFrontmatterFromMap(fmap)
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

	slog.Debug("GetByID", "id", id, "mediaCount", len(media))

	doc.Media = media

	return doc, nil
}

// SaveOrUpdate saves the document. It creates or overwrites the front.md and root.md files and ensures mandatory files exist.
func (s *documentStore) SaveOrUpdate(ctx context.Context, doc *md.Document) error {
	if doc == nil {
		return errors.New("nil document")
	}

	unlock := s.opLocks.Lock("SaveOrUpdate:" + doc.ID)
	defer unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	if strings.TrimSpace(doc.ID) == "" {
		doc.ID = newUUIDLike()
	}
	if err := validateDocID(doc.ID); err != nil {
		return err
	}

	slog.Debug("SaveOrUpdate", "id", doc.ID)

	// Ensure doc directory exists
	if err := s.rwfs.MkdirAll(docDir(doc.ID), 0o755); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := s.rwfs.MkdirAll(mediaDir(doc.ID), 0o755); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	doc.Path = filepath.Join(s.rwfs.Root(), docDir(doc.ID))

	frontmap := doc.Frontmatter.EncodeFrontmatterToMap()
	front, err := format.EncodeYAML(frontmap)
	if err != nil {
		return err
	}
	root := []byte(doc.Markdown)

	// Both mandatory: always write both.
	if err := writeFile(ctx, s.rwfs, frontPath(doc.ID), 0o644, front); err != nil {
		return fmt.Errorf("write front.md: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := writeFile(ctx, s.rwfs, rootPath(doc.ID), 0o644, root); err != nil {
		return fmt.Errorf("write root.md: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	// Invoke update handlers
	if err := s.runUpdateHandlers(ctx, doc.ID); err != nil {
		return err
	}

	return nil
}

func (s *documentStore) Export(ctx context.Context, doc *md.Document, writer io.WriteCloser, mimeType string) error {
	if doc == nil {
		return errors.New("nil document")
	}
	if writer == nil {
		return errors.New("nil writer")
	}
	exp, err := s.exps.selectExporter(ctx, mimeType)
	if err != nil {
		return err
	}
	slog.Debug("Export", "id", doc.ID, "format", mimeType)
	return exp.Export(ctx, doc, writer)
}

// Delete removes all files for the document. It does not return an error if the document does not exist.
func (s *documentStore) Delete(ctx context.Context, id string) error {
	if err := validateDocID(id); err != nil {
		return err
	}

	slog.Debug("Delete", "id", id)
	return s.rwfs.RemoveAll(docDir(id))
}

// ParseFromPath is a convenience wrapper around Parse that takes a filesystem path, opens the file, and constructs an ImportSource.
func (s *documentStore) ParseFromPath(ctx context.Context, p string) (*md.Document, error) {
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

	// MIME type detection (adapter injected via MimeDetector).
	mimeType, err := s.mimeDetector.Detect(p, f)
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

	src := md.ImportSource{
		Reader:   fzero,
		Name:     path.Base(p),
		Size:     info.Size(),
		MimeType: mimeType, // optional; importers can guess based on name or content
		ModTime:  info.ModTime(),
	}
	return s.Parse(ctx, src)
}

// Parse reads the source, selects an importer, imports the document, saves it, and runs update handlers.
func (s *documentStore) Parse(ctx context.Context, src md.ImportSource) (*md.Document, error) {
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

	slog.Debug("Parse", "source", src.Name, "importer", imp.Name(), "mime", src.MimeType)

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

	unlock := s.opLocks.Lock("Parse:" + doc.ID)
	defer unlock()

	// Ensure mandatory files exist by performing a GetByID read.
	// This also normalizes Media listing.
	stored, err := s.GetByID(ctx, doc.ID)
	if err != nil {
		return nil, err
	}

	stored.Path = filepath.Join(s.rwfs.Root(), docDir(doc.ID))

	// Update handlers
	if err := s.runUpdateHandlers(ctx, stored.ID); err != nil {
		return nil, err
	}
	if err := s.SaveOrUpdate(ctx, stored); err != nil {
		return nil, err
	}
	return stored, nil
}

// SaveMedia saves media content for a document. It creates or overwrites the media file under <docID>/media/<mediaName>.
func (s *documentStore) SaveMedia(ctx context.Context, docID string, mediaName string, content io.Reader) error {
	unlock := s.opLocks.Lock("SaveMedia:" + docID)
	defer unlock()

	_ = ctx
	if err := validateDocID(docID); err != nil {
		return err
	}
	if err := validateMediaName(mediaName); err != nil {
		return err
	}

	// slog.Debug("SaveMedia", "id", docID, "media", mediaName)

	wc, err := s.rwfs.Create(mediaPath(docID, mediaName), 0o644)
	if err != nil {
		return err
	}
	defer wc.Close()

	// also close wc on ctx cancel to unblock writes
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = wc.Close()
		case <-done:
		}
	}()
	defer close(done)

	_, err = fsutils.CopyWithContext(ctx, wc, content)
	return err
}

// Importers returns the registry for registering external importers.
func (s *documentStore) Importers() md.ImporterRegistry {
	return s.imps
}

// Exporters returns the registry for registering external exporters.
func (s *documentStore) Exporters() md.ExporterRegistry {
	return s.exps
}

// RegisterUpdateHandler registers a handler function that is called after a document is created or updated.
// Handlers are called in the order they were registered. If any handler returns an error, the process is aborted and the error is returned.
func (s *documentStore) RegisterUpdateHandler(fn func(ctx context.Context, docID string) error) {
	if fn == nil {
		return
	}
	s.handlers = append(s.handlers, fn)
}

// runUpdateHandlers executes all registered update handlers for the given document ID.
// If any handler returns an error, it stops and returns that error.
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
// helpers path construction
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
	list []md.Importer
}

func (r *importerRegistry) Register(i md.Importer) {
	if i == nil {
		return
	}
	r.list = append(r.list, i)
}

func (r *importerRegistry) List() []md.Importer {
	out := make([]md.Importer, 0, len(r.list))
	out = append(out, r.list...)
	return out
}

// selectImporter returns the first importer that Accepts the source.
func (r *importerRegistry) selectImporter(ctx context.Context, src md.ImportSource) (md.Importer, error) {
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
// exporters.go (registry)
// -----------------------------

type exporterRegistry struct {
	list []md.Exporter
}

func (r *exporterRegistry) Register(e md.Exporter) {
	if e == nil {
		return
	}
	r.list = append(r.list, e)
}

func (r *exporterRegistry) List() []md.Exporter {
	out := make([]md.Exporter, 0, len(r.list))
	out = append(out, r.list...)
	return out
}

// selectExporter returns the first exporter that Accepts the MIME type.
func (r *exporterRegistry) selectExporter(ctx context.Context, mimeType string) (md.Exporter, error) {
	for _, exp := range r.list {
		if exp == nil {
			continue
		}
		if exp.Accept(ctx, mimeType) {
			return exp, nil
		}
	}
	return nil, fmt.Errorf("no exporter accepted MIME type %q", mimeType)
}

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

func writeFile(ctx context.Context, wfs md.WriterFS, name string, perm fs.FileMode, b []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	w, err := wfs.Create(name, perm)
	if err != nil {
		return err
	}

	// If ctx cancels, close the writer to unblock IO where possible.
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = w.Close()
		case <-done:
		}
	}()
	defer close(done)

	bw := bufio.NewWriter(w)

	if _, err := bw.Write(b); err != nil {
		_ = w.Close()
		return err
	}

	if err := bw.Flush(); err != nil {
		_ = w.Close()
		return err
	}
	if err := ctx.Err(); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

// newUUIDLike generates a random 16-byte hex string.
// This is not a canonical UUID string but is sufficient as a unique doc ID placeholder.
// Replace with a real UUID generator if you want canonical UUID formatting.
func newUUIDLike() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
