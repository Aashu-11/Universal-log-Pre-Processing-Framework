package collector

import (
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"time"

	"github.com/ulpf/ulpf/internal/telemetry"
	"github.com/ulpf/ulpf/internal/vault"
)

// RefPublisher is notified of every durably-vaulted event so it can be
// announced downstream (Kafka topic ulpf.raw.refs in production). Kept as
// an interface so the pipeline is fully testable without Kafka/Docker.
type RefPublisher interface {
	Publish(ctx context.Context, eventID string, ref vault.RawRef, env Envelope) error
}

// NoopPublisher discards refs. Used when no downstream announcement is
// configured (e.g. local/dev runs without Kafka).
type NoopPublisher struct{}

func (NoopPublisher) Publish(context.Context, string, vault.RawRef, Envelope) error { return nil }

// RefEntry pairs a generated event id with its vault ref and originating
// envelope — the unit a BatchPublisher announces.
type RefEntry struct {
	EventID string
	Ref     vault.RawRef
	Env     Envelope
}

// BatchPublisher is an optional capability a RefPublisher can implement to
// announce an entire flush batch in one call instead of one call per event.
// KafkaPublisher implements this: without it, a flush of BatchSize events
// costs BatchSize sequential synchronous Kafka produce round trips (each
// waiting on RequiredAcks) and stalls the pipeline goroutine that should be
// draining the buffer — with it, the whole batch is a single WriteMessages
// call that kafka-go sends as one (or few) requests.
type BatchPublisher interface {
	PublishBatch(ctx context.Context, entries []RefEntry) error
}

// Buffer is the minimal surface Pipeline needs from batch.Buffer — declared
// here (not imported from the batch package) so this file has no import
// cycle concerns and stays easy to fake in tests.
type Buffer interface {
	Chan() <-chan RawEvent
}

// Pipeline drains a Buffer, writes batches durably to the Vault, and
// announces each resulting RawRef to a RefPublisher. This is the only place
// raw bytes cross from "received" to "durable" — nothing upstream of here
// may claim an event is safe.
type Pipeline struct {
	buf           Buffer
	vault         *vault.Vault
	pub           RefPublisher
	metrics       *telemetry.Metrics
	batchSize     int
	flushInterval time.Duration
	idGen         func() string
}

// PipelineConfig controls batching behavior between the in-memory buffer
// and Vault.WriteBatch.
type PipelineConfig struct {
	BatchSize     int
	FlushInterval time.Duration
	IDGen         func() string // event id generator; defaults to a simple counter-free random hex if nil
}

func (c PipelineConfig) withDefaults() PipelineConfig {
	if c.BatchSize <= 0 {
		c.BatchSize = 500
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 200 * time.Millisecond
	}
	if c.IDGen == nil {
		c.IDGen = randomEventID
	}
	return c
}

func randomEventID() string {
	var b [16]byte
	_, _ = crand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func NewPipeline(buf Buffer, v *vault.Vault, pub RefPublisher, m *telemetry.Metrics, cfg PipelineConfig) *Pipeline {
	cfg = cfg.withDefaults()
	if pub == nil {
		pub = NoopPublisher{}
	}
	return &Pipeline{
		buf: buf, vault: v, pub: pub, metrics: m,
		batchSize: cfg.BatchSize, flushInterval: cfg.FlushInterval, idGen: cfg.IDGen,
	}
}

// Run drains the buffer until ctx is canceled, flushing to the vault
// whenever batchSize events have accumulated or flushInterval has elapsed
// since the last flush, whichever comes first.
func (p *Pipeline) Run(ctx context.Context) error {
	batchEvents := make([]RawEvent, 0, p.batchSize)
	timer := time.NewTimer(p.flushInterval)
	defer timer.Stop()

	flush := func() error {
		if len(batchEvents) == 0 {
			return nil
		}
		if err := p.flush(ctx, batchEvents); err != nil {
			return err
		}
		batchEvents = batchEvents[:0]
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return flush()
		case ev, ok := <-p.buf.Chan():
			if !ok {
				return flush()
			}
			batchEvents = append(batchEvents, ev)
			if len(batchEvents) >= p.batchSize {
				if err := flush(); err != nil {
					return err
				}
				if !timer.Stop() {
					<-timer.C
				}
				timer.Reset(p.flushInterval)
			}
		case <-timer.C:
			if err := flush(); err != nil {
				return err
			}
			timer.Reset(p.flushInterval)
		}
	}
}

func (p *Pipeline) flush(ctx context.Context, events []RawEvent) error {
	payloads := make([][]byte, len(events))
	for i, ev := range events {
		payloads[i] = ev.Payload
	}

	start := time.Now()
	refs, err := p.vault.WriteBatch(ctx, payloads)
	if p.metrics != nil {
		p.metrics.VaultWriteDuration.Observe(time.Since(start).Seconds())
	}
	if err != nil {
		return err
	}

	entries := make([]RefEntry, len(refs))
	for i, ref := range refs {
		entries[i] = RefEntry{EventID: p.idGen(), Ref: ref, Env: events[i].Envelope}
	}

	if bp, ok := p.pub.(BatchPublisher); ok {
		return bp.PublishBatch(ctx, entries)
	}
	for _, e := range entries {
		if err := p.pub.Publish(ctx, e.EventID, e.Ref, e.Env); err != nil {
			return err
		}
	}
	return nil
}
