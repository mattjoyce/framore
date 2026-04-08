package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	ScientificName  string  `json:"scientific_name"`
	CommonName      string  `json:"common_name"`
	MaxConfidence   float64 `json:"max_confidence"`
	TotalDetections int     `json:"total_detections"`
	FileCount       int     `json:"file_count"`
	FirstSeenS      float64 `json:"first_seen_s"`
	LastSeenS       float64 `json:"last_seen_s"`
}

// SessionBirdNetResult is the unified result across all files.
type SessionBirdNetResult struct {
	Species         []SpeciesSummary    `json:"species"`
	TotalFiles      int                 `json:"total_files"`
	TotalDetections int                 `json:"total_detections"`
	FileResults     []BirdNetFileResult `json:"file_results"`
}

// pendingJob tracks a submitted Ductile job awaiting completion.
type pendingJob struct {
	jobID    string
	filePath string
}

// pollProgress tracks running stats during the poll phase.
type pollProgress struct {
	total      int
	completed  int
	failed     int
	detections int
	species    map[string]bool
	started    time.Time
}

func (p *pollProgress) printLine() {
	elapsed := time.Since(p.started).Truncate(time.Second)
	done := p.completed + p.failed
	if p.failed > 0 {
		fmt.Printf("\r\033[2K  [birdnet] %d/%d complete (%d failed) | %d detections | %d species | elapsed %s",
			done, p.total, p.failed, p.detections, len(p.species), elapsed)
	} else {
		fmt.Printf("\r\033[2K  [birdnet] %d/%d complete | %d detections | %d species | elapsed %s",
			done, p.total, p.detections, len(p.species), elapsed)
	}
}

func (p *pollProgress) clearLine() {
	fmt.Print("\r\033[2K")
}

// emit clears the progress line, prints a status message, then reprints the progress line.
func (p *pollProgress) emit(format string, args ...any) {
	p.clearLine()
	fmt.Printf(format, args...)
	p.printLine()
}

// birdnetOutputPath returns the expected BirdNET output path for a WAV file.
// Pattern: <dir>/<basename_without_ext>.BirdNET.selection.table.txt
func birdnetOutputPath(wavPath string) string {
	dir := filepath.Dir(wavPath)
	base := filepath.Base(wavPath)
	ext := filepath.Ext(base)
	stem := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, stem+".BirdNET.selection.table.txt")
}

type BirdNet struct {
	Cfg        *config.Config
	SubmitOnly bool
}

func (bn *BirdNet) Name() string { return "birdnet" }

func (bn *BirdNet) Enabled(b *batch.Batch) bool { return b.Stages.BirdNet }

func (bn *BirdNet) Run(ctx context.Context, b *batch.Batch, results *pipeline.Results) error {
	started := time.Now()

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

	// Phase 1: Submit all audio files, collect job IDs
	var pending []pendingJob
	skipped := 0
	for _, f := range b.Files {
		if f.Type != "audio" {
			continue
		}

		if b.BirdNet.SkipExisting {
			outPath := birdnetOutputPath(f.Path)
			if _, err := os.Stat(outPath); err == nil {
				skipped++
				fmt.Printf("  [birdnet] skip %s (output exists)\n", filepath.Base(f.Path))
				continue
			}
		}

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

		sr, err := client.Submit(ctx, "birda", "handle", payload)
		if err != nil {
			fmt.Printf("  [birdnet] submit error for %s: %v\n", f.Path, err)
			continue
		}

		fmt.Printf("  [birdnet] job %s queued for %s\n", sr.JobID, f.Path)
		pending = append(pending, pendingJob{jobID: sr.JobID, filePath: f.Path})
	}

	if bn.SubmitOnly {
		fmt.Printf("  [birdnet] submitted %d jobs (submit-only mode, skipping poll)\n", len(pending))
		if skipped > 0 {
			fmt.Printf("  [birdnet] skipped %d files (existing output)\n", skipped)
		}
		return nil
	}

	if skipped > 0 {
		fmt.Printf("  [birdnet] submitted %d jobs, skipped %d (existing output), polling for completion…\n", len(pending), skipped)
	} else {
		fmt.Printf("  [birdnet] submitted %d jobs, polling for completion…\n", len(pending))
	}

	// Phase 2: Poll all jobs until all reach terminal state
	var fileResults []BirdNetFileResult

	progress := &pollProgress{
		total:   len(pending),
		species: make(map[string]bool),
		started: started,
	}
	progress.printLine()

	remaining := pending

	for len(remaining) > 0 {
		select {
		case <-ctx.Done():
			progress.clearLine()
			return ctx.Err()
		default:
		}

		var stillPending []pendingJob
		for _, p := range remaining {
			job, err := client.GetJob(ctx, p.jobID)
			if err != nil {
				progress.failed++
				progress.emit("  [birdnet] poll error for %s: %v\n", p.filePath, err)
				continue // drop this job from tracking
			}

			switch job.Status {
			case "queued", "running":
				stillPending = append(stillPending, p)
				continue
			case "succeeded":
				var pluginResp struct {
					Status         string      `json:"status"`
					OutputPath     string      `json:"output_path"`
					Detections     []Detection `json:"detections"`
					DetectionCount int         `json:"detection_count"`
					DurationS      float64     `json:"duration_s"`
					RealtimeFactor float64     `json:"realtime_factor"`
				}
				if err := json.Unmarshal(job.Result, &pluginResp); err != nil {
					progress.failed++
					progress.emit("  [birdnet] parse result error for %s: %v\n", p.filePath, err)
					continue
				}

				fr := BirdNetFileResult{
					FilePath:       p.filePath,
					OutputPath:     pluginResp.OutputPath,
					Detections:     pluginResp.Detections,
					DetectionCount: pluginResp.DetectionCount,
					DurationS:      pluginResp.DurationS,
					RealtimeFactor: pluginResp.RealtimeFactor,
					JobID:          job.JobID,
					JobStatus:      job.Status,
				}

				fileResults = append(fileResults, fr)

				progress.completed++
				progress.detections += fr.DetectionCount
				for _, d := range fr.Detections {
					progress.species[d.ScientificName] = true
				}

				results.Set("birdnet", p.filePath, fr)
				progress.emit("  [birdnet] %s: %d detections (%.0fx realtime)\n",
					p.filePath, fr.DetectionCount, fr.RealtimeFactor)
			default:
				progress.failed++
				progress.emit("  [birdnet] job %s for %s: %s\n", job.JobID, p.filePath, job.Status)
			}
		}

		remaining = stillPending
		if len(remaining) > 0 {
			progress.printLine()
			select {
			case <-ctx.Done():
				progress.clearLine()
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
		}
	}

	progress.clearLine()

	// Unify into session-level species summary
	session := unifyDetections(fileResults, progress.detections)
	results.Set("birdnet", "session", session)

	elapsed := time.Since(started)
	fmt.Printf("  [birdnet] session: %d species across %d files, %d total detections (elapsed %s)\n",
		len(session.Species), session.TotalFiles, session.TotalDetections, elapsed.Truncate(time.Second))

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
