package enrich_test

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
)

// forbiddenNetworkPackages is every stdlib import that could make an
// outbound call. internal/enrich must import none of them — every
// enricher reads from files loaded once at startup (see pipeline.go) and
// nothing else. This is the source-level half of the air-gap proof; the
// Docker-network-isolation half (an actual blocked-egress run) is Phase 10's
// `make airgap` — this test is what makes that CLI at all guaranteed to
// pass rather than getting lucky.
var forbiddenNetworkPackages = []string{
	"net/http", "net/rpc", "net/smtp", "net/mail",
}

// TestNoNetworkImports statically parses every non-test .go file in this
// package and fails if it imports anything that could dial out. "net"
// itself is allowed (net.ParseIP, net.IPNet — used for CIDR math, not
// dialing) but net/http and friends are not.
func TestNoNetworkImports(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	fset := token.NewFileSet()
	for _, f := range files {
		if filepath.Ext(f) != ".go" || hasSuffix(f, "_test.go") {
			continue
		}
		astFile, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", f, err)
		}
		for _, imp := range astFile.Imports {
			path := trimQuotes(imp.Path.Value)
			for _, forbidden := range forbiddenNetworkPackages {
				if path == forbidden {
					t.Errorf("%s imports %q — internal/enrich must never make network calls", f, path)
				}
			}
		}
	}
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func trimQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}
