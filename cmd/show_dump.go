package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/storage"
	"github.com/spf13/cobra"
)

var showDumpCmd = &cobra.Command{
	Use:   "show-dump [dump-id]",
	Short: "Show detailed information about a dump",
	Long:  `Display detailed metadata for a specific dump including tables, row counts, transformers applied, timing, and configuration.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShowDump,
}

func init() {
	rootCmd.AddCommand(showDumpCmd)
	showDumpCmd.Flags().Bool("show-config", false, "display the config snapshot stored with the dump")
}

func runShowDump(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	store, err := storage.New(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	meta, err := store.LoadMetadata(args[0])
	if err != nil {
		return fmt.Errorf("loading dump metadata: %w", err)
	}

	fmt.Printf("Dump: %s\n", meta.ID)
	fmt.Printf("Created: %s\n", meta.CreatedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Source DB: %s\n", meta.SourceDB)
	fmt.Printf("Duration: %s\n", meta.Duration)

	if meta.PGVersion != "" {
		fmt.Printf("PostgreSQL: %s\n", meta.PGVersion)
	}
	if meta.Version != "" {
		fmt.Printf("DataMask Version: %s\n", meta.Version)
	}
	if meta.Description != "" {
		fmt.Printf("Description: %s\n", meta.Description)
	}
	if len(meta.Tags) > 0 {
		fmt.Printf("Tags: %v\n", meta.Tags)
	}

	fmt.Printf("\nSummary:\n")
	fmt.Printf("  Tables: %d\n", len(meta.Tables))
	fmt.Printf("  Total Rows: %d\n", meta.TotalRows)
	if meta.TotalSize > 0 {
		fmt.Printf("  Total Compressed Size: %s\n", formatBytes(meta.TotalSize))
	}

	fmt.Printf("\nTables:\n")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  NAME\tROWS\tSIZE\tTRANSFORMERS\n")
	fmt.Fprintf(w, "  ----\t----\t----\t------------\n")
	for _, tbl := range meta.Tables {
		txCount := len(tbl.Transformers)
		txInfo := fmt.Sprintf("%d columns masked", txCount)
		if txCount == 0 {
			txInfo = "none"
		}
		fmt.Fprintf(w, "  %s\t%d\t%s\t%s\n", tbl.FullName(), tbl.RowCount, formatBytes(tbl.CompressedSize), txInfo)
	}
	w.Flush()

	showConfig, _ := cmd.Flags().GetBool("show-config")
	if showConfig && meta.ConfigSnapshot != "" {
		fmt.Printf("\nConfig Snapshot:\n%s\n", meta.ConfigSnapshot)
	}

	return nil
}

func formatBytes(b int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.2f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.2f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
