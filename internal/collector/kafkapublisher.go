package collector

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	kg "github.com/segmentio/kafka-go"

	"github.com/logkrama/logkrama/internal/vault"
)

// RawRefMessage is the wire format published to logkrama.raw.refs — the
// handoff between the collector (which durably vaults raw bytes) and the
// processor (which reads them back and runs IDENTIFY onward). Shared here
// so cmd/logkrama-collector's publisher and cmd/logkrama-processor's consumer can't
// drift on field names independently.
type RawRefMessage struct {
	EventID    string       `json:"event_id"`
	Ref        vault.RawRef `json:"ref"`
	ListenerID string       `json:"listener_id"`
	PeerIP     string       `json:"peer_ip"`
	ReceivedAt time.Time    `json:"received_at"`
}

// kafkaProducer is the minimal kafka-go surface KafkaPublisher needs —
// mirrors internal/sink/kafka's producer interface so both packages can be
// unit tested with the same shape of fake.
type kafkaProducer interface {
	WriteMessages(ctx context.Context, msgs ...kg.Message) error
	Close() error
}

// KafkaPublisher implements RefPublisher by publishing one JSON
// RawRefMessage per event to logkrama.raw.refs, keyed by event id.
type KafkaPublisher struct {
	prod kafkaProducer
}

func NewKafkaPublisher(brokers []string, topic string) *KafkaPublisher {
	return &KafkaPublisher{prod: &kg.Writer{
		Addr: kg.TCP(brokers...), Topic: topic, Balancer: &kg.Hash{}, RequiredAcks: kg.RequireAll,
		// kafka-go's own default BatchTimeout is 1s — meant for callers that
		// trickle in single messages and want the writer to accumulate its
		// own batch. PublishBatch already hands WriteMessages a complete,
		// pipeline-batched call (up to Pipeline's BatchSize events), so
		// waiting on a second, redundant internal timer only adds latency;
		// a short timeout still lets true single-message batches flush
		// promptly instead of blocking a full second.
		BatchTimeout: 10 * time.Millisecond,
	}}
}

func NewKafkaPublisherWithProducer(prod kafkaProducer) *KafkaPublisher {
	return &KafkaPublisher{prod: prod}
}

func (p *KafkaPublisher) Publish(ctx context.Context, eventID string, ref vault.RawRef, env Envelope) error {
	msg := RawRefMessage{
		EventID: eventID, Ref: ref, ListenerID: env.ListenerID,
		PeerIP: env.PeerIP, ReceivedAt: env.ReceivedAt,
	}
	b, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("collector: marshal raw-ref message: %w", err)
	}
	if err := p.prod.WriteMessages(ctx, kg.Message{Key: []byte(eventID), Value: b}); err != nil {
		return fmt.Errorf("collector: publish raw-ref for %s: %w", eventID, err)
	}
	return nil
}

// PublishBatch implements BatchPublisher: it marshals every entry and sends
// them as a single WriteMessages call, so a flush of up to BatchSize events
// costs one Kafka produce round trip instead of BatchSize sequential ones.
func (p *KafkaPublisher) PublishBatch(ctx context.Context, entries []RefEntry) error {
	if len(entries) == 0 {
		return nil
	}
	msgs := make([]kg.Message, len(entries))
	for i, e := range entries {
		msg := RawRefMessage{
			EventID: e.EventID, Ref: e.Ref, ListenerID: e.Env.ListenerID,
			PeerIP: e.Env.PeerIP, ReceivedAt: e.Env.ReceivedAt,
		}
		b, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("collector: marshal raw-ref message: %w", err)
		}
		msgs[i] = kg.Message{Key: []byte(e.EventID), Value: b}
	}
	if err := p.prod.WriteMessages(ctx, msgs...); err != nil {
		return fmt.Errorf("collector: publish raw-ref batch (%d events): %w", len(entries), err)
	}
	return nil
}

func (p *KafkaPublisher) Close() error { return p.prod.Close() }
