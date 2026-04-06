package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/spf13/cobra"
)

var useCmd = &cobra.Command{
	Use:   "use <batch.yaml>",
	Short: "Set an existing batch file as active",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename := args[0]

		absPath, err := filepath.Abs(filename)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		// If file doesn't exist, create from template
		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			tmpl := batch.DefaultBatchYAML(cfg)
			if err := os.WriteFile(absPath, []byte(tmpl), 0o644); err != nil {
				return fmt.Errorf("write batch file: %w", err)
			}
			fmt.Printf("Created:      %s\n", absPath)
		}

		return setActiveBatch(absPath)
	},
}

func init() {
	rootCmd.AddCommand(useCmd)
}
