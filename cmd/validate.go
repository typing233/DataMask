package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/database"
	"github.com/bojin/datamask/internal/validate"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate configuration and data transformations",
	Long: `Run validation checks on the configuration file and optionally connect to the
database to verify schema compatibility, type safety, and preview transform results.

Available checks:
  config  - Validate YAML syntax, transformer names, and config consistency
  schema  - Compare config references against actual database schema
  types   - Check transformer/column type compatibility
  diff    - Preview before/after transformation on sample rows`,
	RunE: runValidate,
}

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.Flags().StringSlice("check", nil, "specific checks to run (config,schema,types,diff); defaults to all")
	validateCmd.Flags().Int("sample-rows", 5, "number of rows to sample for diff check")
	validateCmd.Flags().String("dsn", "", "override connection DSN from config")
}

func runValidate(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	dsn, _ := cmd.Flags().GetString("dsn")
	if dsn == "" {
		dsn = cfg.Connection.DSN()
	}

	sampleRows, _ := cmd.Flags().GetInt("sample-rows")
	checks, _ := cmd.Flags().GetStringSlice("check")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var db database.Database
	needsDB := len(checks) == 0 || containsAny(checks, "schema", "types", "diff")
	if needsDB {
		db, err = database.GetDriver("postgres")
		if err == nil {
			if connErr := db.Connect(ctx, dsn); connErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: could not connect to database: %v\n", connErr)
				fmt.Fprintf(os.Stderr, "Schema, type, and diff checks will be skipped.\n\n")
				db = nil
			} else {
				defer db.Close(ctx)
			}
		}
	}

	v := validate.New(cfg, db, dsn, sampleRows)

	var findings []validate.Finding
	if len(checks) == 0 {
		findings = v.RunAll(ctx)
	} else {
		findings = v.RunChecks(ctx, checks)
	}

	if len(findings) == 0 {
		fmt.Println("✓ All checks passed")
		return nil
	}

	errorCount := 0
	warningCount := 0
	for _, f := range findings {
		if f.Severity == validate.SeverityError {
			errorCount++
		} else {
			warningCount++
		}
		fmt.Println(f)
	}

	fmt.Printf("\nResults: %d error(s), %d warning(s)\n", errorCount, warningCount)

	if errorCount > 0 {
		return fmt.Errorf("validation failed with %d error(s)", errorCount)
	}
	return nil
}

func containsAny(slice []string, items ...string) bool {
	for _, item := range items {
		for _, s := range slice {
			if s == item {
				return true
			}
		}
	}
	return false
}
