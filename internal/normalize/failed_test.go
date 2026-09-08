package normalize_test

import (
	"testing"
	"time"

	"github.com/ulpf/ulpf/internal/normalize"
	"github.com/ulpf/ulpf/internal/schema"
)

// TestFailedEventNeverDrops proves the total-failure path: even with no
// parser match at all, the resulting event still carries a valid raw
// pointer and lineage, is never nil, and is explicitly marked failed rather
// than silently discarded.
func TestFailedEventNeverDrops(t *testing.T) {
	raw := schema.Raw{
		SHA256:       "9a134e421a5579eba9617f5817d6f2929755b22f2c12eef31b4a89a955589bf5",
		SegmentID:    "seg-1",
		RetrievalURI: "s3a://ulpf-raw/segments/dt=2026-09-07/seg-1.zst",
	}
	lineage := schema.Lineage{ParserID: "source.unknown", ParserVersion: "", NodeID: "node-1"}
	now := time.Now().UnixNano()

	e := normalize.FailedEvent("evt-1", now, raw, lineage)

	if e == nil {
		t.Fatal("FailedEvent must never return nil")
	}
	if e.Lineage.ParseStatus != schema.ParseStatusFailed {
		t.Errorf("parse_status = %q, want %q", e.Lineage.ParseStatus, schema.ParseStatusFailed)
	}
	if e.Raw.SHA256 != raw.SHA256 {
		t.Error("raw pointer not preserved on the failure path")
	}
	if e.Quality.Score != 0 {
		t.Errorf("quality.score = %v, want 0", e.Quality.Score)
	}
	if e.Unmapped == nil {
		t.Error("unmapped must be initialized, not nil, even on total failure")
	}
	if err := e.Validate(); err != nil {
		t.Errorf("failed event must still be valid UES: %v", err)
	}
}
