package fsutil

import (
	"fmt"
	"io"
	"io/fs"
	"path"
	"strings"
)

// WriterFS is a minimal write-capable filesystem used by temp write helpers.
// It matches the WriterFS concept in the simplified docpipe API.
//
// Create must create or truncate the file at name.
// MkdirAll must create directories.
// RemoveAll must delete file or directory trees.
//
// This file is intentionally standalone so it can be used from internal implementations.

type WriterFS interface {
	MkdirAll(path string, perm fs.FileMode) error
	RemoveAll(path string) error
	Create(name string, perm fs.FileMode) (io.WriteCloser, error)
}

// WriteFileBestEffort writes data to the destination file.
//
// Without a Rename operation in WriterFS, full atomic replacement cannot be guaranteed.
// This helper does a best-effort approach:
// - write to a temp sibling file
// - remove destination
// - write destination
// - remove temp
//
// If your WriterFS supports atomic rename, replace this helper with a rename-based implementation.
func WriteFileBestEffort(wfs WriterFS, name string, perm fs.FileMode, data []byte) error {
	dir := path.Dir(name)
	if dir != "." && dir != "" {
		if err := wfs.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	tmp := name + ".tmp"
	// ensure we don't end up with path traversal from weird names
	tmp = strings.TrimPrefix(path.Clean(tmp), "/")

	w, err := wfs.Create(tmp, perm)
	if err != nil {
		return err
	}
	if err := WriteAll(w, data); err != nil {
		_ = wfs.RemoveAll(tmp)
		return err
	}

	_ = wfs.RemoveAll(name)
	w2, err := wfs.Create(name, perm)
	if err != nil {
		_ = wfs.RemoveAll(tmp)
		return err
	}
	if err := WriteAll(w2, data); err != nil {
		_ = wfs.RemoveAll(tmp)
		return err
	}

	if err := wfs.RemoveAll(tmp); err != nil {
		return fmt.Errorf("cleanup temp file: %w", err)
	}
	return nil
}
