package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "datamask",
	Short: "PostgreSQL data masking tool",
	Long:  `datamask integrates with pg_dump/pg_restore to dump, transform, and restore PostgreSQL data with configurable masking rules.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "datamask.yaml", "config file path")
}
