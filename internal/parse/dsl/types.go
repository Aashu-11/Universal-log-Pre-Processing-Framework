// Package dsl defines the YAML Parser artifact schema (metadata, match
// predicates, operator pipeline, on_failure policy).
package dsl

// Parser is one packs/<vendor>/<product>/<class>.yaml artifact: everything
// needed to recognize and extract fields from one log source, compiled once
// at load time into a runtime Plan (see internal/parse/compile.go). Parsers
// are data, never Go code — this is what makes onboarding a new source a
// hot-swap instead of a redeploy.
type Parser struct {
	Metadata  Metadata   `yaml:"metadata"`
	Match     Match      `yaml:"match"`
	Pipeline  []Operator `yaml:"pipeline"`
	OnFailure string     `yaml:"on_failure"` // "emit_partial" | "dlq"
}

type Metadata struct {
	ID           string `yaml:"id"`
	Version      string `yaml:"version"`
	Vendor       string `yaml:"vendor"`
	Product      string `yaml:"product"`
	ObserverType string `yaml:"observer_type"`
	EventClass   string `yaml:"event_class"`
}

// Match decides whether this parser applies to a given raw event, used by
// internal/identify's signature-match tier. All Any predicates are OR'd
// together; every All predicate must hold. Priority breaks ties between
// multiple matching parsers (higher wins); Confidence is surfaced on the
// resulting event's lineage/quality path.
type Match struct {
	Priority   int         `yaml:"priority"`
	All        []Predicate `yaml:"all"`
	Any        []Predicate `yaml:"any"`
	Confidence float64     `yaml:"confidence"`
}

// Predicate is a single structural/content test against the raw event
// bytes. Exactly one of these fields should be set per predicate.
type Predicate struct {
	Contains string `yaml:"contains,omitempty"`
	Prefix   string `yaml:"prefix,omitempty"`
	Regex    string `yaml:"regex,omitempty"`
}

// Operator is one pipeline step. Op selects which operator runs; the
// remaining fields are that operator's own config — only the ones it reads
// are meaningful, the rest are ignored, which keeps this one flat struct
// instead of needing per-operator YAML type discrimination.
type Operator struct {
	Op string `yaml:"op"`

	// Common
	Field string `yaml:"field,omitempty"`
	As    string `yaml:"as,omitempty"`
	From  string `yaml:"from,omitempty"`
	To    string `yaml:"to,omitempty"`

	// dissect / grok / regex
	Pattern string `yaml:"pattern,omitempty"`

	// csv
	Columns   []string `yaml:"columns,omitempty"`
	Delimiter string   `yaml:"delimiter,omitempty"`

	// kv
	PairSep string `yaml:"pair_sep,omitempty"`
	KVSep   string `yaml:"kv_sep,omitempty"`

	// date
	Formats  []string `yaml:"formats,omitempty"`
	Timezone string   `yaml:"timezone,omitempty"`

	// convert
	Type string `yaml:"type,omitempty"` // "int" | "float" | "bool" | "string"

	// lookup
	Dictionary string `yaml:"dictionary,omitempty"`
	Default    string `yaml:"default,omitempty"`

	// gsub
	Match       string `yaml:"match,omitempty"`
	Replacement string `yaml:"replacement,omitempty"`

	// split
	Separator string `yaml:"separator,omitempty"`
	Index     *int   `yaml:"index,omitempty"`

	// conditional
	When *Predicate `yaml:"when,omitempty"`
	Then []Operator `yaml:"then,omitempty"`
	Else []Operator `yaml:"else,omitempty"`

	// drop_field / rename / copy operate purely via Field/As/From/To above.
}
