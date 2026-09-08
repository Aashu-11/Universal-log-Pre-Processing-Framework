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
