// Package parse implements the declarative, hot-swappable parser engine:
// DSL compilation, the operator pipeline and the atomic plan registry.
package parse

import (
	"fmt"
	"time"

	"github.com/ulpf/ulpf/internal/parse/dsl"
	"github.com/ulpf/ulpf/internal/parse/fields"
	"github.com/ulpf/ulpf/internal/parse/ops"
)

// Parse status values, mirrored from internal/schema to avoid a dependency
// from the parse engine (a leaf package) onto the schema package.
const (
	StatusOK      = "ok"
	StatusPartial = "partial"
	StatusFailed  = "failed"
)

// DefaultOperatorDeadline bounds the whole pipeline's wall-clock time per
// CLAUDE.md: "every operator execution is bounded by a deadline (default
// 50ms)". We check elapsed time before each operator rather than trying to
// preempt one mid-execution (Go has no safe way to abort an arbitrary
// function without cooperative cancellation, and RE2 regexes are already
// linear-time so there's no pathological single-operator hang to guard
// against) — on overrun, remaining operators are skipped and the event is
// marked partial with everything extracted so far retained.
const DefaultOperatorDeadline = 50 * time.Millisecond

// Plan is one parser, compiled once at load time: pre-compiled match
// predicates and a linear operator pipeline with no further compilation
// needed on the hot path.
type Plan struct {
	Metadata   dsl.Metadata
	Priority   int
	Confidence float64
	OnFailure  string
	Deadline   time.Duration

	allPreds []func(string) bool
	anyPreds []func(string) bool
	pipeline []ops.Op
}

// Compile turns a dsl.Parser artifact into a ready-to-run Plan.
func Compile(p dsl.Parser, deps ops.Deps) (*Plan, error) {
	plan := &Plan{
		Metadata:   p.Metadata,
		Priority:   p.Match.Priority,
		Confidence: p.Match.Confidence,
		OnFailure:  p.OnFailure,
		Deadline:   DefaultOperatorDeadline,
	}
	if plan.Confidence == 0 {
		plan.Confidence = 1.0
	}
	if plan.OnFailure == "" {
		plan.OnFailure = "emit_partial"
	}

	for i, pred := range p.Match.All {
		fn, err := pred.Compile()
		if err != nil {
			return nil, fmt.Errorf("compile %s: match.all[%d]: %w", p.Metadata.ID, i, err)
		}
		plan.allPreds = append(plan.allPreds, fn)
	}
	for i, pred := range p.Match.Any {
		fn, err := pred.Compile()
		if err != nil {
			return nil, fmt.Errorf("compile %s: match.any[%d]: %w", p.Metadata.ID, i, err)
		}
		plan.anyPreds = append(plan.anyPreds, fn)
	}

	for i, spec := range p.Pipeline {
		op, err := ops.Build(spec, deps)
		if err != nil {
			return nil, fmt.Errorf("compile %s: pipeline[%d] (%s): %w", p.Metadata.ID, i, spec.Op, err)
		}
		plan.pipeline = append(plan.pipeline, op)
	}

	return plan, nil
}

// Matches reports whether raw satisfies this plan's match predicates, and
// the confidence to report if so.
func (p *Plan) Matches(raw []byte) (matched bool, confidence float64) {
	s := string(raw)
	for _, pred := range p.allPreds {
		if !pred(s) {
			return false, 0
		}
	}
	if len(p.anyPreds) > 0 {
		any := false
		for _, pred := range p.anyPreds {
			if pred(s) {
				any = true
				break
			}
		}
		if !any {
			return false, 0
		}
	}
	return true, p.Confidence
}

// Result is one event's extraction outcome.
type Result struct {
	Fields       *fields.Fields
	Status       string // StatusOK | StatusPartial | StatusFailed
	ProcessingMs float64
	Err          error // set when Status == StatusFailed
}

// Run executes the compiled pipeline against raw. It never panics on a
// single operator's error — that operator's failure marks the result
// partial (fields extracted by every other operator are kept) rather than
// aborting the whole event, honoring "never discard, always retain what
// could be extracted."
func (p *Plan) Run(raw []byte) Result {
	start := time.Now()
	f := fields.New(32)
	status := StatusOK

	for _, op := range p.pipeline {
		if p.Deadline > 0 && time.Since(start) > p.Deadline {
			status = StatusPartial
			break
		}
		if err := op(f, raw); err != nil {
			status = StatusPartial
		}
	}

	if f.Len() == 0 && status != StatusOK {
		status = StatusFailed
	}

	return Result{
		Fields:       f,
		Status:       status,
		ProcessingMs: float64(time.Since(start).Microseconds()) / 1000.0,
	}
}
