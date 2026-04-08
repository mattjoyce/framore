package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/mattjoyce/framore/internal/batch"
)

var transcribeDuration int

func init() {
	transcribeCmd.Flags().IntVarP(&transcribeDuration, "duration", "d", 60, "seconds to transcribe from start and end of file")
	rootCmd.AddCommand(transcribeCmd)
}

var transcribeCmd = &cobra.Command{
	Use:   "transcribe <file>",
	Short: "Transcribe a WAV file via the faster-whisper service",
	Long:  "One-shot transcription — sends the file path to the whisper service and prints the result. No batch config needed.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		absPath, err := filepath.Abs(args[0])
		if err != nil {
			return fmt.Errorf("resolve path: %w", err)
		}

		nasPath, err := batch.CheckAllowedPath(absPath, cfg)
		if err != nil {
			return err
		}

		payload := map[string]any{
			"wav_path":         nasPath,
			"duration_seconds": transcribeDuration,
		}
		body, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}

		url := cfg.Services.WhisperURL + "/transcribe"
		fmt.Printf("  [transcribe] %s (first/last %ds)\n", filepath.Base(absPath), transcribeDuration)

		client := &http.Client{Timeout: 120 * time.Second}
		resp, err := client.Post(url, "application/json", bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("whisper request failed: %w", err)
		}
		defer func() { _ = resp.Body.Close() }()

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("whisper returned %d: %s", resp.StatusCode, string(respBody))
		}

		var result struct {
			Transcripts []struct {
				Segment             string  `json:"segment"`
				Text                string  `json:"text"`
				Language            string  `json:"language"`
				LanguageProbability float64 `json:"language_probability"`
			} `json:"transcripts"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}

		for _, t := range result.Transcripts {
			if t.Text == "" {
				fmt.Printf("  [%s] (no speech detected)\n", t.Segment)
			} else {
				fmt.Printf("  [%s] %s\n", t.Segment, t.Text)
			}
		}

		return nil
	},
}
