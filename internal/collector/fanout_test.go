package collector_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ulpf/ulpf/internal/collector"
	"github.com/ulpf/ulpf/internal/vault"
)

type spyPublisher struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (s *spyPublisher) Publish(_ context.Context, eventID string, _ vault.RawRef, _ collector.Envelope) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, eventID)
	return s.err
}

func (s *spyPublisher) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

type spyBatchPublisher struct {
	spyPublisher
	batchCalls int
}

func (s *spyBatchPublisher) PublishBatch(_ context.Context, entries []collector.RefEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.batchCalls++
	for _, e := range entries {
		s.calls = append(s.calls, e.EventID)
	}
	return s.err
}

func TestFanoutPublisherFeedsSecondaryWithoutSlowingPrimary(t *testing.T) {
	primary := &spyBatchPublisher{}
	secondary := &spyPublisher{}
	fp := &collector.FanoutPublisher{Primary: primary, Secondary: []collector.RefPublisher{secondary}}

	entries := []collector.RefEntry{
		{EventID: "e1", Ref: vault.RawRef{SegmentID: "seg", Length: 1}, Env: collector.Envelope{ReceivedAt: time.Now()}},
		{EventID: "e2", Ref: vault.RawRef{SegmentID: "seg", Length: 1}, Env: collector.Envelope{ReceivedAt: time.Now()}},
	}

	if err := fp.PublishBatch(context.Background(), entries); err != nil {
		t.Fatalf("PublishBatch: %v", err)
	}

	if primary.batchCalls != 1 {
		t.Errorf("primary got %d batch calls, want 1", primary.batchCalls)
	}
	if secondary.count() != 2 {
		t.Errorf("secondary got %d calls, want 2 (one per entry)", secondary.count())
	}
}

// TestFanoutPublisherSecondaryFailureDoesNotFailBatch is the reason this
// type exists: vaultindex.IndexSink is a queryable convenience, not the
// durability boundary (vault.Vault + its ledger already are), so a
// secondary sink's failure must not propagate up through Pipeline.flush()
// and kill the pipeline goroutine the way a primary failure correctly does.
func TestFanoutPublisherSecondaryFailureDoesNotFailBatch(t *testing.T) {
	primary := &spyBatchPublisher{}
	secondary := &spyPublisher{err: errors.New("index sink boom")}
	fp := &collector.FanoutPublisher{Primary: primary, Secondary: []collector.RefPublisher{secondary}}

	entries := []collector.RefEntry{
		{EventID: "e1", Ref: vault.RawRef{SegmentID: "seg", Length: 1}, Env: collector.Envelope{ReceivedAt: time.Now()}},
	}

	if err := fp.PublishBatch(context.Background(), entries); err != nil {
		t.Fatalf("PublishBatch returned error from a secondary-only failure: %v", err)
	}
	if primary.batchCalls != 1 {
		t.Errorf("primary got %d batch calls, want 1", primary.batchCalls)
	}
}

func TestFanoutPublisherPrimaryFailurePropagates(t *testing.T) {
	primary := &spyBatchPublisher{}
	primary.err = errors.New("kafka boom")
	fp := &collector.FanoutPublisher{Primary: primary}

	entries := []collector.RefEntry{
		{EventID: "e1", Ref: vault.RawRef{SegmentID: "seg", Length: 1}, Env: collector.Envelope{ReceivedAt: time.Now()}},
	}

	if err := fp.PublishBatch(context.Background(), entries); err == nil {
		t.Fatal("PublishBatch: expected primary failure to propagate, got nil")
	}
}
