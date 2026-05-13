package registry

import (
	"context"

	. "github.com/archeopternix/markdowner"
)

// Registry is a small helper implementation of docpipe.ImporterRegistry.
// It is meant to be used by end users or by the docpipe store internally.

type Registry struct {
	list []Importer
}

func New(importers ...Importer) *Registry {
	r := &Registry{}
	for _, imp := range importers {
		r.Register(imp)
	}
	return r
}

func (r *Registry) Register(i Importer) {
	if i == nil {
		return
	}
	r.list = append(r.list, i)
}

func (r *Registry) List() []Importer {
	out := make([]Importer, 0, len(r.list))
	out = append(out, r.list...)
	return out
}

// Select returns the first importer whose Accept() returns true.
func (r *Registry) Select(ctx context.Context, src ImportSource) Importer {
	for _, imp := range r.list {
		if imp == nil {
			continue
		}
		if imp.Accept(ctx, src) {
			return imp
		}
	}
	return nil
}
