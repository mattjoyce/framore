package cmd

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/mattjoyce/framore/internal/ductile"
	"github.com/spf13/cobra"
)

var queuePlugin string
var queueLimit int

var queueCmd = &cobra.Command{
	Use:   "queue",
	Short: "Show Ductile job queue status",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Quick health check (no auth needed)
		healthURL := cfg.Services.DuctileAPIURL + "/healthz"
		healthResp, err := http.Get(healthURL)
		if err != nil {
			return fmt.Errorf("ductile unreachable at %s: %w", cfg.Services.DuctileAPIURL, err)
		}
		_ = healthResp.Body.Close()

		token := os.Getenv(cfg.Services.DuctileTokenEnv)
		if token == "" {
			return fmt.Errorf("ductile API token not set: export %s=<token>", cfg.Services.DuctileTokenEnv)
		}

		client := ductile.NewClient(cfg.Services.DuctileAPIURL, token)
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := client.ListJobs(ctx, queuePlugin, queueLimit)
		if err != nil {
			return fmt.Errorf("list jobs: %w", err)
		}

		// Tally by status
		counts := make(map[string]int)
		for _, j := range resp.Jobs {
			counts[j.Status]++
		}

		plugin := queuePlugin
		if plugin == "" {
			plugin = "all"
		}
		fmt.Printf("Ductile jobs (plugin=%s, last %d):\n", plugin, queueLimit)
		for _, status := range []string{"queued", "running", "succeeded", "failed", "dead", "timed_out"} {
			if c, ok := counts[status]; ok {
				fmt.Printf("  %-12s %d\n", status, c)
			}
		}
		// Any other statuses
		for status, c := range counts {
			switch status {
			case "queued", "running", "succeeded", "failed", "dead", "timed_out":
				continue
			default:
				fmt.Printf("  %-12s %d\n", status, c)
			}
		}
		fmt.Printf("  %-12s %d\n", "total", len(resp.Jobs))

		// Show recent non-terminal jobs
		fmt.Println()
		active := 0
		for _, j := range resp.Jobs {
			if j.Status == "queued" || j.Status == "running" {
				if active == 0 {
					fmt.Println("Active jobs:")
				}
				active++
				fmt.Printf("  %s  %-8s  %s/%s  (created %s)\n",
					j.JobID[:8], j.Status, j.Plugin, j.Command, j.CreatedAt[:19])
				if active >= 20 {
					remaining := 0
					for _, j2 := range resp.Jobs {
						if j2.Status == "queued" || j2.Status == "running" {
							remaining++
						}
					}
					if remaining > 20 {
						fmt.Printf("  ... and %d more\n", remaining-20)
					}
					break
				}
			}
		}
		if active == 0 {
			fmt.Println("No active jobs.")
		}

		return nil
	},
}

func init() {
	queueCmd.Flags().StringVarP(&queuePlugin, "plugin", "p", "birda", "Filter by plugin name (empty for all)")
	queueCmd.Flags().IntVarP(&queueLimit, "limit", "n", 500, "Max jobs to retrieve")
	rootCmd.AddCommand(queueCmd)
}
