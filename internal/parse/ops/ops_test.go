package ops

import (
	"testing"

	"github.com/logkrama/logkrama/internal/parse/dsl"
	"github.com/logkrama/logkrama/internal/parse/fields"
)

func run(t *testing.T, spec dsl.Operator, deps Deps, raw string, seed map[string]string) *fields.Fields {
	t.Helper()
	op, err := Build(spec, deps)
	if err != nil {
		t.Fatalf("Build(%s): %v", spec.Op, err)
	}
	f := fields.New(8)
	for k, v := range seed {
		f.Set(k, v)
	}
	if err := op(f, []byte(raw)); err != nil {
		t.Fatalf("Apply(%s): %v", spec.Op, err)
	}
	return f
}

func TestSyslogRFC3164(t *testing.T) {
	f := run(t, dsl.Operator{Op: "syslog"}, Deps{}, "<166>Sep 07 2026 14:02:11 asa-fw-01 : %ASA-6-302013: Built outbound TCP connection", nil)
	if f.GetString("hostname") != "asa-fw-01" {
		t.Errorf("hostname = %q", f.GetString("hostname"))
	}
	if got, _ := f.Get("pri"); got != 166 {
		t.Errorf("pri = %v", got)
	}
	if got, _ := f.Get("facility"); got != 20 {
		t.Errorf("facility = %v, want 20", got)
	}
}

func TestSyslogRFC5424(t *testing.T) {
	f := run(t, dsl.Operator{Op: "syslog"}, Deps{}, `<34>1 2026-09-07T14:02:11Z pa-edge-01 panos 1234 ID47 - the message body`, nil)
	if f.GetString("hostname") != "pa-edge-01" {
		t.Errorf("hostname = %q", f.GetString("hostname"))
	}
	if f.GetString("appname") != "panos" {
		t.Errorf("appname = %q", f.GetString("appname"))
	}
	if f.GetString("message") != "the message body" {
		t.Errorf("message = %q", f.GetString("message"))
	}
}

func TestDissectLinearWalk(t *testing.T) {
	spec := dsl.Operator{Op: "dissect", Pattern: "%{srcip}:%{srcport} -> %{dstip}:%{dstport}"}
	f := run(t, spec, Deps{}, "203.0.113.5:51514 -> 10.0.0.10:443", nil)
	if f.GetString("srcip") != "203.0.113.5" || f.GetString("dstport") != "443" {
		t.Errorf("got srcip=%q dstport=%q", f.GetString("srcip"), f.GetString("dstport"))
	}
}

func TestRegexNamedGroups(t *testing.T) {
	spec := dsl.Operator{Op: "regex", Pattern: `srcip=(?P<srcip>\S+) dstport=(?P<dstport>\d+)`}
	f := run(t, spec, Deps{}, "action=allow srcip=1.2.3.4 dstport=443 done", nil)
	if f.GetString("srcip") != "1.2.3.4" || f.GetString("dstport") != "443" {
		t.Errorf("got srcip=%q dstport=%q", f.GetString("srcip"), f.GetString("dstport"))
	}
}

func TestGrokExpansion(t *testing.T) {
	spec := dsl.Operator{Op: "grok", Pattern: `srcip=%{IP:srcip} dstport=%{INT:dstport}`}
	f := run(t, spec, Deps{}, "srcip=10.1.1.1 dstport=8080", nil)
	if f.GetString("srcip") != "10.1.1.1" || f.GetString("dstport") != "8080" {
		t.Errorf("got srcip=%q dstport=%q", f.GetString("srcip"), f.GetString("dstport"))
	}
}

func TestGrokUnknownPatternRejected(t *testing.T) {
	spec := dsl.Operator{Op: "grok", Pattern: `%{NOPE:x}`}
	if _, err := Build(spec, Deps{}); err == nil {
		t.Fatal("expected error for unknown grok pattern")
	}
}

func TestUnsafeRegexRejected(t *testing.T) {
	spec := dsl.Operator{Op: "regex", Pattern: `(a+)+$`}
	if _, err := Build(spec, Deps{}); err == nil {
		t.Fatal("expected nested-quantifier regex to be rejected")
	}
}

func TestJSONFlatten(t *testing.T) {
	spec := dsl.Operator{Op: "json"}
	f := run(t, spec, Deps{}, `{"a":"x","b":5,"c":{"nested":true}}`, nil)
	if f.GetString("a") != "x" {
		t.Errorf("a = %q", f.GetString("a"))
	}
	if v, _ := f.Get("b"); v != float64(5) {
		t.Errorf("b = %v", v)
	}
	if f.GetString("c") == "" {
		t.Error("nested object should be retained as JSON string, not dropped")
	}
}

func TestCSVWithColumns(t *testing.T) {
	spec := dsl.Operator{Op: "csv", Columns: []string{"vendor", "action", "_", "bytes"}}
	f := run(t, spec, Deps{}, "paloalto,allow,skip-me,6000", nil)
	if f.GetString("vendor") != "paloalto" || f.GetString("bytes") != "6000" {
		t.Errorf("got vendor=%q bytes=%q", f.GetString("vendor"), f.GetString("bytes"))
	}
	if _, ok := f.Get("col_3"); ok {
		t.Error("column marked _ should be skipped, not stored as col_3")
	}
}

