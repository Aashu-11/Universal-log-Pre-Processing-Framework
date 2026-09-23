// Package vaultindex writes the Raw Vault segment index as Parquet so
// Presto's vault catalog can query hashes and merkle roots directly.
package vaultindex

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	pq "github.com/parquet-go/parquet-go"

	"github.com/logkrama/logkrama/internal/collector"
	"github.com/logkrama/logkrama/internal/vault"
	"github.com/logkrama/logkrama/internal/vault/store"
)

// IndexRow mirrors vault.logkrama.raw_index (schema/presto/ddl.sql): one row
// per event, pointing at its exact position inside a sealed segment.
type IndexRow struct {
	EventID   string `parquet:"event_id"`
	SegmentID string `parquet:"segment_id"`
	Offset    int64  `parquet:"offset"`
	Length    int64  `parquet:"length"`
	SHA256    string `parquet:"sha256"`
	Dt        string `parquet:"dt"`
}

// SegmentRow mirrors vault.logkrama.raw_segments: one row per sealed segment,
// carrying the Merkle chain CLAUDE.md's traceability query (Q4) joins
// against.
type SegmentRow struct {
	SegmentID  string `parquet:"segment_id"`
	MerkleRoot string `parquet:"merkle_root"`
	PrevRoot   string `parquet:"prev_root"`
	EventCount int64  `parquet:"event_count"`
	SealedAtNs int64  `parquet:"sealed_at_ns"`
	ByteSize   int64  `parquet:"byte_size"`
	Dt         string `parquet:"dt"`
}

// IndexSink implements collector.RefPublisher: every durably-vaulted
// event's (event_id, RawRef) pair is buffered here and rolled to Parquet
// under s3a://logkrama-raw/index/dt=.../ — the same seal-then-upload shape as
// internal/sink/parquet, just keyed by day instead of (dt,hour,vendor).
type IndexSink struct {
	st      store.Store
	maxRows int
	maxAge  time.Duration
	prefix  string

	mu        sync.Mutex
	rows      []IndexRow
	startedAt time.Time
}

func NewIndexSink(st store.Store, maxRows int, maxAge time.Duration) *IndexSink {
	if maxRows <= 0 {
		maxRows = 200_000
	}
	if maxAge <= 0 {
		maxAge = 5 * time.Minute
	}
	return &IndexSink{st: st, maxRows: maxRows, maxAge: maxAge, prefix: "index", startedAt: time.Now()}
}

// Publish satisfies collector.RefPublisher.
func (s *IndexSink) Publish(ctx context.Context, eventID string, ref vault.RawRef, env collector.Envelope) error {
	dt := env.ReceivedAt.UTC().Format("2006-01-02")
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rows = append(s.rows, IndexRow{
		EventID: eventID, SegmentID: ref.SegmentID, Offset: ref.Offset,
		Length: ref.Length, SHA256: ref.SHA256, Dt: dt,
	})

	if len(s.rows) >= s.maxRows || time.Since(s.startedAt) >= s.maxAge {
		return s.rollLocked(ctx)
	}
	return nil
}

func (s *IndexSink) Flush(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.rows) == 0 {
		return nil
	}
	return s.rollLocked(ctx)
}

func (s *IndexSink) rollLocked(ctx context.Context) error {
	dt := s.rows[0].Dt
	var out bytes.Buffer
	if err := pq.Write(&out, s.rows, pq.Compression(&pq.Zstd)); err != nil {
		return fmt.Errorf("vaultindex: encode: %w", err)
	}
	key := fmt.Sprintf("%s/dt=%s/%s.parquet", s.prefix, dt, randomID())
	if err := s.st.Put(ctx, key, out.Bytes()); err != nil {
		return fmt.Errorf("vaultindex: upload %s: %w", key, err)
	}
	s.rows = nil
	s.startedAt = time.Now()
	return nil
}

func randomID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ExportSegments mirrors one day's vault ledger into Parquet at
// s3a://logkrama-raw/index/segments/dt=.../ — run periodically (or as part of
// `logkramactl partitions sync`, Phase 6's other deliverable) since it reads
// the whole day's ledger.jsonl rather than streaming incrementally.
func ExportSegments(ctx context.Context, raw, indexStore store.Store, dt string) error {
	entries, err := vault.ReadLedger(ctx, raw, dt)
	if err != nil {
		return fmt.Errorf("vaultindex: read ledger for %s: %w", dt, err)
	}
	if len(entries) == 0 {
		return nil
	}

	rows := make([]SegmentRow, len(entries))
	for i, e := range entries {
		rows[i] = SegmentRow{
			SegmentID: e.SegmentID, MerkleRoot: e.MerkleRoot, PrevRoot: e.PrevRoot,
			EventCount: int64(e.EventCount), SealedAtNs: e.SealedAt.UnixNano(),
			ByteSize: e.ByteSize, Dt: dt,
		}
	}

	var out bytes.Buffer
	if err := pq.Write(&out, rows, pq.Compression(&pq.Zstd)); err != nil {
		return fmt.Errorf("vaultindex: encode segments for %s: %w", dt, err)
	}
	key := fmt.Sprintf("index/segments/dt=%s/segments.parquet", dt)
	if err := indexStore.Put(ctx, key, out.Bytes()); err != nil {
		return fmt.Errorf("vaultindex: upload %s: %w", key, err)
	}
	return nil
}
