package docx

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
		},
		Markdown: out.String(),
	}

	if err := store.SaveOrUpdate(ctx, doc); err != nil {
		return nil, err
	}
	return doc, nil
}
