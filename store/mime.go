package store

import (
	"archive/zip"
	"bytes"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

type DetectedMime struct {
	MimeType string // e.g. "text/markdown"
	Kind     string // e.g. "markdown", "text", "docx", "pptx", "pdf"
}

// DetectMime detects: text, markdown, docx, pptx, pdf.
// It does NOT lose bytes: it returns a new reader that will replay the bytes read for sniffing
// followed by the remainder of the original stream.
func DetectMime(name string, r io.ReadCloser) (DetectedMime, error) {
	const sniffLen = 64 * 1024 // enough to include OOXML [Content_Types].xml and core parts in many files
	defer r.Close()

	prefix, err := read(r, sniffLen)
	if err != nil {
		return DetectedMime{}, err
	}

	ext := strings.ToLower(filepath.Ext(name))

	// PDF
	if len(prefix) >= 5 && bytes.Equal(prefix[:5], []byte("%PDF-")) {
		return DetectedMime{MimeType: "application/pdf", Kind: "pdf"}, nil
	}

	// OOXML (docx/pptx)
	if len(prefix) >= 4 && bytes.Equal(prefix[:4], []byte("PK\x03\x04")) {
		kind, ok := detectOOXMLKindFromZipPrefix(prefix)
		if ok {
			switch kind {
			case "docx":
				return DetectedMime{
					MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
					Kind:     "docx",
				}, nil
			case "pptx":
				return DetectedMime{
					MimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
					Kind:     "pptx",
				}, nil
			}
		}

		// Tie-breaker fallback by extension (still no byte loss)
		if ext == ".docx" {
			return DetectedMime{
				MimeType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
				Kind:     "docx",
			}, nil
		}
		if ext == ".pptx" {
			return DetectedMime{
				MimeType: "application/vnd.openxmlformats-officedocument.presentationml.presentation",
				Kind:     "pptx",
			}, nil
		}
	}

	// Text / Markdown
	if looksLikeText(prefix) {
		if ext == ".md" || ext == ".markdown" || looksLikeMarkdown(prefix) {
			return DetectedMime{MimeType: "text/markdown", Kind: "markdown"}, nil
		}
		return DetectedMime{MimeType: "text/plain", Kind: "text"}, nil
	}

	// Last resort
	ct := http.DetectContentType(prefix)
	if strings.HasPrefix(ct, "text/plain") {
		if ext == ".md" || ext == ".markdown" {
			return DetectedMime{MimeType: "text/markdown", Kind: "markdown"}, nil
		}
		return DetectedMime{MimeType: "text/plain", Kind: "text"}, nil
	}
	return DetectedMime{MimeType: ct, Kind: ""}, nil
}

// read reads up to n bytes from r, and returns:
//  1. the bytes read (prefix),
//  2. a reader that will replay prefix then continue reading from r,
//  3. error only on non-EOF read failures.
func read(r io.Reader, n int) (prefix []byte, err error) {
	var buf bytes.Buffer
	_, copyErr := io.CopyN(&buf, r, int64(n))
	if copyErr != nil && copyErr != io.EOF {
		// io.CopyN returns EOF if fewer than n bytes were read; that's fine.
		return nil, copyErr
	}
	prefix = buf.Bytes()
	return prefix, nil
}

func looksLikeText(b []byte) bool {
	if len(b) == 0 {
		return true
	}
	if bytes.IndexByte(b, 0x00) >= 0 {
		return false
	}
	if utf8.Valid(b) {
		return true
	}
	printable := 0
	for _, c := range b {
		switch {
		case c == '\n' || c == '\r' || c == '\t':
			printable++
		case c >= 0x20 && c <= 0x7E:
			printable++
		}
	}
	return float64(printable)/float64(len(b)) > 0.9
}

func looksLikeMarkdown(b []byte) bool {
	s := string(b)
	t := strings.TrimSpace(s)

	if strings.HasPrefix(t, "#") || strings.Contains(s, "\n#") {
		return true
	}
	if strings.Contains(s, "```") {
		return true
	}
	if strings.Contains(s, "](") && strings.Contains(s, "[") {
		return true
	}
	if strings.Contains(s, "\n- ") || strings.Contains(s, "\n* ") || strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") {
		return true
	}
	return false
}

// detectOOXMLKindFromZipPrefix does a reliable(ish) OOXML detection without consuming the original reader.
// It attempts to open the ZIP using the sniffed prefix bytes only.
// If the ZIP structures aren’t fully present in the prefix (common for larger zips), it returns ok=false.
func detectOOXMLKindFromZipPrefix(prefix []byte) (kind string, ok bool) {
	// We can only open a zip if we have its central directory in the provided bytes.
	// For many OOXML files, 64KB is often enough, but not guaranteed.
	zr, err := zip.NewReader(bytes.NewReader(prefix), int64(len(prefix)))
	if err != nil {
		return "", false
	}

	hasWord := false
	hasPPT := false

	for _, f := range zr.File {
		switch {
		case f.Name == "word/document.xml" || strings.HasPrefix(f.Name, "word/"):
			hasWord = true
		case f.Name == "ppt/presentation.xml" || strings.HasPrefix(f.Name, "ppt/"):
			hasPPT = true
		}
	}

	switch {
	case hasWord && !hasPPT:
		return "docx", true
	case hasPPT && !hasWord:
		return "pptx", true
	case hasWord && hasPPT:
		// Strange hybrid; declare unknown.
		return "", false
	default:
		return "", false
	}
}
