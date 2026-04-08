package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/pipeline"
	"github.com/mattjoyce/framore/internal/stages"
)

var (
	startStage  string
	startDryRun bool
	verbose     bool
)

var startCmd = &cobra.Command{
	Use:   "start [batch.yaml]",
	Short: "Execute the pipeline against the active batch",
	RunE: func(cmd *cobra.Command, args []string) error {
		batchPath := cfg.CurrentBatch
		if len(args) > 0 {
			batchPath = args[0]
		}
		if batchPath == "" {
			return fmt.Errorf("no active batch — run 'framore new' or 'framore use' first")
		}

		b, err := batch.Load(batchPath)
		if err != nil {
			return fmt.Errorf("load batch: %w", err)
		}

		// Build stage registry
		registry := []pipeline.Stage{
			&stages.EXIF{},
			&stages.Weather{Cfg: cfg},
			&stages.BirdNet{Cfg: cfg},
			&stages.Report{Cfg: cfg},
		}

		// Filter to single stage if requested
		if startStage != "" {
			var filtered []pipeline.Stage
			for _, s := range registry {
				if s.Name() == startStage {
					filtered = append(filtered, s)
				}
			}
			if len(filtered) == 0 {
				return fmt.Errorf("unknown stage: %s", startStage)
			}
			registry = filtered
		}

		if startDryRun {
			fmt.Println("Dry run — stages that would execute:")
			for _, s := range registry {
				en := "skip"
				if s.Enabled(b) {
					en = "run"
				}
				fmt.Printf("  [%s] %s\n", s.Name(), en)
			}
			return nil
		}

		ctx := context.Background()
		_, err = pipeline.Run(ctx, b, registry, verbose)
		return err
	},
}

func init() {
	startCmd.Flags().StringVar(&startStage, "stage", "", "Run a single stage only")
	startCmd.Flags().BoolVar(&startDryRun, "dry-run", false, "Print plan, don't execute")
	startCmd.Flags().BoolVar(&verbose, "verbose", false, "Per-file progress lines")
	rootCmd.AddCommand(startCmd)
}
