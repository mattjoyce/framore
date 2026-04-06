package cmd

import (
	"fmt"
	"os"

	"github.com/mattjoyce/framore/internal/config"
	"github.com/spf13/cobra"
)

var (
	cfg     *config.Config
	rootCmd = &cobra.Command{
		Use:   "framore",
		Short: "Field recording batch CLI",
		Long:  "framore is the Mac-side CLI companion to the fram-harness pipeline running on the NAS.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var err error
			cfg, err = config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			return nil
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
