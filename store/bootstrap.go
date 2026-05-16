package store

import (
	"io"

	"github.com/archeopternix/markdowner"

	expdocx "github.com/archeopternix/markdowner/exporters/docx"
	expmd "github.com/archeopternix/markdowner/exporters/markdown"
	impdocx "github.com/archeopternix/markdowner/importers/docx"
	impmd "github.com/archeopternix/markdowner/importers/markdown"
	impvtt "github.com/archeopternix/markdowner/importers/vtt"
	"github.com/archeopternix/markdowner/internal/format"
)

// NewDocumentStore constructs a store backed by the provided ReaderWriterFS
// and wires built-in importers/exporters for out-of-the-box usage.
func NewDocumentStore(rwfs markdowner.ReaderWriterFS) markdowner.DocumentStore {
	ds := New(rwfs, storeMimeDetector{})
	registerBuiltins(ds)
	return ds
}

type storeMimeDetector struct{}

func (storeMimeDetector) Detect(path string, r io.Reader) (string, error) {
	m, err := format.DetectMime(path, r)
	if err != nil {
		return "", err
	}
	return m.MimeType, nil
}

func registerBuiltins(ds markdowner.DocumentStore) {
	ds.Importers().Register(impmd.New())
	ds.Importers().Register(impdocx.New())
	ds.Importers().Register(impvtt.New())

	ds.Exporters().Register(expmd.New())
	ds.Exporters().Register(expdocx.New())
}
