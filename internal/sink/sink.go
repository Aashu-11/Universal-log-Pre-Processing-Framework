// Package sink defines the common Sink interface implemented by the
// Parquet, Kafka and vault-index sinks.
package sink

import (
	"context"

	"github.com/logkrama/logkrama/internal/schema"
)

// Sink is anything the processor pipeline can route a validated,
// normalized event to. Buffer internally as needed; Flush forces a
// durability boundary (used by graceful shutdown, same idea as
// vault.Vault.Seal), and Close releases resources.
type Sink interface {
	Write(ctx context.Context, e *schema.Event) error
	Flush(ctx context.Context) error
	Close() error
}
