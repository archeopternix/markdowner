// Package markdowner defines the public model and service interfaces for
// document import, storage, and export.
//
// The package itself exposes stable contracts such as Document, Frontmatter,
// DocumentStore, Importer, Exporter, and filesystem abstractions.
//
// Typical usage composes a concrete store from subpackages:
//   - app/localstore provides a ReaderWriterFS implementation for local disks.
//   - app/bootstrap wires a DocumentStore with built-in importers/exporters.
//
// Importers normalize source files (for example Markdown, TXT, VTT, and DOCX)
// into Document, while exporters convert Document into target formats.
package markdowner

