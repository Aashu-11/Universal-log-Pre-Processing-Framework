package collector

import (
	"context"
	"fmt"
	"os"

	"github.com/logkrama/logkrama/internal/vault"
)

// FanoutPublisher hands every flush to a primary announcer (Kafka in
// production — the handoff cmd/logkrama-processor depends on to make forward
// progress) and additionally feeds the same entries to zero or more
// secondary sinks (e.g. vaultindex.IndexSink, which rolls them into
// vault.logkrama.raw_index for Presto). A secondary sink is a queryable
// convenience, not a durability boundary — vault.Vault + its ledger already
// are (see docs/DECISIONS.md D-007) — so a secondary's failure is logged
// and skipped rather than failing the whole batch and killing the pipeline
// goroutine the way a primary failure correctly does.
type FanoutPublisher struct {
	Primary   RefPublisher // may additionally implement BatchPublisher
	Secondary []RefPublisher
}

func (f *FanoutPublisher) Publish(ctx context.Context, eventID string, ref vault.RawRef, env Envelope) error {
	if err := f.Primary.Publish(ctx, eventID, ref, env); err != nil {
		return err
	}
	f.publishSecondary(ctx, []RefEntry{{EventID: eventID, Ref: ref, Env: env}})
	return nil
}

// PublishBatch satisfies BatchPublisher so a Pipeline flush still costs one
// primary round trip regardless of how many secondary sinks are attached.
func (f *FanoutPublisher) PublishBatch(ctx context.Context, entries []RefEntry) error {
	if bp, ok := f.Primary.(BatchPublisher); ok {
		if err := bp.PublishBatch(ctx, entries); err != nil {
			return err
		}
	} else {
		for _, e := range entries {
			if err := f.Primary.Publish(ctx, e.EventID, e.Ref, e.Env); err != nil {
				return err
			}
		}
	}
	f.publishSecondary(ctx, entries)
	return nil
}

func (f *FanoutPublisher) publishSecondary(ctx context.Context, entries []RefEntry) {
	for _, sec := range f.Secondary {
		for _, e := range entries {
			if err := sec.Publish(ctx, e.EventID, e.Ref, e.Env); err != nil {
				fmt.Fprintf(os.Stderr, "collector: secondary publish failed for %s: %v\n", e.EventID, err)
			}
		}
	}
}
