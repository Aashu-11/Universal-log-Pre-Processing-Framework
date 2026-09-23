package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/logkrama/logkrama/internal/normalize"
	"github.com/logkrama/logkrama/internal/normalize/shape"
	"github.com/logkrama/logkrama/internal/parse"
	"github.com/logkrama/logkrama/internal/parse/ops"
	"github.com/logkrama/logkrama/internal/schema"
)

// runResult is one line's result from `logkramactl parser run` — extraction
// plus (if a mapping was supplied) the resulting UES fields, in one JSON
// object per input line. This is what Phase 8's onboarding engine and the
// Phase 9 Parser Workbench's live re-parse both drive off of: one binary,
// one execution path, reused by every caller instead of re-implemented.
type runResult struct {
	Raw      string            `json:"raw"`
	Status   string            `json:"status"`
	Fields   map[string]any    `json:"fields"`
	Mapped   map[string]any    `json:"mapped,omitempty"`
	Unmapped map[string]string `json:"unmapped,omitempty"`
	Shapes   *shapeSet         `json:"shapes,omitempty"`
	Error    string            `json:"error,omitempty"`
}

// shapeSet is PS requirement (g) made concrete: the exact same normalized
// event, rendered four ways — see internal/normalize/shape.
type shapeSet struct {
	UES  map[string]any `json:"ues"`
	ECS  map[string]any `json:"ecs"`
	OCSF map[string]any `json:"ocsf"`
	CEF  string         `json:"cef"`
}

func buildShapeSet(e *schema.Event) *shapeSet {
	uesBytes, _ := json.Marshal(shape.UES(e))
	var uesMap map[string]any
	_ = json.Unmarshal(uesBytes, &uesMap)
	return &shapeSet{UES: uesMap, ECS: shape.ECS(e), OCSF: shape.OCSF(e), CEF: shape.CEF(e)}
}

func newParserRunCmd() *cobra.Command {
	var parserPath, mappingPath string
	var withShapes bool
	c := &cobra.Command{
		Use:   "run",
		Short: "Run a candidate parser (and optional mapping) against newline-delimited sample lines on stdin, emitting one JSON result per line",
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := parse.LoadParserFile(parserPath)
			if err != nil {
				return fmt.Errorf("load parser: %w", err)
			}
			plan, err := parse.Compile(p, ops.Deps{})
			if err != nil {
				return fmt.Errorf("compile parser: %w", err)
			}

			var mapper *normalize.Mapper
			if mappingPath != "" {
				m, err := normalize.LoadMappingFile(mappingPath)
				if err != nil {
					return fmt.Errorf("load mapping: %w", err)
				}
				mapper = normalize.NewMapper(m, nil)
			}

			enc := json.NewEncoder(os.Stdout)
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				res := runResult{Raw: line}

				parseRes := plan.Run([]byte(line))
				res.Status = parseRes.Status
				res.Fields = map[string]any{}
				parseRes.Fields.Each(func(k string, v any) { res.Fields[k] = v })

				if mapper != nil {
					event, _ := mapper.Apply(parseRes.Fields, time.Now())
					b, _ := json.Marshal(event)
					var m map[string]any
					_ = json.Unmarshal(b, &m)
					res.Mapped = m
					res.Unmapped = event.Unmapped
					if withShapes {
						res.Shapes = buildShapeSet(event)
					}
				}

				if err := enc.Encode(res); err != nil {
					return err
				}
			}
			return scanner.Err()
		},
	}
	c.Flags().StringVar(&parserPath, "parser", "", "path to the candidate parser YAML")
	c.Flags().StringVar(&mappingPath, "mapping", "", "path to the candidate mapping YAML (optional)")
	c.Flags().BoolVar(&withShapes, "shapes", false, "also render each mapped event as UES/ECS/OCSF/CEF (requires --mapping)")
	_ = c.MarkFlagRequired("parser")
	return c
}
