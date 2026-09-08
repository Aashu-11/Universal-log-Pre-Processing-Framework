package parse_test

import (
	"path/filepath"
	"testing"

	"github.com/ulpf/ulpf/internal/parse"
	"github.com/ulpf/ulpf/internal/parse/ops"
)

// FuzzParseAllPacks feeds arbitrary bytes — including invalid UTF-8, empty
// input, and adversarially malformed near-matches of each vendor's real
// format — through every loaded parser pack's Plan.Run. The only
// requirement is "no panic": a malformed event must come back
// StatusPartial/StatusFailed with whatever fields could be salvaged, never
// crash the process. Run with:
//
//	go test ./internal/parse/... -fuzz=FuzzParseAllPacks -fuzztime=60s
func FuzzParseAllPacks(f *testing.F) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		f.Fatalf("resolve repo root: %v", err)
	}
	registry := parse.NewRegistry(ops.Deps{})
	if _, err := registry.LoadDir(filepath.Join(root, "packs")); err != nil {
		f.Fatalf("load packs: %v", err)
	}
	plans := registry.All()
	if len(plans) == 0 {
		f.Fatal("no parser packs loaded")
	}

	seeds := []string{
		"",
		"\x00\x00\x00",
		"<14>Sep  7 14:02:11 pa-edge-01 1,2026/09/07 14:02:11,001,TRAFFIC,end,1.2.3.4,5.6.7.8,r1,ssl,untrust,trust,1,2,3,tcp,allow,4,5,6,7,host",
		`<189>date=2026-09-07 time=14:02:11 devname="FG" srcip=1.2.3.4 srcport=1 dstip=5.6.7.8 dstport=2 action="accept" policyid=1 sentbyte=1 rcvdbyte=1 duration=1`,
		"<166>Sep 07 2026 14:02:11 asa : %ASA-6-302013: Built inbound TCP connection 1 for outside:1.2.3.4/1 (1.2.3.4/1) to inside:5.6.7.8/2 (5.6.7.8/2)",
		"<14>TRAFFIC",
		"%ASA-6-302013:",
		"\xff\xfe\x00garbage,,,,",
		"CEF:0|||||||",
		"LEEF:1.0|||||",
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, raw []byte) {
		for _, plan := range plans {
			matched, _ := plan.Matches(raw)
			if !matched {
				continue
			}
			res := plan.Run(raw)
			switch res.Status {
			case parse.StatusOK, parse.StatusPartial, parse.StatusFailed:
			default:
				t.Fatalf("unexpected status %q from parser %s", res.Status, plan.Metadata.ID)
			}
		}
	})
}
