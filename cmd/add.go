package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mattjoyce/framore/internal/batch"
)

var supportedExts = map[string]string{
	".wav":  "audio",
	".jpg":  "image",
	".jpeg": "image",
	".png":  "image",
}

var addCmd = &cobra.Command{
	Use:   "add <path> [path...]",
	Short: "Add files to the active batch",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.CurrentBatch == "" {
			return fmt.Errorf("no active batch — run 'framore new' or 'framore use' first")
		}

		b, err := batch.Load(cfg.CurrentBatch)
		if err != nil {
			return fmt.Errorf("load batch: %w", err)
		}

		var paths []string
		for _, arg := range args {
			// Expand globs
			matches, err := filepath.Glob(arg)
			if err != nil {
				return fmt.Errorf("glob %s: %w", arg, err)
			}
			if matches == nil {
				// Not a glob, treat as literal path
				matches = []string{arg}
			}

			for _, m := range matches {
				info, err := os.Stat(m)
				if err != nil {
					fmt.Printf("✗ %s — %v\n", m, err)
					continue
				}
				if info.IsDir() {
					// Recurse directory
					err := filepath.Walk(m, func(p string, fi os.FileInfo, err error) error {
						if err != nil {
							return nil
						}
						if !fi.IsDir() {
							paths = append(paths, p)
						}
						return nil
					})
					if err != nil {
						fmt.Printf("✗ %s — %v\n", m, err)
					}
				} else {
					paths = append(paths, m)
				}
			}
		}

		added := 0
		for _, p := range paths {
			absPath, err := filepath.Abs(p)
			if err != nil {
				fmt.Printf("✗ %s — %v\n", p, err)
				continue
			}

			// Check extension
			ext := strings.ToLower(filepath.Ext(absPath))
			fileType, ok := supportedExts[ext]
			if !ok {
				fmt.Printf("⚠ %s — unsupported format\n", absPath)
				continue
			}

			// Check allowed path
			if _, err := batch.CheckAllowedPath(absPath, cfg); err != nil {
				fmt.Printf("✗ %s — %v\n", absPath, err)
				continue
			}

			// Check duplicate
			if batch.HasFile(b, absPath) {
				fmt.Printf("· %s — already in batch\n", absPath)
				continue
			}

			// Inspect file
			var meta batch.FileMeta
			switch fileType {
			case "audio":
				meta, err = batch.InspectWAV(absPath)
			case "image":
				meta, err = batch.InspectImage(absPath)
			}
			if err != nil {
				fmt.Printf("⚠ %s — inspect error: %v\n", absPath, err)
				// Still add with empty meta
			}

			entry := batch.FileEntry{
				Path:  absPath,
				Type:  fileType,
				Added: time.Now().UTC().Format(time.RFC3339),
				Meta:  meta,
			}

			b.Files = append(b.Files, entry)
			added++
			fmt.Printf("✓ %s\n", absPath)
		}

		if added > 0 {
			if err := batch.Save(cfg.CurrentBatch, b); err != nil {
				return fmt.Errorf("save batch: %w", err)
			}
			fmt.Printf("\nAdded %d file(s) to batch\n", added)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
