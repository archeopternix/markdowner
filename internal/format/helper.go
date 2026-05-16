package format

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var simpleKV = regexp.MustCompile(`^([A-Za-z0-9_\-]+):\s*(.*)$`)

// EncodeYAML encodes a Frontmatter (as map[string]string) into a minimal YAML representation.
// This is intentionally conservative and supports scalar fields as a YAML list.
func EncodeYAML(m map[string]string) ([]byte, error) {
	var buf bytes.Buffer
	write := func(k, v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		fmt.Fprintf(&buf, "%s: %s\n", k, EscapeYAMLScalar(v))
	}

	write("author", m["author"])
	write("title", m["title"])
	write("subtitle", m["subtitle"])
	write("date", m["date"])
	write("changedDate", m["changedDate"])
	write("originalDocument", m["originalDocument"])
	write("originalFormat", m["originalFormat"])
	write("version", m["version"])
	write("language", m["language"])
	write("abstract", m["abstract"])

	return buf.Bytes(), nil
}

// DecodeYAML decodes a minimal YAML representation into Frontmatter (map[string]string).
// It is not a full YAML parser.
func DecodeYAML(b []byte) (map[string]string, error) {
	fmap := make(map[string]string)

	lines := strings.Split(string(b), "\n")

	for _, ln := range lines {
		ln = strings.TrimRight(ln, "\r\t ")
		if strings.TrimSpace(ln) == "" {
			continue
		}

		m := simpleKV.FindStringSubmatch(ln)
		if m == nil {
			continue
		}
		k := strings.ToLower(m[1])
		v := UnescapeYAMLScalar(strings.TrimSpace(m[2]))
		switch k {
		case "author":
			fmap["author"] = v
		case "title":
			fmap["title"] = v
		case "subtitle":
			fmap["subtitle"] = v
		case "date":
			fmap["date"] = v
		case "changeddate":
			fmap["changedDate"] = v
		case "originaldocument":
			fmap["originalDocument"] = v
		case "originalformat":
			fmap["originalFormat"] = v
		case "version":
			fmap["version"] = v
		case "language":
			fmap["language"] = v
		case "abstract":
			fmap["abstract"] = v
		}
	}
	return fmap, nil
}

func EscapeYAMLScalar(s string) string {
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

func UnescapeYAMLScalar(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		s = strings.ReplaceAll(s, "\\\"", "\"")
		s = strings.ReplaceAll(s, "\\\\", "\\")
	}
	return s
}

// ParseFrontmatterYAMLfromMarkdown extracts a leading YAML frontmatter block if present.
//
// It recognizes:
//
//	---\n
//	<yaml>\n
//	---\n
//
// at the beginning of the document (allowing an optional UTF-8 BOM).
// Returns (frontmatterYAML, bodyMarkdown).
func ParseFrontmatterYAMLfromMarkdown(s string) (string, string) {
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
