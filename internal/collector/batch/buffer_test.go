package batch

import (
	"fmt"
	"testing"
	"time"

	"github.com/logkrama/logkrama/internal/collector"
)

func ev(i int) collector.RawEvent {
	return collector.RawEvent{
		Envelope: collector.Envelope{ListenerID: "test", ReceivedAt: time.Now()},
		Payload:  []byte(fmt.Sprintf("event-%d", i)),
	}
}

func TestBufferBasicPushDrain(t *testing.T) {
	b, err := New(Config{Capacity: 10, SpoolDir: t.TempDir(), ListenerID: "t1"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	for i := 0; i < 5; i++ {
		if err := b.Push(ev(i)); err != nil {
			t.Fatalf("Push(%d): %v", i, err)
		}
	}

	for i := 0; i < 5; i++ {
		select {
		case got := <-b.Chan():
			want := fmt.Sprintf("event-%d", i)
			if string(got.Payload) != want {
				t.Errorf("event %d = %q, want %q", i, got.Payload, want)
			}
		case <-time.After(2 * time.Second):
			t.Fatalf("timed out waiting for event %d", i)
		}
	}
}

// TestBufferOverflowsToSpoolThenDrains fills the channel past its threshold
// without draining it, forcing events into the disk spool, then drains the
// channel and asserts every spooled event eventually arrives — proving the
// TCP/HTTP "never drop" path actually works end-to-end.
func TestBufferOverflowsToSpoolThenDrains(t *testing.T) {
	b, err := New(Config{Capacity: 10, SpoolThreshold: 0.5, SpoolDir: t.TempDir(), ListenerID: "t2"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	const total = 100
	for i := 0; i < total; i++ {
		if err := b.Push(ev(i)); err != nil {
			t.Fatalf("Push(%d): %v", i, err)
		}
	}

	if b.SpoolDepth() == 0 {
		t.Fatal("expected some events to have overflowed to spool, spool depth is 0")
	}

	received := make(map[string]bool)
	timeout := time.After(10 * time.Second)
	for len(received) < total {
		select {
		case got := <-b.Chan():
			received[string(got.Payload)] = true
		case <-timeout:
			t.Fatalf("timed out: received %d/%d events", len(received), total)
		}
	}

	for i := 0; i < total; i++ {
		want := fmt.Sprintf("event-%d", i)
		if !received[want] {
			t.Errorf("missing event %q — spool overflow path dropped an event", want)
		}
	}
}

func TestBufferBackpressureWhenSpoolFull(t *testing.T) {
	b, err := New(Config{Capacity: 2, SpoolThreshold: 0.1, MaxSpoolEvents: 3, SpoolDir: t.TempDir(), ListenerID: "t3"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	var lastErr error
	pushed := 0
	for i := 0; i < 200; i++ {
		if err := b.Push(ev(i)); err != nil {
			lastErr = err
			break
		}
		pushed++
	}
	if lastErr != ErrBackpressure {
		t.Fatalf("expected ErrBackpressure after filling channel+spool, got %v after %d pushes", lastErr, pushed)
	}
}

func TestTryPushDropsWhenFull(t *testing.T) {
	b, err := New(Config{Capacity: 2, SpoolDir: t.TempDir(), ListenerID: "t4"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer b.Close()

	// Fill the channel directly without letting the spool feeder drain it,
	// by pushing faster than any consumer reads.
	dropped := 0
	for i := 0; i < 50; i++ {
		if b.TryPush(ev(i)) {
			dropped++
		}
	}
	if dropped == 0 {
		t.Fatal("expected at least one drop once the channel filled up")
	}
}
