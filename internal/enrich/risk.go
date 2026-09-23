package enrich

import "github.com/logkrama/logkrama/internal/schema"

// RiskScorer computes enrich.risk_score as an explainable sum of
// contributing factors (enrich.risk_factors), per CLAUDE.md: "risk_score
// must be explainable: return the score plus the contributing factors."
// It must run AFTER IOC/CIDRClassify/Asset in the Pipeline — see
// NewDefaultPipeline — since it reads fields those enrichers set on the
// same event.
type RiskScorer struct{}

func NewRiskScorer() *RiskScorer { return &RiskScorer{} }

func (r *RiskScorer) Name() string { return "risk_score" }

func (r *RiskScorer) Enrich(e *schema.Event) {
	if e.Enrich.RiskFactors == nil {
		e.Enrich.RiskFactors = map[string]float64{}
	}
	factors := e.Enrich.RiskFactors

	if e.Enrich.IOCMatch != nil && *e.Enrich.IOCMatch {
		factors["ioc_match"] = 40
	}
	if e.Enrich.SrcIsInternal != nil && !*e.Enrich.SrcIsInternal {
		factors["external_source"] = 15
	}
	if e.Event.Action == "blocked" || e.Event.Action == "dropped" {
		factors["action_blocked"] = 10
	}
	if e.Event.SeverityID >= 5 {
		factors["high_severity"] = float64(e.Event.SeverityID) * 3
	}
	// dst_asset_critical is set directly by the Asset enricher, already in
	// factors by the time this runs.

	total := 0.0
	for _, v := range factors {
		total += v
	}
	if total > 100 {
		total = 100
	}
	e.Enrich.RiskScore = total
}
