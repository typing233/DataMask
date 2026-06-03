package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bojin/datamask/internal/config"
	"github.com/bojin/datamask/internal/storage"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list-dumps",
	Short: "List available dumps",
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	store, err := storage.New(cfg.StorageDir)
	if err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	dumps, err := store.ListDumps()
	if err != nil {
		return fmt.Errorf("listing dumps: %w", err)
	}

	if len(dumps) == 0 {
		fmt.Println("No dumps found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tSOURCE DB\tTABLES\tCREATED AT\tDURATION")
	fmt.Fprintln(w, "--\t---------\t------\t----------\t--------")
	for _, d := range dumps {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			d.ID,
			d.SourceDB,
			len(d.Tables),
			d.CreatedAt.Format("2006-01-02 15:04:05"),
			d.Duration.String(),
		)
	}
	w.Flush()
	return nil
}
