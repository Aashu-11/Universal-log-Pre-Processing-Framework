package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ulpf/ulpf/internal/sink/vaultindex"
	"github.com/ulpf/ulpf/internal/vault"
	"github.com/ulpf/ulpf/internal/vault/store"
)

func openVaultStore() (store.Store, error) {
	dir := os.Getenv("ULPF_VAULT_LOCAL_DIR")
	if dir == "" {
		dir = "./data/vault"
	}
	if endpoint := os.Getenv("MINIO_ENDPOINT"); endpoint != "" {
		return store.NewMinIO(
			endpoint,
			os.Getenv("MINIO_ACCESS_KEY"),
			os.Getenv("MINIO_SECRET_KEY"),
			envOr("MINIO_RAW_BUCKET", "ulpf-raw"),
			os.Getenv("MINIO_USE_SSL") == "true",
		)
	}
	return store.NewLocal(dir)
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func newVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Raw Vault integrity operations (write, read, verify, prove)",
	}
	cmd.AddCommand(newVaultWriteCmd())
	cmd.AddCommand(newVaultReadCmd())
	cmd.AddCommand(newVaultVerifyCmd())
	cmd.AddCommand(newVaultProveCmd())
	cmd.AddCommand(newVaultExportIndexCmd())
	return cmd
}

func newVaultExportIndexCmd() *cobra.Command {
	var from, to string
	c := &cobra.Command{
		Use:   "export-index",
		Short: "Export the segment ledger for --from..--to as Parquet, populating vault.ulpf.raw_segments",
		Long: "Reads each day's ledger.jsonl (already written by every Vault.Seal call) and rolls it into\n" +
			"s3a://ulpf-raw/index/segments/dt=.../segments.parquet — the source vault.ulpf.raw_segments reads\n" +
			"from. Run this after new segments seal (cron, or after a demo load run) and follow with\n" +
			"`ulpfctl partitions sync` so Presto discovers the new dt= partition.",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openVaultStore()
			if err != nil {
				return err
			}
			fromT, toT, err := parseDayRange(from, to)
			if err != nil {
				return err
			}
			ctx := context.Background()
			for d := fromT; !d.After(toT); d = d.AddDate(0, 0, 1) {
				dt := d.Format("2006-01-02")
				if err := vaultindex.ExportSegments(ctx, st, st, dt); err != nil {
					return fmt.Errorf("export-index %s: %w", dt, err)
				}
				fmt.Printf("OK: exported segment index for %s\n", dt)
			}
			return nil
		},
	}
	c.Flags().StringVar(&from, "from", time.Now().UTC().Format("2006-01-02"), "start date (YYYY-MM-DD)")
	c.Flags().StringVar(&to, "to", time.Now().UTC().Format("2006-01-02"), "end date (YYYY-MM-DD)")
	return c
}

func newVaultReadCmd() *cobra.Command {
	var segmentID, sha string
	var offset, length int64
	var asJSON bool
	c := &cobra.Command{
		Use:   "read",
		Short: "Read one event's exact original bytes back from the vault, verifying its SHA-256",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openVaultStore()
			if err != nil {
				return err
			}
			ref := vault.RawRef{SegmentID: segmentID, Offset: offset, Length: length, SHA256: sha}
			raw, err := vault.New(st, vault.Config{}).Read(context.Background(), ref)
			if err != nil {
				return err
			}
			if asJSON {
				return json.NewEncoder(os.Stdout).Encode(map[string]any{
					"sha256_verified": true,
					"length":          len(raw),
					"raw_base64":      base64.StdEncoding.EncodeToString(raw),
				})
			}
			_, err = os.Stdout.Write(raw)
			return err
		},
	}
	c.Flags().StringVar(&segmentID, "segment-id", "", "segment id")
	c.Flags().Int64Var(&offset, "offset", 0, "byte offset within the segment")
	c.Flags().Int64Var(&length, "length", 0, "byte length")
	c.Flags().StringVar(&sha, "sha256", "", "expected sha256")
	c.Flags().BoolVar(&asJSON, "json", false, "emit {sha256_verified, length, raw_base64} JSON instead of raw bytes")
	_ = c.MarkFlagRequired("segment-id")
	return c
}

func newVaultWriteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "write",
		Short: "Read newline-delimited events from stdin and write them to the vault",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openVaultStore()
			if err != nil {
				return err
			}
			v := vault.New(st, vault.Config{NodeID: envOr("ULPF_NODE_ID", "ulpfctl")})
			if err := v.Bootstrap(context.Background()); err != nil {
				return fmt.Errorf("bootstrap vault chain: %w", err)
			}

			var items [][]byte
			scanner := bufio.NewScanner(os.Stdin)
			scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
			for scanner.Scan() {
				line := scanner.Bytes()
				item := make([]byte, len(line))
				copy(item, line)
				items = append(items, item)
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read stdin: %w", err)
			}

			ctx := context.Background()
			refs, err := v.WriteBatch(ctx, items)
			if err != nil {
				return err
			}
			if err := v.Seal(ctx); err != nil {
				return err
			}
			return json.NewEncoder(os.Stdout).Encode(refs)
		},
	}
}

func newVaultVerifyCmd() *cobra.Command {
	var from, to string
	c := &cobra.Command{
		Use:   "verify",
		Short: "Verify the integrity ledger between --from and --to (YYYY-MM-DD), printing a PASS/FAIL table",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openVaultStore()
			if err != nil {
				return err
			}
			fromT, toT, err := parseDayRange(from, to)
			if err != nil {
				return err
			}

			report, err := vault.VerifyRange(context.Background(), st, fromT, toT)
			if err != nil {
				return err
			}

			printReport(report)
			if !report.Pass {
				return fmt.Errorf("verification FAILED")
			}
			return nil
		},
	}
	c.Flags().StringVar(&from, "from", time.Now().UTC().Format("2006-01-02"), "start date (YYYY-MM-DD)")
	c.Flags().StringVar(&to, "to", time.Now().UTC().Format("2006-01-02"), "end date (YYYY-MM-DD)")
	return c
}

func newVaultProveCmd() *cobra.Command {
	var segmentID string
	var offset, length int64
	var sha string
	c := &cobra.Command{
		Use:   "prove",
		Short: "Print a Merkle inclusion proof for one event (chain-of-custody export)",
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := openVaultStore()
			if err != nil {
				return err
			}
			ref := vault.RawRef{SegmentID: segmentID, Offset: offset, Length: length, SHA256: sha}
			proof, err := vault.ProveEvent(context.Background(), st, ref)
			if err != nil {
				return err
			}
			fmt.Printf("proof valid: %v\n", proof.Verify())
			return json.NewEncoder(os.Stdout).Encode(proof)
		},
	}
	c.Flags().StringVar(&segmentID, "segment-id", "", "segment id")
	c.Flags().Int64Var(&offset, "offset", 0, "byte offset within the segment")
	c.Flags().Int64Var(&length, "length", 0, "byte length")
	c.Flags().StringVar(&sha, "sha256", "", "expected sha256")
	_ = c.MarkFlagRequired("segment-id")
	return c
}

func parseDayRange(from, to string) (time.Time, time.Time, error) {
	f, err := time.Parse("2006-01-02", from)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("bad --from date: %w", err)
	}
	t, err := time.Parse("2006-01-02", to)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("bad --to date: %w", err)
	}
	return f, t, nil
}

func printReport(report vault.Report) {
	status := "PASS"
	if !report.Pass {
		status = "FAIL"
	}
	fmt.Printf("VERIFY %s -> %s : %s\n", report.From.Format("2006-01-02"), report.To.Format("2006-01-02"), status)
	fmt.Printf("%-30s %-8s %-10s %-8s %s\n", "SEGMENT", "EVENTS", "CHAIN", "RESULT", "REASON")
	for _, sr := range report.Segments {
		result := "PASS"
		if !sr.Pass {
			result = "FAIL"
		}
		chain := "ok"
		if !sr.ChainOK {
			chain = "BROKEN"
		}
		fmt.Printf("%-30s %-8d %-10s %-8s %s\n", sr.SegmentID, sr.EventCount, chain, result, sr.Reason)
	}
}
