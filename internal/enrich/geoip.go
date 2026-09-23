package enrich

import (
	"fmt"
	"net"

	"github.com/logkrama/logkrama/internal/schema"
)

type geoRecord struct {
	Country, City string
}

// GeoIP enriches src.ip/dst.ip with country/city from a synthetic
// CIDR-range dataset (enrichment/geoip.csv) — see docs/DECISIONS.md D-002
// for why this is CSV, not a downloaded .mmdb.
type GeoIP struct {
	table *cidrTable[geoRecord]
}

func NewGeoIP(csvPath string) (*GeoIP, error) {
	table, err := loadCIDRCSV(csvPath, func(row []string) (geoRecord, error) {
		if len(row) < 3 {
			return geoRecord{}, fmt.Errorf("expected 3 columns, got %d", len(row))
		}
		return geoRecord{Country: row[1], City: row[2]}, nil
	})
	if err != nil {
		return nil, err
	}
	return &GeoIP{table: table}, nil
}

func (g *GeoIP) Name() string { return "geoip" }

func (g *GeoIP) Enrich(e *schema.Event) {
	if ip := net.ParseIP(e.Src.IP); ip != nil {
		if rec, ok := g.table.lookup(ip); ok {
			e.Enrich.SrcGeoCountry = rec.Country
			e.Enrich.SrcGeoCity = rec.City
		}
	}
	if ip := net.ParseIP(e.Dst.IP); ip != nil {
		if rec, ok := g.table.lookup(ip); ok {
			e.Enrich.DstGeoCountry = rec.Country
			e.Enrich.DstGeoCity = rec.City
		}
	}
}
