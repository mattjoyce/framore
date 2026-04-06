package stages

import (
	"time"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/pipeline"
)

// ResolveGPS computes the centroid of all EXIF GPS coordinates,
// falling back to batch pipeline defaults if no photos have GPS.
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
