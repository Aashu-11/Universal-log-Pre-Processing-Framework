package enrich

import (
	"fmt"

	"github.com/logkrama/logkrama/internal/schema"
)

type iocRecord struct {
	Type, Category string
	Confidence     int
}

// IOC matches src.ip/dst.ip against enrichment/iocs.csv via a Bloom filter
// pre-check (cheap, always run) then an exact map (only consulted on a
// Bloom hit) — per CLAUDE.md's spec for this enricher. At the 10k-indicator
// scale this dataset ships, the exact map alone would already be well
// under the p99 budget; the Bloom layer is here because the spec calls for
// it and because it's the right shape at the IOC-feed sizes (hundreds of
// thousands to millions of indicators) a real deployment would load.
type IOC struct {
	bloom *bloomFilter
	exact map[string]iocRecord
}

func NewIOC(csvPath string) (*IOC, error) {
	rows, err := readCSV(csvPath)
	if err != nil {
		return nil, err
	}
	exact := make(map[string]iocRecord, len(rows))
	bloom := newBloomFilter(len(rows), 0.01)
	for i, row := range rows {
		if len(row) < 4 {
			return nil, fmt.Errorf("%s: row %d: expected 4 columns, got %d", csvPath, i, len(row))
		}
		conf := 0
		fmt.Sscanf(row[3], "%d", &conf)
		exact[row[0]] = iocRecord{Type: row[1], Category: row[2], Confidence: conf}
		bloom.add(row[0])
	}
	return &IOC{bloom: bloom, exact: exact}, nil
}

func (i *IOC) Name() string { return "ioc" }

func (i *IOC) Enrich(e *schema.Event) {
	if i.checkAndSet(e, e.Src.IP) {
		return
	}
	i.checkAndSet(e, e.Dst.IP)
}

func (i *IOC) checkAndSet(e *schema.Event, indicator string) bool {
	if indicator == "" || !i.bloom.mightContain(indicator) {
		return false
	}
	rec, ok := i.exact[indicator]
	if !ok {
		return false // bloom false positive
	}
	match := true
	e.Enrich.IOCMatch = &match
	e.Enrich.IOCIndicator = indicator
	e.Threat.Category = rec.Category
	return true
}
