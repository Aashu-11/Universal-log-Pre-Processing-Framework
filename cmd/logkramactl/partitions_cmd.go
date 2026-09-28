package main

import (
	"database/sql"
	"fmt"

	_ "github.com/prestodb/presto-go-client/presto"
	"github.com/spf13/cobra"
)

// newPartitionsCmd registers `logkramactl partitions sync`, which calls
// Presto's system.sync_partition_metadata for every LOGKRAMA table so newly
// written dt=/hour=/vendor= directories become queryable — needed because
// the Hive connector doesn't discover new partitions on its own.
func newPartitionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "partitions",
		Short: "Presto partition metadata operations",
	}
	cmd.AddCommand(newPartitionsSyncCmd())
	return cmd
}

func newPartitionsSyncCmd() *cobra.Command {
	var prestoURL string
	c := &cobra.Command{
		Use:   "sync",
		Short: "Register new dt=/hour=/vendor= partitions with Presto (lake.logkrama.events, vault.logkrama.raw_segments, vault.logkrama.raw_index)",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := sql.Open("presto", prestoURL)
			if err != nil {
				return fmt.Errorf("connect to presto: %w", err)
			}
			defer db.Close()

			calls := []string{
				`CALL lake.system.sync_partition_metadata('logkrama', 'events', 'FULL')`,
				`CALL vault.system.sync_partition_metadata('logkrama', 'raw_segments', 'FULL')`,
				`CALL vault.system.sync_partition_metadata('logkrama', 'raw_index', 'FULL')`,
			}
			for _, stmt := range calls {
				// presto-go-client's database/sql driver never implements
				// Exec (driverStmt.Exec unconditionally returns
				// ErrOperationNotSupported — see its presto/presto.go) —
				// found live running this command for the first time
				// against a real Presto server: every CALL failed with
				// "presto: operation not supported" regardless of the SQL
				// itself. CALL statements still flow through Presto's
				// ordinary query protocol and return a (typically empty)
				// result set, so Query works where Exec cannot.
				rows, err := db.Query(stmt)
				if err != nil {
					return fmt.Errorf("query %q: %w", stmt, err)
				}
				if err := rows.Err(); err != nil {
					rows.Close()
					return fmt.Errorf("query %q: %w", stmt, err)
				}
				rows.Close()
				fmt.Printf("OK: %s\n", stmt)
			}
			return nil
		},
	}
	c.Flags().StringVar(&prestoURL, "presto-url", envOr("PRESTO_URL", "http://logkrama@localhost:8080?catalog=lake&schema=logkrama"), "Presto DSN (presto-go-client format)")
	return c
}
