package parse_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/logkrama/logkrama/internal/parse"
	"github.com/logkrama/logkrama/internal/parse/dsl"
	"github.com/logkrama/logkrama/internal/parse/ops"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	// go test's working directory is the package directory
	// (internal/parse); the repo root is three levels up.
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	return root
}

type goldenFixture struct {
	Raw      string         `json:"raw"`
	ParserID string         `json:"parser_id"`
	Status   string         `json:"status"`
	Fields   map[string]any `json:"fields"`
}

// TestGoldenFixtures is the Go-test form of `logkramactl parser test --all`,
// so `go test ./...` alone proves the Phase 3 gate without needing the CLI
// built first.
func TestGoldenFixtures(t *testing.T) {
	root := repoRoot(t)
	registry := parse.NewRegistry(ops.Deps{})
	if _, err := registry.LoadDir(filepath.Join(root, "packs")); err != nil {
		t.Fatalf("load packs: %v", err)
	}

	goldenDir := filepath.Join(root, "testdata", "golden")
	total, failed := 0, 0
	err := filepath.WalkDir(goldenDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var fixtures []goldenFixture
		if err := json.Unmarshal(b, &fixtures); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		for _, fx := range fixtures {
			total++
			plan, ok := registry.Get(fx.ParserID)
			if !ok {
				t.Errorf("%s: parser %q not loaded", path, fx.ParserID)
				failed++
				continue
			}
			res := plan.Run([]byte(fx.Raw))
			if res.Status != fx.Status {
				t.Errorf("%s: status = %q, want %q (raw=%s)", fx.ParserID, res.Status, fx.Status, fx.Raw)
				failed++
				continue
			}
			mismatch := false
			for k, want := range fx.Fields {
				got, ok := res.Fields.Get(k)
				if !ok {
					t.Errorf("%s: missing field %q (raw=%s)", fx.ParserID, k, fx.Raw)
					mismatch = true
					continue
				}
				if gi, ok := got.(int64); ok {
					if wf, ok := want.(float64); ok && float64(gi) == wf {
						continue
					}
				}
				if fmt.Sprintf("%v", got) != fmt.Sprintf("%v", want) {
					t.Errorf("%s: field %q = %v, want %v (raw=%s)", fx.ParserID, k, got, want, fx.Raw)
					mismatch = true
				}
			}
			if mismatch {
				failed++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk golden dir: %v", err)
	}
	if total == 0 {
		t.Fatal("no golden fixtures found")
	}
	t.Logf("golden fixtures: %d/%d passed", total-failed, total)
}

// TestHotReloadAtomicSwap proves the registry swap is atomic and immediate:
// publishing a new version changes what the NEXT Run call sees, with no
// restart, per CLAUDE.md's "parsers are data, hot-swapped at runtime".
func TestHotReloadAtomicSwap(t *testing.T) {
	registry := parse.NewRegistry(ops.Deps{})

	v1 := dsl.Parser{
		Metadata: dsl.Metadata{ID: "test.hotreload", Version: "1.0.0"},
		Match:    dsl.Match{Any: []dsl.Predicate{{Contains: "x"}}},
		Pipeline: []dsl.Operator{{Op: "rename", From: "_missing_", To: "noop"}},
	}
	if _, err := registry.Publish(v1); err != nil {
		t.Fatalf("publish v1: %v", err)
	}
	plan, _ := registry.Get("test.hotreload")
	if plan.Metadata.Version != "1.0.0" {
		t.Fatalf("got version %s, want 1.0.0", plan.Metadata.Version)
	}

	v2 := v1
	v2.Metadata.Version = "1.1.0"
	v2.Pipeline = []dsl.Operator{{Op: "dissect", Pattern: "%{value}"}}
	if _, err := registry.Publish(v2); err != nil {
		t.Fatalf("publish v2: %v", err)
	}

	plan2, _ := registry.Get("test.hotreload")
	if plan2.Metadata.Version != "1.1.0" {
		t.Fatalf("got version %s, want 1.1.0 after hot reload", plan2.Metadata.Version)
	}

	res := plan2.Run([]byte("hello-x"))
	if v, ok := res.Fields.Get("value"); !ok || v != "hello-x" {
		t.Fatalf("v2 pipeline did not run: %+v", res.Fields)
	}
}

// TestBadPublishKeepsPreviousWorkingPlan proves a broken new version never
// takes a working parser offline.
func TestBadPublishKeepsPreviousWorkingPlan(t *testing.T) {
	registry := parse.NewRegistry(ops.Deps{})
	good := dsl.Parser{
		Metadata: dsl.Metadata{ID: "test.badpublish", Version: "1.0.0"},
		Pipeline: []dsl.Operator{{Op: "dissect", Pattern: "%{value}"}},
	}
	if _, err := registry.Publish(good); err != nil {
		t.Fatalf("publish good: %v", err)
	}

	bad := good
	bad.Metadata.Version = "2.0.0"
	bad.Pipeline = []dsl.Operator{{Op: "does_not_exist"}}
	if _, err := registry.Publish(bad); err == nil {
		t.Fatal("expected publish of an unknown operator to fail")
	}

	plan, _ := registry.Get("test.badpublish")
	if plan.Metadata.Version != "1.0.0" {
		t.Fatalf("bad publish should not have replaced the working plan, got version %s", plan.Metadata.Version)
	}
}
