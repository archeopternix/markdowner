package docpipe

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// Markdowner converts between the split-disk layout:
// - front.md: YAML frontmatter only
// - root.md:  Markdown body only
//
// This is intentionally minimal: it does not prescribe a particular YAML library.
// Implementations can swap in yaml.v3, json, or custom encoders.

type Markdowner interface {
	EncodeFrontmatter(fm Frontmatter) ([]byte, error)
	DecodeFrontmatter(b []byte) (Frontmatter, error)

	EncodeBody(md Markdown) ([]byte, error)
	DecodeBody(b []byte) (Markdown, error)
}

// DefaultMarkdowner is a small reference implementation:
// - Body is treated as UTF-8 text pass-through.
// - Frontmatter encoding/decoding is intentionally conservative and supports
//   only a simple "key: value" subset plus "keywords" as a YAML list.
//
// If you need full YAML compatibility, replace this with a yaml.v3-based implementation.

type DefaultMarkdowner struct{}

func NewDefaultMarkdowner() DefaultMarkdowner { return DefaultMarkdowner{} }

func (DefaultMarkdowner) EncodeBody(md Markdown) ([]byte, error) {
	return []byte(md), nil
}

func (DefaultMarkdowner) DecodeBody(b []byte) (Markdown, error) {
	return Markdown(string(b)), nil
}

func (DefaultMarkdowner) EncodeFrontmatter(fm Frontmatter) ([]byte, error) {
	var buf bytes.Buffer
	write := func(k, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		fmt.Fprintf(&buf, "%s: %s\n", k, escapeYAMLScalar(v))
	}

	write("author", fm.Author)
	write("title", fm.Title)
	write("subtitle", fm.Subtitle)
	write("date", fm.Date)
	write("changedDate", fm.ChangedDate)
	write("originalDocument", fm.OriginalDocument)
	write("originalFormat", fm.OriginalFormat)
	write("version", fm.Version)
	write("language", fm.Language)
	write("abstract", fm.Abstract)

	if len(fm.Keywords) > 0 {
		buf.WriteString("keywords:\n")
		for _, kw := range fm.Keywords {
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}
			fmt.Fprintf(&buf, "  - %s\n", escapeYAMLScalar(kw))
		}
	}

	return buf.Bytes(), nil
}

var (
	simpleKV = regexp.MustCompile(`^([A-Za-z0-9_\-]+):\s*(.*)$`)
)

func (DefaultMarkdowner) DecodeFrontmatter(b []byte) (Frontmatter, error) {
	lines := strings.Split(string(b), "\n")
	var fm Frontmatter

	inKeywords := false
	for _, ln := range lines {
		ln = strings.TrimRight(ln, "\r\t ")
		if strings.TrimSpace(ln) == "" {
			continue
		}

		if strings.HasPrefix(ln, "keywords:") {
			inKeywords = true
			continue
		}

		if inKeywords {
			trim := strings.TrimSpace(ln)
			if strings.HasPrefix(trim, "-") {
				kw := strings.TrimSpace(strings.TrimPrefix(trim, "-"))
				fm.Keywords = append(fm.Keywords, unescapeYAMLScalar(kw))
				continue
			}
			// any non-list line ends keywords block
			inKeywords = false
		}

		m := simpleKV.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		k := strings.ToLower(m[1])
		v := unescapeYAMLScalar(strings.TrimSpace(m[2]))
		switch k {
		case "author":
			fm.Author = v
		case "title":
			fm.Title = v
		case "subtitle":
			fm.Subtitle = v
		case "date":
			fm.Date = v
		case "changeddate":
			fm.ChangedDate = v
		case "originaldocument":
			fm.OriginalDocument = v
		case "originalformat":
			fm.OriginalFormat = v
		case "version":
			fm.Version = v
		case "language":
			fm.Language = v
		case "abstract":
			fm.Abstract = v
		}
	}

	return fm, nil
}

func escapeYAMLScalar(s string) string {
	// Minimal quoting: quote if it contains ':' leading/trailing spaces or starts with special chars.
	needs := false
	if strings.HasPrefix(s, "[") || strings.HasPrefix(s, "{") || strings.HasPrefix(s, "-") || strings.HasPrefix(s, "#") {
		needs = true
	}
	if strings.ContainsAny(s, ":\n\r\t") {
		needs = true
	}
	if strings.TrimSpace(s) != s {
		needs = true
	}
	if !needs {
		return s
	}
	q := strings.ReplaceAll(s, "\\", "\\\\")
	q = strings.ReplaceAll(q, "\"", "\\\"")
	return "\"" + q + "\""
}

func unescapeYAMLScalar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, "\\\"", "\"")
		s = strings.ReplaceAll(s, "\\\\", "\\")
	}
	return s
}
