package format

// EncodeMarkdownBody encodes the Markdown body to bytes for root.md.
func EncodeMarkdownBody(md string) ([]byte, error) {
	return []byte(md), nil
}

// DecodeMarkdownBody decodes bytes from root.md into a Markdown body.
func DecodeMarkdownBody(b []byte) (string, error) {
	return string(b), nil
}
