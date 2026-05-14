package format

import (
	"strings"
)

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
