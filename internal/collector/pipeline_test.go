package collector

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/logkrama/logkrama/internal/telemetry"
	"github.com/logkrama/logkrama/internal/vault"
	"github.com/logkrama/logkrama/internal/vault/store"
)

type fakeBuffer struct {
	ch chan RawEvent
}

func (f *fakeBuffer) Chan() <-chan RawEvent { return f.ch }

type recordingPublisher struct {
	mu   sync.Mutex
	refs []vault.RawRef
}

func (p *recordingPublisher) Publish(_ context.Context, _ string, ref vault.RawRef, _ Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.refs = append(p.refs, ref)
	return nil
}

type recordingBatchPublisher struct {
	mu        sync.Mutex
	calls     int
	entryLens []int
}

func (p *recordingBatchPublisher) Publish(context.Context, string, vault.RawRef, Envelope) error {
	panic("Publish should not be called when BatchPublisher is available")
}

func (p *recordingBatchPublisher) PublishBatch(_ context.Context, entries []RefEntry) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.calls++
	p.entryLens = append(p.entryLens, len(entries))
	return nil
}

// TestPipelineFlushUsesBatchPublisherInOneCall guards against a real
// regression: flush() used to call Publish once per event synchronously
// (up to BatchSize sequential Kafka round trips per flush), which stalled
// the pipeline goroutine under real network latency and silently starved
// the buffer/spool of a consumer. A BatchPublisher must be given the whole
// flush in a single call.
func TestPipelineFlushUsesBatchPublisherInOneCall(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	v := vault.New(st, vault.Config{NodeID: "test"})
	m := telemetry.New(prometheus.NewRegistry())
	pub := &recordingBatchPublisher{}

	buf := &fakeBuffer{ch: make(chan RawEvent, 100)}
	p := NewPipeline(buf, v, pub, m, PipelineConfig{BatchSize: 10, FlushInterval: 100 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	for i := 0; i < 10; i++ {
		buf.ch <- RawEvent{Envelope: Envelope{ReceivedAt: time.Now()}, Payload: []byte(fmt.Sprintf("e%d", i))}
	}

	deadline := time.After(2 * time.Second)
	for {
		pub.mu.Lock()
		calls := pub.calls
		pub.mu.Unlock()
		if calls >= 1 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("batch publish did not happen")
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if pub.calls != 1 {
		t.Fatalf("got %d PublishBatch calls, want 1 (one per flush, not one per event)", pub.calls)
	}
	if len(pub.entryLens) != 1 || pub.entryLens[0] != 10 {
		t.Fatalf("got entryLens %v, want [10]", pub.entryLens)
	}
}

func TestPipelineFlushesBySizeAndTime(t *testing.T) {
	dir := t.TempDir()
	st, err := store.NewLocal(dir)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	v := vault.New(st, vault.Config{NodeID: "test"})
	m := telemetry.New(prometheus.NewRegistry())
	pub := &recordingPublisher{}

	buf := &fakeBuffer{ch: make(chan RawEvent, 100)}
	p := NewPipeline(buf, v, pub, m, PipelineConfig{BatchSize: 10, FlushInterval: 100 * time.Millisecond})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	// Push exactly one batch's worth — should flush on size, fast.
	for i := 0; i < 10; i++ {
		buf.ch <- RawEvent{Envelope: Envelope{ReceivedAt: time.Now()}, Payload: []byte(fmt.Sprintf("e%d", i))}
	}

	deadline := time.After(2 * time.Second)
	for {
		pub.mu.Lock()
		n := len(pub.refs)
		pub.mu.Unlock()
		if n >= 10 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("size-triggered flush did not happen: got %d refs", n)
		case <-time.After(10 * time.Millisecond):
		}
	}

	// Push fewer than a batch — should flush on the timer instead.
	buf.ch <- RawEvent{Envelope: Envelope{ReceivedAt: time.Now()}, Payload: []byte("tail-event")}
	deadline = time.After(2 * time.Second)
	for {
		pub.mu.Lock()
		n := len(pub.refs)
		pub.mu.Unlock()
		if n >= 11 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("time-triggered flush did not happen: got %d refs", n)
		case <-time.After(10 * time.Millisecond):
		}
	}

	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	pub.mu.Lock()
	defer pub.mu.Unlock()
	if len(pub.refs) != 11 {
		t.Fatalf("got %d refs total, want 11", len(pub.refs))
	}
	for i, ref := range pub.refs {
		if ref.SegmentID == "" || ref.SHA256 == "" {
			t.Errorf("ref %d incomplete: %+v", i, ref)
		}
	}
}
