package vault

import (
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/logkrama/logkrama/internal/vault/store"
)

func newTestVault(t *testing.T) (*Vault, store.Store, string) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("new local store: %v", err)
	}
	v := New(st, Config{MaxSegmentBytes: 64 << 20, MaxSegmentAge: time.Hour, NodeID: "test-node"})
	return v, st, dir
}

// TestRoundTripByteExact writes 10,000 events of varying, adversarial byte
// content (invalid UTF-8, embedded newlines, null bytes, empty, 1MB
// oversized) and reads every one back, asserting byte-for-byte equality —
// the Phase 1 gate's core claim.
func TestRoundTripByteExact(t *testing.T) {
	v, _, _ := newTestVault(t)
	ctx := context.Background()

	const n = 10000
	items := make([][]byte, n)
	rng := rand.New(rand.NewPCG(42, 7))
	for i := range items {
		items[i] = randomAdversarialBytes(rng, i)
	}

	var allRefs []RawRef
	const batchSize = 200
	for i := 0; i < n; i += batchSize {
		end := i + batchSize
		if end > n {
			end = n
		}
		refs, err := v.WriteBatch(ctx, items[i:end])
		if err != nil {
			t.Fatalf("WriteBatch: %v", err)
		}
		allRefs = append(allRefs, refs...)
	}
	if err := v.Seal(ctx); err != nil {
		t.Fatalf("final seal: %v", err)
	}

	if len(allRefs) != n {
		t.Fatalf("got %d refs, want %d", len(allRefs), n)
	}

	for i, ref := range allRefs {
		got, err := v.Read(ctx, ref)
		if err != nil {
			t.Fatalf("Read(%d): %v", i, err)
		}
		if !bytes.Equal(got, items[i]) {
			t.Fatalf("event %d not byte-exact: got %d bytes, want %d bytes", i, len(got), len(items[i]))
		}
	}
}

// randomAdversarialBytes covers every byte-hostile shape the vault must
// round-trip exactly. The 1MB-oversized case is deliberately rare (one in
// 2000, not one in 7): it's pure-random incompressible data, and zstd spends
// real CPU failing to find patterns in it — at one-in-seven this test took
// nearly 8 minutes to compress ~1.4GB of noise. One oversized event is
// enough to prove the "oversized event" path works; the rest of the
// distribution stays cheap so the test suite stays fast.
func randomAdversarialBytes(rng *rand.Rand, i int) []byte {
	switch {
	case i%2000 == 1000:
		b := make([]byte, 1<<20) // 1MB oversized
		_, _ = crand.Read(b)
		return b
	case i%6 == 0:
		return nil // empty event
	case i%6 == 1:
		return []byte{0x00, 0x00, 0x00} // null bytes
	case i%6 == 2:
		return []byte("line one\nline two\r\nline three\x00end") // embedded newlines + null
	case i%6 == 3:
		return []byte{0xFF, 0xFE, 0xC0, 0xC1, 0xF5, 0x80, 0x80, 0x80} // invalid UTF-8
	default:
		n := 1 + rng.IntN(1024)
		b := make([]byte, n)
		_, _ = crand.Read(b)
		return b
	}
}

// TestIntegrityDetectsCorruption seals a segment, flips one bit directly in
// the stored object (simulating tampering after the fact), then asserts
// VerifyRange names that exact segment as FAILed while every other segment
// still PASSes.
func TestIntegrityDetectsCorruption(t *testing.T) {
	v, st, dir := newTestVault(t)
	ctx := context.Background()

	if _, err := v.WriteBatch(ctx, [][]byte{[]byte("event-a"), []byte("event-b")}); err != nil {
		t.Fatalf("write segment 1: %v", err)
	}
	if err := v.Seal(ctx); err != nil {
		t.Fatalf("seal segment 1: %v", err)
	}
	if _, err := v.WriteBatch(ctx, [][]byte{[]byte("event-c"), []byte("event-d")}); err != nil {
		t.Fatalf("write segment 2: %v", err)
	}
	if err := v.Seal(ctx); err != nil {
		t.Fatalf("seal segment 2: %v", err)
	}

	today := dayPartition(time.Now())

	before, err := VerifyRange(ctx, st, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("VerifyRange before corruption: %v", err)
	}
	if !before.Pass {
		t.Fatalf("expected PASS before corruption, got: %+v", before)
	}
	if len(before.Segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(before.Segments))
	}

	corruptedSegmentID := before.Segments[0].SegmentID
	segmentPath := filepath.Join(dir, "segments", "dt="+today, corruptedSegmentID+".zst")
	corruptOneByte(t, segmentPath)

	after, err := VerifyRange(ctx, st, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("VerifyRange after corruption: %v", err)
	}
	if after.Pass {
		t.Fatal("expected FAIL after corrupting one byte, got PASS")
	}

	var found bool
	for _, sr := range after.Segments {
		if sr.SegmentID == corruptedSegmentID {
			found = true
			if sr.Pass {
				t.Errorf("corrupted segment %s reported PASS", corruptedSegmentID)
			}
			t.Logf("corrupted segment correctly reported: %s FAIL: %s", sr.SegmentID, sr.Reason)
		} else if !sr.Pass {
			t.Errorf("uncorrupted segment %s reported FAIL: %s", sr.SegmentID, sr.Reason)
		}
	}
	if !found {
		t.Fatalf("corrupted segment id %s not present in report", corruptedSegmentID)
	}
}

