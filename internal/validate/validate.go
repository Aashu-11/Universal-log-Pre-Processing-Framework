// Package validate checks every normalized event against the UES JSON
// Schema plus type/range checks and computes the per-event quality score.
package validate

import (
	"fmt"
	"net"
	"time"

	"github.com/logkrama/logkrama/internal/schema"
)

// DLQ reason codes. A failed event still carries these plus its raw_ref —
// per CLAUDE.md, a validation failure never means the event disappears.
const (
	ReasonSchemaViolation = "schema_violation"
	ReasonBadPort         = "bad_port"
	ReasonBadIP           = "bad_ip"
	ReasonTimestampSkew   = "timestamp_out_of_range"
)

// Result is one event's validation outcome.
type Result struct {
	SchemaValid  bool
	Violations   []Violation
	QualityScore float64
}

// Violation is one specific rule failure, with a reason code suitable for
// grouping in the DLQ (`logkramactl dlq` / the console's DLQ page group by
// exactly this field).
type Violation struct {
	Reason  string
	Message string
}

// Event validates e against the embedded UES JSON Schema plus the
// additional type/range rules CLAUDE.md calls out (ports 0-65535, IPs
// parse, timestamps within +/-1 year of now), and computes quality.score.
// now is injected (not time.Now()) so timestamp-range tests are
// deterministic.
func Event(e *schema.Event, mappedFields, expectedFields int, parseConfidence float64, now time.Time) Result {
	res := Result{SchemaValid: true}

	if err := e.Validate(); err != nil {
		res.SchemaValid = false
		res.Violations = append(res.Violations, Violation{Reason: ReasonSchemaViolation, Message: err.Error()})
	}

	if e.Src.Port != 0 && !validPort(e.Src.Port) {
		res.Violations = append(res.Violations, Violation{Reason: ReasonBadPort, Message: fmt.Sprintf("src.port %d out of range", e.Src.Port)})
	}
	if e.Dst.Port != 0 && !validPort(e.Dst.Port) {
		res.Violations = append(res.Violations, Violation{Reason: ReasonBadPort, Message: fmt.Sprintf("dst.port %d out of range", e.Dst.Port)})
	}
	if e.Src.IP != "" && net.ParseIP(e.Src.IP) == nil {
		res.Violations = append(res.Violations, Violation{Reason: ReasonBadIP, Message: fmt.Sprintf("src.ip %q does not parse", e.Src.IP)})
	}
	if e.Dst.IP != "" && net.ParseIP(e.Dst.IP) == nil {
		res.Violations = append(res.Violations, Violation{Reason: ReasonBadIP, Message: fmt.Sprintf("dst.ip %q does not parse", e.Dst.IP)})
	}
	if e.Event.ObservedAt != 0 {
		observed := time.Unix(0, e.Event.ObservedAt)
		delta := now.Sub(observed)
		if delta < 0 {
			delta = -delta
		}
		if delta > 365*24*time.Hour {
			res.Violations = append(res.Violations, Violation{
				Reason:  ReasonTimestampSkew,
				Message: fmt.Sprintf("event_observed_at %v is more than 1 year from now (%v)", observed, now),
			})
		}
	}

	res.QualityScore = ComputeQualityScore(mappedFields, expectedFields, parseConfidence)
	return res
}

func validPort(p int) bool { return p >= 0 && p <= 65535 }

// ComputeQualityScore implements CLAUDE.md's formula:
// quality.score = (mapped_fields / expected_for_class) * parse_confidence,
// clamped to [0,1]. expectedFields <= 0 (a mapping with no declared fields)
// scores 0 rather than dividing by zero.
func ComputeQualityScore(mappedFields, expectedFields int, parseConfidence float64) float64 {
	if expectedFields <= 0 {
		return 0
	}
	ratio := float64(mappedFields) / float64(expectedFields)
	if ratio > 1 {
		ratio = 1
	}
	score := ratio * parseConfidence
	if score < 0 {
		score = 0
	}
	if score > 1 {
		score = 1
	}
	return score
}
