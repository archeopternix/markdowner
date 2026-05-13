package format

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	. "github.com/archeopternix/markdowner"
)

// EncodeFrontmatterYAML encodes docpipe.Frontmatter into a minimal YAML representation.
// This is intentionally conservative and supports scalar fields plus Keywords as a YAML list.
func EncodeFrontmatterYAML(fm Frontmatter) ([]byte, error) {
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

var simpleKV = regexp.MustCompile(`^([A-Za-z0-9_\-]+):\s*(.*)$`)

// DecodeFrontmatterYAML decodes a minimal YAML representation into docpipe.Frontmatter.
// It is not a full YAML parser.
func DecodeFrontmatterYAML(b []byte) (Frontmatter, error) {
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
