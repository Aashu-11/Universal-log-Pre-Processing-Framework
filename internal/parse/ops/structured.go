package ops

import (
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/ulpf/ulpf/internal/parse/dsl"
	"github.com/ulpf/ulpf/internal/parse/fields"
)

func init() {
	register("json", buildJSON)
	register("xml", buildXML)
	register("csv", buildCSV)
	register("kv", buildKV)
}

func sourceString(spec dsl.Operator, f *fields.Fields, raw []byte) string {
	if spec.Field != "" {
		return f.GetString(spec.Field)
	}
	return string(raw)
}

// buildJSON flattens a top-level JSON object into fields — nested
// objects/arrays are kept as their JSON-encoded string value rather than
// silently dropped, so nothing under a nested key is lost before
// normalization decides what to do with it.
func buildJSON(spec dsl.Operator, _ Deps) (Op, error) {
	return func(f *fields.Fields, raw []byte) error {
		s := sourceString(spec, f, raw)
		var doc map[string]any
		if err := json.Unmarshal([]byte(s), &doc); err != nil {
			return fmt.Errorf("json: unmarshal: %w", err)
		}
		for k, v := range doc {
			switch vv := v.(type) {
			case string:
				f.Set(k, vv)
			case float64, bool, nil:
				f.Set(k, vv)
			default:
				b, _ := json.Marshal(vv)
				f.Set(k, string(b))
			}
		}
		return nil
	}, nil
}

// xmlNode is a generic XML tree used to flatten arbitrary documents without
// requiring a schema up front.
type xmlNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content string     `xml:",chardata"`
	Nodes   []xmlNode  `xml:",any"`
}

// buildXML flattens element text content into dotted field names
// (parent.child) and attributes into parent.child_attr. Best-effort by
// design — arbitrary XML shapes vary too much for a single fixed schema.
func buildXML(spec dsl.Operator, _ Deps) (Op, error) {
	return func(f *fields.Fields, raw []byte) error {
		s := sourceString(spec, f, raw)
		var root xmlNode
		if err := xml.Unmarshal([]byte(s), &root); err != nil {
			return fmt.Errorf("xml: unmarshal: %w", err)
		}
		flattenXML(f, root.XMLName.Local, root)
		return nil
	}, nil
}

func flattenXML(f *fields.Fields, prefix string, n xmlNode) {
	for _, a := range n.Attrs {
		f.Set(prefix+"_"+a.Name.Local, a.Value)
	}
	if len(n.Nodes) == 0 {
		if text := strings.TrimSpace(n.Content); text != "" {
			f.Set(prefix, text)
		}
		return
	}
	for _, child := range n.Nodes {
		flattenXML(f, prefix+"."+child.XMLName.Local, child)
	}
}

// buildCSV parses one CSV/TSV line against spec.Columns (positional field
// names) using spec.Delimiter (default ",").
func buildCSV(spec dsl.Operator, _ Deps) (Op, error) {
	delim := ','
	if spec.Delimiter != "" {
		delim = rune(spec.Delimiter[0])
	}
	columns := spec.Columns

	return func(f *fields.Fields, raw []byte) error {
		s := sourceString(spec, f, raw)
		r := csv.NewReader(strings.NewReader(s))
		r.Comma = delim
		r.FieldsPerRecord = -1
		record, err := r.Read()
		if err != nil {
			return fmt.Errorf("csv: parse: %w", err)
		}
		for i, val := range record {
			name := fmt.Sprintf("col_%d", i+1)
			if i < len(columns) && columns[i] != "" && columns[i] != "_" {
				name = columns[i]
			} else if i < len(columns) {
				continue // explicit "_" or "" means skip this column
			}
			f.Set(name, val)
		}
		return nil
	}, nil
}

// buildKV parses "key=value pair_sep key=value ..." strings, honoring
// double-quoted values that may themselves contain the pair separator
// (e.g. FortiGate's `msg="deny by policy"`).
func buildKV(spec dsl.Operator, _ Deps) (Op, error) {
	pairSep := " "
	if spec.PairSep != "" {
		pairSep = spec.PairSep
	}
	kvSep := "="
	if spec.KVSep != "" {
		kvSep = spec.KVSep
	}

	return func(f *fields.Fields, raw []byte) error {
		s := sourceString(spec, f, raw)
		for _, pair := range splitKVPairs(s, pairSep[0]) {
			i := strings.Index(pair, kvSep)
			if i < 0 {
				continue
			}
			key := strings.TrimSpace(pair[:i])
			val := strings.Trim(pair[i+len(kvSep):], `"`)
			if key == "" {
				continue
			}
			f.Set(key, val)
		}
		return nil
	}, nil
}

// splitKVPairs splits on sep, but never inside a double-quoted value.
func splitKVPairs(s string, sep byte) []string {
	var out []string
	inQuotes := false
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			inQuotes = !inQuotes
		case sep:
			if !inQuotes {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}
