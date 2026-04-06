package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Print all files in the active batch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.CurrentBatch == "" {
			return fmt.Errorf("no active batch — run 'framore new' or 'framore use' first")
		}

		b, err := batch.Load(cfg.CurrentBatch)
		if err != nil {
			return fmt.Errorf("load batch: %w", err)
		}

		if len(b.Files) == 0 {
			fmt.Println("No files in batch")
			return nil
		}

		for _, f := range b.Files {
			name := filepath.Base(f.Path)
			switch f.Type {
			case "audio":
				fmt.Printf("  %s  %s  %.1fs  %dbit/%dkHz/%dch  %dMB\n",
					f.Type, name,
					f.Meta.DurationSeconds,
					f.Meta.BitDepth,
					f.Meta.SampleRate/1000,
					f.Meta.Channels,
					f.Meta.SizeBytes/(1024*1024),
				)
			case "image":
				fmt.Printf("  %s  %s  lat=%.4f lon=%.4f  %s\n",
					f.Type, name,
					f.Meta.Lat, f.Meta.Lon,
					f.Meta.Device,
				)
			default:
				fmt.Printf("  %s  %s\n", f.Type, name)
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
}
