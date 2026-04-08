package stages

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	olc "github.com/google/open-location-code/go"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/pipeline"
)

// ResolveGPS returns coordinates using the priority:
//  1. EXIF centroid from photos
//  2. Plus code from batch pipeline config
//  3. Batch pipeline default lat/lon
func ResolveGPS(b *batch.Batch, results *pipeline.Results) (lat, lon float64) {
	lat = b.Pipeline.DefaultLat
	lon = b.Pipeline.DefaultLon

	// Try plus code (tier 2)
	if b.Pipeline.PlusCode != "" {
		if plat, plon, err := DecodePlusCode(b.Pipeline.PlusCode); err == nil {
			lat = plat
			lon = plon
		}
	}

	// Try EXIF centroid (tier 1 — overrides plus code)
	raw, ok := results.Get("exif", "session")
	if !ok {
		return lat, lon
	}
	photos, ok := raw.([]PhotoGPS)
	if !ok {
		return lat, lon
	}

	var sumLat, sumLon float64
	var count int
	for _, p := range photos {
		if p.Lat != 0 || p.Lon != 0 {
			sumLat += p.Lat
			sumLon += p.Lon
			count++
		}
	}
	if count == 0 {
		return lat, lon
	}
	return sumLat / float64(count), sumLon / float64(count)
}

// DecodePlusCode decodes a plus code to lat/lon. Accepts:
//   - Full codes: "4RRH64J6+HM"
//   - Compound codes: "64J6+HM Marsfield, New South Wales"
//     (short code + locality, geocoded via Nominatim)
func DecodePlusCode(code string) (lat, lon float64, err error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return 0, 0, fmt.Errorf("empty plus code")
	}

	// Check if it's a full code first
	if olc.CheckFull(code) == nil {
		return decodeFull(code)
	}

	// Try parsing as compound code: "SHORTCODE LOCALITY"
	// The short code ends after the + and subsequent characters
	plusIdx := strings.Index(code, "+")
	if plusIdx < 0 {
		return 0, 0, fmt.Errorf("invalid plus code %q — no '+' found", code)
	}

	// Find end of the code part (characters after + until a space)
	afterPlus := code[plusIdx+1:]
	spaceIdx := strings.Index(afterPlus, " ")
	if spaceIdx < 0 {
		// No locality — it's a short code without context
		return 0, 0, fmt.Errorf("short plus code %q — add locality (e.g. '64J6+HM Marsfield, NSW') or use full code", code)
	}

	shortCode := code[:plusIdx+1+spaceIdx]
	locality := strings.TrimSpace(afterPlus[spaceIdx:])

	if olc.CheckShort(shortCode) != nil {
		return 0, 0, fmt.Errorf("invalid short plus code %q", shortCode)
	}

	// Geocode the locality
	refLat, refLon, err := geocodeNominatim(locality)
	if err != nil {
		return 0, 0, fmt.Errorf("geocode %q: %w", locality, err)
	}

	// Recover full code from short + reference
	fullCode, err := olc.RecoverNearest(shortCode, refLat, refLon)
	if err != nil {
		return 0, 0, fmt.Errorf("recover plus code %q near %q: %w", shortCode, locality, err)
	}

	return decodeFull(fullCode)
}

func decodeFull(code string) (lat, lon float64, err error) {
	area, err := olc.Decode(code)
	if err != nil {
		return 0, 0, fmt.Errorf("decode plus code %q: %w", code, err)
	}
	return area.LatLo + (area.LatHi-area.LatLo)/2, area.LngLo + (area.LngHi-area.LngLo)/2, nil
}

// geocodeNominatim looks up a locality string using the OpenStreetMap Nominatim API.
func geocodeNominatim(locality string) (lat, lon float64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	u := fmt.Sprintf("https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=1",
		url.QueryEscape(locality))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", "framore/0.1 (field recording batch tool)")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer func() { _ = resp.Body.Close() }()

	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, 0, fmt.Errorf("parse nominatim response: %w", err)
	}
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("no results found for %q", locality)
	}

	var rlat, rlon float64
	if _, err := fmt.Sscanf(results[0].Lat, "%f", &rlat); err != nil {
		return 0, 0, fmt.Errorf("parse lat: %w", err)
	}
	if _, err := fmt.Sscanf(results[0].Lon, "%f", &rlon); err != nil {
		return 0, 0, fmt.Errorf("parse lon: %w", err)
	}
	return rlat, rlon, nil
}

// BirdNETWeek computes the ISO week number from a date string (YYYY-MM-DD),
// clamped to the range 1–48 as required by BirdNET.
func BirdNETWeek(sessionDate string) (int, error) {
	t, err := time.Parse("2006-01-02", sessionDate)
	if err != nil {
		return 0, err
	}
	_, week := t.ISOWeek()
	if week > 48 {
		week = 48
	}
	return week, nil
}
