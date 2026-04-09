package stages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/config"
	"github.com/mattjoyce/framore/internal/ollama"
	"github.com/mattjoyce/framore/internal/pipeline"
)

const defaultModel = "gemma3:4b"

// largeSessionThreshold is the detection count above which the report
// uses summarised context (top-N species, hourly bucketing, two-pass).
const largeSessionThreshold = 5000

// topSpeciesLimit is the maximum number of species sent to the LLM
// for large sessions.
const topSpeciesLimit = 20

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

// SupportsNoWait returns false — report depends on completed birdnet results
// and blocks on synchronous LLM generation.
func (r *Report) SupportsNoWait() bool { return false }

func (r *Report) Run(ctx context.Context, b *batch.Batch, results *pipeline.Results) error {
	systemPrompt := loadPromptTemplate(r.Cfg.Report.PromptFile)
	client := ollama.NewClient(r.Cfg.Services.OllamaURL)

	isLarge := isLargeSession(results)
	var response string

	if isLarge {
		fmt.Printf("  [report] large session detected — using two-pass generation\n")

		// Pass 1: per-species summaries
		speciesContext := buildSpeciesSummaryContext(b, results)
		pass1Prompt := "You are a field naturalist. Write a brief 1-2 sentence summary for each of the following species detected in this bioacoustic recording session. Focus on detection frequency, confidence levels, and any notable patterns.\n\n" + speciesContext

		fmt.Printf("  [report] pass 1: species summaries via ollama (%s)…\n", defaultModel)
		speciesSummaries, err := client.Generate(ctx, defaultModel, pass1Prompt)
		if err != nil {
			return fmt.Errorf("ollama pass 1: %w", err)
		}

		// Pass 2: synthesise narrative using summaries + compact context
		dataContext := buildLargeSessionContext(b, results)
		pass2Prompt := systemPrompt + "\n\nHere is the session data (summarised for a large recording set):\n\n" + dataContext + "\n\n## Species Summaries (from analysis pass)\n\n" + speciesSummaries

		fmt.Printf("  [report] pass 2: narrative synthesis via ollama (%s)…\n", defaultModel)
		response, err = client.Generate(ctx, defaultModel, pass2Prompt)
		if err != nil {
			return fmt.Errorf("ollama pass 2: %w", err)
		}
	} else {
		dataContext := buildDataContext(b, results)
		prompt := systemPrompt + "\n\nHere is the session data:\n\n" + dataContext

		fmt.Printf("  [report] generating narrative via ollama (%s)…\n", defaultModel)
		var err error
		response, err = client.Generate(ctx, defaultModel, prompt)
		if err != nil {
			return fmt.Errorf("ollama generate: %w", err)
		}
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

// isLargeSession checks if the birdnet session result exceeds the large session threshold.
func isLargeSession(results *pipeline.Results) bool {
	raw, ok := results.Get("birdnet", "session")
	if !ok {
		return false
	}
	sr, ok := raw.(SessionBirdNetResult)
	if !ok {
		return false
	}
	return sr.TotalDetections >= largeSessionThreshold
}

// loadPromptTemplate reads the report prompt from the config directory.
// If the file doesn't exist, writes the built-in default and returns it.
func loadPromptTemplate(filename string) string {
	path := filepath.Join(config.ConfigDir(), filename)
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
			writeBirdNETSection(&sb, sr, 0) // 0 = no limit (show all)
		}
	}

	return sb.String()
}

// sortedSpeciesByDetections returns a copy of species sorted by TotalDetections descending.
func sortedSpeciesByDetections(species []SpeciesSummary) []SpeciesSummary {
	sorted := make([]SpeciesSummary, len(species))
	copy(sorted, species)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TotalDetections > sorted[j].TotalDetections
	})
	return sorted
}

// writeBirdNETSection writes the BirdNET detections table. If limit > 0,
// only the top N species by detection count are included and the rest noted.
func writeBirdNETSection(sb *strings.Builder, sr SessionBirdNetResult, limit int) {
	sb.WriteString("## BirdNET Detections\n")
	fmt.Fprintf(sb, "- Total detections: %d across %d files\n", sr.TotalDetections, sr.TotalFiles)
	fmt.Fprintf(sb, "- Species count: %d\n\n", len(sr.Species))

	sorted := sortedSpeciesByDetections(sr.Species)

	show := sorted
	remainder := 0
	if limit > 0 && len(sorted) > limit {
		show = sorted[:limit]
		remainder = len(sorted) - limit
	}

	sb.WriteString("| Species | Detections | Files | Max Confidence | Time Range |\n")
	sb.WriteString("|---------|-----------|-------|---------------|------------|\n")
	for _, s := range show {
		name := s.CommonName
		if s.ScientificName != "" {
			name = fmt.Sprintf("%s (%s)", s.CommonName, s.ScientificName)
		}
		fmt.Fprintf(sb, "| %s | %d | %d | %.1f%% | %.0fs–%.0fs |\n",
			name, s.TotalDetections, s.FileCount, s.MaxConfidence*100, s.FirstSeenS, s.LastSeenS)
	}

	if remainder > 0 {
		fmt.Fprintf(sb, "\n*…and %d more species with fewer detections*\n", remainder)
	}
	sb.WriteString("\n")
}

