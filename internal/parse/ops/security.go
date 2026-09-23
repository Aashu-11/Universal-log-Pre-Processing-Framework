package ops

import (
	"fmt"
	"strings"

	"github.com/logkrama/logkrama/internal/parse/dsl"
	"github.com/logkrama/logkrama/internal/parse/fields"
)

func init() {
	register("cef", buildCEF)
	register("leef", buildLEEF)
}

// buildCEF parses ArcSight CEF: "CEF:Version|Vendor|Product|Version|
// SignatureID|Name|Severity|Extension", where Extension is space-separated
// key=value pairs (values may contain escaped spaces/pipes, honored here).
func buildCEF(spec dsl.Operator, _ Deps) (Op, error) {
	return func(f *fields.Fields, raw []byte) error {
		s := sourceString(spec, f, raw)
		if !strings.HasPrefix(s, "CEF:") {
			return fmt.Errorf("cef: missing CEF: prefix")
		}
		// 7 header fields (Version..Severity) plus one Extension field = 8
		// parts from 7 unescaped pipe splits.
		parts := splitUnescaped(s[4:], '|', 8)
		if len(parts) < 7 {
			return fmt.Errorf("cef: expected 7 header fields, got %d", len(parts))
		}
		f.Set("cef_version", parts[0])
		f.Set("device_vendor", parts[1])
		f.Set("device_product", parts[2])
		f.Set("device_version", parts[3])
		f.Set("signature_id", parts[4])
		f.Set("name", parts[5])
		f.Set("severity", parts[6])

		if len(parts) == 8 {
			for _, pair := range splitKVPairs(parts[7], ' ') {
				i := strings.Index(pair, "=")
				if i < 0 {
					continue
				}
				f.Set(strings.TrimSpace(pair[:i]), pair[i+1:])
			}
		}
		return nil
	}, nil
}

// buildLEEF parses IBM QRadar LEEF: "LEEF:Version|Vendor|Product|Version|
// EventID|[Delimiter|]Extension". LEEF 1.0 extensions are tab-separated;
// LEEF 2.0 adds an explicit delimiter field right before the extension.
func buildLEEF(spec dsl.Operator, _ Deps) (Op, error) {
	return func(f *fields.Fields, raw []byte) error {
		s := sourceString(spec, f, raw)
		if !strings.HasPrefix(s, "LEEF:") {
			return fmt.Errorf("leef: missing LEEF: prefix")
		}
		body := s[5:]
		version := body[:strings.IndexByte(body, '|')]
		parts := strings.SplitN(body, "|", 6)
		if len(parts) < 5 {
			return fmt.Errorf("leef: expected at least 5 header fields, got %d", len(parts))
		}
		f.Set("leef_version", parts[0])
		f.Set("device_vendor", parts[1])
		f.Set("device_product", parts[2])
		f.Set("device_version", parts[3])
		f.Set("event_id", parts[4])

		delim := byte('\t')
		ext := ""
		if strings.HasPrefix(version, "2.0") && len(parts) == 6 {
			delimField := parts[5]
			if i := strings.IndexByte(delimField, '|'); i >= 0 {
				if len(delimField[:i]) == 1 {
					delim = delimField[i-1]
				}
				ext = delimField[i+1:]
			} else {
				ext = delimField
			}
		} else if len(parts) >= 5 {
			ext = strings.Join(parts[5:], "|")
		}

		for _, pair := range splitKVPairs(ext, delim) {
			i := strings.Index(pair, "=")
			if i < 0 {
				continue
			}
			f.Set(strings.TrimSpace(pair[:i]), pair[i+1:])
		}
		return nil
	}, nil
}

// splitUnescaped splits s on sep up to maxParts times, treating "\<sep>" as
// a literal (escaped) separator rather than a boundary, per CEF's escaping
// rules. The final element (the extension) is left un-split even if it
// contains sep, since maxParts caps the header fields only.
func splitUnescaped(s string, sep byte, maxParts int) []string {
	var out []string
	var cur strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) && s[i+1] == sep {
			cur.WriteByte(sep)
			i++
			continue
		}
		if s[i] == sep && len(out) < maxParts-1 {
			out = append(out, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteByte(s[i])
	}
	out = append(out, cur.String())
	return out
}
