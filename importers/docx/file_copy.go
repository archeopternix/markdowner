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

		if newPath, ok := normalizePandocMediaTarget(rawTarget, newPrefix); ok {
			return "![" + alt + "](" + newPath + ")"
		}

		return m
	})
}

// rewritePandocFigureImageHTML rewrites pandoc-generated HTML <figure> blocks with <img> into markdown image syntax.
// Example:
//
//	<figure><img src=".../media/image1.png" alt="My Image" /></figure>
//
// becomes:
//
//	![My Image](media/image1.png)
func rewritePandocFigureImageHTML(md string, newPrefix string) string {
	figureRe := regexp.MustCompile(`(?is)<figure\b[^>]*>.*?</figure>`)
	imgRe := regexp.MustCompile(`(?is)<img\b[^>]*>`)
	srcRe := regexp.MustCompile(`(?is)\bsrc\s*=\s*(?:"([^"]*)"|'([^']*)')`)
	altRe := regexp.MustCompile(`(?is)\balt\s*=\s*(?:"([^"]*)"|'([^']*)')`)

	return figureRe.ReplaceAllStringFunc(md, func(fig string) string {
		img := imgRe.FindString(fig)
		if img == "" {
			return fig
		}

		srcMatch := srcRe.FindStringSubmatch(img)
		if len(srcMatch) < 3 {
			return fig
		}
		src := srcMatch[1]
		if src == "" {
			src = srcMatch[2]
		}
		src = strings.TrimSpace(src)
		if src == "" {
			return fig
		}

		alt := ""
		altMatch := altRe.FindStringSubmatch(img)
		if len(altMatch) >= 3 {
			alt = altMatch[1]
			if alt == "" {
				alt = altMatch[2]
			}
		}
		alt = strings.TrimSpace(alt)

		target := strings.ReplaceAll(src, `\`, `/`)
		if newPath, ok := normalizePandocMediaTarget(target, newPrefix); ok {
			target = newPath
		}

		return "![" + alt + "](" + target + ")"
	})
}

func normalizePandocMediaTarget(rawTarget string, newPrefix string) (string, bool) {
	prefix := strings.TrimSuffix(newPrefix, "/")
	if prefix != "" {
		prefix += "/"
	}

	target := strings.ReplaceAll(strings.TrimSpace(rawTarget), `\`, `/`)
	i := strings.LastIndex(target, "/")
	if i < 0 || i == len(target)-1 {
		return "", false
	}
	filename := target[i+1:]
	parent := target[:i]

	if parent == "media" || strings.HasSuffix(parent, "/media") {
		return prefix + filename, true
	}
	return "", false
}
