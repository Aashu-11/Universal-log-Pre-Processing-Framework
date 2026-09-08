package main

import (
	"database/sql"
	"fmt"

	_ "github.com/prestodb/presto-go-client/presto"
	"github.com/spf13/cobra"
)

// newPartitionsCmd registers `ulpfctl partitions sync`, which calls
// Presto's system.sync_partition_metadata for every ULPF table so newly
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
		Short: "Register new dt=/hour=/vendor= partitions with Presto (lake.ulpf.events, vault.ulpf.raw_segments, vault.ulpf.raw_index)",
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := sql.Open("presto", prestoURL)
			if err != nil {
				return fmt.Errorf("connect to presto: %w", err)
			}
			defer db.Close()

			calls := []string{
				`CALL lake.system.sync_partition_metadata('ulpf', 'events', 'FULL')`,
				`CALL vault.system.sync_partition_metadata('ulpf', 'raw_segments', 'FULL')`,
				`CALL vault.system.sync_partition_metadata('ulpf', 'raw_index', 'FULL')`,
			}
			for _, stmt := range calls {
				if _, err := db.Exec(stmt); err != nil {
					return fmt.Errorf("exec %q: %w", stmt, err)
				}
				fmt.Printf("OK: %s\n", stmt)
			}
			return nil
		},
	}
	c.Flags().StringVar(&prestoURL, "presto-url", envOr("PRESTO_URL", "http://ulpf@localhost:8080?catalog=lake&schema=ulpf"), "Presto DSN (presto-go-client format)")
	return c
}
