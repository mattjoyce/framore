package cmd

import (
	"fmt"
	"time"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/stages"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show active batch and pipeline stage summary",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.CurrentBatch == "" {
			fmt.Println("No active batch — run 'framore new' or 'framore use' first")
			return nil
		}

		b, err := batch.Load(cfg.CurrentBatch)
		if err != nil {
			return fmt.Errorf("load batch: %w", err)
		}

		fmt.Printf("Active batch: %s\n", cfg.CurrentBatch)
		fmt.Printf("Session dir:  %s\n", b.SessionDir)

		// Compute BirdNET week
		weekStr := "?"
		if b.SessionDate != "" {
			if bw, err := stages.BirdNETWeek(b.SessionDate); err == nil {
				if t, err := time.Parse("2006-01-02", b.SessionDate); err == nil {
					_, w := t.ISOWeek()
					weekStr = fmt.Sprintf("week %d → BirdNET week %d", w, bw)
				}
			}
		}
		fmt.Printf("Session date: %s  (%s)\n", b.SessionDate, weekStr)

		// Count files
		audioCount, imageCount := 0, 0
		for _, f := range b.Files {
			switch f.Type {
			case "audio":
				audioCount++
			case "image":
				imageCount++
			}
		}
		fmt.Printf("\nFiles: %d audio, %d images\n\n", audioCount, imageCount)

		// Stage summary
		fmt.Println("Stages:")
		printStage("exif", "local", b.Stages.EXIF, false,
			fmt.Sprintf("GPS fallback: %.4f, %.4f", b.Pipeline.DefaultLat, b.Pipeline.DefaultLon))
		printStage("weather", "local", b.Stages.Weather, false,
			fmt.Sprintf("timezone=%s", b.Weather.Timezone))
		printStage("birdnet", "→ ductile", b.Stages.BirdNet, false,
			fmt.Sprintf("min_conf=%.1f", b.BirdNet.MinConf))
		printStage("transcribe", "deferred", false, true, "")
		printStage("report", "→ ollama", b.Stages.Report, false,
			cfg.Services.OllamaURL)

		return nil
	},
}

func printStage(name, target string, enabled, deferred bool, detail string) {
	mark := "[ ]"
	if enabled {
		mark = "[x]"
	}
	if deferred {
		mark = "[-]"
	}
	fmt.Printf("  %s %-12s %-12s %s\n", mark, name, target, detail)
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
