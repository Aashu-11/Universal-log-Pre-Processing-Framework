package route

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/ulpf/ulpf/internal/schema"
	"github.com/ulpf/ulpf/internal/telemetry"
	"github.com/ulpf/ulpf/internal/validate"
)

type fakeSink struct {
	writes  []*schema.Event
	flushed bool
	closed  bool
}

func (f *fakeSink) Write(_ context.Context, e *schema.Event) error {
	f.writes = append(f.writes, e)
	return nil
}
func (f *fakeSink) Flush(context.Context) error { f.flushed = true; return nil }
func (f *fakeSink) Close() error                { f.closed = true; return nil }

// fakeDLQSink additionally implements DLQSink, recording the reasons/detail
// it was handed — the thing plain fakeSink.Write has no way to receive.
type fakeDLQSink struct {
	fakeSink
	dlqWrites []struct {
		event   *schema.Event
		reasons []string
		detail  string
	}
}

func (f *fakeDLQSink) WriteDLQ(_ context.Context, e *schema.Event, reasons []string, detail string) error {
	f.dlqWrites = append(f.dlqWrites, struct {
		event   *schema.Event
		reasons []string
		detail  string
	}{e, reasons, detail})
	return nil
}

func testEvent(id string) *schema.Event {
	return &schema.Event{Event: schema.EventMeta{ID: id, Kind: "event"}, Unmapped: map[string]string{}}
}

func TestRouteWritesLakeAndStreamAlways(t *testing.T) {
	lake, stream := &fakeSink{}, &fakeSink{}
	r := &Router{Lake: lake, Stream: stream}
	e := testEvent("evt-1")

	if err := r.Route(context.Background(), e, validate.Result{}); err != nil {
		t.Fatalf("Route: %v", err)
	}
	if len(lake.writes) != 1 || lake.writes[0] != e {
		t.Errorf("lake writes = %v, want [e]", lake.writes)
	}
	if len(stream.writes) != 1 || stream.writes[0] != e {
		t.Errorf("stream writes = %v, want [e]", stream.writes)
	}
}

func TestRouteSkipsDLQWhenNoViolations(t *testing.T) {
	lake, stream, dlq := &fakeSink{}, &fakeSink{}, &fakeDLQSink{}
	r := &Router{Lake: lake, Stream: stream, DLQ: dlq}

	if err := r.Route(context.Background(), testEvent("evt-1"), validate.Result{}); err != nil {
		t.Fatalf("Route: %v", err)
	}
	if len(dlq.dlqWrites) != 0 {
		t.Errorf("got %d DLQ writes for a clean event, want 0", len(dlq.dlqWrites))
	}
}

// TestRouteUsesDLQSinkCapabilityWhenAvailable is the regression test for
// the actual fix: a DLQ sink implementing the optional DLQSink interface
// must receive the real violation reasons/messages, not just the bare
// event — this is what previously silently dropped why an event landed in
// DLQ, leaving the console's DLQ page with no reason to show.
func TestRouteUsesDLQSinkCapabilityWhenAvailable(t *testing.T) {
	lake, stream, dlq := &fakeSink{}, &fakeSink{}, &fakeDLQSink{}
	r := &Router{Lake: lake, Stream: stream, DLQ: dlq}
	e := testEvent("evt-bad")

	res := validate.Result{Violations: []validate.Violation{
		{Reason: validate.ReasonBadPort, Message: "dst.port 99999 out of range"},
		{Reason: validate.ReasonBadIP, Message: `src.ip "999.1.2.3" does not parse`},
	}}

	if err := r.Route(context.Background(), e, res); err != nil {
		t.Fatalf("Route: %v", err)
	}
	if len(dlq.fakeSink.writes) != 0 {
		t.Errorf("plain Write called (%d times) even though DLQSink capability was available", len(dlq.fakeSink.writes))
	}
	if len(dlq.dlqWrites) != 1 {
		t.Fatalf("got %d WriteDLQ calls, want 1", len(dlq.dlqWrites))
	}
	got := dlq.dlqWrites[0]
	if got.event != e {
		t.Errorf("WriteDLQ event = %v, want %v", got.event, e)
	}
	wantReasons := []string{validate.ReasonBadPort, validate.ReasonBadIP}
	if len(got.reasons) != 2 || got.reasons[0] != wantReasons[0] || got.reasons[1] != wantReasons[1] {
		t.Errorf("WriteDLQ reasons = %v, want %v", got.reasons, wantReasons)
	}
	if got.detail == "" {
		t.Error("WriteDLQ detail was empty despite two violation messages")
	}
}

// TestRouteFallsBackToPlainWriteWithoutDLQSink proves a DLQ sink that only
// implements the base sink.Sink interface (no WriteDLQ) still receives the
// event — losing the reason detail is an accepted degradation for that
// sink type, but the event itself must never be dropped.
func TestRouteFallsBackToPlainWriteWithoutDLQSink(t *testing.T) {
	lake, stream, dlq := &fakeSink{}, &fakeSink{}, &fakeSink{}
	r := &Router{Lake: lake, Stream: stream, DLQ: dlq}
	e := testEvent("evt-bad")

	res := validate.Result{Violations: []validate.Violation{{Reason: validate.ReasonBadPort, Message: "bad port"}}}
	if err := r.Route(context.Background(), e, res); err != nil {
		t.Fatalf("Route: %v", err)
	}
	if len(dlq.writes) != 1 || dlq.writes[0] != e {
		t.Errorf("dlq writes = %v, want [e]", dlq.writes)
	}
}

func TestRouteIncrementsDLQMetricPerViolationReason(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.New(reg)
	lake, stream, dlq := &fakeSink{}, &fakeSink{}, &fakeDLQSink{}
	r := &Router{Lake: lake, Stream: stream, DLQ: dlq, Metrics: m}

	res := validate.Result{Violations: []validate.Violation{
		{Reason: validate.ReasonBadPort, Message: "x"},
		{Reason: validate.ReasonBadPort, Message: "y"},
		{Reason: validate.ReasonBadIP, Message: "z"},
	}}
	if err := r.Route(context.Background(), testEvent("evt-1"), res); err != nil {
		t.Fatalf("Route: %v", err)
	}

	badPortCount := testutil.ToFloat64(m.DLQTotal.WithLabelValues(validate.ReasonBadPort))
	badIPCount := testutil.ToFloat64(m.DLQTotal.WithLabelValues(validate.ReasonBadIP))
	if badPortCount != 2 {
		t.Errorf("bad_port counter = %v, want 2", badPortCount)
	}
	if badIPCount != 1 {
		t.Errorf("bad_ip counter = %v, want 1", badIPCount)
	}
}
