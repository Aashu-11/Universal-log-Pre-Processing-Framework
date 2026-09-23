package shape

import (
	"fmt"
	"strings"

	"github.com/logkrama/logkrama/internal/schema"
)

// CEF renders e as one ArcSight CEF line — the reverse direction of
// internal/parse/ops's cef.go operator, proving a normalized event can be
// re-exported to a CEF-speaking SIEM just as easily as it was ingested from
// one. Header fields escape "\" and "|"; extension values escape "\" and
// "=", per the CEF spec.
func CEF(e *schema.Event) string {
	vendor := orDefault(e.Observer.Vendor, "unknown")
	product := orDefault(e.Observer.Product, "unknown")
	version := orDefault(e.Observer.Version, "1.0")
	sigID := orDefault(e.Threat.SignatureID, e.Event.Dataset)
	name := orDefault(e.Event.Action, e.Event.Type)

	header := fmt.Sprintf("CEF:0|%s|%s|%s|%s|%s|%d",
		cefEscapeHeader(vendor), cefEscapeHeader(product), cefEscapeHeader(version),
		cefEscapeHeader(sigID), cefEscapeHeader(name), e.Event.SeverityID)

	ext := []string{
		cefExt("src", e.Src.IP),
		cefExt("spt", intStr(e.Src.Port)),
		cefExt("dst", e.Dst.IP),
		cefExt("dpt", intStr(e.Dst.Port)),
		cefExt("proto", e.Network.Transport),
		cefExt("act", e.Event.Action),
		cefExt("outcome", e.Event.Outcome),
		cefExt("out", intStr64(e.Network.BytesOut)),
		cefExt("in", intStr64(e.Network.BytesIn)),
		cefExt("dhost", e.Dst.Hostname),
		cefExt("shost", e.Src.Hostname),
		cefExt("cs1", e.Event.Dataset),
		cefExt("cs1Label", "logkramaDataset"),
	}
	var nonEmpty []string
	for _, e := range ext {
		if e != "" {
			nonEmpty = append(nonEmpty, e)
		}
	}
	return header + "|" + strings.Join(nonEmpty, " ")
}

func cefExt(key, val string) string {
	if val == "" {
		return ""
	}
	return key + "=" + cefEscapeExtension(val)
}

func cefEscapeHeader(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `|`, `\|`)
	return s
}

func cefEscapeExtension(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `=`, `\=`)
	return s
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func intStr(n int) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d", n)
}

func intStr64(n int64) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprintf("%d", n)
}
