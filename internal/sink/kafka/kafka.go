// Package kafka publishes normalized events as JSON to the
// ulpf.events.normalized topic with at-least-once, idempotent delivery.
package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kg "github.com/segmentio/kafka-go"

	"github.com/ulpf/ulpf/internal/schema"
)

// producer is the minimal surface Sink needs from a Kafka writer — kept as
// an interface so unit tests never need a live broker (see sink_test.go's
// fakeProducer), while *kg.Writer satisfies it directly for production.
type producer interface {
	WriteMessages(ctx context.Context, msgs ...kg.Message) error
	Close() error
}

// Sink writes each event as one JSON message keyed by event_id — the
// idempotency key CLAUDE.md specifies, so a redelivery/retry after a
// partial failure never produces a logically duplicate row downstream
// (consumers that dedupe on key, or a compacted topic, both work off it).
// Batched via kafka-go's own internal batching (BatchTimeout/BatchBytes on
// the underlying *kg.Writer); Flush here just waits out in-flight writes.
type Sink struct {
	topic string
	prod  producer
}

func New(brokers []string, topic string) *Sink {
	w := &kg.Writer{
		Addr:         kg.TCP(brokers...),
		Topic:        topic,
		Balancer:     &kg.Hash{}, // key (event_id) determines partition, so retries land consistently
		RequiredAcks: kg.RequireAll,
		Async:        false,
		// kafka-go's own default BatchTimeout is 1s, meant for callers
		// trickling in single messages who want the writer to accumulate
		// its own batch before flushing. Write() is called once per event
		// from a serial per-event consume loop (cmd/ulpf-processor), so
		// without this every single WriteMessages call would sit idle for
		// up to a full second waiting on a batch that never grows — the
		// same latency bug fixed in internal/collector/kafkapublisher.go.
		BatchTimeout: 10 * time.Millisecond,
	}
	return &Sink{topic: topic, prod: w}
}

// NewWithProducer is used by tests to inject a fake producer.
func NewWithProducer(topic string, prod producer) *Sink {
	return &Sink{topic: topic, prod: prod}
}

func (s *Sink) Write(ctx context.Context, e *schema.Event) error {
	row := e.ToFlatRow()
	b, err := json.Marshal(row)
	if err != nil {
		return fmt.Errorf("kafka sink: marshal %s: %w", e.Event.ID, err)
	}
	msg := kg.Message{Key: []byte(e.Event.ID), Value: b}
	if err := s.prod.WriteMessages(ctx, msg); err != nil {
		return fmt.Errorf("kafka sink: write %s: %w", e.Event.ID, err)
	}
	return nil
}

// DLQMessage is what actually lands on the DLQ topic — FlatRow plus why.
// Route.Route knows every validate.Violation an event triggered but the
// plain Sink interface has nowhere to carry that; without this the DLQ
// topic held the same JSON as the lake/stream copies with the one thing
// that makes it a DLQ entry — the reason — silently dropped.
type DLQMessage struct {
	schema.FlatRow
	DLQReasons []string `json:"dlq_reasons"`
	DLQDetail  string   `json:"dlq_detail"`
}

// WriteDLQ satisfies route.DLQSink — an optional capability route.Router
// type-asserts for so a DLQ-routed event carries its violation reasons,
// while ordinary lake/stream sinks are untouched by this at all.
func (s *Sink) WriteDLQ(ctx context.Context, e *schema.Event, reasons []string, detail string) error {
	msg := DLQMessage{FlatRow: e.ToFlatRow(), DLQReasons: reasons, DLQDetail: detail}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("kafka sink: marshal dlq %s: %w", e.Event.ID, err)
	}
	kmsg := kg.Message{Key: []byte(e.Event.ID), Value: b}
	if err := s.prod.WriteMessages(ctx, kmsg); err != nil {
		return fmt.Errorf("kafka sink: write dlq %s: %w", e.Event.ID, err)
	}
	return nil
}

// Flush is a no-op beyond what WriteMessages already guarantees
// (RequiredAcks: All means every successful Write call already returned
// only after the broker acknowledged it) — kept to satisfy the Sink
// interface uniformly with the buffered Parquet/vault-index sinks.
func (s *Sink) Flush(context.Context) error { return nil }

func (s *Sink) Close() error { return s.prod.Close() }
