package docx

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// copyFile copies src -> dst, creating parent dirs.
func copyFile(srcPath, dstPath string, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dstPath), 0o755); err != nil {
		return err
	}

	in, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

// copyDir recursively copies contents of srcDir into dstDir.
func copyDir(srcDir, dstDir string) error {
	return filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		dstPath := filepath.Join(dstDir, rel)

		if d.IsDir() {
			return os.MkdirAll(dstPath, 0o755)
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		return copyFile(path, dstPath, info.Mode())
	})
}

// rewritePandocMediaLinks rewrites Markdown image links that pandoc emits like:
//
//	![](media/image.png)
//	![](<media/image.png>)
//
// into:
//
//	![](media/image.png)  (same) or with a prefix if desired
//
// If you want them under a document-specific folder, pass prefix like "media/<docID>/".
func rewritePandocMediaLinks(md string, newPrefix string) string {
	// match: ![alt](media/xyz) and ![alt](<media/xyz>)
	re := regexp.MustCompile(`!\[([^\]]*)\]\(\s*(<)?(media/[^)\s>]+)(>)?\s*\)`)
	return re.ReplaceAllStringFunc(md, func(m string) string {
		sub := re.FindStringSubmatch(m)
		alt := sub[1]
		path := sub[3] // "media/..."
		// keep "media/" tail, but optionally prefix it
		if newPrefix != "" {
			// newPrefix expected to end with "/" or be empty; normalize
			p := strings.TrimSuffix(newPrefix, "/") + "/"
			// strip "media/" from original and put under prefix
			path = p + strings.TrimPrefix(path, "media/")
		}
		return "![" + alt + "](" + path + ")"
	})
}
