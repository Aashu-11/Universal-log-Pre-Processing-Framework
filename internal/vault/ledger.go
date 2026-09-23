package vault

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/logkrama/logkrama/internal/vault/store"
)

// LedgerEntry is one line of ledger/dt=YYYY-MM-DD/ledger.jsonl — one entry
// per sealed segment, chained by PrevRoot/MerkleRoot so the whole day forms
// a tamper-evident append-only log.
type LedgerEntry struct {
	SegmentID  string    `json:"segment_id"`
	EventCount int       `json:"event_count"`
	MerkleRoot string    `json:"merkle_root"`
	PrevRoot   string    `json:"prev_root"`
	SealedAt   time.Time `json:"sealed_at"`
	ByteSize   int64     `json:"byte_size"`
}

func ledgerKey(dt string) string {
	return fmt.Sprintf("ledger/dt=%s/ledger.jsonl", dt)
}

// appendLedgerEntry appends one line to the day's ledger. Object stores
// don't support true append, so this does a read-modify-write; acceptable
// at prototype scale (segments seal at most every 5 minutes / 64MB, so a
// day's ledger is at most a few hundred lines) — see docs/DECISIONS.md if
// this ever needs to scale further (an obvious next step is a local
// write-ahead buffer flushed periodically instead of per-seal).
func (v *Vault) appendLedgerEntry(ctx context.Context, dt string, entry LedgerEntry) error {
	key := ledgerKey(dt)
	existing, err := v.st.Get(ctx, key)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("vault: read ledger %s: %w", key, err)
	}

	line, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("vault: marshal ledger entry: %w", err)
	}

	var buf bytes.Buffer
	buf.Write(existing)
	buf.Write(line)
	buf.WriteByte('\n')

	return v.st.Put(ctx, key, buf.Bytes())
}

// ReadLedger returns every sealed-segment entry recorded for dt
// (YYYY-MM-DD, UTC). Exported for internal/sink/vaultindex, which mirrors
// the ledger into Presto-queryable Parquet.
func ReadLedger(ctx context.Context, st store.Store, dt string) ([]LedgerEntry, error) {
	return readLedger(ctx, st, dt)
}

