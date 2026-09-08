package enrich_test

import (
	"path/filepath"
	"testing"

	"github.com/ulpf/ulpf/internal/enrich"
	"github.com/ulpf/ulpf/internal/schema"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

func enrichmentDir(t *testing.T) string {
	return filepath.Join(repoRoot(t), "enrichment")
}

func newTestPipeline(t *testing.T) *enrich.Pipeline {
	t.Helper()
	p, err := enrich.NewDefaultPipeline(enrichmentDir(t), nil, nil)
	if err != nil {
		t.Fatalf("build pipeline: %v", err)
	}
	return p
}

func TestGeoASNAndInternalClassification(t *testing.T) {
	p := newTestPipeline(t)
	e := &schema.Event{
		Src: schema.Src{IP: "8.0.0.1"},  // public, first-octet 8
		Dst: schema.Dst{IP: "10.0.0.5"}, // RFC1918
	}
	p.Run(e)

	if e.Enrich.SrcGeoCountry == "" {
		t.Error("src geo country not populated for a public IP")
	}
	if e.Enrich.SrcASN == 0 {
		t.Error("src ASN not populated for a public IP")
	}
	if e.Enrich.SrcIsInternal == nil || *e.Enrich.SrcIsInternal {
		t.Error("public src IP incorrectly classified as internal")
	}
	if e.Enrich.DstIsInternal == nil || !*e.Enrich.DstIsInternal {
		t.Error("RFC1918 dst IP not classified as internal")
	}
}

func TestIOCMatchAndRiskScore(t *testing.T) {
	p := newTestPipeline(t)
	e := &schema.Event{
		Event: schema.EventMeta{Action: "blocked"},
		Src:   schema.Src{IP: "203.0.113.66"}, // seeded IOC in gen-enrichment
		Dst:   schema.Dst{IP: "10.0.0.5"},
	}
	p.Run(e)

	if e.Enrich.IOCMatch == nil || !*e.Enrich.IOCMatch {
		t.Fatal("expected IOC match for the seeded indicator 203.0.113.66")
	}
	if e.Enrich.IOCIndicator != "203.0.113.66" {
		t.Errorf("ioc_indicator = %q", e.Enrich.IOCIndicator)
	}
	if e.Enrich.RiskScore == 0 {
		t.Fatal("expected a non-zero risk score for an IOC-matched, blocked, external-source event")
	}
	if _, ok := e.Enrich.RiskFactors["ioc_match"]; !ok {
		t.Errorf("risk_factors missing ioc_match, got %+v", e.Enrich.RiskFactors)
	}
	if _, ok := e.Enrich.RiskFactors["action_blocked"]; !ok {
		t.Errorf("risk_factors missing action_blocked, got %+v", e.Enrich.RiskFactors)
	}
}

func TestIOCNoMatchForCleanIP(t *testing.T) {
	p := newTestPipeline(t)
	e := &schema.Event{Src: schema.Src{IP: "10.0.0.1"}, Dst: schema.Dst{IP: "10.0.0.2"}}
	p.Run(e)
	if e.Enrich.IOCMatch != nil && *e.Enrich.IOCMatch {
		t.Fatal("unexpected IOC match for clean internal IPs")
	}
}

func TestMITREMapping(t *testing.T) {
	p := newTestPipeline(t)
	e := &schema.Event{Threat: schema.Threat{SignatureID: "2210000"}}
	p.Run(e)
	if e.Threat.MitreTactic != "TA0001" || e.Threat.MitreTechnique != "T1190" {
		t.Errorf("got tactic=%q technique=%q", e.Threat.MitreTactic, e.Threat.MitreTechnique)
	}
}

func TestOneFailingEnricherDoesNotStopThePipeline(t *testing.T) {
	p := enrich.NewPipeline(nil, panicEnricher{}, echoEnricher{})
	e := &schema.Event{}
	p.Run(e) // must not panic out of the test
	if e.Enrich.SrcGeoCity != "ran" {
		t.Fatal("enricher after the panicking one did not run")
	}
}

type panicEnricher struct{}

func (panicEnricher) Name() string           { return "panic" }
func (panicEnricher) Enrich(e *schema.Event) { panic("boom") }

type echoEnricher struct{}

func (echoEnricher) Name() string           { return "echo" }
func (echoEnricher) Enrich(e *schema.Event) { e.Enrich.SrcGeoCity = "ran" }
