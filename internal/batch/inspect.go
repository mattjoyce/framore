package batch

import (
	"fmt"
	"math"
	"os"

	"github.com/go-audio/wav"
	"github.com/rwcarlsen/goexif/exif"
)

// InspectWAV reads WAV file metadata and returns a FileMeta with duration,
// bit depth, sample rate, channels, and file size populated.
func InspectWAV(path string) (FileMeta, error) {
	f, err := os.Open(path) // #nosec G304 — path comes from user's batch file
	if err != nil {
		return FileMeta{}, err
	}
	defer func() { _ = f.Close() }()

	dec := wav.NewDecoder(f)
	if !dec.IsValidFile() {
		return FileMeta{}, fmt.Errorf("invalid WAV file: %s", path)
	}

	dur, err := dec.Duration()
	if err != nil {
		return FileMeta{}, err
	}

	info, err := os.Stat(path)
	if err != nil {
		return FileMeta{}, err
	}

	meta := FileMeta{
		DurationSeconds: dur.Seconds(),
		BitDepth:        int(dec.BitDepth),
		SampleRate:      int(dec.SampleRate),
		Channels:        int(dec.NumChans),
		SizeBytes:       info.Size(),
	}

	return meta, nil
}

// InspectImage reads EXIF metadata from an image file and returns a FileMeta
// with datetime, GPS coordinates, altitude, and device name populated.
// Missing EXIF fields are silently ignored (zero values used).
func InspectImage(path string) (FileMeta, error) {
	f, err := os.Open(path) // #nosec G304 — path comes from user's batch file
	if err != nil {
		return FileMeta{}, err
	}
	defer func() { _ = f.Close() }()

	x, err := exif.Decode(f)
	if err != nil {
		// No EXIF at all — return empty meta with just file size
		info, statErr := os.Stat(path)
		if statErr != nil {
			return FileMeta{}, statErr
		}
		return FileMeta{SizeBytes: info.Size()}, nil
	}

	var meta FileMeta

	// DateTime
	dt, err := x.DateTime()
	if err == nil {
		meta.Datetime = dt.Format("2006-01-02T15:04:05")
	}

	// GPS
	lat, lon, err := x.LatLong()
	if err == nil {
		meta.Lat = lat
		meta.Lon = lon
	}

	// Altitude
	altTag, err := x.Get(exif.GPSAltitude)
	if err == nil {
		numer, denom, ratErr := altTag.Rat2(0)
		if ratErr == nil && denom != 0 {
			meta.AltitudeM = float64(numer) / float64(denom)
			meta.AltitudeM = math.Round(meta.AltitudeM*100) / 100
		}
	}

	// Device model
	modelTag, err := x.Get(exif.Model)
	if err == nil {
		meta.Device, _ = modelTag.StringVal()
	}

	// File size
	info, err := os.Stat(path)
	if err == nil {
		meta.SizeBytes = info.Size()
	}

	return meta, nil
}
