package cmd

import (
	"fmt"

	"github.com/bojin/datamask/internal/transformer"
	"github.com/spf13/cobra"
)

var showTransformerCmd = &cobra.Command{
	Use:   "show-transformer [name]",
	Short: "Show detailed information about a transformer",
	Long:  `Display detailed documentation for a specific transformer including description, supported types, and usage examples.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runShowTransformer,
}

func init() {
	rootCmd.AddCommand(showTransformerCmd)
}

func runShowTransformer(cmd *cobra.Command, args []string) error {
	name := args[0]

	t, err := transformer.Get(name)
	if err != nil {
		return err
	}

	fmt.Printf("Name: %s\n", t.Name())

	d, ok := t.(transformer.Described)
	if !ok {
		fmt.Println("No additional documentation available for this transformer.")
		return nil
	}

	fmt.Printf("Description: %s\n\n", d.Description())
	fmt.Printf("Supported Types:\n")
	for _, st := range d.SupportedTypes() {
		fmt.Printf("  - %s\n", st)
	}

	fmt.Printf("\nDetails:\n%s\n", d.DetailedHelp())

	examples := d.Examples()
	if len(examples) > 0 {
		fmt.Printf("\nExamples:\n")
		for _, ex := range examples {
			fmt.Printf("  [%s] %q → %q\n", ex.DataType, ex.Input, ex.Output)
		}
	}

	return nil
}
