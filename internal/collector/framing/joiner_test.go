package framing

import (
	"bytes"
	"testing"
)

func TestJoinerNoPatternEachLineIsAnEvent(t *testing.T) {
	j := New(Config{})
	res, ok := j.Feed([]byte("line one"))
	if !ok || string(res.Payload) != "line one" {
		t.Fatalf("got %q ok=%v", res.Payload, ok)
	}
}

func TestJoinerMultilineJoinsContinuations(t *testing.T) {
	j := New(Config{MultilineStart: `^\d{4}-\d{2}-\d{2}`})

	lines := [][]byte{
		[]byte("2026-09-07 first event line 1"),
		[]byte("  continuation of first event"),
		[]byte("  more continuation"),
		[]byte("2026-09-07 second event"),
	}

	var completed []Result
	for _, l := range lines {
		if res, ok := j.Feed(l); ok {
			completed = append(completed, res)
		}
	}
	if flushRes, ok := j.Flush(); ok {
		completed = append(completed, flushRes)
	}

	if len(completed) != 2 {
		t.Fatalf("got %d completed events, want 2", len(completed))
	}
	want0 := "2026-09-07 first event line 1\n  continuation of first event\n  more continuation"
	if string(completed[0].Payload) != want0 {
		t.Errorf("event 0 = %q, want %q", completed[0].Payload, want0)
	}
	want1 := "2026-09-07 second event"
	if string(completed[1].Payload) != want1 {
		t.Errorf("event 1 = %q, want %q", completed[1].Payload, want1)
	}
}

func TestJoinerTruncatesOversizedEvents(t *testing.T) {
	j := New(Config{MaxEventBytes: 10})
	res, ok := j.Feed([]byte("this line is definitely longer than ten bytes"))
	if !ok {
		t.Fatal("expected completed event")
	}
	if !res.Truncated {
		t.Error("expected Truncated=true")
	}
	if len(res.Payload) != 10 {
		t.Errorf("payload length = %d, want 10", len(res.Payload))
	}
}

func TestJoinerPreservesInvalidUTF8AndNullBytes(t *testing.T) {
	j := New(Config{})
	adversarial := []byte{0xFF, 0xFE, 0x00, 0x00, 'a', 'b', 0xC0, 0xC1}
	res, ok := j.Feed(adversarial)
	if !ok {
		t.Fatal("expected completed event")
	}
	if !bytes.Equal(res.Payload, adversarial) {
		t.Fatalf("bytes altered: got %v want %v", res.Payload, adversarial)
	}
}
