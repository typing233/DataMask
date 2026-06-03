package cmd

import (
	// Ensure all built-in transformers are registered via init().
	_ "github.com/bojin/datamask/internal/transformer"
)
