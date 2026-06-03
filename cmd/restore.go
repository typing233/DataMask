package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/restore"
	"github.com/bojin/datamask/internal/storage"
	"github.com/spf13/cobra"
)

var restoreCmd = &cobra.Command{
	Use:   "restore [dump-id]",
	Short: "Restore masked data to target database",
	Long:  `Restore schema via psql, then stream decompressed data into the target database using PostgreSQL COPY protocol.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runRestore,
}

func init() {
	rootCmd.AddCommand(restoreCmd)
	restoreCmd.Flags().String("target-dsn", "", "target database connection string (required)")
	restoreCmd.Flags().Bool("drop-schema", false, "drop and recreate public schema before restore")
	restoreCmd.MarkFlagRequired("target-dsn")
}

func runRestore(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	targetDSN, _ := cmd.Flags().GetString("target-dsn")
	dropSchema, _ := cmd.Flags().GetBool("drop-schema")

	store, err := storage.New(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	r := restore.New(cfg, store, targetDSN, dropSchema)
	if err := r.Run(ctx, args[0]); err != nil {
		return fmt.Errorf("restore failed: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Restore completed successfully.")
	return nil
}
