package stages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/config"
	"github.com/mattjoyce/framore/internal/pipeline"
)

// TranscriptSegment is one segment (start/end) of a transcription.
type TranscriptSegment struct {
	Segment             string  `json:"segment"`
	Text                string  `json:"text"`
	Language            string  `json:"language"`
	LanguageProbability float64 `json:"language_probability"`
}

// TranscribeFileResult holds the whisper response for one WAV file.
type TranscribeFileResult struct {
	FilePath    string              `json:"file_path"`
	Transcripts []TranscriptSegment `json:"transcripts"`
}

type Transcribe struct {
	Cfg *config.Config
}

func (t *Transcribe) Name() string { return "transcribe" }

func (t *Transcribe) Enabled(b *batch.Batch) bool { return b.Stages.Transcribe }

// SupportsNoWait returns false — transcribe blocks on synchronous HTTP per file
// and has no queued/fire-and-forget mode.
func (t *Transcribe) SupportsNoWait() bool { return false }

func (t *Transcribe) Run(ctx context.Context, b *batch.Batch, results *pipeline.Results) error {
	whisperURL := t.Cfg.Services.WhisperURL
	if whisperURL == "" {
		return fmt.Errorf("whisper_url not configured in config.toml")
	}

	duration := b.Transcribe.DurationSeconds
	if duration == 0 {
		duration = 60
	}

	client := &http.Client{Timeout: 120 * time.Second}
	processed := 0

	for _, f := range b.Files {
		if f.Type != "audio" {
			continue
		}

		nasPath, err := batch.CheckAllowedPath(f.Path, t.Cfg)
		if err != nil {
			fmt.Printf("  [transcribe] skip %s: %v\n", f.Path, err)
			continue
		}

		payload := map[string]any{
			"wav_path":         nasPath,
			"duration_seconds": duration,
		}

		body, err := json.Marshal(payload)
		if err != nil {
			fmt.Printf("  [transcribe] marshal error for %s: %v\n", f.Path, err)
			continue
		}

		fmt.Printf("  [transcribe] %s (first/last %ds)\n", filepath.Base(f.Path), duration)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, whisperURL+"/transcribe", bytes.NewReader(body))
		if err != nil {
			fmt.Printf("  [transcribe] request error for %s: %v\n", f.Path, err)
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("  [transcribe] %s: %v\n", filepath.Base(f.Path), err)
			continue
		}

		respBody, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			fmt.Printf("  [transcribe] read error for %s: %v\n", f.Path, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("  [transcribe] %s: HTTP %d: %s\n", filepath.Base(f.Path), resp.StatusCode, string(respBody))
			continue
		}

		var result struct {
			Transcripts []TranscriptSegment `json:"transcripts"`
		}
		if err := json.Unmarshal(respBody, &result); err != nil {
			fmt.Printf("  [transcribe] parse error for %s: %v\n", f.Path, err)
			continue
		}

		fr := TranscribeFileResult{
			FilePath:    f.Path,
			Transcripts: result.Transcripts,
		}
		results.Set("transcribe", f.Path, fr)
		processed++

		for _, seg := range result.Transcripts {
			if seg.Text != "" {
				fmt.Printf("  [transcribe] [%s] %s\n", seg.Segment, seg.Text)
			}
		}
	}

	fmt.Printf("  [transcribe] processed %d files\n", processed)
	return nil
}
