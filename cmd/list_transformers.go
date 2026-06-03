package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/bojin/datamask/internal/transformer"
	"github.com/spf13/cobra"
)

var listTransformersCmd = &cobra.Command{
	Use:   "list-transformers",
	Short: "List all available transformers",
	Long:  `Display all registered transformers with their descriptions and supported data types.`,
	RunE:  runListTransformers,
}

func init() {
	rootCmd.AddCommand(listTransformersCmd)
}

func runListTransformers(cmd *cobra.Command, args []string) error {
	all := transformer.GetAll()
	names := transformer.List()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tDESCRIPTION\tSUPPORTED TYPES\n")
	fmt.Fprintf(w, "----\t-----------\t---------------\n")

	for _, name := range names {
		t := all[name]
		desc := "-"
		types := "-"
		if d, ok := t.(transformer.Described); ok {
			desc = d.Description()
			st := d.SupportedTypes()
			if len(st) > 3 {
				types = fmt.Sprintf("%s, %s, %s, ... (%d total)", st[0], st[1], st[2], len(st))
			} else {
				types = joinStrings(st, ", ")
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, desc, types)
	}

	return w.Flush()
}

func joinStrings(ss []string, sep string) string {
	if len(ss) == 0 {
		return ""
	}
	result := ss[0]
	for _, s := range ss[1:] {
		result += sep + s
	}
	return result
}
