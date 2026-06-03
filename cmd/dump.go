package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/dump"
	"github.com/bojin/datamask/internal/storage"
	"github.com/bojin/datamask/internal/transformer"
	"github.com/spf13/cobra"
)

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Dump and mask database data",
	Long:  `Export schema via pg_dump, then stream table data through configured transformers and store compressed results.`,
	RunE:  runDump,
}

func init() {
	rootCmd.AddCommand(dumpCmd)
	dumpCmd.Flags().String("name", "", "optional dump name suffix")
	dumpCmd.Flags().String("dsn", "", "override connection DSN from config")
}

func runDump(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	if err := validateTransformers(cfg); err != nil {
		return err
	}

	dsn, _ := cmd.Flags().GetString("dsn")
	if dsn == "" {
		dsn = cfg.Connection.DSN()
	}

	store, err := storage.New(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	nameSuffix, _ := cmd.Flags().GetString("name")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	d := dump.New(cfg, store, dsn, nameSuffix)
	if err := d.Run(ctx); err != nil {
		return fmt.Errorf("dump failed: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Dump completed successfully.")
	return nil
}

func validateTransformers(cfg *config.Config) error {
	for tableName, tblCfg := range cfg.Tables {
		for colName, txName := range tblCfg.Columns {
			if _, err := transformer.Get(txName); err != nil {
				return fmt.Errorf("table %q column %q: %w", tableName, colName, err)
			}
		}
	}
	return nil
}
