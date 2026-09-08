package enrich

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
)

type cidrEntry[T any] struct {
	network *net.IPNet
	value   T
}

// cidrTable is a linear-scan CIDR->value lookup. At the dataset sizes this
// prototype ships (~220 geo/ASN ranges), a linear scan comfortably beats
// the p99 < 1ms enrichment budget without needing a radix trie — see
// BenchmarkEnrich in enrich_bench_test.go for the measured number.
type cidrTable[T any] struct {
	entries []cidrEntry[T]
}

func (t *cidrTable[T]) lookup(ip net.IP) (T, bool) {
	for _, e := range t.entries {
		if e.network.Contains(ip) {
			return e.value, true
		}
	}
	var zero T
	return zero, false
}

// loadCIDRCSV reads a CSV with a "cidr" column plus parse into T via
// rowFn, building a cidrTable.
func loadCIDRCSV[T any](path string, rowFn func(row []string) (T, error)) (*cidrTable[T], error) {
	rows, err := readCSV(path)
	if err != nil {
		return nil, err
	}
	table := &cidrTable[T]{entries: make([]cidrEntry[T], 0, len(rows))}
	for i, row := range rows {
		if len(row) == 0 {
			continue
		}
		_, ipnet, err := net.ParseCIDR(row[0])
		if err != nil {
			return nil, fmt.Errorf("%s: row %d: bad cidr %q: %w", path, i, row[0], err)
		}
		val, err := rowFn(row)
		if err != nil {
			return nil, fmt.Errorf("%s: row %d: %w", path, i, err)
		}
		table.entries = append(table.entries, cidrEntry[T]{network: ipnet, value: val})
	}
	return table, nil
}

// readCSV reads path (with a header row, which is skipped) and returns
// every data row.
func readCSV(path string) ([][]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(bufio.NewReader(f))
	var rows [][]string
	first := true
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if first {
			first = false
			continue // skip header
		}
		rows = append(rows, rec)
	}
	return rows, nil
}
