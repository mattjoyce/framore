package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/config"
	"github.com/mattjoyce/framore/internal/ductile"
	"github.com/mattjoyce/framore/internal/pipeline"
)

// Detection is a single BirdNET detection from the birda plugin.
type Detection struct {
	StartS         float64 `json:"start_s"`
	EndS           float64 `json:"end_s"`
	ScientificName string  `json:"scientific_name"`
	CommonName     string  `json:"common_name"`
	Confidence     float64 `json:"confidence"`
}

// BirdNetFileResult holds the birda response for one WAV file.
type BirdNetFileResult struct {
	FilePath       string      `json:"file_path"`
	OutputPath     string      `json:"output_path"`
	Detections     []Detection `json:"detections"`
	DetectionCount int         `json:"detection_count"`
	DurationS      float64     `json:"duration_s"`
	RealtimeFactor float64     `json:"realtime_factor"`
	JobID          string      `json:"job_id"`
	JobStatus      string      `json:"job_status"`
}

// SpeciesSummary is one row in the session-unified species list.
type SpeciesSummary struct {
	ScientificName string  `json:"scientific_name"`
	CommonName     string  `json:"common_name"`
	MaxConfidence  float64 `json:"max_confidence"`
	TotalDetections int    `json:"total_detections"`
	FileCount      int     `json:"file_count"`
	FirstSeenS     float64 `json:"first_seen_s"`
	LastSeenS      float64 `json:"last_seen_s"`
}

// SessionBirdNetResult is the unified result across all files.
type SessionBirdNetResult struct {
	Species        []SpeciesSummary    `json:"species"`
	TotalFiles     int                 `json:"total_files"`
	TotalDetections int               `json:"total_detections"`
	FileResults    []BirdNetFileResult `json:"file_results"`
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

	// Setup ductile API client
	token := os.Getenv(bn.Cfg.Services.DuctileTokenEnv)
	if token == "" {
		return fmt.Errorf("ductile API token not set: export %s=<token>", bn.Cfg.Services.DuctileTokenEnv)
	}
	client := ductile.NewClient(bn.Cfg.Services.DuctileAPIURL, token)

	minConf := b.BirdNet.MinConf
	if minConf == 0 {
		minConf = bn.Cfg.Defaults.BirdnetMinConf
	}

	var fileResults []BirdNetFileResult
	totalDetections := 0

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

		payload := map[string]any{
			"wav_path": nasPath,
			"lat":      lat,
			"lon":      lon,
			"min_conf": minConf,
			"week":     week,
		}

		fmt.Printf("  [birdnet] submitting %s (week %d)\n", f.Path, week)

		// Submit job via API
		sr, err := client.Submit(ctx, "birda", "handle", payload)
		if err != nil {
			fmt.Printf("  [birdnet] submit error for %s: %v\n", f.Path, err)
			continue
		}

		fmt.Printf("  [birdnet] job %s queued for %s\n", sr.JobID, f.Path)

		// Poll for completion (sync)
		job, err := client.WaitForJob(ctx, sr.JobID, 3*time.Second)
		if err != nil {
			fmt.Printf("  [birdnet] poll error for %s: %v\n", f.Path, err)
			continue
		}

		if job.Status != "succeeded" {
			fmt.Printf("  [birdnet] job %s for %s: %s\n", job.JobID, f.Path, job.Status)
			continue
		}

		// Parse the result from the job response
		var pluginResp struct {
			Status         string      `json:"status"`
			OutputPath     string      `json:"output_path"`
			Detections     []Detection `json:"detections"`
			DetectionCount int         `json:"detection_count"`
			DurationS      float64     `json:"duration_s"`
			RealtimeFactor float64     `json:"realtime_factor"`
		}
		if err := json.Unmarshal(job.Result, &pluginResp); err != nil {
			fmt.Printf("  [birdnet] parse result error for %s: %v\n", f.Path, err)
			continue
		}

		fr := BirdNetFileResult{
			FilePath:       f.Path,
			OutputPath:     pluginResp.OutputPath,
			Detections:     pluginResp.Detections,
			DetectionCount: pluginResp.DetectionCount,
			DurationS:      pluginResp.DurationS,
			RealtimeFactor: pluginResp.RealtimeFactor,
			JobID:          job.JobID,
			JobStatus:      job.Status,
		}

		fileResults = append(fileResults, fr)
		totalDetections += fr.DetectionCount

		// Store per-file result
		results.Set("birdnet", f.Path, fr)
		fmt.Printf("  [birdnet] %s: %d detections (%.0fx realtime)\n",
			f.Path, fr.DetectionCount, fr.RealtimeFactor)
	}

	// Unify into session-level species summary
	session := unifyDetections(fileResults, totalDetections)
	results.Set("birdnet", "session", session)

	fmt.Printf("  [birdnet] session: %d species across %d files, %d total detections\n",
		len(session.Species), session.TotalFiles, session.TotalDetections)

	return nil
}

// unifyDetections merges per-file detections into a session-level species summary.
func unifyDetections(fileResults []BirdNetFileResult, totalDetections int) SessionBirdNetResult {
	type specKey struct{ sci, common string }
	specMap := make(map[specKey]*SpeciesSummary)
	fileSet := make(map[specKey]map[string]bool)

	for _, fr := range fileResults {
		for _, d := range fr.Detections {
			key := specKey{d.ScientificName, d.CommonName}
			s, ok := specMap[key]
			if !ok {
				s = &SpeciesSummary{
					ScientificName: d.ScientificName,
					CommonName:     d.CommonName,
					FirstSeenS:     d.StartS,
					LastSeenS:      d.EndS,
				}
				specMap[key] = s
				fileSet[key] = make(map[string]bool)
			}
			s.TotalDetections++
			if d.Confidence > s.MaxConfidence {
				s.MaxConfidence = d.Confidence
			}
			if d.StartS < s.FirstSeenS {
				s.FirstSeenS = d.StartS
			}
			if d.EndS > s.LastSeenS {
				s.LastSeenS = d.EndS
			}
			fileSet[key][fr.FilePath] = true
		}
	}

	species := make([]SpeciesSummary, 0, len(specMap))
	for key, s := range specMap {
		s.FileCount = len(fileSet[key])
		species = append(species, *s)
	}

	return SessionBirdNetResult{
		Species:         species,
		TotalFiles:      len(fileResults),
		TotalDetections: totalDetections,
		FileResults:     fileResults,
	}
}
