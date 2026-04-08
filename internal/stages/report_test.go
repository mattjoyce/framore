package stages

import (
	"strings"
	"testing"

	"github.com/mattjoyce/framore/internal/pipeline"
)

func TestIsLargeSession(t *testing.T) {
	results := pipeline.NewResults()
	if isLargeSession(results) {
		t.Error("empty results should not be large session")
	}

	results.Set("birdnet", "session", SessionBirdNetResult{TotalDetections: 100})
	if isLargeSession(results) {
		t.Error("100 detections should not be large session")
	}

	results.Set("birdnet", "session", SessionBirdNetResult{TotalDetections: 5000})
	if !isLargeSession(results) {
		t.Error("5000 detections should be large session")
	}
}

func TestWriteBirdNETSectionTopN(t *testing.T) {
	species := make([]SpeciesSummary, 25)
	for i := range species {
		species[i] = SpeciesSummary{
			CommonName:      "Species" + strings.Repeat("X", i),
			ScientificName:  "Sci" + strings.Repeat("X", i),
			TotalDetections: 100 - i,
			FileCount:       5,
			MaxConfidence:   0.9,
			FirstSeenS:      0,
			LastSeenS:       30,
		}
	}

	sr := SessionBirdNetResult{
		Species:         species,
		TotalDetections: 2500,
		TotalFiles:      50,
	}

	// No limit — all species
	var sb strings.Builder
	writeBirdNETSection(&sb, sr, 0)
	out := sb.String()
	if strings.Contains(out, "more species") {
		t.Error("no-limit output should not mention 'more species'")
	}

	// With limit of 20
	sb.Reset()
	writeBirdNETSection(&sb, sr, 20)
	out = sb.String()
	if !strings.Contains(out, "5 more species") {
		t.Errorf("limited output should mention '5 more species', got:\n%s", out)
	}
}

func TestParseF3Hour(t *testing.T) {
	tests := []struct {
		name string
		want int
	}{
		{"221053_0001.WAV", 22},
		{"070000_0002.WAV", 7},
		{"000000_0001.WAV", 0},
		{"235959_0001.WAV", 23},
		{"IMG_4521.jpg", -1},
		{"short", -1},
		{"99ABCD_0001.WAV", -1},
	}

	for _, tt := range tests {
		got := parseF3Hour(tt.name)
		if got != tt.want {
			t.Errorf("parseF3Hour(%q) = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestBuildHourlyBuckets(t *testing.T) {
	results := pipeline.NewResults()

	// Simulate two files at different hours with detections
	results.Set("birdnet", "/test/070000_0001.WAV", BirdNetFileResult{
		FilePath: "/test/070000_0001.WAV",
		Detections: []Detection{
			{StartS: 10, EndS: 13, ScientificName: "Corvus coronoides", CommonName: "Australian Raven", Confidence: 0.8},
			{StartS: 120, EndS: 123, ScientificName: "Gymnorhina tibicen", CommonName: "Australian Magpie", Confidence: 0.9},
		},
	})
	results.Set("birdnet", "/test/180000_0002.WAV", BirdNetFileResult{
		FilePath: "/test/180000_0002.WAV",
		Detections: []Detection{
			{StartS: 5, EndS: 8, ScientificName: "Corvus coronoides", CommonName: "Australian Raven", Confidence: 0.7},
		},
	})

	buckets := buildHourlyBuckets(results)

	if buckets[7] == nil {
		t.Fatal("expected bucket for hour 7")
	}
	if buckets[7].detections != 2 {
		t.Errorf("hour 7: got %d detections, want 2", buckets[7].detections)
	}
	if len(buckets[7].species) != 2 {
		t.Errorf("hour 7: got %d species, want 2", len(buckets[7].species))
	}

	if buckets[18] == nil {
		t.Fatal("expected bucket for hour 18")
	}
	if buckets[18].detections != 1 {
		t.Errorf("hour 18: got %d detections, want 1", buckets[18].detections)
	}
}
