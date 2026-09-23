package vault

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"

	"github.com/logkrama/logkrama/internal/vault/store"
)

// RawRef points at exactly one event's bytes inside a sealed segment.
// Offset/Length address the *uncompressed* payload directly, so Read never
// needs the segment's index block — only VerifyRange and ProveEvent do.
type RawRef struct {
	SegmentID string `json:"segment_id"`
	Offset    int64  `json:"offset"`
	Length    int64  `json:"length"`
	SHA256    string `json:"sha256"`
}

// Config controls sealing behavior and identifies this vault instance in the
// integrity ledger.
type Config struct {
	MaxSegmentBytes   int64         // seal at this many uncompressed payload bytes
	MaxSegmentAge     time.Duration // or after this long since the segment opened, whichever first
	NodeID            string
	RawBucketPrefix   string // key prefix under the store, e.g. "" for a store already scoped to the raw bucket
	SegmentCacheBytes int64  // bound on Read()'s decompressed-segment LRU cache; 0 = 256MB default
}

func (c Config) withDefaults() Config {
	if c.MaxSegmentBytes <= 0 {
		c.MaxSegmentBytes = 64 << 20 // 64MB
	}
	if c.MaxSegmentAge <= 0 {
		c.MaxSegmentAge = 5 * time.Minute
	}
	if c.NodeID == "" {
		c.NodeID = "node-1"
	}
	return c
}

// Vault is the append-only Raw Vault: every WriteBatch call durably persists
// original bytes to segments in Store, sealing (compressing, uploading, and
// chaining into the daily Merkle ledger) automatically on size or age.
type Vault struct {
	st    store.Store
	cfg   Config
	cache *segmentCache

	mu       sync.Mutex
	active   *segmentBuilder
	lastDay  string
	lastRoot [32]byte // chain tip; reset per UTC day, since the ledger chains within a day
}

type segmentBuilder struct {
	id        string
	dt        string
	buf       *bytes.Buffer
	entries   []indexEntry
	leaves    [][32]byte
	rawBytes  int64
	startedAt time.Time
	prevRoot  [32]byte
}

// New returns a Vault writing through st.
func New(st store.Store, cfg Config) *Vault {
	cfg = cfg.withDefaults()
	return &Vault{st: st, cfg: cfg, cache: newSegmentCache(cfg.SegmentCacheBytes)}
}

