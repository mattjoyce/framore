package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/stages"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Interactive wizard to configure the active batch",
	RunE: func(cmd *cobra.Command, args []string) error {
		if cfg.CurrentBatch == "" {
			return fmt.Errorf("no active batch — run 'framore new' or 'framore use' first")
		}

		b, err := batch.Load(cfg.CurrentBatch)
		if err != nil {
			return fmt.Errorf("load batch: %w", err)
		}

		// Pre-fill session_dir: existing value → first audio file's dir → cwd
		sessionDir := b.SessionDir
		if sessionDir == "" {
			for _, f := range b.Files {
				if f.Type == "audio" {
					sessionDir = filepath.Dir(f.Path)
					break
				}
			}
		}
		if sessionDir == "" {
			sessionDir, _ = os.Getwd()
		}
		sessionDate := b.SessionDate
		plusCode := b.Pipeline.PlusCode
		defaultLatStr := formatCoord(b.Pipeline.DefaultLat, cfg.Defaults.DefaultLat)
		defaultLonStr := formatCoord(b.Pipeline.DefaultLon, cfg.Defaults.DefaultLon)
		enableWeather := b.Stages.Weather
		timezone := b.Weather.Timezone
		if timezone == "" {
			timezone = cfg.Defaults.Timezone
		}
		enableBirdNet := b.Stages.BirdNet
		skipExisting := b.BirdNet.SkipExisting
		minConfStr := fmt.Sprintf("%.1f", b.BirdNet.MinConf)
		if b.BirdNet.MinConf == 0 {
			minConfStr = fmt.Sprintf("%.1f", cfg.Defaults.BirdnetMinConf)
		}
		enableReport := b.Stages.Report

		// ── Session group ──
		sessionGroup := huh.NewGroup(
			huh.NewNote().
				Title("── Session ──────────────────────────────"),
			huh.NewInput().
				Title("Session directory").
				Description("Path to the recording session folder").
				Value(&sessionDir),
			huh.NewInput().
				Title("Recording date (YYYY-MM-DD or YYYYMMDD)").
				Description("Sets BirdNET week; not inferred from folder name").
				Value(&sessionDate).
				Validate(func(s string) error {
					if s == "" {
						return fmt.Errorf("session date is required")
					}
					_, err := parseDate(s)
					return err
				}),
		)

		// Count images in batch
		imageCount := 0
		for _, f := range b.Files {
			if f.Type == "image" {
				imageCount++
			}
		}

		var gpsDesc string
		if imageCount == 0 {
			gpsDesc = "No images in batch — use a plus code or enter GPS manually"
		} else {
			gpsDesc = fmt.Sprintf("%d images in batch — EXIF GPS will be used if available", imageCount)
		}

		// ── GPS / EXIF group ──
		exifGroup := huh.NewGroup(
			huh.NewNote().
				Title("── GPS / EXIF ───────────────────────────").
				Description(gpsDesc),
			huh.NewInput().
				Title("Plus code").
				Description("Full (4RRH64J6+HM) or compound (64J6+HM Marsfield, NSW) from Google Maps").
				Value(&plusCode).
				Validate(func(s string) error {
					if s == "" {
						return nil
					}
					_, _, err := stages.DecodePlusCode(s)
					return err
				}),
			huh.NewInput().
				Title("Default latitude").
				Value(&defaultLatStr).
				Validate(validateFloat),
			huh.NewInput().
				Title("Default longitude").
				Value(&defaultLonStr).
				Validate(validateFloat),
		)

		// ── Weather group ──
		weatherGroup := huh.NewGroup(
			huh.NewNote().
				Title("── Weather ──────────────────────────────"),
			huh.NewConfirm().
				Title("Enable weather lookup?").
				Value(&enableWeather),
			huh.NewInput().
				Title("Timezone").
				Value(&timezone),
		)

		// ── BirdNET group ──
		birdnetGroup := huh.NewGroup(
			huh.NewNote().
				Title("── BirdNET ──────────────────────────────"),
			huh.NewConfirm().
				Title("Enable BirdNET detection?").
				Value(&enableBirdNet),
			huh.NewConfirm().
				Title("Skip files with existing BirdNET output?").
				Description("Skips WAVs that already have a .BirdNET.selection.table.txt file").
				Value(&skipExisting),
			huh.NewInput().
				Title("Min confidence (0.0–1.0)").
				Value(&minConfStr).
				Validate(func(s string) error {
					v, err := strconv.ParseFloat(s, 64)
					if err != nil {
						return fmt.Errorf("must be a number")
					}
					if v < 0 || v > 1 {
						return fmt.Errorf("must be between 0.0 and 1.0")
					}
					return nil
				}),
		)

		enableTranscribe := b.Stages.Transcribe
		transcribeDurStr := fmt.Sprintf("%d", b.Transcribe.DurationSeconds)
		if b.Transcribe.DurationSeconds == 0 {
			transcribeDurStr = "60"
		}

		// ── Transcription group ──
		transcribeGroup := huh.NewGroup(
			huh.NewNote().
				Title("── Transcription ────────────────────────").
				Description(fmt.Sprintf("Uses faster-whisper at %s", cfg.Services.WhisperURL)),
			huh.NewConfirm().
				Title("Enable transcription?").
				Value(&enableTranscribe),
			huh.NewInput().
				Title("Duration (seconds from start/end of each file)").
				Value(&transcribeDurStr).
				Validate(func(s string) error {
					v, err := strconv.Atoi(s)
					if err != nil {
						return fmt.Errorf("must be an integer")
					}
					if v < 1 {
						return fmt.Errorf("must be at least 1")
					}
					return nil
				}),
		)

		// ── Report group ──
		reportGroup := huh.NewGroup(
			huh.NewNote().
				Title("── Report ───────────────────────────────").
				Description(fmt.Sprintf("Uses Ollama at %s", cfg.Services.OllamaURL)),
			huh.NewConfirm().
				Title("Enable report generation?").
				Value(&enableReport),
		)

		// ── Confirm ──
		var doSave bool
		confirmGroup := huh.NewGroup(
			huh.NewConfirm().
				Title(fmt.Sprintf("Save changes to %s?", filepath.Base(cfg.CurrentBatch))).
				Value(&doSave),
		)

		form := huh.NewForm(
			sessionGroup,
			exifGroup,
			weatherGroup,
			birdnetGroup,
			transcribeGroup,
			reportGroup,
			confirmGroup,
		)

		if err := form.Run(); err != nil {
			return fmt.Errorf("config wizard: %w", err)
		}

		if !doSave {
			fmt.Println("Cancelled — no changes saved.")
			return nil
		}

		// Normalize date to YYYY-MM-DD
		if t, err := parseDate(sessionDate); err == nil {
			sessionDate = t.Format("2006-01-02")
		}

		// Apply values back to batch
		b.SessionDir = sessionDir
		b.SessionDate = sessionDate
		b.Pipeline.PlusCode = plusCode

		// If plus code was entered, update lat/lon from it
		if plusCode != "" {
			if plat, plon, err := stages.DecodePlusCode(plusCode); err == nil {
				defaultLatStr = fmt.Sprintf("%.4f", plat)
				defaultLonStr = fmt.Sprintf("%.4f", plon)
			}
		}
		b.Pipeline.DefaultLat, _ = strconv.ParseFloat(defaultLatStr, 64)
		b.Pipeline.DefaultLon, _ = strconv.ParseFloat(defaultLonStr, 64)
		b.Stages.Weather = enableWeather
		b.Weather.Timezone = timezone
		b.Stages.BirdNet = enableBirdNet
		b.BirdNet.MinConf, _ = strconv.ParseFloat(minConfStr, 64)
		b.BirdNet.SkipExisting = skipExisting
		b.Stages.Transcribe = enableTranscribe
		b.Transcribe.DurationSeconds, _ = strconv.Atoi(transcribeDurStr)
		b.Stages.Report = enableReport
		b.Stages.EXIF = imageCount > 0

		if err := batch.Save(cfg.CurrentBatch, b); err != nil {
			return fmt.Errorf("save batch: %w", err)
		}

		// Show BirdNET week if date is set
		weekInfo := ""
		if b.SessionDate != "" {
			if bw, err := stages.BirdNETWeek(b.SessionDate); err == nil {
				if t, err := time.Parse("2006-01-02", b.SessionDate); err == nil {
					_, w := t.ISOWeek()
					weekInfo = fmt.Sprintf("  (ISO week %d → BirdNET week %d)", w, bw)
				}
			}
		}

		fmt.Printf("\nSaved to %s\n", cfg.CurrentBatch)
		fmt.Printf("  session_date: %s%s\n", b.SessionDate, weekInfo)
		fmt.Printf("  session_dir:  %s\n", b.SessionDir)
		if b.Pipeline.PlusCode != "" {
			fmt.Printf("  plus_code:    %s → %.4f, %.4f\n", b.Pipeline.PlusCode, b.Pipeline.DefaultLat, b.Pipeline.DefaultLon)
		} else {
			fmt.Printf("  gps:          %.4f, %.4f\n", b.Pipeline.DefaultLat, b.Pipeline.DefaultLon)
		}
		fmt.Printf("  stages:       exif=%t weather=%t birdnet=%t report=%t\n",
			b.Stages.EXIF, b.Stages.Weather, b.Stages.BirdNet, b.Stages.Report)

		return nil
	},
}

func parseDate(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("20060102", s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("must be YYYY-MM-DD or YYYYMMDD format")
}

func formatCoord(batchVal, defaultVal float64) string {
	if batchVal != 0 {
		return fmt.Sprintf("%.4f", batchVal)
	}
	return fmt.Sprintf("%.4f", defaultVal)
}

func validateFloat(s string) error {
	if _, err := strconv.ParseFloat(s, 64); err != nil {
		return fmt.Errorf("must be a number")
	}
	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
}
