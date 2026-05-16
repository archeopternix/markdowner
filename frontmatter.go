package markdowner

import (
	"fmt"
	"strings"

	"github.com/archeopternix/markdowner/internal/format"
)

// Frontmatter represents the metadata of a markdown document, typically found in the YAML front matter section.
func DecodeFrontmatterFromMap(m map[string]string) Frontmatter {
	f := Frontmatter{}

	for k, v := range m {
		switch strings.ToLower(k) {
		case "author":
			f.Author = fmt.Sprintf("%v", v)
		case "title":
			f.Title = fmt.Sprintf("%v", v)
		case "subtitle":
			f.Subtitle = fmt.Sprintf("%v", v)
		case "date":
			f.Date = fmt.Sprintf("%v", v)
		case "changeddate", "changed_date", "changed-date":
			f.ChangedDate = fmt.Sprintf("%v", v)
		case "originaldocument", "original_document", "original-document":
			f.OriginalDocument = fmt.Sprintf("%v", v)
		case "originalformat", "original_format", "original-format":
			f.OriginalFormat = fmt.Sprintf("%v", v)
		case "version":
			f.Version = fmt.Sprintf("%v", v)
		case "language":
			f.Language = fmt.Sprintf("%v", v)
		case "abstract":
			f.Abstract = fmt.Sprintf("%v", v)
		}
	}
	return f
}

// EncodeFrontmatterToMap encodes the Frontmatter struct into a map[string]string, omitting empty fields.
func (f Frontmatter) EncodeFrontmatterToMap() map[string]string {
	m := make(map[string]string)
	if f.Author != "" {
		m["author"] = f.Author
	}
	if f.Title != "" {
		m["title"] = f.Title
	}
	if f.Subtitle != "" {
		m["subtitle"] = f.Subtitle
	}
	if f.Date != "" {
		m["date"] = f.Date
	}
	if f.ChangedDate != "" {
		m["changedDate"] = f.ChangedDate
	}
	if f.OriginalDocument != "" {
		m["originalDocument"] = f.OriginalDocument
	}
	if f.OriginalFormat != "" {
		m["originalFormat"] = f.OriginalFormat
	}
	if f.Version != "" {
		m["version"] = f.Version
	}
	if f.Language != "" {
		m["language"] = f.Language
	}
	if f.Abstract != "" {
		m["abstract"] = f.Abstract
	}
	return m
}

// String returns a string representation of the Frontmatter in YAML format without leading or trailing dashes.
func (f Frontmatter) String() string {
	fmap := f.EncodeFrontmatterToMap()
	fmYAML, _ := format.EncodeYAML(fmap)
	return string(fmYAML)
}

// String returns a string representation of the Document, combining frontmatter and markdown.
func (d Document) String() string {
	fmap := d.Frontmatter.EncodeFrontmatterToMap()
	fmYAML, _ := format.EncodeYAML(fmap)
	return fmt.Sprintf("---\n%s---\n%s", string(fmYAML), d.Markdown)
}
