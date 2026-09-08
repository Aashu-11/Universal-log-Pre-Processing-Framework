// Package route dispatches validated events to their configured sinks
// (lake, stream, DLQ) based on routing rules.
package route

import (
	"context"
	"fmt"

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
	for _, v := range res.Violations {
		if r.Metrics != nil {
			r.Metrics.DLQTotal.WithLabelValues(v.Reason).Inc()
		}
	}
	if r.DLQ != nil {
		if err := r.DLQ.Write(ctx, e); err != nil {
			return fmt.Errorf("route: dlq: %w", err)
		}
	}
	return nil
}
