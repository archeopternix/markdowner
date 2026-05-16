package markdown

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	md "github.com/archeopternix/markdowner"
	"github.com/archeopternix/markdowner/internal/format"
)

// Importer imports markdown-like sources into the docpipe store.
//
// Behavior:
//  1) Assumes the markdown file may contain YAML frontmatter; if present, extracts it.
//  2) If no markdown body is available (empty input), creates a new one.
//  3) If no frontmatter is available, creates default frontmatter:
//     - Title: filename without extension
//     - Date, ChangedDate: src.ModTime if present, else current time
//     - Version: "V1.0"
//     - OriginalDocument: src.Name
//     - OriginalFormat: "markdown"
//
// The importer writes the resulting md.Document via store.SaveOrUpdate.

type MarkdownImporter struct{}

func New() *MarkdownImporter { return &MarkdownImporter{} }

func (*MarkdownImporter) Name() string { return "markdown" }

func (*MarkdownImporter) Accept(ctx context.Context, src md.ImportSource) bool {
	_ = ctx
	name := strings.ToLower(src.Name)
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".txt":
		return true
	default:
		mt := strings.ToLower(strings.TrimSpace(src.MimeType))
		return mt == "text/markdown" || mt == "text/plain"
	}
}

// Import reads the markdown source, extracts frontmatter if present, and saves a md.Document to the store.
func (*MarkdownImporter) Import(ctx context.Context, store md.DocumentStore, src md.ImportSource) (*md.Document, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("store is nil")
	}
	if src.Reader == nil {
		return nil, fmt.Errorf("source reader is nil")
	}

	b, err := ioReadAllWithContext(ctx, src.Reader)
	if err != nil {
		return nil, err
	}

	fmBytes, bodyText := format.ParseFrontmatterYAMLfromMarkdown(string(b))

	fmap, err := format.DecodeYAML([]byte(fmBytes))
	if err != nil {
		return nil, fmt.Errorf("decode YAML: %w", err)
	}
	fm := md.DecodeFrontmatterFromMap(fmap)

	now := time.Now().UTC()
	t := src.ModTime
	if t.IsZero() {
		t = now
	}
	stamp := t.Format("02.01.2006 15:04")

	// Defaults if missing or no frontmatter.
	titleDefault := strings.TrimSuffix(filepath.Base(src.Name), filepath.Ext(src.Name))
	if strings.TrimSpace(fm.Title) == "" {
		fm.Title = titleDefault
	}
	if strings.TrimSpace(fm.Date) == "" {
		fm.Date = stamp
	}
	if strings.TrimSpace(fm.ChangedDate) == "" {
		fm.ChangedDate = stamp
	}
	if strings.TrimSpace(fm.Version) == "" {
		fm.Version = "V1.0"
	}
	if strings.TrimSpace(fm.OriginalDocument) == "" {
		fm.OriginalDocument = src.Name
	}
	if strings.TrimSpace(fm.OriginalFormat) == "" {
		fm.OriginalFormat = "markdown"
	}

	slog.Debug("Parsed markdown", "title", fm.Title, "date", fm.Date)

	bodyText = strings.TrimRight(bodyText, "\r\n")
	if strings.TrimSpace(bodyText) == "" {
		// Create a minimal markdown document if none is present.
		// Keep it simple and deterministic.
		bodyText = "# " + fm.Title + "\n"
	}

	doc := &md.Document{
		Frontmatter: fm,
		Markdown:    bodyText,
		Media:       nil,
	}

	if err := store.SaveOrUpdate(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func ioReadAllWithContext(ctx context.Context, r io.Reader) ([]byte, error) {
	_ = ctx
	return io.ReadAll(r)
}
