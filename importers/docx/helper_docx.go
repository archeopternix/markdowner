package docx

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	fsutil "github.com/archeopternix/markdowner/internal/fsutils"
)

// Minimal subset of the OOXML core properties part.
// Spec namespaces vary; decoding with explicit tags is easiest if we bind namespaces in struct tags.
type docxCoreProps struct {
	XMLName xml.Name `xml:"coreProperties"`

	Title       string `xml:"http://purl.org/dc/elements/1.1/ title"`
	Subject     string `xml:"http://purl.org/dc/elements/1.1/ subject"`
	Creator     string `xml:"http://purl.org/dc/elements/1.1/ creator"`
	Description string `xml:"http://purl.org/dc/elements/1.1/ description"`

	Keywords string `xml:"http://schemas.openxmlformats.org/package/2006/metadata/core-properties keywords"`
	Language string `xml:"http://purl.org/dc/elements/1.1/ language"`

	Created  string `xml:"http://purl.org/dc/terms/ created"`
	Modified string `xml:"http://purl.org/dc/terms/ modified"`
}

// parseW3CDTF tries to parse typical dcterms timestamps (W3CDTF / RFC3339-ish).
func parseW3CDTF(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	// Most DOCX use RFC3339 like 2020-01-02T15:04:05Z
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	// Some might omit timezone; treat as local/UTC fallback (choose what you prefer).
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

type extractedDocxMeta struct {
	Title       string
	Subtitle    string // DOCX core props doesn't really have "subtitle"; keep for possible extensions.
	Author      string
	Language    string
	Abstract    string
	Keywords    []string
	CreatedAt   *time.Time
	ModifiedAt  *time.Time
	Subject     string
	Description string
}

func extractDocxMetadata(_ context.Context, docxPath string) (extractedDocxMeta, error) {
	var meta extractedDocxMeta

	zr, err := zip.OpenReader(docxPath)
	if err != nil {
		return meta, err
	}
	defer zr.Close()

	coreXML, err := fsutil.ReadZipFile(zr, "docProps/core.xml")
	if err != nil {
		// No core.xml is not fatal; just return empty meta.
		if errors.Is(err, os.ErrNotExist) {
			return meta, nil
		}
		return meta, err
	}

	var props docxCoreProps
	dec := xml.NewDecoder(bytes.NewReader(coreXML))
	if err := dec.Decode(&props); err != nil {
		return meta, fmt.Errorf("decode docProps/core.xml: %w", err)
	}

	meta.Title = strings.TrimSpace(props.Title)
	meta.Author = strings.TrimSpace(props.Creator)
	meta.Language = strings.TrimSpace(props.Language)

	// Prefer description as "abstract" if present
	meta.Description = strings.TrimSpace(props.Description)
	meta.Abstract = meta.Description

	// Keywords: often semicolon- or comma-separated; normalize to []string
	kw := strings.TrimSpace(props.Keywords)
	if kw != "" {
		parts := strings.FieldsFunc(kw, func(r rune) bool {
			return r == ';' || r == ',' || r == '\n' || r == '\r' || r == '\t'
		})
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				meta.Keywords = append(meta.Keywords, p)
			}
		}
	}

	if t, ok := parseW3CDTF(props.Created); ok {
		meta.CreatedAt = &t
	}
	if t, ok := parseW3CDTF(props.Modified); ok {
		meta.ModifiedAt = &t
	}

	meta.Subject = strings.TrimSpace(props.Subject)

	return meta, nil
}
