package batch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	original := &Batch{
		SessionDir:  "/Volumes/field_Recording/F3/Orig/260329-Test",
		SessionDate: "2026-03-29",
		Stages: StageConfig{
			EXIF:    true,
			Weather: true,
			BirdNet: true,
			Report:  true,
		},
		BirdNet: BirdNetConfig{MinConf: 0.6},
		Weather: WeatherStageConfig{Timezone: "Australia/Sydney"},
		Pipeline: PipelineConfig{
			DefaultLat: -34.0021,
			DefaultLon: 150.4987,
		},
		Files: []FileEntry{
			{
				Path:  "/Volumes/field_Recording/F3/Orig/260329-Test/221053_0001.WAV",
				Type:  "audio",
				Added: "2026-04-06T21:00:00Z",
				Meta: FileMeta{
					DurationSeconds: 182.4,
					BitDepth:        24,
					SampleRate:      48000,
					Channels:        2,
					SizeBytes:       52428800,
				},
			},
		},
	}

	if err := Save(path, original); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.SessionDir != original.SessionDir {
		t.Errorf("SessionDir: got %q, want %q", loaded.SessionDir, original.SessionDir)
	}
	if loaded.SessionDate != original.SessionDate {
		t.Errorf("SessionDate: got %q, want %q", loaded.SessionDate, original.SessionDate)
	}
	if len(loaded.Files) != 1 {
		t.Fatalf("Files: got %d, want 1", len(loaded.Files))
	}
	if loaded.Files[0].Meta.DurationSeconds != 182.4 {
		t.Errorf("DurationSeconds: got %f, want 182.4", loaded.Files[0].Meta.DurationSeconds)
	}
	if loaded.Files[0].Meta.BitDepth != 24 {
		t.Errorf("BitDepth: got %d, want 24", loaded.Files[0].Meta.BitDepth)
	}
}

func TestHasFile(t *testing.T) {
	b := &Batch{
		Files: []FileEntry{
			{Path: "/a/b/c.wav"},
		},
	}
	if !HasFile(b, "/a/b/c.wav") {
		t.Error("expected HasFile to return true for existing path")
	}
	if HasFile(b, "/a/b/d.wav") {
		t.Error("expected HasFile to return false for missing path")
	}
}

func TestInspectWAV(t *testing.T) {
	// Use the committed test fixture
	fixture := filepath.Join("..", "..", "testdata", "mono_24bit_48k.wav")
	if _, err := os.Stat(fixture); os.IsNotExist(err) {
		t.Skip("test fixture not found:", fixture)
	}

	meta, err := InspectWAV(fixture)
	if err != nil {
		t.Fatalf("InspectWAV: %v", err)
	}

	if meta.SampleRate != 48000 {
		t.Errorf("SampleRate: got %d, want 48000", meta.SampleRate)
	}
	if meta.BitDepth != 24 {
		t.Errorf("BitDepth: got %d, want 24", meta.BitDepth)
	}
	if meta.Channels != 1 {
		t.Errorf("Channels: got %d, want 1", meta.Channels)
	}
	if meta.DurationSeconds <= 0 {
		t.Errorf("DurationSeconds: got %f, want > 0", meta.DurationSeconds)
	}
	if meta.SizeBytes <= 0 {
		t.Errorf("SizeBytes: got %d, want > 0", meta.SizeBytes)
	}
}
