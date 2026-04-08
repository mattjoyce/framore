package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/spf13/cobra"
)

var noEdit bool

var newCmd = &cobra.Command{
	Use:   "new <name>",
	Short: "Create a new batch YAML and set it as active",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		// Strip .yaml/.yml suffix if user included it
		ext := filepath.Ext(name)
		if ext == ".yaml" || ext == ".yml" {
			name = name[:len(name)-len(ext)]
		}
		filename := name + ".yaml"

		// Write template
		tmpl := batch.DefaultBatchYAML(cfg)
		if err := os.WriteFile(filename, []byte(tmpl), 0o600); err != nil {
			return fmt.Errorf("write batch file: %w", err)
		}

		absPath, err := filepath.Abs(filename)
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		fmt.Printf("Created:      %s\n", absPath)

		// Set as active batch
		return setActiveBatch(absPath)
	},
}

func setActiveBatch(absPath string) error {
	cfg.CurrentBatch = absPath
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	fmt.Printf("Active batch: %s\n", absPath)
	return nil
}

func init() {
	newCmd.Flags().BoolVar(&noEdit, "no-edit", false, "Don't open in $EDITOR")
	rootCmd.AddCommand(newCmd)
}