// corruptOneByte flips one bit in the middle of the file at path, simulating
// tampering with a sealed segment after the fact.
func corruptOneByte(t *testing.T, path string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read segment file to corrupt: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("segment file %s is empty, cannot corrupt", path)
	}
	mid := len(b) / 2
	b[mid] ^= 0xFF
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatalf("write corrupted segment file: %v", err)
	}
}

func TestConcurrentWritesDoNotCorrupt(t *testing.T) {
	v, _, _ := newTestVault(t)
	ctx := context.Background()

	const goroutines = 8
	const perGoroutine = 500

	var wg sync.WaitGroup
	refsCh := make(chan []RawRef, goroutines)
	itemsCh := make(chan [][]byte, goroutines)
	errCh := make(chan error, goroutines)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			items := make([][]byte, perGoroutine)
			for i := range items {
				items[i] = []byte(fmt.Sprintf("g%d-event-%d-%s", g, i, hex.EncodeToString(randBytes(8))))
			}
			refs, err := v.WriteBatch(ctx, items)
			if err != nil {
				errCh <- err
				return
			}
			itemsCh <- items
			refsCh <- refs
		}(g)
	}
	wg.Wait()
	close(refsCh)
	close(itemsCh)
	close(errCh)

	for err := range errCh {
		t.Fatalf("concurrent WriteBatch error: %v", err)
	}
	if err := v.Seal(ctx); err != nil {
		t.Fatalf("final seal: %v", err)
	}

	var allItems [][][]byte
	var allRefs [][]RawRef
	for items := range itemsCh {
		allItems = append(allItems, items)
	}
	for refs := range refsCh {
		allRefs = append(allRefs, refs)
	}

	total := 0
	for gi, refs := range allRefs {
		items := allItems[gi]
		for i, ref := range refs {
			got, err := v.Read(ctx, ref)
			if err != nil {
				t.Fatalf("Read after concurrent write: %v", err)
			}
			if !bytes.Equal(got, items[i]) {
				t.Fatalf("concurrent write corrupted event: got %q want %q", got, items[i])
			}
			total++
		}
	}
	if total != goroutines*perGoroutine {
		t.Fatalf("got %d verified events, want %d", total, goroutines*perGoroutine)
	}
}

// TestBootstrapResumesChainAcrossRestart is a regression test for a real
// bug found while manually testing `logkramactl vault verify` against a store
// that had been written to by several restarted collector processes during
// a debugging session: every process restart created a fresh Vault with
// lastRoot=0, so the first segment sealed after each restart broke the
// Merkle chain against the previous process's last real root — a false
// integrity failure, not real tampering or loss. Bootstrap must let a new
// process resume the exact chain tip a prior process left off at.
func TestBootstrapResumesChainAcrossRestart(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("new local store: %v", err)
	}
	ctx := context.Background()

	// "Process A": writes and seals one segment, then "exits" (its Vault is
	// simply discarded — nothing more is done with v1).
	v1 := New(st, Config{MaxSegmentBytes: 64 << 20, MaxSegmentAge: time.Hour, NodeID: "test-node"})
	if _, err := v1.WriteBatch(ctx, [][]byte{[]byte("event-from-process-a")}); err != nil {
		t.Fatalf("process A WriteBatch: %v", err)
	}
	if err := v1.Seal(ctx); err != nil {
		t.Fatalf("process A Seal: %v", err)
	}

	// "Process B": a fresh Vault instance on the same store, simulating a
	// restart. Bootstrap must pick up where process A's chain left off.
	v2 := New(st, Config{MaxSegmentBytes: 64 << 20, MaxSegmentAge: time.Hour, NodeID: "test-node"})
	if err := v2.Bootstrap(ctx); err != nil {
		t.Fatalf("process B Bootstrap: %v", err)
	}
	if _, err := v2.WriteBatch(ctx, [][]byte{[]byte("event-from-process-b")}); err != nil {
		t.Fatalf("process B WriteBatch: %v", err)
	}
	if err := v2.Seal(ctx); err != nil {
		t.Fatalf("process B Seal: %v", err)
	}

	report, err := VerifyRange(ctx, st, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("VerifyRange: %v", err)
	}
	if !report.Pass {
		for _, sr := range report.Segments {
			t.Logf("segment %s chainOK=%v pass=%v reason=%q", sr.SegmentID, sr.ChainOK, sr.Pass, sr.Reason)
		}
		t.Fatal("chain verification FAILED across a Bootstrap-ped restart — restart should not break the chain")
	}
	if len(report.Segments) != 2 {
		t.Fatalf("got %d segments, want 2", len(report.Segments))
	}
}

