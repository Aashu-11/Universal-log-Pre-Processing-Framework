// Package ops implements the individual parse operators (syslog, dissect,
// grok, regex, json, xml, csv, kv, cef, leef, date, convert, lookup,
// rename, copy, drop_field, gsub, split, conditional).
package ops

import (
	"fmt"

	"github.com/ulpf/ulpf/internal/parse/dsl"
	"github.com/ulpf/ulpf/internal/parse/fields"
)

// Op is one compiled, ready-to-run pipeline step. Compilation (regex
// compiling, dissect pattern splitting, etc.) happens once in Build; Apply
// is the hot-path call, once per event, and must not compile or allocate
// more than necessary.
type Op func(f *fields.Fields, raw []byte) error

// Deps are the compile-time dependencies an operator's Build function may
// need beyond its own YAML spec — currently just named dictionaries for the
// `lookup` operator. Kept minimal and separate from Phase 4's normalize-time
// dictionaries (config/dictionaries/*), which operate on UES fields, not
// raw extracted ones.
type Deps struct {
	Dictionaries map[string]map[string]string
}

// Builder compiles one dsl.Operator spec into a runtime Op.
type Builder func(spec dsl.Operator, deps Deps) (Op, error)

var registry = map[string]Builder{}

func register(name string, b Builder) {
	registry[name] = b
}

// Build compiles spec using the registered builder for spec.Op.
func Build(spec dsl.Operator, deps Deps) (Op, error) {
	b, ok := registry[spec.Op]
	if !ok {
		return nil, fmt.Errorf("ops: unknown operator %q", spec.Op)
	}
	op, err := b(spec, deps)
	if err != nil {
		return nil, fmt.Errorf("ops: build %q: %w", spec.Op, err)
	}
	return op, nil
}
