package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/ulpf/ulpf/internal/parse"
	"github.com/ulpf/ulpf/internal/parse/ops"
)

// newParserCmd registers `ulpfctl parser test` (golden fixture regression)
// and `ulpfctl parser lint` (DSL validation + unsafe-regex rejection).
func newParserCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "parser",
		Short: "Parser test/lint operations",
	}
	cmd.AddCommand(newParserTestCmd())
	cmd.AddCommand(newParserLintCmd())
	cmd.AddCommand(newParserRunCmd())
	return cmd
}

type goldenFixture struct {
	Raw      string         `json:"raw"`
	ParserID string         `json:"parser_id"`
	Status   string         `json:"status"`
	Fields   map[string]any `json:"fields"`
}

func newParserTestCmd() *cobra.Command {
	var all bool
	var packsDir, goldenDir string
	c := &cobra.Command{
		Use:   "test [parser-id]",
		Short: "Run golden fixtures against the current parser packs, print a unified diff on mismatch",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && len(args) == 0 {
				return fmt.Errorf("specify a parser id or pass --all")
			}
			registry := parse.NewRegistry(ops.Deps{})
			if _, err := registry.LoadDir(packsDir); err != nil {
				return fmt.Errorf("load packs: %w", err)
			}

			fixtureFiles, err := findGoldenFiles(goldenDir)
			if err != nil {
				return err
			}

			totalFixtures, totalFailed := 0, 0
			for _, path := range fixtureFiles {
				fixtures, err := loadGolden(path)
				if err != nil {
					return fmt.Errorf("load %s: %w", path, err)
				}
				for _, fx := range fixtures {
					if !all && !containsStr(args, fx.ParserID) {
						continue
					}
					totalFixtures++
					if !checkFixture(registry, fx) {
						totalFailed++
					}
				}
			}

			if totalFixtures == 0 {
				return fmt.Errorf("no fixtures matched")
			}
			fmt.Printf("\nparser test: %d/%d fixtures passed\n", totalFixtures-totalFailed, totalFixtures)
			if totalFailed > 0 {
				return fmt.Errorf("%d fixture(s) failed", totalFailed)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "test every parser with golden fixtures")
	c.Flags().StringVar(&packsDir, "packs", "packs", "parser packs directory")
	c.Flags().StringVar(&goldenDir, "golden", "testdata/golden", "golden fixtures directory")
	return c
}

func checkFixture(registry *parse.Registry, fx goldenFixture) bool {
	plan, ok := registry.Get(fx.ParserID)
	if !ok {
		fmt.Printf("FAIL %s: parser not loaded\n", fx.ParserID)
		return false
	}
	res := plan.Run([]byte(fx.Raw))

	var diffs []string
	if res.Status != fx.Status {
		diffs = append(diffs, fmt.Sprintf("  status: got %q, want %q", res.Status, fx.Status))
	}

	got := map[string]any{}
	res.Fields.Each(func(k string, v any) { got[k] = v })

	keys := map[string]bool{}
	for k := range fx.Fields {
		keys[k] = true
	}
	for k := range got {
		keys[k] = true
	}
	sortedKeys := make([]string, 0, len(keys))
	for k := range keys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		gv, gok := got[k]
		wv, wok := fx.Fields[k]
		if !fieldsEqual(gv, wv, gok, wok) {
			diffs = append(diffs, fmt.Sprintf("  field %q: got %v (present=%v), want %v (present=%v)", k, gv, gok, wv, wok))
		}
	}

	if len(diffs) == 0 {
		return true
	}
	fmt.Printf("FAIL %s\n  raw: %s\n", fx.ParserID, fx.Raw)
	for _, d := range diffs {
		fmt.Println(d)
	}
	return false
}

// fieldsEqual compares JSON-decoded golden values (float64/string/bool)
// against live values (int64/float64/string/bool), tolerating the
// int64-vs-float64 numeric-type difference that JSON round-tripping always
// introduces.
func fieldsEqual(got, want any, gok, wok bool) bool {
	if gok != wok {
		return false
	}
	if !gok {
		return true
	}
	if gi, ok := got.(int64); ok {
		if wf, ok := want.(float64); ok {
			return float64(gi) == wf
		}
	}
	return fmt.Sprintf("%v", got) == fmt.Sprintf("%v", want)
}

func findGoldenFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && filepath.Ext(path) == ".json" {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

func loadGolden(path string) ([]goldenFixture, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var fixtures []goldenFixture
	if err := json.Unmarshal(b, &fixtures); err != nil {
		return nil, err
	}
	return fixtures, nil
}

func containsStr(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}

func newParserLintCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "lint <file>",
		Short: "Validate a parser DSL file and reject unsafe regex patterns",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := parse.LoadParserFile(args[0])
			if err != nil {
				return err
			}
			if _, err := parse.Compile(p, ops.Deps{}); err != nil {
				return err
			}
			fmt.Printf("OK: %s (%s v%s) compiles cleanly\n", args[0], p.Metadata.ID, p.Metadata.Version)
			return nil
		},
	}
}
