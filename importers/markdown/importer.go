package markdown

import (
	"context"
	"io"
	"path/filepath"
	"strings"

	. "github.com/archeopternix/markdowner"
)

// Importer is a minimal markdown importer.
// It expects ImportSource.Reader to provide the markdown body.
// This reference importer does not extract additional media.

type MarkdownImporter struct{}

func New() *MarkdownImporter { return &MarkdownImporter{} }

func (*MarkdownImporter) Name() string { return "markdown" }

func (*MarkdownImporter) Accept(ctx context.Context, src ImportSource) bool {
	_ = ctx
	name := strings.ToLower(src.Name)
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".md", ".markdown", ".txt":
		return true
	default:
		// fallback: treat explicit text/markdown as acceptable
		mt := strings.ToLower(strings.TrimSpace(src.MimeType))
		return mt == "text/markdown" || mt == "text/plain"
	}
}

func (*MarkdownImporter) Import(ctx context.Context, store DocumentStore, src ImportSource) (*Document, error) {
	// Minimal: read stream as Markdown body.
	b, err := ioReadAllWithContext(ctx, src.Reader)
	if err != nil {
		return nil, err
	}

	doc := &Document{
		Frontmatter: Frontmatter{},
		Markdown:    string(b),
		Media:       nil,
	}
	

	// Save will allocate doc.ID if empty.
	if err := store.SaveOrUpdate(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}

func ioReadAllWithContext(ctx context.Context, r io.Reader) ([]byte, error) {
	// Simple implementation: does not interrupt io.ReadAll on ctx cancellation.
	// Replace with a ctx-aware reader wrapper if needed.
	_ = ctx
	return io.ReadAll(r)
}
