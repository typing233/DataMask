package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a sample datamask.yaml config file",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().StringP("output", "o", "datamask.yaml", "output file path")
}

func runInit(cmd *cobra.Command, args []string) error {
	output, _ := cmd.Flags().GetString("output")

	if _, err := os.Stat(output); err == nil {
		return fmt.Errorf("file %q already exists; remove it first or choose a different path", output)
	}

	if err := os.WriteFile(output, []byte(sampleConfig), 0644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	fmt.Printf("Sample config written to %s\n", output)
	return nil
}

const sampleConfig = `# datamask configuration
# Documentation: https://github.com/bojin/datamask

storage_dir: ./dumps
parallelism: 4

connection:
  host: localhost
  port: 5432
  user: postgres
  password: ""
  dbname: mydb
  sslmode: prefer

# Global table filters (mutually exclusive)
# include_tables: ["users", "orders"]
exclude_tables:
  - schema_migrations

# Per-table transformation rules
tables:
  users:
    columns:
      email: mask-email
      first_name: mask-name
      last_name: mask-name
      password_hash: redact
      phone: redact

  orders:
    columns:
      customer_notes: redact

  # Exclude a table entirely
  # audit_log:
  #   exclude: true
`
