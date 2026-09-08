package enrich

import (
	"fmt"
	"path/filepath"

	"github.com/ulpf/ulpf/internal/telemetry"
)

// NewDefaultPipeline builds every enricher from dir (normally
// "enrichment/") and wires them in the order RiskScorer depends on:
// geo/ASN/cidr_classify/asset/identity/ioc all run before it, since it
// reads fields they set.
func NewDefaultPipeline(dir string, extraInternalCIDRs []string, metrics *telemetry.Metrics) (*Pipeline, error) {
	geo, err := NewGeoIP(filepath.Join(dir, "geoip.csv"))
	if err != nil {
		return nil, fmt.Errorf("enrich: geoip: %w", err)
	}
	asn, err := NewASN(filepath.Join(dir, "asn.csv"))
	if err != nil {
		return nil, fmt.Errorf("enrich: asn: %w", err)
	}
	cidr, err := NewCIDRClassify(extraInternalCIDRs)
	if err != nil {
		return nil, fmt.Errorf("enrich: cidr_classify: %w", err)
	}
	asset, err := NewAsset(filepath.Join(dir, "assets.csv"))
	if err != nil {
		return nil, fmt.Errorf("enrich: asset: %w", err)
	}
	identity, err := NewIdentity(filepath.Join(dir, "identities.csv"))
	if err != nil {
		return nil, fmt.Errorf("enrich: identity: %w", err)
	}
	ioc, err := NewIOC(filepath.Join(dir, "iocs.csv"))
	if err != nil {
		return nil, fmt.Errorf("enrich: ioc: %w", err)
	}
	mitre, err := NewMITRE(filepath.Join(dir, "mitre-map.csv"))
	if err != nil {
		return nil, fmt.Errorf("enrich: mitre: %w", err)
	}

	return NewPipeline(metrics, geo, asn, cidr, asset, identity, ioc, mitre, NewRiskScorer()), nil
}
