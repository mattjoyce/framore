package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/config"
	"github.com/mattjoyce/framore/internal/pipeline"
)

type HourlyWeather struct {
	Time          string  `json:"time"`
	Temperature   float64 `json:"temperature_2m"`
	Humidity      float64 `json:"relative_humidity_2m"`
	Precipitation float64 `json:"precipitation"`
	WindSpeed     float64 `json:"wind_speed_10m"`
	WeatherCode   int     `json:"weather_code"`
	CloudCover    float64 `json:"cloud_cover"`
	PressureMSL   float64 `json:"pressure_msl"`
}

type WeatherResult struct {
	Date    string
	Hourly  []HourlyWeather
	Sunrise string
	Sunset  string
}

type Weather struct {
	Cfg *config.Config
}

func (w *Weather) Name() string { return "weather" }

func (w *Weather) Enabled(b *batch.Batch) bool { return b.Stages.Weather }

func (w *Weather) Run(ctx context.Context, b *batch.Batch, results *pipeline.Results) error {
	lat, lon := ResolveGPS(b, results)

	date := b.SessionDate
	tz := b.Weather.Timezone
	if tz == "" {
		tz = "Australia/Sydney"
	}

	// Check cache
	cacheDir := w.Cfg.Weather.CacheDir
	if strings.HasPrefix(cacheDir, "~") {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, cacheDir[1:])
	}

	cacheFile := filepath.Join(cacheDir, fmt.Sprintf("%.4f_%.4f_%s.json", lat, lon, date))

	if data, err := os.ReadFile(cacheFile); err == nil { // #nosec G304 — cache path from config
		info, _ := os.Stat(cacheFile)
		maxAge := time.Duration(w.Cfg.Weather.CacheMaxAgeDays) * 24 * time.Hour
		if time.Since(info.ModTime()) < maxAge {
			fmt.Printf("  [weather] using cached data for %s\n", date)
			var result WeatherResult
			if err := json.Unmarshal(data, &result); err == nil {
				results.Set("weather", "session", result)
				return nil
			}
		}
	}

	// Fetch from open-meteo
	url := fmt.Sprintf(
		"https://archive-api.open-meteo.com/v1/archive?latitude=%.4f&longitude=%.4f&start_date=%s&end_date=%s&hourly=temperature_2m,relative_humidity_2m,precipitation,wind_speed_10m,weather_code,cloud_cover,pressure_msl&daily=sunrise,sunset&timezone=%s",
		lat, lon, date, date, tz,
	)

	timeout := time.Duration(w.Cfg.Weather.TimeoutSeconds) * time.Second
	client := &http.Client{Timeout: timeout}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch weather: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("open-meteo returned %d: %s", resp.StatusCode, string(body))
	}

	// Parse the API response
	var apiResp struct {
		Hourly struct {
			Time          []string  `json:"time"`
			Temperature2m []float64 `json:"temperature_2m"`
			RelHumidity2m []float64 `json:"relative_humidity_2m"`
			Precipitation []float64 `json:"precipitation"`
			WindSpeed10m  []float64 `json:"wind_speed_10m"`
			WeatherCode   []int     `json:"weather_code"`
			CloudCover    []float64 `json:"cloud_cover"`
			PressureMSL   []float64 `json:"pressure_msl"`
		} `json:"hourly"`
		Daily struct {
			Sunrise []string `json:"sunrise"`
			Sunset  []string `json:"sunset"`
		} `json:"daily"`
	}

	if err := json.Unmarshal(body, &apiResp); err != nil {
		return fmt.Errorf("parse weather response: %w", err)
	}

	result := WeatherResult{
		Date: date,
	}

	if len(apiResp.Daily.Sunrise) > 0 {
		result.Sunrise = apiResp.Daily.Sunrise[0]
	}
	if len(apiResp.Daily.Sunset) > 0 {
		result.Sunset = apiResp.Daily.Sunset[0]
	}

	for i, t := range apiResp.Hourly.Time {
		h := HourlyWeather{Time: t}
		if i < len(apiResp.Hourly.Temperature2m) {
			h.Temperature = apiResp.Hourly.Temperature2m[i]
		}
		if i < len(apiResp.Hourly.RelHumidity2m) {
			h.Humidity = apiResp.Hourly.RelHumidity2m[i]
		}
		if i < len(apiResp.Hourly.Precipitation) {
			h.Precipitation = apiResp.Hourly.Precipitation[i]
		}
		if i < len(apiResp.Hourly.WindSpeed10m) {
			h.WindSpeed = apiResp.Hourly.WindSpeed10m[i]
		}
		if i < len(apiResp.Hourly.WeatherCode) {
			h.WeatherCode = apiResp.Hourly.WeatherCode[i]
		}
		if i < len(apiResp.Hourly.CloudCover) {
			h.CloudCover = apiResp.Hourly.CloudCover[i]
		}
		if i < len(apiResp.Hourly.PressureMSL) {
			h.PressureMSL = apiResp.Hourly.PressureMSL[i]
		}
		result.Hourly = append(result.Hourly, h)
	}

	// Cache the result
	if err := os.MkdirAll(cacheDir, 0o750); err == nil {
		cacheData, _ := json.MarshalIndent(result, "", "  ")
		_ = os.WriteFile(cacheFile, cacheData, 0o600)
	}

	results.Set("weather", "session", result)
	fmt.Printf("  [weather] fetched weather for %s at %.4f, %.4f\n", date, lat, lon)
	return nil
}
