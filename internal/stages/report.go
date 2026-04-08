package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/config"
	"github.com/mattjoyce/framore/internal/ollama"
	"github.com/mattjoyce/framore/internal/pipeline"
)

const defaultModel = "gemma3:4b"

const defaultReportPrompt = `You are a field naturalist writing a concise session report for a bioacoustic recording session.

Write a narrative summary in markdown format. Be factual and specific. Include:
- Location and date context
- Weather conditions during the recording
- Bird species detected, highlighting notable or significant detections
- Any patterns (e.g. dawn chorus, activity peaks)
- A brief ecological interpretation

Use a warm but scientific tone. Keep it under 500 words.

Do not invent species or observations that are not in the data provided.
Only reference species listed in the BirdNET detections table.`

type Report struct {
	Cfg *config.Config
}

func (r *Report) Name() string { return "report" }

func (r *Report) Enabled(b *batch.Batch) bool { return b.Stages.Report }

func (r *Report) Run(ctx context.Context, b *batch.Batch, results *pipeline.Results) error {
	systemPrompt := loadPromptTemplate()
	dataContext := buildDataContext(b, results)
	prompt := systemPrompt + "\n\nHere is the session data:\n\n" + dataContext

	fmt.Printf("  [report] generating narrative via ollama (%s)…\n", defaultModel)

	client := ollama.NewClient(r.Cfg.Services.OllamaURL)
	response, err := client.Generate(ctx, defaultModel, prompt)
	if err != nil {
		return fmt.Errorf("ollama generate: %w", err)
	}

	// Write report to session folder
	outPath := filepath.Join(b.SessionDir, "session_report.md")
	report := formatReport(b, response)
	if err := os.WriteFile(outPath, []byte(report), 0o600); err != nil { // #nosec G306
		return fmt.Errorf("write report: %w", err)
	}

	results.Set("report", "session", outPath)
	fmt.Printf("  [report] written to %s\n", outPath)
	return nil
}

// loadPromptTemplate reads the report prompt from ~/.config/framore/report_prompt.md.
// If the file doesn't exist, writes the built-in default and returns it.
func loadPromptTemplate() string {
	path := filepath.Join(config.ConfigDir(), "report_prompt.md")
	data, err := os.ReadFile(path) // #nosec G304 — path is user's config dir
	if err != nil {
		// Write default so the user can edit it
		_ = os.MkdirAll(config.ConfigDir(), 0o750)
		_ = os.WriteFile(path, []byte(defaultReportPrompt+"\n"), 0o600)
		fmt.Printf("  [report] created prompt template: %s\n", path)
		return defaultReportPrompt
	}
	content := strings.TrimSpace(string(data))
	if content == "" {
		return defaultReportPrompt
	}
	return content
}

func buildDataContext(b *batch.Batch, results *pipeline.Results) string {
	var sb strings.Builder

	// Session info
	fmt.Fprintf(&sb, "## Session\n- Date: %s\n- Location: %s\n", b.SessionDate, b.SessionDir)
	if b.Pipeline.PlusCode != "" {
		fmt.Fprintf(&sb, "- Plus code: %s\n", b.Pipeline.PlusCode)
	}
	fmt.Fprintf(&sb, "- GPS: %.4f, %.4f\n", b.Pipeline.DefaultLat, b.Pipeline.DefaultLon)

	// File summary
	audioCount := 0
	var totalDuration float64
	for _, f := range b.Files {
		if f.Type == "audio" {
			audioCount++
			totalDuration += f.Meta.DurationSeconds
		}
	}
	fmt.Fprintf(&sb, "- Audio files: %d (%.1f hours total)\n\n", audioCount, totalDuration/3600)

	// Weather
	if raw, ok := results.Get("weather", "session"); ok {
		if wr, ok := raw.(WeatherResult); ok {
			sb.WriteString("## Weather\n")
			fmt.Fprintf(&sb, "- Sunrise: %s\n- Sunset: %s\n", wr.Sunrise, wr.Sunset)

			if len(wr.Hourly) > 0 {
				var minT, maxT float64 = 100, -100
				var totalPrecip float64
				var maxWind float64
				for _, h := range wr.Hourly {
					if h.Temperature < minT {
						minT = h.Temperature
					}
					if h.Temperature > maxT {
						maxT = h.Temperature
					}
					totalPrecip += h.Precipitation
					if h.WindSpeed > maxWind {
						maxWind = h.WindSpeed
					}
				}
				fmt.Fprintf(&sb, "- Temperature: %.1f–%.1f°C\n", minT, maxT)
				fmt.Fprintf(&sb, "- Total precipitation: %.1f mm\n", totalPrecip)
				fmt.Fprintf(&sb, "- Max wind: %.1f km/h\n", maxWind)
			}
			sb.WriteString("\n")
		}
	}

	// BirdNET detections
	if raw, ok := results.Get("birdnet", "session"); ok {
		if sr, ok := raw.(SessionBirdNetResult); ok {
			sb.WriteString("## BirdNET Detections\n")
			fmt.Fprintf(&sb, "- Total detections: %d across %d files\n", sr.TotalDetections, sr.TotalFiles)
			fmt.Fprintf(&sb, "- Species count: %d\n\n", len(sr.Species))

			// Sort by total detections descending
			sorted := make([]SpeciesSummary, len(sr.Species))
			copy(sorted, sr.Species)
			sort.Slice(sorted, func(i, j int) bool {
				return sorted[i].TotalDetections > sorted[j].TotalDetections
			})

			sb.WriteString("| Species | Detections | Files | Max Confidence | Time Range |\n")
			sb.WriteString("|---------|-----------|-------|---------------|------------|\n")
			for _, s := range sorted {
				name := s.CommonName
				if s.ScientificName != "" {
					name = fmt.Sprintf("%s (%s)", s.CommonName, s.ScientificName)
				}
				fmt.Fprintf(&sb, "| %s | %d | %d | %.1f%% | %.0fs–%.0fs |\n",
					name, s.TotalDetections, s.FileCount, s.MaxConfidence*100, s.FirstSeenS, s.LastSeenS)
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

func formatReport(b *batch.Batch, narrative string) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "# Session Report: %s\n\n", b.SessionDate)
	fmt.Fprintf(&sb, "**Generated:** %s  \n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Fprintf(&sb, "**Session:** %s  \n", filepath.Base(b.SessionDir))
	if b.Pipeline.PlusCode != "" {
		fmt.Fprintf(&sb, "**Location:** %s (%.4f, %.4f)  \n", b.Pipeline.PlusCode, b.Pipeline.DefaultLat, b.Pipeline.DefaultLon)
	} else {
		fmt.Fprintf(&sb, "**Location:** %.4f, %.4f  \n", b.Pipeline.DefaultLat, b.Pipeline.DefaultLon)
	}
	sb.WriteString("\n---\n\n")
	sb.WriteString(narrative)
	sb.WriteString("\n")

	return sb.String()
}
