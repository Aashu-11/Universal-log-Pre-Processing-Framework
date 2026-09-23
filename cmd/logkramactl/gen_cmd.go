package main

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/logkrama/logkrama/internal/loggen"
)

// newGenCmd prints synthetic sample lines for a vendor not shipped as a
// pack — used by Phase 8's onboarding-flow proof, which needs a source the
// system has genuinely never seen before (see docs/DECISIONS.md and
// tools/gen-corpus's parser-pack corpus, which is a separate, in-repo
// concern).
func newGenCmd() *cobra.Command {
	var count int
	var seed int64
	c := &cobra.Command{
		Use:   "gen <vendor>",
		Short: "Print N synthetic sample log lines for a vendor, one per line (currently: sonicwall)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "sonicwall" {
				return fmt.Errorf("unknown gen target %q (only 'sonicwall' is wired up — see docs/BUILD_PLAN.md's onboarding demo)", args[0])
			}
			gen := loggen.NewGenerator(seed)
			now := time.Now()
			for i := 0; i < count; i++ {
				fmt.Println(gen.SonicWallTraffic(now.Add(time.Duration(i) * time.Second)))
			}
			return nil
		},
	}
	c.Flags().IntVar(&count, "count", 200, "number of lines to print")
	c.Flags().Int64Var(&seed, "seed", 42, "PRNG seed")
	return c
}
