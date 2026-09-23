package kafka_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	kg "github.com/segmentio/kafka-go"

	"github.com/logkrama/logkrama/internal/schema"
	sinkkafka "github.com/logkrama/logkrama/internal/sink/kafka"
)

type fakeProducer struct {
	mu       sync.Mutex
	messages []kg.Message
	closed   bool
}

func (f *fakeProducer) WriteMessages(_ context.Context, msgs ...kg.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.messages = append(f.messages, msgs...)
	return nil
}

func (f *fakeProducer) Close() error {
	f.closed = true
	return nil
}

func TestKafkaSinkKeysByEventID(t *testing.T) {
	fp := &fakeProducer{}
	s := sinkkafka.NewWithProducer("logkrama.events.normalized", fp)

	e := &schema.Event{
		Event:    schema.EventMeta{ID: "evt-123", Kind: "event", Dataset: "paloalto.panos.traffic"},
		Observer: schema.Observer{Vendor: "paloalto"},
		Src:      schema.Src{IP: "1.2.3.4"},
		Raw:      schema.Raw{SHA256: "abc"},
		Unmapped: map[string]string{},
	}

	if err := s.Write(context.Background(), e); err != nil {
		t.Fatalf("Write: %v", err)
	}

	fp.mu.Lock()
	defer fp.mu.Unlock()
	if len(fp.messages) != 1 {
		t.Fatalf("got %d messages, want 1", len(fp.messages))
	}
	if string(fp.messages[0].Key) != "evt-123" {
		t.Errorf("key = %q, want evt-123 (idempotency key)", fp.messages[0].Key)
	}

	var row schema.FlatRow
	if err := json.Unmarshal(fp.messages[0].Value, &row); err != nil {
		t.Fatalf("unmarshal message value: %v", err)
	}
	if row.EventID != "evt-123" || row.SrcIP != "1.2.3.4" {
		t.Errorf("decoded row wrong: %+v", row)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !fp.closed {
		t.Error("Close did not close the underlying producer")
	}
}
