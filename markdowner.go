package markdowner

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	format "github.com/archeopternix/markdowner/internal/format"
)

var simpleKV = regexp.MustCompile(`^([A-Za-z0-9_\-]+):\s*(.*)$`)

// EncodeYAML encodes Document.Frontmatter into a minimal YAML representation.
// This is intentionally conservative and supports scalar fields plus Keywords as a YAML list.
func (f Frontmatter) EncodeYAML() ([]byte, error) {
	var buf bytes.Buffer
	write := func(k, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		fmt.Fprintf(&buf, "%s: %s\n", k, format.EscapeYAMLScalar(v))
	}

	write("author", f.Author)
	write("title", f.Title)
	write("subtitle", f.Subtitle)
	write("date", f.Date)
	write("changedDate", f.ChangedDate)
	write("originalDocument", f.OriginalDocument)
	write("originalFormat", f.OriginalFormat)
	write("version", f.Version)
	write("language", f.Language)
	write("abstract", f.Abstract)

	if len(f.Keywords) > 0 {
		buf.WriteString("keywords:\n")
		for _, kw := range f.Keywords {
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}
			fmt.Fprintf(&buf, "  - %s\n", format.EscapeYAMLScalar(kw))
		}
	}

	return buf.Bytes(), nil
}

// DecodeYAML decodes a minimal YAML representation into Document.Frontmatter.
// It is not a full YAML parser.
func (f Frontmatter) DecodeYAML(b []byte) error {
	lines := strings.Split(string(b), "\n")

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
				f.Keywords = append(f.Keywords, format.UnescapeYAMLScalar(kw))
				continue
			}
			inKeywords = false
		}

		m := simpleKV.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		k := strings.ToLower(m[1])
		v := format.UnescapeYAMLScalar(strings.TrimSpace(m[2]))
		switch k {
		case "author":
			f.Author = v
		case "title":
			f.Title = v
		case "subtitle":
			f.Subtitle = v
		case "date":
			f.Date = v
		case "changeddate":
			f.ChangedDate = v
		case "originaldocument":
			f.OriginalDocument = v
		case "originalformat":
			f.OriginalFormat = v
		case "version":
			f.Version = v
		case "language":
			f.Language = v
		case "abstract":
			f.Abstract = v
		}
	}
	return nil
}

// String returns a string representation of the Document, combining frontmatter and markdown.
func (d Document) String() string {
	fmYAML, _ := d.Frontmatter.EncodeYAML()
	return fmt.Sprintf("---\n%s---\n%s", string(fmYAML), d.Markdown)
}
