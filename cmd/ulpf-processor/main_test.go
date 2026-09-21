package main

import (
	"context"
	"testing"
	"time"

	"github.com/ulpf/ulpf/internal/vault"
	"github.com/ulpf/ulpf/internal/vault/store"
)

// TestReadWithRetrySucceedsImmediatelyOnSealedSegment guards the common
// case: once a segment has sealed, a read must not pay any retry latency.
func TestReadWithRetrySucceedsImmediatelyOnSealedSegment(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	v := vault.New(st, vault.Config{NodeID: "test"})

	refs, err := v.WriteBatch(context.Background(), [][]byte{[]byte("hello")})
	if err != nil {
		t.Fatalf("WriteBatch: %v", err)
	}
	if err := v.Seal(context.Background()); err != nil {
		t.Fatalf("Seal: %v", err)
	}

	start := time.Now()
	raw, err := readWithRetry(context.Background(), v, refs[0], 5*time.Second)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("readWithRetry: %v", err)
	}
	if string(raw) != "hello" {
		t.Errorf("got %q, want %q", raw, "hello")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("readWithRetry took %v for an already-sealed segment, want near-instant", elapsed)
	}
}

// TestReadWithRetryRespectsCallerDeadline is a regression test for a real
// bug: the retry deadline used to be a hardcoded 20s constant, well under
// the collector's default ULPF_VAULT_SEGMENT_MAX_SECONDS (300s) — meaning a
// ref for an event near the front of a freshly-opened segment would be
// permanently, silently dropped long before that segment ever had a chance
// to seal. The deadline must be a caller-supplied parameter (wired from
// ULPF_VAULT_READ_RETRY_SECONDS in main()), not a fixed constant, and must
// actually be honored — this proves both: a short deadline gives up close
// to on time (not instantly, not way past it) for a segment that will never
// exist.
func TestReadWithRetryRespectsCallerDeadline(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	v := vault.New(st, vault.Config{NodeID: "test"})

	missingRef := vault.RawRef{SegmentID: "does-not-exist", Offset: 0, Length: 5, SHA256: "deadbeef"}

	deadline := 700 * time.Millisecond
	start := time.Now()
	_, err = readWithRetry(context.Background(), v, missingRef, deadline)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("readWithRetry: expected error for a segment that will never exist, got nil")
	}
	if elapsed < deadline {
		t.Errorf("readWithRetry gave up after %v, want at least the %v deadline honored", elapsed, deadline)
	}
	if elapsed > deadline+2*time.Second {
		t.Errorf("readWithRetry took %v, way past its %v deadline", elapsed, deadline)
	}
}

// TestMissingSegmentCacheAvoidsRepeatedFullDeadlineWaits is a regression
// test for a real, severe throughput bug found live: a batch of many
// events referencing the same permanently-orphaned segment (e.g. from an
// ungraceful collector restart, D-007) each independently paid the full
// readRetryDeadline before giving up — confirmed live at 364 events
// referencing one lost segment in a single Kafka partition, which at the
// production-appropriate 330s deadline is over 33 hours of pure serial
// waiting for that one partition alone. The fix: once a segment has been
// proven missing for one event, every other event referencing the same
// segment must fail immediately, not re-pay the deadline.
func TestMissingSegmentCacheAvoidsRepeatedFullDeadlineWaits(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	v := vault.New(st, vault.Config{NodeID: "test"})
	missing := newMissingSegmentCache()

	lostRef := vault.RawRef{SegmentID: "orphaned-segment", Offset: 0, Length: 5, SHA256: "deadbeef"}
	deadline := 300 * time.Millisecond

	// Event 1: pays the full deadline, same as today, then gets cached.
	start := time.Now()
	if !missing.has(lostRef.SegmentID) {
		if _, err := readWithRetry(context.Background(), v, lostRef, deadline); err == nil {
			t.Fatal("expected error for a segment that will never exist")
		}
		missing.add(lostRef.SegmentID)
	}
	firstElapsed := time.Since(start)
	if firstElapsed < deadline {
		t.Fatalf("first lookup returned in %v, want it to have paid the %v deadline", firstElapsed, deadline)
	}

	// Events 2-20: same segment, must all be near-instant via the cache —
	// simulating the rest of a batch referencing the same lost segment.
	start = time.Now()
	for i := 0; i < 19; i++ {
		if missing.has(lostRef.SegmentID) {
			continue // this is the fix: skip straight past, no readWithRetry call
		}
		t.Fatal("cache miss for a segment already proven missing")
	}
	restElapsed := time.Since(start)
	if restElapsed > 50*time.Millisecond {
		t.Fatalf("19 cached lookups took %v, want near-instant (no repeated deadline waits)", restElapsed)
	}
}
