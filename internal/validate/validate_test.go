package validate_test

import (
	"testing"
	"time"

	"github.com/ulpf/ulpf/internal/schema"
	"github.com/ulpf/ulpf/internal/validate"
)

func sampleValidEvent() *schema.Event {
	return &schema.Event{
		Event: schema.EventMeta{
			ID: "018f4d2e-7b1a-7c3e-9f2a-1e6b4c8d0a11", Kind: "event",
			Dataset: "paloalto.panos.traffic", ObservedAt: time.Now().UnixNano(), IngestedAt: time.Now().UnixNano(),
		},
		Observer: schema.Observer{Vendor: "paloalto", Product: "panos", Type: "firewall"},
		Src:      schema.Src{IP: "203.0.113.5", Port: 51514},
		Dst:      schema.Dst{IP: "10.0.0.10", Port: 443},
		Raw: schema.Raw{
			SHA256:    "9a134e421a5579eba9617f5817d6f2929755b22f2c12eef31b4a89a955589bf5",
			SegmentID: "seg-1", RetrievalURI: "s3a://x",
		},
		Lineage:  schema.Lineage{ParserID: "paloalto.panos.traffic", ParserVersion: "1.0.0", ParseStatus: "ok", NodeID: "n1"},
		Quality:  schema.Quality{},
		Unmapped: map[string]string{},
	}
}

func TestValidEventPassesSchemaAndRanges(t *testing.T) {
	e := sampleValidEvent()
	res := validate.Event(e, 8, 10, 0.9, time.Now())
	if !res.SchemaValid {
		t.Fatalf("expected schema valid, violations: %+v", res.Violations)
	}
	if len(res.Violations) != 0 {
		t.Fatalf("unexpected violations: %+v", res.Violations)
	}
}

func TestBadPortDetected(t *testing.T) {
	e := sampleValidEvent()
	e.Src.Port = 99999
	res := validate.Event(e, 8, 10, 0.9, time.Now())
	if !hasReason(res, validate.ReasonBadPort) {
		t.Fatalf("expected bad_port violation, got %+v", res.Violations)
	}
}

func TestBadIPDetected(t *testing.T) {
	e := sampleValidEvent()
	e.Dst.IP = "not-an-ip"
	res := validate.Event(e, 8, 10, 0.9, time.Now())
	if !hasReason(res, validate.ReasonBadIP) {
		t.Fatalf("expected bad_ip violation, got %+v", res.Violations)
	}
}

func TestTimestampSkewDetected(t *testing.T) {
	e := sampleValidEvent()
	e.Event.ObservedAt = time.Now().Add(-2 * 365 * 24 * time.Hour).UnixNano()
	res := validate.Event(e, 8, 10, 0.9, time.Now())
	if !hasReason(res, validate.ReasonTimestampSkew) {
		t.Fatalf("expected timestamp_out_of_range violation, got %+v", res.Violations)
	}
}

func TestQualityScoreFormula(t *testing.T) {
	cases := []struct {
		mapped, expected int
		confidence, want float64
	}{
		{8, 10, 0.9, 0.72},
		{10, 10, 1.0, 1.0},
		{20, 10, 1.0, 1.0}, // clamped at 1.0 even if mapped > expected
		{0, 10, 1.0, 0.0},
		{5, 0, 1.0, 0.0}, // no division by zero
	}
	for _, c := range cases {
		got := validate.ComputeQualityScore(c.mapped, c.expected, c.confidence)
		if diff := got - c.want; diff > 1e-9 || diff < -1e-9 {
			t.Errorf("ComputeQualityScore(%d,%d,%v) = %v, want %v", c.mapped, c.expected, c.confidence, got, c.want)
		}
	}
}

func hasReason(res validate.Result, reason string) bool {
	for _, v := range res.Violations {
		if v.Reason == reason {
			return true
		}
	}
	return false
}
