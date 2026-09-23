package kafka

import (
	"testing"
	"time"

	kg "github.com/segmentio/kafka-go"
)

// TestNewSetsShortBatchTimeout is a regression test for a real throughput
// bug: kafka-go's own default BatchTimeout is 1s, and Sink.Write is called
// once per event from cmd/logkrama-processor's serial per-event consume loop —
// without an explicit short BatchTimeout, every single-message WriteMessages
// call sat idle for up to a full second waiting on a batch that never grows
// past one message, capping the whole pipeline at roughly one event per
// second regardless of how fast IDENTIFY/PARSE/NORMALIZE/ENRICH ran.
func TestNewSetsShortBatchTimeout(t *testing.T) {
	s := New([]string{"localhost:9092"}, "logkrama.events.normalized")
	defer s.Close()

	w, ok := s.prod.(*kg.Writer)
	if !ok {
		t.Fatalf("prod is %T, want *kafka.Writer", s.prod)
	}
	if w.BatchTimeout <= 0 || w.BatchTimeout > 100*time.Millisecond {
		t.Fatalf("BatchTimeout = %v, want a short explicit value (<=100ms), not kafka-go's 1s default", w.BatchTimeout)
	}
}
