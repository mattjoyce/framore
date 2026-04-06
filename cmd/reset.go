package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Clear the active batch setting",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg.CurrentBatch = ""
		if err := cfg.Save(); err != nil {
			return fmt.Errorf("save config: %w", err)
		}
		fmt.Println("Active batch cleared")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
