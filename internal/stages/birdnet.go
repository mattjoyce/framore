package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/config"
	"github.com/mattjoyce/framore/internal/ductile"
	"github.com/mattjoyce/framore/internal/pipeline"
)

type BirdNetPayload struct {
	Payload BirdNetPayloadInner `json:"payload"`
}

type BirdNetPayloadInner struct {
	WAVPath string  `json:"wav_path"`
	Lat     float64 `json:"lat"`
	Lon     float64 `json:"lon"`
	MinConf float64 `json:"min_conf"`
	Week    int     `json:"week"`
}

type BirdNetResult struct {
	OutputPath     string  `json:"output_path"`
	Detections     []any   `json:"detections"`
	DetectionCount int     `json:"detection_count"`
	DurationS      float64 `json:"duration_s"`
}

type BirdNet struct {
	Cfg *config.Config
}

func (bn *BirdNet) Name() string { return "birdnet" }

func (bn *BirdNet) Enabled(b *batch.Batch) bool { return b.Stages.BirdNet }

func (bn *BirdNet) Run(ctx context.Context, b *batch.Batch, results *pipeline.Results) error {
	week, err := BirdNETWeek(b.SessionDate)
	if err != nil {
		return fmt.Errorf("parse session_date: %w", err)
	}

	lat, lon := ResolveGPS(b, results)

	// Setup ductile client
	secret := os.Getenv(bn.Cfg.Services.DuctileSecretEnv)
	client := ductile.NewClient(bn.Cfg.Services.DuctileURL, secret)

	minConf := b.BirdNet.MinConf
	if minConf == 0 {
		minConf = bn.Cfg.Defaults.BirdnetMinConf
	}

	for _, f := range b.Files {
		if f.Type != "audio" {
			continue
		}

		// Translate Mac path to NAS path
		nasPath, err := batch.CheckAllowedPath(f.Path, bn.Cfg)
		if err != nil {
			fmt.Printf("  [birdnet] skip %s: %v\n", f.Path, err)
			continue
		}

		payload := BirdNetPayload{
			Payload: BirdNetPayloadInner{
				WAVPath: nasPath,
				Lat:     lat,
				Lon:     lon,
				MinConf: minConf,
				Week:    week,
			},
		}

		fmt.Printf("  [birdnet] processing %s (week %d)\n", f.Path, week)

		respBody, err := client.Post(ctx, payload)
		if err != nil {
			fmt.Printf("  [birdnet] error for %s: %v\n", f.Path, err)
			continue
		}

		var result BirdNetResult
		if err := json.Unmarshal(respBody, &result); err != nil {
			fmt.Printf("  [birdnet] parse error for %s: %v\n", f.Path, err)
			continue
		}

		results.Set("birdnet", f.Path, result)
		fmt.Printf("  [birdnet] %s: %d detections\n", f.Path, result.DetectionCount)
	}

	return nil
}
