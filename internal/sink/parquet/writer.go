// Package parquet implements the buffered, size/time-rolled Parquet writer
// that lands normalized events in the MinIO lake.
package parquet

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"

	pq "github.com/parquet-go/parquet-go"

	"github.com/logkrama/logkrama/internal/schema"
	"github.com/logkrama/logkrama/internal/vault/store"
)

// Config controls partition file rolling. Per CLAUDE.md: roll at 128MB or
// 5 minutes, target 100-200MB files — many small files destroy Presto
// performance via row-group/metadata overhead, so MaxRows defaults high
// enough that a busy partition rolls on size, not on a timer tick.
type Config struct {
	KeyPrefix string // e.g. "normalized" — object key becomes <prefix>/dt=.../hour=.../vendor=.../<id>.parquet
	MaxRows   int    // approximate row-count proxy for the 128MB target — see rollThresholdRows doc
	MaxAge    time.Duration
}

func (c Config) withDefaults() Config {
	if c.KeyPrefix == "" {
		c.KeyPrefix = "normalized"
	}
	if c.MaxRows <= 0 {
		// ~600 bytes/row observed for our UES flat row -> ~200k rows is
		// roughly 120MB, landing in CLAUDE.md's 100-200MB target band.
		c.MaxRows = 200_000
	}
	if c.MaxAge <= 0 {
		c.MaxAge = 5 * time.Minute
	}
	return c
}

type partitionKey struct{ dt, hour, vendor string }

func (k partitionKey) objectPrefix(keyPrefix string) string {
	return fmt.Sprintf("%s/dt=%s/hour=%s/vendor=%s", keyPrefix, k.dt, k.hour, k.vendor)
}

type partitionBuf struct {
	rows      []schema.FlatRow
	startedAt time.Time
}

// Writer buffers normalized events per (dt, hour, vendor) partition and
// rolls each partition to one compressed Parquet object independently —
// a busy vendor's partition can roll on size while a quiet vendor's rolls
// on the age timer, without one affecting the other's file count.
type Writer struct {
	st  store.Store
	cfg Config

	mu         sync.Mutex
	partitions map[partitionKey]*partitionBuf
}

func New(st store.Store, cfg Config) *Writer {
	return &Writer{st: st, cfg: cfg.withDefaults(), partitions: make(map[partitionKey]*partitionBuf)}
}

func (w *Writer) Write(ctx context.Context, e *schema.Event) error {
	row := e.ToFlatRow()
	dt, hour := partitionTime(e.Event.ObservedAt)
	vendor := e.Observer.Vendor
	if vendor == "" {
		vendor = "unknown"
	}
	row.Dt, row.Hour, row.Vendor = dt, hour, vendor

	key := partitionKey{dt: dt, hour: hour, vendor: vendor}

	w.mu.Lock()
	defer w.mu.Unlock()

	buf, ok := w.partitions[key]
	if !ok {
		buf = &partitionBuf{startedAt: time.Now()}
		w.partitions[key] = buf
	}
	buf.rows = append(buf.rows, row)

	if len(buf.rows) >= w.cfg.MaxRows || time.Since(buf.startedAt) >= w.cfg.MaxAge {
		if err := w.rollLocked(ctx, key, buf); err != nil {
			return err
		}
		delete(w.partitions, key)
	}
	return nil
}

func partitionTime(observedAtNs int64) (dt, hour string) {
	t := time.Unix(0, observedAtNs).UTC()
	if observedAtNs == 0 {
		t = time.Now().UTC()
	}
	return t.Format("2006-01-02"), t.Format("15")
}

// Flush rolls every partition regardless of threshold — graceful shutdown
// and tests both need "everything buffered is now durable" without waiting
// on a timer.
func (w *Writer) Flush(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	for key, buf := range w.partitions {
		if len(buf.rows) == 0 {
			continue
		}
		if err := w.rollLocked(ctx, key, buf); err != nil {
			return err
		}
		delete(w.partitions, key)
	}
	return nil
}

func (w *Writer) rollLocked(ctx context.Context, key partitionKey, buf *partitionBuf) error {
	// Sort by (dst_ip, event_observed_at) so Presto's Parquet row-group
	// min/max statistics can actually prune — CLAUDE.md's tuning note.
	sort.Slice(buf.rows, func(i, j int) bool {
		if buf.rows[i].DstIP != buf.rows[j].DstIP {
			return buf.rows[i].DstIP < buf.rows[j].DstIP
		}
		return buf.rows[i].EventObservedAt < buf.rows[j].EventObservedAt
	})

	var out bytes.Buffer
	if err := pq.Write(&out, buf.rows, pq.Compression(&pq.Zstd)); err != nil {
		return fmt.Errorf("parquet: encode partition %+v: %w", key, err)
	}

	objectKey := fmt.Sprintf("%s/%s.parquet", key.objectPrefix(w.cfg.KeyPrefix), randomFileID())
	if err := w.st.Put(ctx, objectKey, out.Bytes()); err != nil {
		return fmt.Errorf("parquet: upload %s: %w", objectKey, err)
	}
	return nil
}

func randomFileID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (w *Writer) Close() error {
	return nil
}
