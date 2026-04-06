package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Interactive wizard to configure the active batch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.CurrentBatch == "" {
			return fmt.Errorf("no active batch — run 'framore new' or 'framore use' first")
		}

		// TODO: implement charmbracelet/huh interactive wizard
		fmt.Println("Interactive config wizard not yet implemented.")
		fmt.Printf("Edit the batch file directly: %s\n", cfg.CurrentBatch)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
}
