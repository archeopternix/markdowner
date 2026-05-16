package markdown

import (
	"context"
	"fmt"
	"io"
	"strings"

	md "github.com/archeopternix/markdowner"
)

type MarkdownExporter struct{}

func New() *MarkdownExporter { return &MarkdownExporter{} }

func (*MarkdownExporter) Name() string { return "markdown" }

func (*MarkdownExporter) Accept(ctx context.Context, mimeType string) bool {
	_ = ctx
	mt := strings.ToLower(strings.TrimSpace(mimeType))
	return mt == "text/markdown" || mt == "text/plain" || mt == "text" || mt == "md" || mt == "markdown"
}

func (*MarkdownExporter) Export(ctx context.Context, doc *md.Document, writer io.WriteCloser) error {
	_ = ctx
	if doc.String() == "" {
		return fmt.Errorf("md.Document is empty")
	}

	_, err := writer.Write([]byte(doc.String()))
	if err != nil {
		return fmt.Errorf("Failed to write document: %w", err)
	}
	return nil
}
