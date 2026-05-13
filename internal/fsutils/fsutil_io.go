package fsutil

import (
	"bufio"
	"io"
	"io/fs"
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
