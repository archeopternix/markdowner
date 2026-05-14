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

// rewritePandocMediaLinks Rewrite only image links where the *last directory* before the filename is "media".
// Examples rewritten:
//   ![](media/img.png)                -> ![](NEWPREFIX/img.png)
//   ![](foo/bar/media/img.png)        -> ![](NEWPREFIX/img.png)
//   ![](<foo/bar/media/img.png>)      -> ![](NEWPREFIX/img.png)
//
// Examples NOT rewritten:
//   ![](immediate/img.png)            (no ".../media/<file>")
//   ![](media/sub/img.png)            (last dir before filename is "sub", not "media")

func rewritePandocMediaLinks(md string, newPrefix string) string {

	prefix := strings.TrimSuffix(newPrefix, "/")
	if prefix != "" {
		prefix += "/"
	}

	// Groups:
	// 1 alt text
	// 2 optional "<"
	// 3 the link target (trim spaces later)
	// 4 optional ">"
	re := regexp.MustCompile(`!\[([^\]]*)\]\(\s*(<)?([^)>]+?)(>)?\s*\)`)

	return re.ReplaceAllStringFunc(md, func(m string) string {
		sub := re.FindStringSubmatch(m)
		alt := sub[1]
		rawTarget := strings.TrimSpace(sub[3])

		// Drop optional surrounding angle brackets if the regex captured oddly;
		// also trim again for cases like "< /tmp/x >".
		rawTarget = strings.TrimPrefix(rawTarget, "<")
		rawTarget = strings.TrimSuffix(rawTarget, ">")
		rawTarget = strings.TrimSpace(rawTarget)

		// Normalize backslashes just in case.
		target := strings.ReplaceAll(rawTarget, `\`, `/`)
		target = strings.TrimSpace(target)

		// Extract filename and check last dir is "media".
		// We only rewrite if the target ends with "/media/<filename>" (no extra subdir after media).
		i := strings.LastIndex(target, "/")
		if i < 0 || i == len(target)-1 {
			return m
		}
		filename := target[i+1:]
		parent := target[:i]

		// parent must end with "/media" (or be exactly "media")
		if parent == "media" || strings.HasSuffix(parent, "/media") {
			newPath := prefix + filename
			return "![" + alt + "](" + newPath + ")"
		}

		return m
	})
}
