package normalize_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/logkrama/logkrama/internal/loggen"
	"github.com/logkrama/logkrama/internal/normalize"
	"github.com/logkrama/logkrama/internal/parse"
	"github.com/logkrama/logkrama/internal/parse/fields"
	"github.com/logkrama/logkrama/internal/parse/ops"
	"github.com/logkrama/logkrama/internal/schema"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

type harness struct {
	registry *parse.Registry
	mappings map[string]normalize.Mapping
	dicts    map[string]map[string]string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	root := repoRoot(t)

	registry := parse.NewRegistry(ops.Deps{})
	if _, err := registry.LoadDir(filepath.Join(root, "packs")); err != nil {
		t.Fatalf("load packs: %v", err)
	}
	mappings, err := normalize.LoadMappingsDir(filepath.Join(root, "packs"))
	if err != nil {
		t.Fatalf("load mappings: %v", err)
	}
	dicts, err := normalize.LoadDictionaries(filepath.Join(root, "config", "dictionaries"))
	if err != nil {
		t.Fatalf("load dictionaries: %v", err)
	}
	return &harness{registry: registry, mappings: mappings, dicts: dicts}
}

func (h *harness) normalize(t *testing.T, parserID string, raw []byte, now time.Time) (*fields.Fields, normalize.Report, *schema.Event) {
	t.Helper()
	plan, ok := h.registry.Get(parserID)
	if !ok {
		t.Fatalf("parser %q not loaded", parserID)
	}
	res := plan.Run(raw)
	mapping, ok := h.mappings[parserID]
	if !ok {
		t.Fatalf("mapping for %q not loaded", parserID)
	}
	mapper := normalize.NewMapper(mapping, h.dicts)
	event, report := mapper.Apply(res.Fields, now)
	return res.Fields, report, event
}

// TestZeroFieldLoss is the Phase 4 gate's core claim: every field the
// parser extracted is accounted for afterward — either it fed at least one
// UES field (referenced by some mapping's `from:`) or it survived verbatim
// in `unmapped`. Never both, never neither. Note this is a stronger and
// more direct check than counting report.MappedFields against extracted
// field count would be: MappedFields counts UES *output* fields, and one
// source field legitimately feeds two UES fields in some of our mappings
// (e.g. PAN-OS's receive_time drives both event.original_time_string and
// event.observed_at), so that count is not the same number as "distinct
// source fields consumed" — this test checks the real invariant directly.
func TestZeroFieldLoss(t *testing.T) {
	h := newHarness(t)
	gen := loggen.NewGenerator(7)
	now := time.Date(2026, 9, 7, 14, 2, 11, 0, time.UTC)

	cases := []struct {
		vendor   loggen.Vendor
		parserID string
	}{
		{loggen.VendorPaloAlto, "paloalto.panos.traffic"},
		{loggen.VendorFortinet, "fortinet.fortigate.traffic"},
		{loggen.VendorCiscoASA, "cisco.asa.302013"},
	}

	for _, c := range cases {
		line := gen.Line(c.vendor, now)
		extracted, report, event := h.normalize(t, c.parserID, []byte(line), now)

		mapping := h.mappings[c.parserID]
		referenced := make(map[string]bool)
		for _, fm := range mapping.Fields {
			if fm.From != "" {
				referenced[fm.From] = true
			}
		}

		for _, key := range extracted.Keys() {
			_, inUnmapped := event.Unmapped[key]
			inReferenced := referenced[key]
			if inUnmapped == inReferenced {
				t.Errorf("%s: field %q lost or double-counted: in_unmapped=%v in_referenced_mapping=%v",
					c.vendor, key, inUnmapped, inReferenced)
			}
		}
		if len(event.Unmapped) != report.UnmappedFields {
			t.Errorf("%s: Report.UnmappedFields(%d) != len(event.Unmapped)(%d)", c.vendor, report.UnmappedFields, len(event.Unmapped))
		}
		if report.MappedFields == 0 {
			t.Errorf("%s: zero fields mapped, mapping is broken", c.vendor)
		}
	}
}

func TestActionAndOutcomeDictionaries(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, 9, 7, 14, 2, 11, 0, time.UTC)

	line := "<14>Sep  7 14:02:11 pa-edge-01 1,2026/09/07 14:02:11,001,TRAFFIC,end,1.2.3.4,5.6.7.8,r1,ssl,untrust,trust,1,2,3,tcp,deny,4,5,6,7,host"
	_, _, ne := h.normalize(t, "paloalto.panos.traffic", []byte(line), now)
	if ne.Event.Action != "blocked" {
		t.Errorf("action = %q, want blocked", ne.Event.Action)
	}
	if ne.Event.Outcome != "failure" {
		t.Errorf("outcome = %q, want failure", ne.Event.Outcome)
	}
}

func TestPANOSSeverityInversion(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, 9, 7, 14, 2, 11, 0, time.UTC)
	// PRI 14 = facility 1, severity 6 (informational) -> UES severity_id 1.
	line := "<14>Sep  7 14:02:11 pa-edge-01 1,2026/09/07 14:02:11,001,TRAFFIC,end,1.2.3.4,5.6.7.8,r1,ssl,untrust,trust,1,2,3,tcp,allow,4,5,6,7,host"
	_, _, ne := h.normalize(t, "paloalto.panos.traffic", []byte(line), now)
	if ne.Event.SeverityID != 1 {
		t.Errorf("severity_id = %d, want 1", ne.Event.SeverityID)
	}
}

