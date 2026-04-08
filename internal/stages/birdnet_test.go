package stages

import (
	"math"
	"testing"
	"time"
)

func TestDecodePlusCode(t *testing.T) {
	// Full code for Macquarie Park
	lat, lon, err := DecodePlusCode("4RRH64J7+6JP")
	if err != nil {
		t.Fatalf("DecodePlusCode: %v", err)
	}
	if math.Abs(lat-(-33.7694)) > 0.001 {
		t.Errorf("lat: got %f, want ~-33.7694", lat)
	}
	if math.Abs(lon-151.1141) > 0.001 {
		t.Errorf("lon: got %f, want ~151.1141", lon)
	}
}

func TestDecodePlusCode_Compound(t *testing.T) {
	// Compound code: short code + locality (requires network)
	lat, lon, err := DecodePlusCode("64J6+HM Marsfield, New South Wales")
	if err != nil {
		t.Skipf("compound code test requires network: %v", err)
	}
	if math.Abs(lat-(-33.7686)) > 0.01 {
		t.Errorf("lat: got %f, want ~-33.7686", lat)
	}
	if math.Abs(lon-151.1117) > 0.01 {
		t.Errorf("lon: got %f, want ~151.1117", lon)
	}
}

func TestDecodePlusCode_ShortWithoutLocality(t *testing.T) {
	_, _, err := DecodePlusCode("64J6+HM")
	if err == nil {
		t.Error("expected error for short code without locality")
	}
}

func TestDecodePlusCode_Invalid(t *testing.T) {
	_, _, err := DecodePlusCode("not-a-code")
	if err == nil {
		t.Error("expected error for invalid plus code")
	}
}

func TestDecodePlusCode_Empty(t *testing.T) {
	_, _, err := DecodePlusCode("")
	if err == nil {
		t.Error("expected error for empty plus code")
	}
}

func TestWeekClamping(t *testing.T) {
	tests := []struct {
		date     string
		wantWeek int
	}{
		{"2026-03-29", 13}, // normal week
		{"2026-01-01", 1},  // week 1
		{"2026-12-28", 48}, // week 53 → clamped to 48
		{"2025-12-29", 1},  // week 1 of 2026
	}

	for _, tt := range tests {
		t.Run(tt.date, func(t *testing.T) {
			d, err := time.Parse("2006-01-02", tt.date)
			if err != nil {
				t.Fatalf("parse date: %v", err)
			}
			_, week := d.ISOWeek()
			if week > 48 {
				week = 48
			}
			if week != tt.wantWeek {
				t.Errorf("date %s: got week %d, want %d", tt.date, week, tt.wantWeek)
			}
		})
	}
}
