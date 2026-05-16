package fsutil

import (
	"archive/zip"
	"bufio"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ReadAll opens name from rfs and reads it fully.
func ReadAll(rfs fs.FS, name string) ([]byte, error) {
	f, err := rfs.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	return io.ReadAll(f)
}

// WriteAll writes b to w using buffering.
func WriteAll(w io.WriteCloser, b []byte) error {
	bw := bufio.NewWriter(w)
	_, errWrite := bw.Write(b)
	errFlush := bw.Flush()
	errClose := w.Close()
	if errWrite != nil {
		return errWrite
	}
	if errFlush != nil {
		return errFlush
	}
	return errClose
}

// ReadZipFile opens name from zr and reads it fully.
func ReadZipFile(zr *zip.ReadCloser, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, os.ErrNotExist
}

// ListAllFiles returns all file paths (not directories) under dirpath.
// Paths are returned as full paths as encountered by WalkDir (you can Rel() them if needed).
func ListAllFiles(dirpath string) ([]string, error) {
	var paths []string

	err := filepath.WalkDir(dirpath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		paths = append(paths, path)
		return nil
	})

	if err != nil {
		return nil, err
	}
	return paths, nil
}

// CopyFile copies src -> dst, creating parent dirs.
func CopyFile(srcPath, dstPath string, perm fs.FileMode) error {
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

// CopyDir recursively copies contents of srcDir into dstDir.
func CopyDir(srcDir, dstDir string) error {
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
		return CopyFile(path, dstPath, info.Mode())
	})
}
