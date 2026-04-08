package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/mattjoyce/framore/internal/batch"
)

var removeCmd = &cobra.Command{
	Use:   "remove <path>",
	Short: "Remove a file entry from the batch",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.CurrentBatch == "" {
			return fmt.Errorf("no active batch — run 'framore new' or 'framore use' first")
		}

		b, err := batch.Load(cfg.CurrentBatch)
		if err != nil {
			return fmt.Errorf("load batch: %w", err)
		}

		absPath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		found := false
		var filtered []batch.FileEntry
		for _, f := range b.Files {
			if f.Path == absPath {
				found = true
				continue
			}
			filtered = append(filtered, f)
		}

		if !found {
			return fmt.Errorf("file not in batch: %s", absPath)
		}

		b.Files = filtered
		if err := batch.Save(cfg.CurrentBatch, b); err != nil {
			return fmt.Errorf("save batch: %w", err)
		}

		fmt.Printf("Removed: %s\n", absPath)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
