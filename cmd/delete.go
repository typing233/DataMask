package cmd

import (
	"fmt"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/storage"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [dump-id]",
	Short: "Delete a dump",
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func init() {
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	store, err := storage.New(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	if err := store.DeleteDump(args[0]); err != nil {
		return fmt.Errorf("deleting dump: %w", err)
	}

	fmt.Printf("Dump %q deleted.\n", args[0])
	return nil
}
