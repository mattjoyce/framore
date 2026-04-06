package stages

import (
	"time"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/pipeline"
)

// ResolveGPS returns the best GPS coordinates from EXIF results,
// falling back to the batch pipeline defaults.
func ResolveGPS(b *batch.Batch, results *pipeline.Results) (lat, lon float64) {
	lat = b.Pipeline.DefaultLat
	lon = b.Pipeline.DefaultLon

	raw, ok := results.Get("exif", "session")
	if !ok {
		return lat, lon
	}
	photos, ok := raw.([]PhotoGPS)
	if !ok {
		return lat, lon
	}
	for _, p := range photos {
		if p.Lat != 0 || p.Lon != 0 {
			return p.Lat, p.Lon
		}
	}
	return lat, lon
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
