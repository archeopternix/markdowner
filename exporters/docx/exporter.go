package docx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/archeopternix/markdowner"
)

type DocxExporter struct {
	// PandocPath optionally overrides the pandoc binary name/path.
	// If empty, "pandoc" is used.
	PandocPath string

	// ReferenceDocPath optionally points to a DOCX template used by pandoc's --reference-doc.
	ReferenceDocPath string

	// ExtraArgs are appended to the pandoc invocation.
	ExtraArgs []string
}

func New() *DocxExporter {
	return &DocxExporter{
		ReferenceDocPath: "template.docx",
	}
}

func (*DocxExporter) Name() string { return "docx" }

func (*DocxExporter) Accept(ctx context.Context, mimeType string) bool {
	_ = ctx

	return IsMSWord(mimeType)
}

func (e *DocxExporter) Export(ctx context.Context, doc *Document, writer io.WriteCloser) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if doc == nil {
		return fmt.Errorf("document is nil")
	}
	if writer == nil {
		return fmt.Errorf("writer is nil")
	}
	content := doc.String()
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("Document is empty")
	}

	pandoc := strings.TrimSpace(e.PandocPath)
	if pandoc == "" {
		pandoc = "pandoc"
	}

	tmpDir, err := os.MkdirTemp("", "markdowner-docx-export-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	inputPath := filepath.Join(tmpDir, "input.md")
	outputPath := filepath.Join(tmpDir, "output.docx")

	if err := os.WriteFile(inputPath, []byte(content), 0o644); err != nil {
		return err
	}

	args := []string{
		"-f", "markdown",
		"-t", "docx",
		"--wrap=none",
	}

	if refDoc := strings.TrimSpace(e.ReferenceDocPath); refDoc != "" {
		args = append(args, "--reference-doc="+refDoc)
	}

	if doc.Media != nil {
		args = append(args, "--resource-path=media")
	}

	if len(e.ExtraArgs) > 0 {
		args = append(args, e.ExtraArgs...)
	}

	args = append(args, "-o", outputPath, inputPath)

	cmd := exec.CommandContext(ctx, pandoc, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		emsg := strings.TrimSpace(stderr.String())
		if emsg == "" {
			emsg = err.Error()
		}
		return fmt.Errorf("pandoc docx export failed: %s", emsg)
	}

	outFile, err := os.Open(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = outFile.Close() }()

	if _, err := io.Copy(writer, outFile); err != nil {
		return err
	}

	return nil
}

func IsMSWord(mime string) bool {
	m := strings.ToLower(strings.TrimSpace(mime))

	// MIME-based (useful when filename is missing or untrusted)
	switch m {
	case "application/msword": // .doc, .dot (legacy)
		return true
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.document": // .docx
		return true
	case "application/vnd.ms-word.document.macroenabled.12": // .docm
		return true
	case "application/vnd.openxmlformats-officedocument.wordprocessingml.template": // .dotx
		return true
	case "application/vnd.ms-word.template.macroenabled.12": // .dotm
		return true
	case "application/rtf", "text/rtf": // .rtf (varies)
		return true
	}

	return false
}
