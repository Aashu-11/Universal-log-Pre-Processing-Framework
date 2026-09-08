package vaultindex_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	pq "github.com/parquet-go/parquet-go"

	"github.com/ulpf/ulpf/internal/collector"
	"github.com/ulpf/ulpf/internal/sink/vaultindex"
	"github.com/ulpf/ulpf/internal/vault"
	"github.com/ulpf/ulpf/internal/vault/store"
)

func TestIndexSinkPublishAndFlush(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	sink := vaultindex.NewIndexSink(st, 1000, time.Hour)
	ctx := context.Background()

	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	ref := vault.RawRef{SegmentID: "2026-09-07-abc123", Offset: 10, Length: 20, SHA256: "deadbeef"}
	if err := sink.Publish(ctx, "evt-1", ref, collector.Envelope{ReceivedAt: now}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if err := sink.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	keys, err := st.List(ctx, "index/dt=2026-09-07/")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 1 {
		t.Fatalf("got %d files, want 1: %v", len(keys), keys)
	}

	data, err := st.Get(ctx, keys[0])
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	rows, err := pq.Read[vaultindex.IndexRow](bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parquet read: %v", err)
	}
	if len(rows) != 1 || rows[0].EventID != "evt-1" || rows[0].SHA256 != "deadbeef" {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}

func TestExportSegmentsMirrorsLedger(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	v := vault.New(st, vault.Config{NodeID: "test"})
	ctx := context.Background()

	if _, err := v.WriteBatch(ctx, [][]byte{[]byte("a"), []byte("b")}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := v.Seal(ctx); err != nil {
		t.Fatalf("seal: %v", err)
	}

	today := time.Now().UTC().Format("2006-01-02")
	if err := vaultindex.ExportSegments(ctx, st, st, today); err != nil {
		t.Fatalf("export segments: %v", err)
	}

	key := "index/segments/dt=" + today + "/segments.parquet"
	data, err := st.Get(ctx, key)
	if err != nil {
		t.Fatalf("get %s: %v", key, err)
	}
	rows, err := pq.Read[vaultindex.SegmentRow](bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parquet read: %v", err)
	}
	if len(rows) != 1 || rows[0].EventCount != 2 || rows[0].MerkleRoot == "" {
		t.Fatalf("unexpected segment rows: %+v", rows)
	}
}
