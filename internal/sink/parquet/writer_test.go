package parquet_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	pq "github.com/parquet-go/parquet-go"

	"github.com/ulpf/ulpf/internal/schema"
	sinkparquet "github.com/ulpf/ulpf/internal/sink/parquet"
	"github.com/ulpf/ulpf/internal/vault/store"
)

func newEvent(i int, vendor, dstIP string, observedAt time.Time) *schema.Event {
	return &schema.Event{
		Event: schema.EventMeta{
			ID: fmt.Sprintf("evt-%d", i), Kind: "event", Dataset: vendor + ".test",
			ObservedAt: observedAt.UnixNano(), IngestedAt: observedAt.UnixNano(),
		},
		Observer: schema.Observer{Vendor: vendor, Product: "test", Type: "firewall"},
		Src:      schema.Src{IP: "203.0.113.5", Port: 1000 + i},
		Dst:      schema.Dst{IP: dstIP, Port: 443},
		Network:  schema.Network{BytesIn: 100, BytesOut: 200, BytesTotal: 300},
		Raw:      schema.Raw{SHA256: "abc", SegmentID: "seg-1", RetrievalURI: "s3a://x"},
		Lineage:  schema.Lineage{ParserID: vendor + ".test", ParserVersion: "1.0.0", ParseStatus: "ok", NodeID: "n1"},
		Unmapped: map[string]string{},
	}
}

func TestParquetRoundTripAndPartitioning(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	w := sinkparquet.New(st, sinkparquet.Config{KeyPrefix: "normalized", MaxRows: 1000, MaxAge: time.Hour})

	now := time.Date(2026, 9, 7, 14, 0, 0, 0, time.UTC)
	ctx := context.Background()

	dstIPs := []string{"10.0.0.30", "10.0.0.10", "10.0.0.20"} // deliberately unsorted
	for i, ip := range dstIPs {
		if err := w.Write(ctx, newEvent(i, "paloalto", ip, now)); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	// A different vendor and a different hour must land in separate
	// partitions (separate files), not get mixed into the same one.
	if err := w.Write(ctx, newEvent(99, "fortinet", "10.0.0.99", now)); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Write(ctx, newEvent(100, "paloalto", "10.0.0.40", now.Add(2*time.Hour))); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := w.Flush(ctx); err != nil {
		t.Fatalf("flush: %v", err)
	}

	keys, err := st.List(ctx, "normalized/")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(keys) != 3 {
		t.Fatalf("got %d partition files, want 3 (paloalto/hour14, fortinet/hour14, paloalto/hour16): %v", len(keys), keys)
	}

	var paloaltoHour14Key string
	for _, k := range keys {
		if strings.Contains(k, "vendor=paloalto") && strings.Contains(k, "hour=14") {
			paloaltoHour14Key = k
		}
	}
	if paloaltoHour14Key == "" {
		t.Fatalf("did not find the paloalto/hour=14 partition file among: %v", keys)
	}

	data, err := st.Get(ctx, paloaltoHour14Key)
	if err != nil {
		t.Fatalf("get %s: %v", paloaltoHour14Key, err)
	}
	rows, err := pq.Read[schema.FlatRow](bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("parquet read: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows in paloalto/hour=14 partition, want 3", len(rows))
	}

	// Must be sorted by dst_ip, proving the row-group pruning sort ran.
	for i := 1; i < len(rows); i++ {
		if rows[i].DstIP < rows[i-1].DstIP {
			t.Fatalf("rows not sorted by dst_ip: %v", collectDstIPs(rows))
		}
	}
	want := []string{"10.0.0.10", "10.0.0.20", "10.0.0.30"}
	got := collectDstIPs(rows)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d dst_ip = %q, want %q", i, got[i], want[i])
		}
	}

	if rows[0].NetworkBytesTotal != 300 {
		t.Errorf("network_bytes_total = %d, want 300", rows[0].NetworkBytesTotal)
	}
	if rows[0].Vendor != "paloalto" || rows[0].Dt != "2026-09-07" || rows[0].Hour != "14" {
		t.Errorf("partition columns wrong: vendor=%s dt=%s hour=%s", rows[0].Vendor, rows[0].Dt, rows[0].Hour)
	}
}

func collectDstIPs(rows []schema.FlatRow) []string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.DstIP
	}
	return out
}
