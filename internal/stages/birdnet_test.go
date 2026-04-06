package stages

import (
	"testing"
	"time"
)

func TestWeekClamping(t *testing.T) {
	tests := []struct {
		date     string
		wantWeek int
	}{
		{"2026-03-29", 13},  // normal week
		{"2026-01-01", 1},   // week 1
		{"2026-12-28", 48},  // week 53 → clamped to 48
		{"2025-12-29", 1},   // week 1 of 2026
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