func readLedger(ctx context.Context, st store.Store, dt string) ([]LedgerEntry, error) {
	b, err := st.Get(ctx, ledgerKey(dt))
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("vault: read ledger for %s: %w", dt, err)
	}

	var entries []LedgerEntry
	dec := json.NewDecoder(bytes.NewReader(b))
	for dec.More() {
		var e LedgerEntry
		if err := dec.Decode(&e); err != nil {
			return nil, fmt.Errorf("vault: decode ledger for %s: %w", dt, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// SegmentReport is the verification result for one sealed segment.
type SegmentReport struct {
	SegmentID      string `json:"segment_id"`
	Dt             string `json:"dt"`
	Pass           bool   `json:"pass"`
	Reason         string `json:"reason,omitempty"`
	EventCount     int    `json:"event_count"`
	RecomputedRoot string `json:"recomputed_root"`
	LedgerRoot     string `json:"ledger_root"`
	ChainOK        bool   `json:"chain_ok"`
}

// Report is the overall result of VerifyRange: PASS only if every segment
// in range passed and the chain is unbroken.
type Report struct {
	From     time.Time       `json:"from"`
	To       time.Time       `json:"to"`
	Pass     bool            `json:"pass"`
	Segments []SegmentReport `json:"segments"`
}

// VerifyRange re-fetches every segment sealed between from and to
// (inclusive, by UTC day), recomputes each segment's Merkle root directly
// from its raw payload bytes (never trusting the stored index), and checks
// the PrevRoot chain within each day. Any single altered byte in any segment
// changes that segment's recomputed root, so it is reported as a named
// FAIL — this is what makes tampering detectable and attributable to an
// exact segment.
func VerifyRange(ctx context.Context, st store.Store, from, to time.Time) (Report, error) {
	report := Report{From: from, To: to, Pass: true}

	for d := dayPartition(from); ; {
		entries, err := readLedger(ctx, st, d)
		if err != nil {
			return report, err
		}

		var chainPrev [32]byte
		for i, entry := range entries {
			sr := verifySegment(ctx, st, d, entry)
			if i == 0 {
				sr.ChainOK = entry.PrevRoot == hex.EncodeToString(chainPrev[:])
			} else {
				sr.ChainOK = entry.PrevRoot == entries[i-1].MerkleRoot
			}
			if !sr.ChainOK {
				sr.Pass = false
				if sr.Reason == "" {
					sr.Reason = "merkle chain broken: prev_root does not match previous segment's root"
				}
			}
			if !sr.Pass {
				report.Pass = false
			}
			report.Segments = append(report.Segments, sr)
		}

		if d == dayPartition(to) {
			break
		}
		next, _ := time.Parse("2006-01-02", d)
		d = dayPartition(next.AddDate(0, 0, 1))
	}

	return report, nil
}

func verifySegment(ctx context.Context, st store.Store, dt string, entry LedgerEntry) SegmentReport {
	sr := SegmentReport{SegmentID: entry.SegmentID, Dt: dt, EventCount: entry.EventCount, LedgerRoot: entry.MerkleRoot, Pass: true}

	compressed, err := st.Get(ctx, fmt.Sprintf("segments/dt=%s/%s.zst", dt, entry.SegmentID))
	if err != nil {
		sr.Pass = false
		sr.Reason = fmt.Sprintf("segment object missing or unreadable: %v", err)
		return sr
	}
	raw, err := zstdDecompress(compressed)
	if err != nil {
		sr.Pass = false
		sr.Reason = fmt.Sprintf("segment failed to decompress (corrupted): %v", err)
		return sr
	}

	dec, err := decodeSegment(raw)
	if err != nil {
		sr.Pass = false
		sr.Reason = fmt.Sprintf("segment structurally invalid: %v", err)
		return sr
	}

	leaves := make([][32]byte, len(dec.entries))
	for i, e := range dec.entries {
		payload := raw[e.offset : e.offset+e.length]
		leaves[i] = sha256.Sum256(payload)
		if leaves[i] != e.sha {
			sr.Pass = false
			sr.Reason = fmt.Sprintf("event %d payload does not match its own stored hash (tampered)", i)
		}
	}

	root := merkleRoot(leaves)
	sr.RecomputedRoot = hex.EncodeToString(root[:])
	if sr.RecomputedRoot != entry.MerkleRoot {
		sr.Pass = false
		if sr.Reason == "" {
			sr.Reason = "recomputed Merkle root does not match ledger root (segment bytes altered)"
		}
	}
	return sr
}

// ProveEvent returns a Merkle inclusion proof for ref: hashing Leaf up
// through Path reproduces the segment's Merkle root exactly when the
// segment (and this event within it) is intact.
func ProveEvent(ctx context.Context, st store.Store, ref RawRef) (MerkleProof, error) {
	dt := dtFromSegmentID(ref.SegmentID)
	compressed, err := st.Get(ctx, fmt.Sprintf("segments/dt=%s/%s.zst", dt, ref.SegmentID))
	if err != nil {
		return MerkleProof{}, fmt.Errorf("vault: fetch segment %s: %w", ref.SegmentID, err)
	}
	raw, err := zstdDecompress(compressed)
	if err != nil {
		return MerkleProof{}, fmt.Errorf("vault: decompress segment %s: %w", ref.SegmentID, err)
	}
	dec, err := decodeSegment(raw)
	if err != nil {
		return MerkleProof{}, fmt.Errorf("vault: decode segment %s: %w", ref.SegmentID, err)
	}

	leaves := make([][32]byte, len(dec.entries))
	index := -1
	for i, e := range dec.entries {
		leaves[i] = e.sha
		if int64(e.offset) == ref.Offset && int64(e.length) == ref.Length {
			index = i
		}
	}
	if index == -1 {
		return MerkleProof{}, fmt.Errorf("vault: ref not found in segment %s index", ref.SegmentID)
	}

	return buildMerkleProof(leaves, index), nil
}

func randomSegmentSuffix() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
