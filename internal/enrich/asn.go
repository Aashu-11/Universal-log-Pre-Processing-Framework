package enrich

import (
	"fmt"
	"net"
	"strconv"

	"github.com/logkrama/logkrama/internal/schema"
)

type asnRecord struct {
	ASN int
	Org string
}

// ASN enriches src.ip/dst.ip with autonomous-system number and
// organization from enrichment/asn.csv.
type ASN struct {
	table *cidrTable[asnRecord]
}

func NewASN(csvPath string) (*ASN, error) {
	table, err := loadCIDRCSV(csvPath, func(row []string) (asnRecord, error) {
		if len(row) < 3 {
			return asnRecord{}, fmt.Errorf("expected 3 columns, got %d", len(row))
		}
		n, err := strconv.Atoi(row[1])
		if err != nil {
			return asnRecord{}, fmt.Errorf("bad asn %q: %w", row[1], err)
		}
		return asnRecord{ASN: n, Org: row[2]}, nil
	})
	if err != nil {
		return nil, err
	}
	return &ASN{table: table}, nil
}

func (a *ASN) Name() string { return "asn" }

func (a *ASN) Enrich(e *schema.Event) {
	if ip := net.ParseIP(e.Src.IP); ip != nil {
		if rec, ok := a.table.lookup(ip); ok {
			e.Enrich.SrcASN = rec.ASN
			e.Enrich.SrcASOrg = rec.Org
		}
	}
	if ip := net.ParseIP(e.Dst.IP); ip != nil {
		if rec, ok := a.table.lookup(ip); ok {
			e.Enrich.DstASN = rec.ASN
			e.Enrich.DstASOrg = rec.Org
		}
	}
}
