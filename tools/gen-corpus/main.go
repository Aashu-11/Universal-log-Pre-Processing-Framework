// Command gen-corpus derives testdata/corpus/<vendor>/*.log and
// testdata/golden/<vendor>/*.json from the same generator tools/loggen uses
// to send live traffic, then runs them through the real compiled parser
// packs to produce golden fixtures. This is deliberate: corpus and golden
// output can never drift from what the demo generator actually produces,
// because they're derived from it mechanically, not hand-typed. Re-run this
// (and review the diff) whenever a parser pack or the generator changes on
// purpose.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/ulpf/ulpf/internal/loggen"
	"github.com/ulpf/ulpf/internal/parse"
	"github.com/ulpf/ulpf/internal/parse/ops"
)

const linesPerVendor = 25

var vendorToParserID = map[loggen.Vendor]string{
	loggen.VendorPaloAlto: "paloalto.panos.traffic",
	loggen.VendorFortinet: "fortinet.fortigate.traffic",
	loggen.VendorCiscoASA: "cisco.asa.302013",
}

type goldenFixture struct {
	Raw         string         `json:"raw"`
	ParserID    string         `json:"parser_id"`
	Status      string         `json:"status"`
	MatchedTier string         `json:"matched"`
	Fields      map[string]any `json:"fields"`
}

func main() {
	registry := parse.NewRegistry(ops.Deps{})
	if _, err := registry.LoadDir("packs"); err != nil {
		fatal("load packs: %v", err)
	}

	gen := loggen.NewGenerator(42) // fixed seed: fully reproducible corpus
	now := time.Date(2026, 9, 7, 14, 2, 11, 0, time.UTC)

	for _, v := range loggen.AllVendors {
		corpusDir := filepath.Join("testdata", "corpus", string(v))
		goldenDir := filepath.Join("testdata", "golden", string(v))
		mustMkdir(corpusDir)
		mustMkdir(goldenDir)

		var lines []string
		for i := 0; i < linesPerVendor; i++ {
			lines = append(lines, gen.Line(v, now.Add(time.Duration(i)*time.Second)))
		}

		corpusPath := filepath.Join(corpusDir, "sample.log")
		mustWriteLines(corpusPath, lines)

		parserID := vendorToParserID[v]
		plan, ok := registry.Get(parserID)
		if !ok {
			fatal("no parser loaded for id %q (vendor %s)", parserID, v)
		}

		fixtures := make([]goldenFixture, 0, len(lines))
		for _, line := range lines {
			matched, _ := plan.Matches([]byte(line))
			if !matched {
				fatal("parser %s does not match its own generator output:\n%s", parserID, line)
			}
			res := plan.Run([]byte(line))
			fx := goldenFixture{Raw: line, ParserID: parserID, Status: res.Status, MatchedTier: "signature", Fields: map[string]any{}}
			res.Fields.Each(func(k string, v any) { fx.Fields[k] = v })
			fixtures = append(fixtures, fx)
		}

		goldenPath := filepath.Join(goldenDir, "sample.json")
		mustWriteJSON(goldenPath, fixtures)
		fmt.Printf("gen-corpus: %-12s %2d lines -> %s, %s\n", v, len(lines), corpusPath, goldenPath)
	}
}

func mustMkdir(dir string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fatal("mkdir %s: %v", dir, err)
	}
}

func mustWriteLines(path string, lines []string) {
	f, err := os.Create(path)
	if err != nil {
		fatal("create %s: %v", path, err)
	}
	defer f.Close()
	for _, l := range lines {
		fmt.Fprintln(f, l)
	}
}

func mustWriteJSON(path string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fatal("marshal %s: %v", path, err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		fatal("write %s: %v", path, err)
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "gen-corpus: "+format+"\n", args...)
	os.Exit(1)
}