func randBytes(n int) []byte {
	b := make([]byte, n)
	_, _ = crand.Read(b)
	return b
}

// TestSealIfStaleSealsAnAgedOutSegmentWithNoNewWrites is a regression test
// for a real bug found while load-testing against live Docker: WriteBatch
// only checks MaxSegmentAge reactively, on the next write — so a segment
// that receives no further writes (a load test ends, a source goes quiet)
// sat open indefinitely, and Kafka refs already published for its
// earlier-flushed events failed to read until unrelated traffic eventually
// arrived to trigger another WriteBatch call. SealIfStale must seal it on
// its own once the age threshold passes, with no new write required.
func TestSealIfStaleSealsAnAgedOutSegmentWithNoNewWrites(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("new local store: %v", err)
	}
	v := New(st, Config{MaxSegmentBytes: 64 << 20, MaxSegmentAge: 20 * time.Millisecond, NodeID: "test-node"})
	ctx := context.Background()

	refs, err := v.WriteBatch(ctx, [][]byte{[]byte("lonely-event")})
	if err != nil {
		t.Fatalf("WriteBatch: %v", err)
	}

	// Immediately after write, the segment is still fresh — SealIfStale must
	// be a no-op (must not seal every segment on every tick regardless of age,
	// which would fragment output into far too many tiny files).
	if err := v.SealIfStale(ctx); err != nil {
		t.Fatalf("SealIfStale (fresh): %v", err)
	}
	if _, err := v.Read(ctx, refs[0]); err == nil {
		t.Fatal("event readable before its segment aged out or sealed — test invariant broken")
	}

	time.Sleep(30 * time.Millisecond) // past MaxSegmentAge, with no further WriteBatch call

	if err := v.SealIfStale(ctx); err != nil {
		t.Fatalf("SealIfStale (stale): %v", err)
	}

	got, err := v.Read(ctx, refs[0])
	if err != nil {
		t.Fatalf("Read after SealIfStale: %v (event should now be durable without any new write)", err)
	}
	if string(got) != "lonely-event" {
		t.Fatalf("got %q, want %q", got, "lonely-event")
	}
}

// countingStore wraps a Store and counts Get calls, so tests can assert on
// how many times the backing object store was actually hit.
type countingStore struct {
	store.Store
	mu       sync.Mutex
	getCalls int
}

func (c *countingStore) Get(ctx context.Context, key string) ([]byte, error) {
	c.mu.Lock()
	c.getCalls++
	c.mu.Unlock()
	return c.Store.Get(ctx, key)
}

// TestReadCachesDecompressedSegment is a regression test for a real
// throughput bug: Read used to fetch and zstd-decompress the entire segment
// from the object store on every single call, even when reading many events
// out of the same segment back to back — the actual bottleneck behind
// cmd/logkrama-processor's per-event consume loop running far slower than the
// collector could write, since a segment holding hundreds of events meant
// hundreds of redundant full-segment fetches for what should be one. A
// segment's bytes must be fetched at most once regardless of how many of
// its events are read.
func TestReadCachesDecompressedSegment(t *testing.T) {
	dir := t.TempDir()
	local, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("new local store: %v", err)
	}
	cs := &countingStore{Store: local}
	v := New(cs, Config{MaxSegmentBytes: 64 << 20, MaxSegmentAge: time.Hour, NodeID: "test-node"})
	ctx := context.Background()

	const n = 50
	items := make([][]byte, n)
	for i := range items {
		items[i] = []byte(fmt.Sprintf("event-%d", i))
	}
	refs, err := v.WriteBatch(ctx, items)
	if err != nil {
		t.Fatalf("WriteBatch: %v", err)
	}
	if err := v.Seal(ctx); err != nil {
		t.Fatalf("Seal: %v", err)
	}
	// Seal itself issues one Get, unrelated to segment caching: appendLedgerEntry
	// read-modify-writes ledger.jsonl. Baseline after that before asserting on
	// segment-read behavior specifically.
	cs.mu.Lock()
	baseline := cs.getCalls
	cs.mu.Unlock()

	for i, ref := range refs {
		got, err := v.Read(ctx, ref)
		if err != nil {
			t.Fatalf("Read(%d): %v", i, err)
		}
		if !bytes.Equal(got, items[i]) {
			t.Fatalf("Read(%d) = %q, want %q", i, got, items[i])
		}
	}

	cs.mu.Lock()
	segmentGets := cs.getCalls - baseline
	cs.mu.Unlock()
	if segmentGets != 1 {
		t.Fatalf("store.Get called %d times for the segment reading %d events from it, want 1 (cache miss on the first read only)", segmentGets, n)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
