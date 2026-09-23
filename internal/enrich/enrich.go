// Package enrich adds offline context (geo, ASN, internal/external
// classification, asset, identity, IOC, MITRE, risk score) with zero
// network calls at runtime.
package enrich

import (
	"time"

	"github.com/logkrama/logkrama/internal/schema"
	"github.com/logkrama/logkrama/internal/telemetry"
)

// Enricher adds context to one event. Enrich must never make a network
// call — every enricher here reads from data loaded once at startup from
// enrichment/ (see tools/gen-enrichment) — and a failing enricher must
// never stop the pipeline: Run recovers panics and swallows errors from
// each enricher independently, per CLAUDE.md.
type Enricher interface {
	Name() string
	Enrich(e *schema.Event)
}

// Pipeline runs an ordered list of Enrichers. Order matters for RiskScorer,
// which reads fields other enrichers set on the same event — see NewDefaultPipeline.
type Pipeline struct {
	enrichers []Enricher
	metrics   *telemetry.Metrics
}

func NewPipeline(metrics *telemetry.Metrics, enrichers ...Enricher) *Pipeline {
	return &Pipeline{enrichers: enrichers, metrics: metrics}
}

// Run applies every enricher to e in order. Individually toggleable in
// practice by simply omitting an Enricher from the Pipeline's construction.
func (p *Pipeline) Run(e *schema.Event) {
	for _, en := range p.enrichers {
		start := time.Now()
		safeEnrich(en, e)
		if p.metrics != nil {
			p.metrics.EnrichDuration.WithLabelValues(en.Name()).Observe(time.Since(start).Seconds())
		}
	}
}

// safeEnrich isolates one enricher's panic from the rest of the pipeline —
// a bug in the IOC matcher must never take down geo/ASN/risk enrichment for
// the same event, let alone the whole processor.
func safeEnrich(en Enricher, e *schema.Event) {
	defer func() { _ = recover() }()
	en.Enrich(e)
}