func TestNetworkBytesTotalDerived(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, 9, 7, 14, 2, 11, 0, time.UTC)
	line := "<14>Sep  7 14:02:11 pa-edge-01 1,2026/09/07 14:02:11,001,TRAFFIC,end,1.2.3.4,5.6.7.8,r1,ssl,untrust,trust,1,2,3,tcp,allow,1000,2000,6,7,host"
	_, _, ne := h.normalize(t, "paloalto.panos.traffic", []byte(line), now)
	if ne.Network.BytesTotal != 3000 {
		t.Errorf("bytes_total = %d, want 3000 (1000+2000 derived)", ne.Network.BytesTotal)
	}
}

func TestFortinetEpochTimestamp(t *testing.T) {
	h := newHarness(t)
	now := time.Date(2026, 9, 7, 14, 2, 11, 0, time.UTC)
	line := `<189>date=2026-09-07 time=14:02:11 devname="FG" devid="X" logid="0" type="traffic" srcip=1.2.3.4 srcport=1 dstip=5.6.7.8 dstport=2 action="accept" policyid=1 proto=6 service="HTTPS" sentbyte=1 rcvdbyte=1 duration=1 eventtime=1757253731`
	_, _, ne := h.normalize(t, "fortinet.fortigate.traffic", []byte(line), now)
	want := time.Unix(1757253731, 0).UTC()
	got := time.Unix(0, ne.Event.ObservedAt).UTC()
	if !got.Equal(want) {
		t.Errorf("observed_at = %v, want %v", got, want)
	}
}

func TestCiscoMissingYearDecJanRollover(t *testing.T) {
	// A pure RFC3164 layout (no year) parsed on a date near year boundary
	// must resolve to the correct calendar year, not the parser's zero
	// year. We drive this through the same resolveTimestamp logic the
	// mapping uses by using cisco's ACTUAL parser (which DOES carry a
	// year), so instead we test the underlying rollover behavior directly
	// via a synthetic mapping+event to exercise infer_missing_year.
	now := time.Date(2027, 1, 2, 0, 30, 0, 0, time.UTC)
	m := normalize.Mapping{
		Metadata: normalize.MappingMetadata{ParserID: "test.rollover"},
		Fields: map[string]normalize.FieldMap{
			"event.observed_at": {
				From: "ts",
				Timestamp: &normalize.TimestampSpec{
					Formats:          []string{"Jan 2 15:04:05"},
					InferMissingYear: true,
				},
			},
		},
	}
	mapper := normalize.NewMapper(m, nil)
	f := fields.New(1)
	f.Set("ts", "Dec 31 23:59:00")

	event, _ := mapper.Apply(f, now)
	got := time.Unix(0, event.Event.ObservedAt).UTC()
	if got.Year() != 2026 {
		t.Errorf("Dec 31 near a Jan-2 'now' resolved to year %d, want 2026 (rollover)", got.Year())
	}
	if got.Month() != time.December || got.Day() != 31 {
		t.Errorf("got %v, want Dec 31", got)
	}
}

func TestTwoDigitYearFormat(t *testing.T) {
	now := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	m := normalize.Mapping{
		Metadata: normalize.MappingMetadata{ParserID: "test.twodigit"},
		Fields: map[string]normalize.FieldMap{
			"event.observed_at": {
				From:      "ts",
				Timestamp: &normalize.TimestampSpec{Formats: []string{"01/02/06 15:04:05"}},
			},
		},
	}
	mapper := normalize.NewMapper(m, nil)
	f := fields.New(1)
	f.Set("ts", "09/07/26 14:02:11")

	event, _ := mapper.Apply(f, now)
	got := time.Unix(0, event.Event.ObservedAt).UTC()
	if got.Year() != 2026 || got.Month() != time.September || got.Day() != 7 {
		t.Errorf("got %v, want 2026-09-07", got)
	}
}

func TestDSTBoundary(t *testing.T) {
	// US DST spring-forward 2026-03-08 02:00 -> 03:00 America/New_York.
	// 2:30 AM local time does not exist that day; Go's time.Parse with a
	// named zone still resolves it (rolling forward), which is the
	// behavior we assert rather than crash/error.
	m := normalize.Mapping{
		Metadata: normalize.MappingMetadata{ParserID: "test.dst"},
		Fields: map[string]normalize.FieldMap{
			"event.observed_at": {
				From: "ts",
				Timestamp: &normalize.TimestampSpec{
					Formats:  []string{"2006-01-02 15:04:05"},
					Timezone: "America/New_York",
				},
			},
		},
	}
	mapper := normalize.NewMapper(m, nil)
	f := fields.New(1)
	f.Set("ts", "2026-03-08 02:30:00")

	event, report := mapper.Apply(f, time.Now())
	if len(report.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", report.Errors)
	}
	if event.Event.ObservedAt == 0 {
		t.Fatal("DST-boundary timestamp did not parse at all")
	}
}
