// Package normalize maps parser-extracted fields into the Universal Event
// Schema via a separate Mapping artifact and value dictionaries.
package normalize

// Mapping is one packs/<vendor>/<product>/<class>.mapping.yaml artifact:
// how to project one parser's extracted fields onto UES paths. Kept as a
// file separate from the Parser artifact deliberately — see CLAUDE.md: the
// same extracted fields could target UES, ECS, OCSF or CEF with a different
// Mapping file, without touching the parser at all.
type Mapping struct {
	Metadata       MappingMetadata     `yaml:"metadata"`
	Fields         map[string]FieldMap `yaml:"fields"`
	UnmappedPolicy string              `yaml:"unmapped_policy"` // "retain" (the only supported value)
}

type MappingMetadata struct {
	ParserID string `yaml:"parser_id"`
	Version  string `yaml:"version"`
}

// FieldMap describes how to produce one UES field. Exactly one of From or
// Const is normally set; From reads an extracted field by name, Const is a
// fixed value the parser doesn't need to emit (e.g. observer.vendor).
type FieldMap struct {
	From       string         `yaml:"from,omitempty"`
	Const      string         `yaml:"const,omitempty"`
	Transform  string         `yaml:"transform,omitempty"` // "int" | "float" | "bool" | "string" (default)
	Default    string         `yaml:"default,omitempty"`
	SkipIf     string         `yaml:"skip_if,omitempty"`
	Dictionary string         `yaml:"dictionary,omitempty"`
	Timestamp  *TimestampSpec `yaml:"timestamp,omitempty"`
}

// TimestampSpec configures internal/normalize/timestamp.go's central
// timestamp resolution for one FieldMap targeting event.observed_at (or any
// other epoch-ns field).
type TimestampSpec struct {
	// Formats are Go reference-time layouts tried in order against the
	// string value of From. Leave empty (and set EpochUnit instead) when
	// From is already a numeric epoch, as with Fortinet's `eventtime`.
	Formats []string `yaml:"formats,omitempty"`
	// Timezone is an IANA zone name, or "" for UTC. It's the zone that
	// naive (non-offset) formats are interpreted in.
	Timezone string `yaml:"timezone,omitempty"`
	// EpochUnit, if set, treats From's value as an already-numeric epoch
	// in this unit instead of parsing it as a formatted string: "seconds"
	// | "milliseconds" | "nanoseconds".
	EpochUnit string `yaml:"epoch_unit,omitempty"`
	// InferMissingYear enables December/January rollover-aware year
	// inference for layouts (like RFC3164's) that carry no year — see
	// timestamp.go's resolveMissingYear.
	InferMissingYear bool `yaml:"infer_missing_year,omitempty"`
}
