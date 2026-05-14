package markdowner

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"

	format "github.com/archeopternix/markdowner/internal/format"
)

var simpleKV = regexp.MustCompile(`^([A-Za-z0-9_\-]+):\s*(.*)$`)

// GetFrontmatterYAML encodes Document.Frontmatter into a minimal YAML representation.
// This is intentionally conservative and supports scalar fields plus Keywords as a YAML list.
func (d Document) EncodeFrontmatterYAML() ([]byte, error) {
	var buf bytes.Buffer
	write := func(k, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		fmt.Fprintf(&buf, "%s: %s\n", k, format.EscapeYAMLScalar(v))
	}

	write("author", d.Frontmatter.Author)
	write("title", d.Frontmatter.Title)
	write("subtitle", d.Frontmatter.Subtitle)
	write("date", d.Frontmatter.Date)
	write("changedDate", d.Frontmatter.ChangedDate)
	write("originalDocument", d.Frontmatter.OriginalDocument)
	write("originalFormat", d.Frontmatter.OriginalFormat)
	write("version", d.Frontmatter.Version)
	write("language", d.Frontmatter.Language)
	write("abstract", d.Frontmatter.Abstract)

	if len(d.Frontmatter.Keywords) > 0 {
		buf.WriteString("keywords:\n")
		for _, kw := range d.Frontmatter.Keywords {
			kw = strings.TrimSpace(kw)
			if kw == "" {
				continue
			}
			fmt.Fprintf(&buf, "  - %s\n", format.EscapeYAMLScalar(kw))
		}
	}

	return buf.Bytes(), nil
}

// DecodeFrontmatterYAML decodes a minimal YAML representation into Document.Frontmatter.
// It is not a full YAML parser.
func (d *Document) DecodeFrontmatterYAML(b []byte) error {
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
				d.Frontmatter.Keywords = append(d.Frontmatter.Keywords, format.UnescapeYAMLScalar(kw))
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
			d.Frontmatter.Author = v
		case "title":
			d.Frontmatter.Title = v
		case "subtitle":
			d.Frontmatter.Subtitle = v
		case "date":
			d.Frontmatter.Date = v
		case "changeddate":
			d.Frontmatter.ChangedDate = v
		case "originaldocument":
			d.Frontmatter.OriginalDocument = v
		case "originalformat":
			d.Frontmatter.OriginalFormat = v
		case "version":
			d.Frontmatter.Version = v
		case "language":
			d.Frontmatter.Language = v
		case "abstract":
			d.Frontmatter.Abstract = v
		}
	}
	return nil
}

// String returns a string representation of the Document, combining frontmatter and markdown.
func (d Document) String() string {
	fmYAML, _ := d.EncodeFrontmatterYAML()
	return fmt.Sprintf("---\n%s---\n%s", string(fmYAML), d.Markdown)
}