// buildSpeciesSummaryContext creates a concise species list for the first
// LLM pass in two-pass generation.
func buildSpeciesSummaryContext(b *batch.Batch, results *pipeline.Results) string {
	var sb strings.Builder

	fmt.Fprintf(&sb, "Session: %s on %s\n\n", filepath.Base(b.SessionDir), b.SessionDate)

	raw, ok := results.Get("birdnet", "session")
	if !ok {
		return sb.String()
	}
	sr, ok := raw.(SessionBirdNetResult)
	if !ok {
		return sb.String()
	}

	sorted := sortedSpeciesByDetections(sr.Species)

	show := sorted
	if len(sorted) > topSpeciesLimit {
		show = sorted[:topSpeciesLimit]
	}

	for i, s := range show {
		name := s.CommonName
		if s.ScientificName != "" {
			name = fmt.Sprintf("%s (%s)", s.CommonName, s.ScientificName)
		}
		fmt.Fprintf(&sb, "%d. %s — %d detections across %d files, max confidence %.0f%%\n",
			i+1, name, s.TotalDetections, s.FileCount, s.MaxConfidence*100)
	}

	return sb.String()
}

// buildLargeSessionContext creates a compact data context for large sessions
// with top-N species and hourly activity bucketing.
func buildLargeSessionContext(b *batch.Batch, results *pipeline.Results) string {
	var sb strings.Builder

	// Session info (same as normal)
	fmt.Fprintf(&sb, "## Session\n- Date: %s\n- Location: %s\n", b.SessionDate, b.SessionDir)
	if b.Pipeline.PlusCode != "" {
		fmt.Fprintf(&sb, "- Plus code: %s\n", b.Pipeline.PlusCode)
	}
	fmt.Fprintf(&sb, "- GPS: %.4f, %.4f\n", b.Pipeline.DefaultLat, b.Pipeline.DefaultLon)

	audioCount := 0
	var totalDuration float64
	for _, f := range b.Files {
		if f.Type == "audio" {
			audioCount++
			totalDuration += f.Meta.DurationSeconds
		}
	}
	fmt.Fprintf(&sb, "- Audio files: %d (%.1f hours total)\n\n", audioCount, totalDuration/3600)

	// Weather (same as normal)
	if raw, ok := results.Get("weather", "session"); ok {
		if wr, ok := raw.(WeatherResult); ok {
			sb.WriteString("## Weather\n")
			fmt.Fprintf(&sb, "- Sunrise: %s\n- Sunset: %s\n", wr.Sunrise, wr.Sunset)
			if len(wr.Hourly) > 0 {
				var minT, maxT float64 = 100, -100
				for _, h := range wr.Hourly {
					if h.Temperature < minT {
						minT = h.Temperature
					}
					if h.Temperature > maxT {
						maxT = h.Temperature
					}
				}
				fmt.Fprintf(&sb, "- Temperature: %.1f–%.1f°C\n", minT, maxT)
			}
			sb.WriteString("\n")
		}
	}

	// BirdNET with top-N limit
	if raw, ok := results.Get("birdnet", "session"); ok {
		if sr, ok := raw.(SessionBirdNetResult); ok {
			writeBirdNETSection(&sb, sr, topSpeciesLimit)

			// Hourly activity bucketing
			hourlyActivity := buildHourlyBuckets(results)
			if len(hourlyActivity) > 0 {
				sb.WriteString("## Hourly Activity\n")
				sb.WriteString("| Hour | Detections | Species |\n")
				sb.WriteString("|------|-----------|--------|\n")

				hours := make([]int, 0, len(hourlyActivity))
				for h := range hourlyActivity {
					hours = append(hours, h)
				}
				sort.Ints(hours)

				for _, h := range hours {
					bucket := hourlyActivity[h]
					fmt.Fprintf(&sb, "| %02d:00 | %d | %d |\n", h, bucket.detections, len(bucket.species))
				}
				sb.WriteString("\n")
			}
		}
	}

	return sb.String()
}

type hourlyBucket struct {
	detections int
	species    map[string]bool
}

// buildHourlyBuckets aggregates detections into hourly bins based on
// the F3 filename convention (HHMMSS_NNNN.WAV) and detection offset.
func buildHourlyBuckets(results *pipeline.Results) map[int]*hourlyBucket {
	buckets := make(map[int]*hourlyBucket)

	allBirdnet := results.AllForStage("birdnet")
	for key, val := range allBirdnet {
		if key == "session" {
			continue
		}
		fr, ok := val.(BirdNetFileResult)
		if !ok {
			continue
		}

		// Try to parse hour from F3 filename
		baseHour := parseF3Hour(filepath.Base(fr.FilePath))
		if baseHour < 0 {
			continue
		}

		for _, d := range fr.Detections {
			// Approximate the detection hour: file start hour + offset seconds
			detHour := min(baseHour+int(d.StartS/3600), 23)

			if buckets[detHour] == nil {
				buckets[detHour] = &hourlyBucket{species: make(map[string]bool)}
			}
			buckets[detHour].detections++
			buckets[detHour].species[d.ScientificName] = true
		}
	}

	return buckets
}

// parseF3Hour extracts the hour from an F3 filename like "221053_0001.WAV".
// Returns -1 if the filename doesn't match the expected format.
func parseF3Hour(filename string) int {
	// F3 format: HHMMSS_NNNN.WAV
	if len(filename) < 6 {
		return -1
	}
	hh, err := strconv.Atoi(filename[:2])
	if err != nil || hh < 0 || hh > 23 {
		return -1
	}
	// Verify the rest looks like F3 format
	if len(filename) >= 7 && filename[6] != '_' {
		return -1
	}
	return hh
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
