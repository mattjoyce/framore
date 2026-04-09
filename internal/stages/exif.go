package stages

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/rwcarlsen/goexif/exif"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/pipeline"
)

type PhotoGPS struct {
	Path     string
	Time     time.Time
	Lat, Lon float64
	Altitude float64
}

type EXIF struct{}

func (e *EXIF) Name() string { return "exif" }

func (e *EXIF) Enabled(b *batch.Batch) bool { return b.Stages.EXIF }

func (e *EXIF) SupportsNoWait() bool { return true }

func (e *EXIF) Run(ctx context.Context, b *batch.Batch, results *pipeline.Results) error {
	var photos []PhotoGPS

	for _, f := range b.Files {
		if f.Type != "image" {
			continue
		}

		file, err := os.Open(f.Path)
		if err != nil {
			fmt.Printf("  [exif] skip %s: %v\n", f.Path, err)
			continue
		}

		x, err := exif.Decode(file)
		_ = file.Close()
		if err != nil {
			fmt.Printf("  [exif] no EXIF in %s: %v\n", f.Path, err)
			continue
		}

		var pg PhotoGPS
		pg.Path = f.Path

		if lat, lon, err := x.LatLong(); err == nil {
			pg.Lat = lat
			pg.Lon = lon
		}

		if dt, err := x.DateTime(); err == nil {
			pg.Time = dt
		}

		if alt, err := x.Get(exif.GPSAltitude); err == nil {
			if num, den, err := alt.Rat2(0); err == nil && den != 0 {
				pg.Altitude = float64(num) / float64(den)
			}
		}

		photos = append(photos, pg)
	}

	// Sort by time for nearest-timestamp lookup
	sort.Slice(photos, func(i, j int) bool {
		return photos[i].Time.Before(photos[j].Time)
	})

	results.Set("exif", "session", photos)
	fmt.Printf("  [exif] extracted GPS from %d images\n", len(photos))
	return nil
}
