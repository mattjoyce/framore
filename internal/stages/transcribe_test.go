package stages

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/config"
	"github.com/mattjoyce/framore/internal/pipeline"
)

func TestTranscribeRun(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transcribe" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Verify payload fields
		if _, ok := req["wav_path"]; !ok {
			t.Error("missing wav_path in payload")
		}
		if _, ok := req["duration_seconds"]; !ok {
			t.Error("missing duration_seconds in payload")
		}

		resp := map[string]any{
			"transcripts": []map[string]any{
				{
					"segment":              "start",
					"text":                 "Walking along the creek trail",
					"language":             "en",
					"language_probability": 0.98,
				},
				{
					"segment":              "end",
					"text":                 "",
					"language":             "",
					"language_probability": 0.0,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Services.WhisperURL = srv.URL

	b := &batch.Batch{
		Stages: batch.StageConfig{Transcribe: true},
		Transcribe: batch.TranscribeConfig{
			DurationSeconds: 30,
		},
		Files: []batch.FileEntry{
			{
				Path: "/Volumes/field_Recording/F3/Orig/test/221053_0001.WAV",
				Type: "audio",
			},
		},
	}

	stage := &Transcribe{Cfg: cfg}

	if stage.Name() != "transcribe" {
		t.Errorf("Name() = %q, want %q", stage.Name(), "transcribe")
	}

	if !stage.Enabled(b) {
		t.Error("Enabled() = false, want true")
	}

	results := pipeline.NewResults()
	err := stage.Run(context.Background(), b, results)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	raw, ok := results.Get("transcribe", "/Volumes/field_Recording/F3/Orig/test/221053_0001.WAV")
	if !ok {
		t.Fatal("no result stored for test file")
	}

	fr, ok := raw.(TranscribeFileResult)
	if !ok {
		t.Fatal("result is not TranscribeFileResult")
	}

	if len(fr.Transcripts) != 2 {
		t.Fatalf("got %d transcripts, want 2", len(fr.Transcripts))
	}

	if fr.Transcripts[0].Text != "Walking along the creek trail" {
		t.Errorf("transcript[0].Text = %q, want %q", fr.Transcripts[0].Text, "Walking along the creek trail")
	}

	if fr.Transcripts[0].Language != "en" {
		t.Errorf("transcript[0].Language = %q, want %q", fr.Transcripts[0].Language, "en")
	}
}

func TestTranscribeDisabled(t *testing.T) {
	b := &batch.Batch{
		Stages: batch.StageConfig{Transcribe: false},
	}
	stage := &Transcribe{Cfg: config.DefaultConfig()}
	if stage.Enabled(b) {
		t.Error("Enabled() = true for disabled stage")
	}
}

func TestTranscribePayloadHasNoLanguage(t *testing.T) {
	// The whisper container does not read `language` from the request body;
	// model language is fixed at container startup via WHISPER_MODEL.
	// Ensure we don't send a ghost field.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
			return
		}
		if _, ok := req["language"]; ok {
			t.Error("payload should not include 'language' field — server ignores it")
		}

		resp := map[string]any{
			"transcripts": []map[string]any{
				{"segment": "start", "text": "test", "language": "en", "language_probability": 0.95},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode response: %v", err)
		}
	}))
	defer srv.Close()

	cfg := config.DefaultConfig()
	cfg.Services.WhisperURL = srv.URL

	b := &batch.Batch{
		Stages:     batch.StageConfig{Transcribe: true},
		Transcribe: batch.TranscribeConfig{DurationSeconds: 30},
		Files: []batch.FileEntry{
			{Path: "/Volumes/field_Recording/F3/Orig/test/file.WAV", Type: "audio"},
		},
	}

	results := pipeline.NewResults()
	if err := (&Transcribe{Cfg: cfg}).Run(context.Background(), b, results); err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}