// Bootstrap resumes today's Merkle chain tip from the persisted ledger, so
// a freshly started process (a deploy, a crash-recovery — not just a hard
// kill) continues the same day's chain instead of starting a new one at
// zero. Without this, the first segment sealed after any process restart
// carries prevRoot=0, which won't match the real previous segment's root —
// VerifyRange (and `logkramactl vault verify`) correctly reports that boundary
// as a BROKEN chain even though nothing was actually lost or tampered
// with. Call once after New, before the first WriteBatch; a day with no
// prior ledger entries (fresh day, fresh deployment) is a no-op.
func (v *Vault) Bootstrap(ctx context.Context) error {
	dt := dayPartition(time.Now())
	entries, err := readLedger(ctx, v.st, dt)
	if err != nil {
		return fmt.Errorf("vault: bootstrap: %w", err)
	}
	if len(entries) == 0 {
		return nil
	}
	last := entries[len(entries)-1]
	root, err := hex.DecodeString(last.MerkleRoot)
	if err != nil || len(root) != 32 {
		return fmt.Errorf("vault: bootstrap: bad merkle root %q in ledger for segment %s", last.MerkleRoot, last.SegmentID)
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	copy(v.lastRoot[:], root)
	v.lastDay = dt
	return nil
}

func dayPartition(t time.Time) string {
	return t.UTC().Format("2006-01-02")
}

// WriteBatch durably writes every item in order and returns one RawRef per
// item, in the same order. It never partially fails: either every item in
// the batch is durable before returning nil, or an error is returned and
// nothing in the batch is assumed persisted.
func (v *Vault) WriteBatch(ctx context.Context, items [][]byte) ([]RawRef, error) {
	if len(items) == 0 {
		return nil, nil
	}

	v.mu.Lock()
	defer v.mu.Unlock()

	now := time.Now().UTC()
	dt := dayPartition(now)

	if v.active != nil && v.active.dt != dt {
		if err := v.sealActiveLocked(ctx); err != nil {
			return nil, fmt.Errorf("vault: seal on day rollover: %w", err)
		}
	}
	if v.active == nil {
		v.startSegmentLocked(dt)
	}

	refs := make([]RawRef, len(items))
	for i, item := range items {
		offset, sha := appendFrame(v.active.buf, item)
		v.active.entries = append(v.active.entries, indexEntry{offset: offset, length: uint64(len(item)), sha: sha})
		v.active.leaves = append(v.active.leaves, sha)
		v.active.rawBytes += int64(len(item))

		refs[i] = RawRef{
			SegmentID: v.active.id,
			Offset:    int64(offset),
			Length:    int64(len(item)),
			SHA256:    hex.EncodeToString(sha[:]),
		}
	}

	if v.active.rawBytes >= v.cfg.MaxSegmentBytes || time.Since(v.active.startedAt) >= v.cfg.MaxSegmentAge {
		if err := v.sealActiveLocked(ctx); err != nil {
			return nil, fmt.Errorf("vault: seal after write: %w", err)
		}
	}

	return refs, nil
}

// Seal forces the currently open segment to seal immediately, even if it
// hasn't hit the size/age threshold. Used by tests and by graceful shutdown.
func (v *Vault) Seal(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.sealActiveLocked(ctx)
}

// SealIfStale seals the active segment only if it has been open longer than
// MaxSegmentAge. WriteBatch only checks that threshold reactively — on the
// next write to the active segment — so a segment that stops receiving
// traffic (a load test ends, a source goes quiet) can otherwise sit open
// indefinitely: any Kafka ref already published for its earlier-flushed
// events then fails to read until either more traffic arrives to trigger
// the next WriteBatch's reactive check, or the process shuts down
// gracefully. Call this periodically (e.g. from a ticker in the collector's
// main loop) so segment age is enforced on a wall-clock schedule
// independent of write volume.
func (v *Vault) SealIfStale(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.active == nil || time.Since(v.active.startedAt) < v.cfg.MaxSegmentAge {
		return nil
	}
	return v.sealActiveLocked(ctx)
}

func (v *Vault) startSegmentLocked(dt string) {
	id := fmt.Sprintf("%s-%s", dt, randomSegmentSuffix())
	prevRoot := v.lastRoot
	if v.lastDay != dt {
		prevRoot = [32]byte{} // new day, new chain
	}
	v.active = &segmentBuilder{
		id:        id,
		dt:        dt,
		buf:       newSegmentBuf(id),
		startedAt: time.Now(),
		prevRoot:  prevRoot,
	}
}

func (v *Vault) sealActiveLocked(ctx context.Context) error {
	if v.active == nil || len(v.active.entries) == 0 {
		v.active = nil
		return nil
	}
	a := v.active
	root := sealSegmentBuf(a.buf, a.entries, a.leaves, a.prevRoot)

	rawBytes := a.buf.Bytes()
	compressed, err := zstdCompress(rawBytes)
	if err != nil {
		return fmt.Errorf("vault: compress segment %s: %w", a.id, err)
	}

	key := v.segmentKey(a.dt, a.id)
	if err := v.st.Put(ctx, key, compressed); err != nil {
		return fmt.Errorf("vault: upload segment %s: %w", a.id, err)
	}

	entry := LedgerEntry{
		SegmentID:  a.id,
		EventCount: len(a.entries),
		MerkleRoot: hex.EncodeToString(root[:]),
		PrevRoot:   hex.EncodeToString(a.prevRoot[:]),
		SealedAt:   time.Now().UTC(),
		ByteSize:   int64(len(compressed)),
	}
	if err := v.appendLedgerEntry(ctx, a.dt, entry); err != nil {
		return fmt.Errorf("vault: append ledger for segment %s: %w", a.id, err)
	}

	v.lastRoot = root
	v.lastDay = a.dt
	v.active = nil
	return nil
}

func (v *Vault) segmentKey(dt, segmentID string) string {
	return fmt.Sprintf("segments/dt=%s/%s.zst", dt, segmentID)
}

// RetrievalURI builds the s3a:// URI a normalized event's raw.retrieval_uri
// field should carry for ref, given the bucket the vault's Store is backed
// by (not tracked by Store itself, since Local has no notion of "bucket" —
// callers pass it explicitly, e.g. "logkrama-raw" in production).
func RetrievalURI(bucket string, ref RawRef) string {
	dt := dtFromSegmentID(ref.SegmentID)
	return fmt.Sprintf("s3a://%s/segments/dt=%s/%s.zst", bucket, dt, ref.SegmentID)
}

// Read fetches and returns the exact original bytes for ref, verifying the
// SHA-256 before returning. A mismatch means the segment's bytes were
// altered after sealing and is always returned as an error, never silently
// ignored.
func (v *Vault) Read(ctx context.Context, ref RawRef) ([]byte, error) {
	raw, cached := v.cache.get(ref.SegmentID)
	if !cached {
		dt := dtFromSegmentID(ref.SegmentID)
		compressed, err := v.st.Get(ctx, v.segmentKey(dt, ref.SegmentID))
		if err != nil {
			return nil, fmt.Errorf("vault: fetch segment %s: %w", ref.SegmentID, err)
		}
		raw, err = zstdDecompress(compressed)
		if err != nil {
			return nil, fmt.Errorf("vault: decompress segment %s: %w", ref.SegmentID, err)
		}
		v.cache.put(ref.SegmentID, raw)
	}
	if ref.Offset < 0 || ref.Length < 0 || ref.Offset+ref.Length > int64(len(raw)) {
		return nil, fmt.Errorf("vault: ref out of bounds for segment %s (offset=%d length=%d segment_bytes=%d)",
			ref.SegmentID, ref.Offset, ref.Length, len(raw))
	}
	payload := raw[ref.Offset : ref.Offset+ref.Length]

	sum := sha256.Sum256(payload)
	got := hex.EncodeToString(sum[:])
	if got != ref.SHA256 {
		return nil, fmt.Errorf("vault: integrity check FAILED for segment %s offset %d: expected sha256 %s, got %s",
			ref.SegmentID, ref.Offset, ref.SHA256, got)
	}

	out := make([]byte, len(payload))
	copy(out, payload)
	return out, nil
}

func dtFromSegmentID(segmentID string) string {
	if len(segmentID) >= 10 {
		return segmentID[:10]
	}
	return segmentID
}

var zstdEncoder, _ = zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
var zstdDecoder, _ = zstd.NewReader(nil)

func zstdCompress(data []byte) ([]byte, error) {
	return zstdEncoder.EncodeAll(data, nil), nil
}

func zstdDecompress(data []byte) ([]byte, error) {
	return zstdDecoder.DecodeAll(data, nil)
}
