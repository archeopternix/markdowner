package markdown

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	. "github.com/archeopternix/markdowner"
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
// The importer writes the resulting Document via store.SaveOrUpdate.

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
		mt := strings.ToLower(strings.TrimSpace(src.MimeType))
		return mt == "text/markdown" || mt == "text/plain"
	}
}

// Import reads the markdown source, extracts frontmatter if present, and saves a Document to the store.
func (*MarkdownImporter) Import(ctx context.Context, store DocumentStore, src ImportSource) (*Document, error) {
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

	fmText, bodyText := splitYAMLFrontmatter(string(b))
	fm := Frontmatter{}
	if strings.TrimSpace(fmText) != "" {
		fm = parseFrontmatterMinimal(fmText)
	}

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

	bodyText = strings.TrimRight(bodyText, "\r\n")
	if strings.TrimSpace(bodyText) == "" {
		// Create a minimal markdown document if none is present.
		// Keep it simple and deterministic.
		bodyText = "# " + fm.Title + "\n"
	}

	doc := &Document{
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

// splitYAMLFrontmatter extracts a leading YAML frontmatter block if present.
//
// It recognizes:
//
//	---\n
//	<yaml>\n
//	---\n
//
// at the beginning of the document (allowing an optional UTF-8 BOM).
// Returns (frontmatterYAML, bodyMarkdown).
func splitYAMLFrontmatter(s string) (string, string) {
	// strip UTF-8 BOM
	s = strings.TrimPrefix(s, "\ufeff")
	if !strings.HasPrefix(s, "---") {
		return "", s
	}
	// Must start with "---" line
	r := bufio.NewReader(strings.NewReader(s))
	first, err := r.ReadString('\n')
	if err != nil && err != io.EOF {
		return "", s
	}
	if strings.TrimSpace(first) != "---" {
		return "", s
	}

	var fm bytes.Buffer
	for {
		ln, e := r.ReadString('\n')
		if e != nil && e != io.EOF {
			return "", s
		}
		if strings.TrimSpace(ln) == "---" {
			break
		}
		fm.WriteString(ln)
		if e == io.EOF {
			// no closing marker
			return "", s
		}
	}
	body, _ := io.ReadAll(r)
	return fm.String(), string(body)
}

// parseFrontmatterMinimal parses a minimal YAML subset:
// - key: value pairs
// - keywords:\n  - item\n  - item
//
// Unknown keys are ignored.
func parseFrontmatterMinimal(yaml string) Frontmatter {
	lines := strings.Split(yaml, "\n")
	var fm Frontmatter
	inKeywords := false
	for _, ln := range lines {
		ln = strings.TrimRight(ln, "\r\t ")
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(ln), "keywords:") {
			inKeywords = true
			continue
		}
		if inKeywords {
			trim := strings.TrimSpace(ln)
			if strings.HasPrefix(trim, "-") {
				kw := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
				fm.Keywords = append(fm.Keywords, unquoteYAML(kw))
				continue
			}
			inKeywords = false
		}

		k, v, ok := cutKeyValue(ln)
		if !ok {
			continue
		}
		k = strings.ToLower(k)
		v = unquoteYAML(strings.TrimSpace(v))
		switch k {
		case "author":
			fm.Author = v
		case "title":
			fm.Title = v
		case "subtitle":
			fm.Subtitle = v
		case "date":
			fm.Date = v
		case "changeddate", "changed_date", "changed-date":
			fm.ChangedDate = v
		case "originaldocument", "original_document", "original-document":
			fm.OriginalDocument = v
		case "originalformat", "original_format", "original-format":
			fm.OriginalFormat = v
		case "version":
			fm.Version = v
		case "language":
			fm.Language = v
		case "abstract":
			fm.Abstract = v
		}
	}
	return fm
}

func cutKeyValue(ln string) (key, val string, ok bool) {
	idx := strings.Index(ln, ":")
	if idx <= 0 {
		return "", "", false
	}
	key = strings.TrimSpace(ln[:idx])
	val = strings.TrimSpace(ln[idx+1:])
	if key == "" {
		return "", "", false
	}
	return key, val, true
}

func unquoteYAML(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, "\\\"", "\"")
		s = strings.ReplaceAll(s, "\\\\", "\\")
	}
	return s
}