func TestKVWithQuotedValues(t *testing.T) {
	spec := dsl.Operator{Op: "kv"}
	f := run(t, spec, Deps{}, `srcip=1.2.3.4 msg="deny by policy id 5" dstport=443`, nil)
	if f.GetString("msg") != "deny by policy id 5" {
		t.Errorf("msg = %q", f.GetString("msg"))
	}
	if f.GetString("dstport") != "443" {
		t.Errorf("dstport = %q", f.GetString("dstport"))
	}
}

func TestCEFParsing(t *testing.T) {
	spec := dsl.Operator{Op: "cef"}
	f := run(t, spec, Deps{}, `CEF:0|PaloAlto|PAN-OS|10.2|100|Traffic|3|src=1.2.3.4 dst=5.6.7.8 spt=51514`, nil)
	if f.GetString("device_vendor") != "PaloAlto" {
		t.Errorf("device_vendor = %q", f.GetString("device_vendor"))
	}
	if f.GetString("src") != "1.2.3.4" {
		t.Errorf("src = %q", f.GetString("src"))
	}
}

func TestLEEFParsing(t *testing.T) {
	spec := dsl.Operator{Op: "leef"}
	f := run(t, spec, Deps{}, "LEEF:1.0|Fortinet|FortiGate|6.0|traffic|src=1.2.3.4\tdst=5.6.7.8", nil)
	if f.GetString("device_vendor") != "Fortinet" {
		t.Errorf("device_vendor = %q", f.GetString("device_vendor"))
	}
	if f.GetString("src") != "1.2.3.4" || f.GetString("dst") != "5.6.7.8" {
		t.Errorf("src=%q dst=%q", f.GetString("src"), f.GetString("dst"))
	}
}

func TestConvertTypes(t *testing.T) {
	f := run(t, dsl.Operator{Op: "convert", Field: "n", Type: "int"}, Deps{}, "", map[string]string{"n": "42"})
	if v, _ := f.Get("n"); v != int64(42) {
		t.Errorf("n = %v (%T)", v, v)
	}
}

func TestRenameCopyDrop(t *testing.T) {
	f := run(t, dsl.Operator{Op: "rename", From: "a", To: "b"}, Deps{}, "", map[string]string{"a": "1"})
	if f.GetString("b") != "1" {
		t.Fatal("rename failed")
	}
	if _, ok := f.Get("a"); ok {
		t.Fatal("rename should remove original field")
	}

	f = run(t, dsl.Operator{Op: "copy", From: "x", To: "y"}, Deps{}, "", map[string]string{"x": "v"})
	if f.GetString("y") != "v" || f.GetString("x") != "v" {
		t.Fatal("copy should keep both fields")
	}

	f = run(t, dsl.Operator{Op: "drop_field", Field: "z"}, Deps{}, "", map[string]string{"z": "gone"})
	if _, ok := f.Get("z"); ok {
		t.Fatal("drop_field failed")
	}
}

func TestGsubAndSplit(t *testing.T) {
	f := run(t, dsl.Operator{Op: "gsub", Field: "s", Match: `\s+`, Replacement: "_"}, Deps{}, "", map[string]string{"s": "a  b   c"})
	if f.GetString("s") != "a_b_c" {
		t.Errorf("gsub = %q", f.GetString("s"))
	}

	idx1 := 1
	f = run(t, dsl.Operator{Op: "split", Field: "path", Separator: "/", Index: &idx1}, Deps{}, "", map[string]string{"path": "ethernet1/2"})
	if f.GetString("path") != "2" {
		t.Errorf("split = %q", f.GetString("path"))
	}
}

func TestLookupWithDictionaryAndDefault(t *testing.T) {
	deps := Deps{Dictionaries: map[string]map[string]string{
		"action": {"allow": "allowed", "deny": "blocked"},
	}}
	f := run(t, dsl.Operator{Op: "lookup", Field: "a", Dictionary: "action", Default: "unknown"}, deps, "", map[string]string{"a": "allow"})
	if f.GetString("a") != "allowed" {
		t.Errorf("lookup = %q", f.GetString("a"))
	}
	f = run(t, dsl.Operator{Op: "lookup", Field: "a", Dictionary: "action", Default: "unknown"}, deps, "", map[string]string{"a": "weird"})
	if f.GetString("a") != "unknown" {
		t.Errorf("lookup default = %q", f.GetString("a"))
	}
}

func TestConditional(t *testing.T) {
	spec := dsl.Operator{
		Op:    "conditional",
		Field: "action",
		When:  &dsl.Predicate{Contains: "deny"},
		Then:  []dsl.Operator{{Op: "rename", From: "action", To: "outcome_blocked"}},
		Else:  []dsl.Operator{{Op: "rename", From: "action", To: "outcome_allowed"}},
	}
	f := run(t, spec, Deps{}, "", map[string]string{"action": "deny-policy-5"})
	if f.GetString("outcome_blocked") != "deny-policy-5" {
		t.Errorf("conditional then-branch failed: %+v", f)
	}

	f = run(t, spec, Deps{}, "", map[string]string{"action": "allow-policy-1"})
	if f.GetString("outcome_allowed") != "allow-policy-1" {
		t.Errorf("conditional else-branch failed: %+v", f)
	}
}
