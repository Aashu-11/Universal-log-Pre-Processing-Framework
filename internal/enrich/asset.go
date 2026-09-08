package enrich

import (
	"fmt"

	"github.com/ulpf/ulpf/internal/schema"
)

type assetRecord struct {
	AssetID, Owner, Criticality, Environment string
}

// Asset enriches src/dst IPs against a synthetic CMDB
// (enrichment/assets.csv), recording the result in Enrich.RiskFactors so
// it's visible even though UES's Enrich struct has no dedicated asset.*
// fields yet — deliberately additive rather than widening the schema for
// a Phase 5 nice-to-have.
type Asset struct {
	byIP map[string]assetRecord
}

func NewAsset(csvPath string) (*Asset, error) {
	rows, err := readCSV(csvPath)
	if err != nil {
		return nil, err
	}
	m := make(map[string]assetRecord, len(rows))
	for i, row := range rows {
		if len(row) < 5 {
			return nil, fmt.Errorf("%s: row %d: expected 5 columns, got %d", csvPath, i, len(row))
		}
		m[row[0]] = assetRecord{AssetID: row[1], Owner: row[2], Criticality: row[3], Environment: row[4]}
	}
	return &Asset{byIP: m}, nil
}

func (a *Asset) Name() string { return "asset" }

func (a *Asset) Enrich(e *schema.Event) {
	if rec, ok := a.byIP[e.Dst.IP]; ok {
		if e.Enrich.RiskFactors == nil {
			e.Enrich.RiskFactors = map[string]float64{}
		}
		if rec.Criticality == "critical" {
			e.Enrich.RiskFactors["dst_asset_critical"] = 15
		}
	}
}

// Lookup exposes the raw record for callers that need more than the
// risk-factor side effect (the traceability API, the console).
func (a *Asset) Lookup(ip string) (assetID, owner, criticality, environment string, ok bool) {
	rec, found := a.byIP[ip]
	return rec.AssetID, rec.Owner, rec.Criticality, rec.Environment, found
}
