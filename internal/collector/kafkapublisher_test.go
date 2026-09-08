package collector_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	kg "github.com/segmentio/kafka-go"

	"github.com/ulpf/ulpf/internal/collector"
	"github.com/ulpf/ulpf/internal/vault"
)

type fakeProducer struct {
	messages   []kg.Message
	batchCalls int
}

func (f *fakeProducer) WriteMessages(_ context.Context, msgs ...kg.Message) error {
	f.messages = append(f.messages, msgs...)
	f.batchCalls++
	return nil
}
func (f *fakeProducer) Close() error { return nil }

func TestKafkaPublisherPublishesRawRefMessage(t *testing.T) {
	fp := &fakeProducer{}
	pub := collector.NewKafkaPublisherWithProducer(fp)

	ref := vault.RawRef{SegmentID: "seg-1", Offset: 10, Length: 20, SHA256: "abc"}
	env := collector.Envelope{ListenerID: "syslog-tcp", PeerIP: "10.0.0.1", ReceivedAt: time.Now()}

	if err := pub.Publish(context.Background(), "evt-1", ref, env); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	if len(fp.messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(fp.messages))
	}
	if string(fp.messages[0].Key) != "evt-1" {
		t.Errorf("key = %q, want evt-1", fp.messages[0].Key)
	}

	var got collector.RawRefMessage
	if err := json.Unmarshal(fp.messages[0].Value, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Ref.SegmentID != "seg-1" || got.ListenerID != "syslog-tcp" {
		t.Errorf("decoded message wrong: %+v", got)
	}

	if err := pub.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

// TestKafkaPublisherPublishBatchSingleRoundTrip proves KafkaPublisher
// satisfies collector.BatchPublisher and sends an entire batch as one
// WriteMessages call — the fix for a real stall where the collector
// serialized one synchronous Kafka round trip per event.
func TestKafkaPublisherPublishBatchSingleRoundTrip(t *testing.T) {
	fp := &fakeProducer{}
	pub := collector.NewKafkaPublisherWithProducer(fp)

	entries := make([]collector.RefEntry, 5)
	for i := range entries {
		entries[i] = collector.RefEntry{
			EventID: fmt.Sprintf("evt-%d", i),
			Ref:     vault.RawRef{SegmentID: "seg-1", Offset: int64(i * 10), Length: 10, SHA256: "abc"},
			Env:     collector.Envelope{ListenerID: "syslog-tcp", ReceivedAt: time.Now()},
		}
	}

	if err := pub.PublishBatch(context.Background(), entries); err != nil {
		t.Fatalf("PublishBatch: %v", err)
	}

	if len(fp.messages) != 5 {
		t.Fatalf("got %d messages, want 5", len(fp.messages))
	}
	if fp.batchCalls != 1 {
		t.Fatalf("got %d WriteMessages calls, want 1 (one round trip for the whole batch)", fp.batchCalls)
	}
}
