package docx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/archeopternix/markdowner"
)

// DOCXImporter imports .docx using pandoc.
//
// Expected pandoc behavior:
// - Converts DOCX to Markdown.
// - Extracts embedded media into a directory when --extract-media is used.
//
// Note: This importer writes temporary files because pandoc's docx reader expects a file path.
// It cleans up its temp artifacts after import.

type DOCXImporter struct {
	// PandocPath optionally overrides the pandoc binary name/path.
	// If empty, "pandoc" is used.
	PandocPath string

	// ExtraArgs are appended to the pandoc invocation.
	// Use with care; must not override required args like -f/-t/--extract-media.
	ExtraArgs []string
}

func New() *DOCXImporter { return &DOCXImporter{} }

func (i DOCXImporter) Name() string { return "docx(pandoc)" }

func (i DOCXImporter) Accept(ctx context.Context, src ImportSource) bool {
	_ = ctx
	name := strings.TrimSpace(src.Name)
	mime := strings.TrimSpace(src.MimeType)

	if strings.EqualFold(filepath.Ext(name), ".docx") {
		return true
	}
	return strings.EqualFold(mime, "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
}

// Import converts the DOCX source to Markdown using pandoc, extracts media, and saves the Document and media to the store.
func (i DOCXImporter) Import(ctx context.Context, store DocumentStore, src ImportSource) (*Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	if src.Reader == nil {
		return nil, fmt.Errorf("source reader is nil")
	}

	pandoc := strings.TrimSpace(i.PandocPath)
	if pandoc == "" {
		pandoc = "pandoc"
	}

	// Stage source to a temp .docx because pandoc's docx reader is path-based.
	tmpDocx, err := os.CreateTemp("", "markdowner-*.docx")
	if err != nil {
		return nil, err
	}
	tmpDocxPath := tmpDocx.Name()
	defer func() { _ = os.Remove(tmpDocxPath) }()

	if _, err := io.Copy(tmpDocx, src.Reader); err != nil {
		_ = tmpDocx.Close()
		return nil, err
	}
	if err := tmpDocx.Close(); err != nil {
		return nil, err
	}

	// Extract metadata from the staged docx (core properties).
	meta, err := extractDocxMetadata(ctx, tmpDocxPath)
	if err != nil {
		// Usually best: non-fatal metadata failure; import content anyway.
		// If you prefer strict behavior, return err instead.
		meta = extractedDocxMeta{}
	}

	slog.Debug("Extracted DOCX", "title", meta.Title, "created", meta.CreatedAt)

	// pandoc media extraction directory
	tmpMediaDir, err := os.MkdirTemp("", "media")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(tmpMediaDir) }()

	// Convert to Markdown, extracting media.
	// We force github-flavored markdown with pipe tables when possible.
	// Users of the store can normalize further downstream.
	args := []string{
		"-f", "docx",
		"-t", "gfm",
		"--wrap=none",
		"--extract-media", tmpMediaDir,
		tmpDocxPath,
	}
	if len(i.ExtraArgs) > 0 {
		args = append(args[:len(args)-1], append(i.ExtraArgs, args[len(args)-1])...) // insert before input path
	}

	cmd := exec.CommandContext(ctx, pandoc, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		emsg := strings.TrimSpace(stderr.String())
		if emsg == "" {
			emsg = err.Error()
		}
		return nil, fmt.Errorf("pandoc docx import failed: %s", emsg)
	}

	doc := &Document{
		// ID empty: store decides.
		Frontmatter: Frontmatter{
			OriginalDocument: src.Name,
			OriginalFormat:   "docx",
			Author:           "",                                        // pandoc doesn't reliably extract author metadata from docx, so we leave it empty.
			Title:            "",                                        // pandoc doesn't reliably extract title metadata from docx, so we leave it empty.
			Subtitle:         "",                                        // pandoc doesn't reliably extract subtitle metadata from docx, so we leave it empty.
			Date:             src.ModTime.Format("2006-01-02 15:04:05"), // fallback to file mod time if docx metadata is missing
			ChangedDate:      src.ModTime.Format("2006-01-02 15:04:05"), // fallback to file mod time if docx metadata is missing
			Version:          "1.0",
			Language:         "",  // pandoc doesn't reliably extract language metadata from docx, so we leave it empty.
			Abstract:         "",  // pandoc doesn't reliably extract abstract metadata from docx, so we leave it empty.
			Keywords:         nil, // pandoc doesn't reliably extract keywords metadata from docx, so we leave it empty.
		},
		Markdown: out.String(),
	}

	// Write metadata to frontmatter *when available* (don’t stomp defaults/empties).
	if meta.Title != "" && doc.Frontmatter.Title == "" {
		doc.Frontmatter.Title = meta.Title
	}
	if meta.Author != "" && doc.Frontmatter.Author == "" {
		doc.Frontmatter.Author = meta.Author
	}
	if meta.Language != "" && doc.Frontmatter.Language == "" {
		doc.Frontmatter.Language = meta.Language
	}
	if meta.Abstract != "" && doc.Frontmatter.Abstract == "" {
		doc.Frontmatter.Abstract = meta.Abstract
	}
	if len(meta.Keywords) > 0 && len(doc.Frontmatter.Keywords) == 0 {
		doc.Frontmatter.Keywords = meta.Keywords
	}

	// Prefer real created/modified times if present in the docx.
	if meta.CreatedAt != nil {
		doc.Frontmatter.Date = meta.CreatedAt.Format("2006-01-02 15:04:05")
	}
	if meta.ModifiedAt != nil {
		doc.Frontmatter.ChangedDate = meta.ModifiedAt.Format("2006-01-02 15:04:05")
	}

	if err := store.SaveOrUpdate(ctx, doc); err != nil {
		return nil, err
	}

	store.SaveOrUpdate(ctx, doc)

	// Pandoc writes extracted files under: tmpMediaDir/media/...
	extractedMediaRoot := filepath.Join(tmpMediaDir, "media")

	// Only copy if media exists (some docs have none).
	if st, err := os.Stat(extractedMediaRoot); err == nil && st.IsDir() {
		// per-document folder to avoid name collisions:

		// walk extractedMediaRoot recursively and returns all files.
		// It does not keep file handles open; each MediaFile.Open opens on demand.
		paths, err := mediaFilePaths(extractedMediaRoot)
		if err != nil {
			return nil, fmt.Errorf("list extracted media: %w", err)
		}
		doc.Media = paths

		for _, p := range doc.Media {
			f, err := os.Open(p)
			if err != nil {
				return nil, fmt.Errorf("open media %q: %w", p, err)
			}

			mediaName := filepath.Base(p)
			// If you prefer preserving subfolders, use:
			// rel, _ := filepath.Rel(extractedMediaRoot, p)
			// mediaName := filepath.ToSlash(rel)

			if err := store.SaveMedia(ctx, doc.ID, mediaName, f); err != nil {
				_ = f.Close()
				return nil, fmt.Errorf("save media %q: %w", mediaName, err)
			}
			if err := f.Close(); err != nil {
				return nil, fmt.Errorf("close media %q: %w", mediaName, err)
			}

		}
		// rewrite markdown accordingly:
		doc.Markdown = rewritePandocFigureImageHTML(doc.Markdown, "media/")
		doc.Markdown = rewritePandocMediaLinks(doc.Markdown, "media/")
	}
	store.SaveOrUpdate(ctx, doc)
	return doc, nil
}

// MediaFilePaths returns all file paths (not directories) under extractedMediaRoot.
// Paths are returned as full paths as encountered by WalkDir (you can Rel() them if needed).
func mediaFilePaths(extractedMediaRoot string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(extractedMediaRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return paths, nil
}
