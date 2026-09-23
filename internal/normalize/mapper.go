package normalize

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/logkrama/logkrama/internal/parse/fields"
	"github.com/logkrama/logkrama/internal/schema"
)

// Report summarizes one Apply call, feeding internal/validate's quality
// score (mapped_fields / expected_for_class * parse_confidence).
type Report struct {
	MappedFields   int
	UnmappedFields int
	Errors         []string
}

// Mapper applies one compiled Mapping to a parser's extracted Fields.
type Mapper struct {
	mapping Mapping
	dicts   map[string]map[string]string
}

// NewMapper builds a Mapper. dicts is every loaded dictionary, keyed by
// name (see config/dictionaries/ and LoadDictionaries).
func NewMapper(m Mapping, dicts map[string]map[string]string) *Mapper {
	return &Mapper{mapping: m, dicts: dicts}
}

// Apply projects f onto a new *schema.Event's UES fields. It never returns
// an error for a single bad field mapping — that field is simply skipped
// (and reported in Report.Errors) — because one malformed field must not
// discard everything else extracted, per CLAUDE.md's "never discard"
// principle. Event.Raw/Lineage/Quality are left zero-valued; the caller
// (the Phase 6 processor pipeline) fills those in from context this
// package doesn't have (vault ref, parser id/version, node id).
func (m *Mapper) Apply(f *fields.Fields, now time.Time) (*schema.Event, Report) {
	event := &schema.Event{Unmapped: map[string]string{}}
	report := Report{}
	consumed := make(map[string]bool, f.Len())

	for uesPath, fm := range m.mapping.Fields {
		val, from, ok := m.resolveValue(f, fm)
		if from != "" {
			consumed[from] = true
		}
		if !ok {
			continue
		}
		strVal := str(val)
		if fm.SkipIf != "" && strVal == fm.SkipIf {
			continue
		}
		if fm.Dictionary != "" {
			strVal = m.lookup(fm.Dictionary, strVal)
		}

		if fm.Timestamp != nil {
			ns, err := resolveTimestamp(strVal, *fm.Timestamp, now)
			if err != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", uesPath, err))
				continue
			}
			if err := setUESField(event, uesPath, ns); err != nil {
				report.Errors = append(report.Errors, err.Error())
				continue
			}
			report.MappedFields++
			continue
		}

		typed, err := coerce(strVal, fm.Transform)
		if err != nil {
			report.Errors = append(report.Errors, fmt.Sprintf("%s: %v", uesPath, err))
			continue
		}
		if err := setUESField(event, uesPath, typed); err != nil {
			report.Errors = append(report.Errors, err.Error())
			continue
		}
		report.MappedFields++
	}

	f.Each(func(k string, v any) {
		if consumed[k] {
			return
		}
		event.Unmapped[k] = str(v)
	})
	report.UnmappedFields = len(event.Unmapped)

	// event.kind is required by schema/ues-1.0.json and every mapping we
	// ship targets ordinary log events, not alerts/metrics/state — default
	// it here rather than requiring every Mapping author to remember a
	// `event.kind: {const: event}` line (a missing one fails UES schema
	// validation silently late, at the validate stage, instead of loudly
	// where the actual mistake is).
	if event.Event.Kind == "" {
		event.Event.Kind = "event"
	}

	deriveNetworkTotals(event)

	return event, report
}

// resolveValue returns (value, source-field-name, found). source-field-name
// is empty for Const-only mappings, since nothing was "consumed" from the
// extracted fields in that case.
func (m *Mapper) resolveValue(f *fields.Fields, fm FieldMap) (any, string, bool) {
	if fm.Const != "" {
		return fm.Const, "", true
	}
	if fm.From != "" {
		if v, ok := f.Get(fm.From); ok {
			return v, fm.From, true
		}
	}
	if fm.Default != "" {
		return fm.Default, "", true
	}
	return nil, "", false
}

// lookup is case-insensitive: vendors are wildly inconsistent about casing
// ("TCP" vs "tcp", "Allow" vs "allow"), and a dictionary miss silently
// falling through to the raw value is exactly the kind of "field
// technically extracted but not actually normalized" bug that's easy to
// miss in review, so we fold case before matching rather than requiring
// every dictionary to enumerate every casing variant.
func (m *Mapper) lookup(dictName, key string) string {
	dict, ok := m.dicts[dictName]
	if !ok {
		return key
	}
	if v, ok := dict[strings.ToLower(key)]; ok {
		return v
	}
	if def, ok := dict["_default"]; ok {
		return def
	}
	return key
}

func coerce(s, transform string) (any, error) {
	switch transform {
	case "int":
		n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("coerce %q to int: %w", s, err)
		}
		return n, nil
	case "float":
		n, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return nil, fmt.Errorf("coerce %q to float: %w", s, err)
		}
		return n, nil
	case "bool":
		b, err := strconv.ParseBool(strings.TrimSpace(s))
		if err != nil {
			return nil, fmt.Errorf("coerce %q to bool: %w", s, err)
		}
		return b, nil
	default:
		return s, nil
	}
}

// deriveNetworkTotals fills network.bytes_total from bytes_in+bytes_out
// when the mapping didn't set it explicitly — true for every vendor pack
// we ship, so this is a built-in derivation rather than something every
// Mapping author has to repeat.
func deriveNetworkTotals(e *schema.Event) {
	if e.Network.BytesTotal == 0 && (e.Network.BytesIn != 0 || e.Network.BytesOut != 0) {
		e.Network.BytesTotal = e.Network.BytesIn + e.Network.BytesOut
	}
}
