// Package route dispatches validated events to their configured sinks
// (lake, stream, DLQ) based on routing rules.
package route

import (
	"context"
	"fmt"
	"strings"

	"github.com/ulpf/ulpf/internal/schema"
	"github.com/ulpf/ulpf/internal/sink"
	"github.com/ulpf/ulpf/internal/telemetry"
	"github.com/ulpf/ulpf/internal/validate"
)

// Router fans one validated event out to every configured sink. Every
// event reaches Lake regardless of validation outcome — CLAUDE.md's "never
// drop" applies even to schema-invalid events, which still need to be
// queryable (that's exactly what Q5's missing_raw_ref check in
// docs/QUERIES.md proves). DLQ is additive: a validation failure also
// gets a copy on the DLQ topic, tagged with why, for the console's DLQ
// page and `ulpfctl dlq replay`.
type Router struct {
	Lake    sink.Sink
	Stream  sink.Sink
	DLQ     sink.Sink // may be nil to disable DLQ routing (e.g. in tests)
	Metrics *telemetry.Metrics
}

// Route writes e to Lake and Stream unconditionally, and to DLQ (plus a
// per-reason metric bump) whenever res carries any violation.
func (r *Router) Route(ctx context.Context, e *schema.Event, res validate.Result) error {
	if err := r.Lake.Write(ctx, e); err != nil {
		return fmt.Errorf("route: lake: %w", err)
	}
	if err := r.Stream.Write(ctx, e); err != nil {
		return fmt.Errorf("route: stream: %w", err)
	}

	if len(res.Violations) == 0 {
		return nil
	}
	reasons := make([]string, len(res.Violations))
	messages := make([]string, len(res.Violations))
	for i, v := range res.Violations {
		reasons[i] = v.Reason
		messages[i] = v.Message
		if r.Metrics != nil {
			r.Metrics.DLQTotal.WithLabelValues(v.Reason).Inc()
		}
	}
	if r.DLQ != nil {
		if dlqSink, ok := r.DLQ.(DLQSink); ok {
			if err := dlqSink.WriteDLQ(ctx, e, reasons, strings.Join(messages, "; ")); err != nil {
				return fmt.Errorf("route: dlq: %w", err)
			}
		} else if err := r.DLQ.Write(ctx, e); err != nil {
			return fmt.Errorf("route: dlq: %w", err)
		}
	}
	return nil
}

// DLQSink is an optional capability a DLQ sink can implement to receive an
// event's violation reasons alongside it — the plain sink.Sink interface
// (Write(ctx, *Event)) has nowhere to carry that, so without this a DLQ
// entry would be indistinguishable from a normal lake/stream copy once it
// left this function.
type DLQSink interface {
	WriteDLQ(ctx context.Context, e *schema.Event, reasons []string, detail string) error
}
