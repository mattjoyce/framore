package stages

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/mattjoyce/framore/internal/batch"
	"github.com/mattjoyce/framore/internal/config"
	"github.com/mattjoyce/framore/internal/pipeline"
)

func TestBirdNetSubmitAllThenPoll(t *testing.T) {
	// Track request ordering to verify submit-all-before-poll.
	var mu sync.Mutex
	var requestLog []string // "submit:<path>" or "poll:<jobID>"

	// Map file paths to job IDs, assigned on submit.
	jobResults := map[string]json.RawMessage{
		"job-001": mustJSON(t, map[string]any{
			"status":          "ok",
			"output_path":     "/out/file1.csv",
			"detections":      []any{map[string]any{"start_s": 0.0, "end_s": 3.0, "scientific_name": "Corvus coronoides", "common_name": "Australian Raven", "confidence": 0.85}},
			"detection_count": 1,
			"duration_s":      60.0,
			"realtime_factor":  4.5,
		}),
		"job-002": mustJSON(t, map[string]any{
			"status":          "ok",
			"output_path":     "/out/file2.csv",
			"detections":      []any{map[string]any{"start_s": 10.0, "end_s": 13.0, "scientific_name": "Gymnorhina tibicen", "common_name": "Australian Magpie", "confidence": 0.92}},
			"detection_count": 1,
			"duration_s":      60.0,
			"realtime_factor":  5.0,
		}),
		"job-003": mustJSON(t, map[string]any{
			"status":          "ok",
			"output_path":     "/out/file3.csv",
			"detections":      []any{},
			"detection_count": 0,
			"duration_s":      60.0,
			"realtime_factor":  6.0,
		}),
	}

	// Track which submit maps to which job ID.
	var submitMu sync.Mutex
	submitCount := 0
	jobIDs := []string{"job-001", "job-002", "job-003"}

	// Poll counts per job — first poll returns "running", second returns "succeeded".
	pollCounts := map[string]int{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/plugin/birda/handle") {
			// Submit endpoint
			body, _ := io.ReadAll(r.Body)
			var req map[string]any
			json.Unmarshal(body, &req)

			payload, _ := req["payload"].(map[string]any)
			wavPath, _ := payload["wav_path"].(string)

			submitMu.Lock()
			idx := submitCount
			submitCount++
			submitMu.Unlock()

			mu.Lock()
			requestLog = append(requestLog, "submit:"+wavPath)
			mu.Unlock()

			jobID := jobIDs[idx]
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]any{
				"job_id":  jobID,
				"status":  "queued",
				"plugin":  "birda",
				"command": "handle",
			})
			return
		}

		if jobID, ok := strings.CutPrefix(r.URL.Path, "/job/"); ok {
			// Poll endpoint
			mu.Lock()
			requestLog = append(requestLog, "poll:"+jobID)
			pollCounts[jobID]++
			count := pollCounts[jobID]
			mu.Unlock()

			status := "running"
			var result json.RawMessage
			if count >= 2 {
				status = "succeeded"
				result = jobResults[jobID]
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"job_id":  jobID,
				"status":  status,
				"plugin":  "birda",
				"command": "handle",
				"result":  json.RawMessage(result),
			})
			return
		}

		t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	// Set up config pointing to test server.
	cfg := &config.Config{
		Services: config.Services{
			DuctileAPIURL:   srv.URL,
			DuctileTokenEnv: "TEST_DUCTILE_TOKEN",
		},
		Defaults: config.Defaults{
			BirdnetMinConf: 0.5,
		},
		Paths: config.Paths{
			ProcessingRoot: "/mnt/user/field_Recording",
			AllowedPaths:   []string{"/Volumes/field_Recording"},
		},
	}

	os.Setenv("TEST_DUCTILE_TOKEN", "test-token-123")
	defer os.Unsetenv("TEST_DUCTILE_TOKEN")

	b := &batch.Batch{
		SessionDate: "2026-03-29",
		Stages:      batch.StageConfig{BirdNet: true},
		Pipeline: batch.PipelineConfig{
			DefaultLat: -34.0,
			DefaultLon: 150.5,
		},
		Files: []batch.FileEntry{
			{Path: "/Volumes/field_Recording/F3/Orig/file1.WAV", Type: "audio"},
			{Path: "/Volumes/field_Recording/F3/Orig/file2.WAV", Type: "audio"},
			{Path: "/Volumes/field_Recording/F3/Orig/file3.WAV", Type: "audio"},
			{Path: "/Volumes/field_Recording/F3/Orig/photo.JPG", Type: "photo"}, // should be skipped
		},
	}

	results := pipeline.NewResults()

	// Capture stdout for elapsed timer check.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	bn := &BirdNet{Cfg: cfg}
	err := bn.Run(context.Background(), b, results)

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	os.Stdout = oldStdout
	output := buf.String()

	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// --- Verify submit-all-before-poll ordering ---
	mu.Lock()
	log := make([]string, len(requestLog))
	copy(log, requestLog)
	mu.Unlock()

	lastSubmitIdx := -1
	firstPollIdx := -1
	for i, entry := range log {
		if strings.HasPrefix(entry, "submit:") {
			lastSubmitIdx = i
		}
		if strings.HasPrefix(entry, "poll:") && firstPollIdx == -1 {
			firstPollIdx = i
		}
	}

	if lastSubmitIdx == -1 {
		t.Fatal("no submit requests recorded")
	}
	if firstPollIdx == -1 {
		t.Fatal("no poll requests recorded")
	}
	if lastSubmitIdx >= firstPollIdx {
		t.Errorf("submit-all-before-poll violated: last submit at index %d, first poll at index %d\nlog: %v",
			lastSubmitIdx, firstPollIdx, log)
	}

	// Verify exactly 3 submits.
	submitCountActual := 0
	for _, entry := range log {
		if strings.HasPrefix(entry, "submit:") {
			submitCountActual++
		}
	}
	if submitCountActual != 3 {
		t.Errorf("expected 3 submits, got %d", submitCountActual)
	}

	// --- Verify per-file results stored ---
	for _, f := range []string{
		"/Volumes/field_Recording/F3/Orig/file1.WAV",
		"/Volumes/field_Recording/F3/Orig/file2.WAV",
		"/Volumes/field_Recording/F3/Orig/file3.WAV",
	} {
		val, ok := results.Get("birdnet", f)
		if !ok {
			t.Errorf("missing per-file result for %s", f)
			continue
		}
		fr, ok := val.(BirdNetFileResult)
		if !ok {
			t.Errorf("per-file result for %s is not BirdNetFileResult: %T", f, val)
		}
		if fr.JobStatus != "succeeded" {
			t.Errorf("per-file result for %s has status %q, want succeeded", f, fr.JobStatus)
		}
	}

	// --- Verify session result ---
	sessVal, ok := results.Get("birdnet", "session")
	if !ok {
		t.Fatal("missing session result")
	}
	session, ok := sessVal.(SessionBirdNetResult)
	if !ok {
		t.Fatalf("session result is not SessionBirdNetResult: %T", sessVal)
	}
	if session.TotalFiles != 3 {
		t.Errorf("TotalFiles: got %d, want 3", session.TotalFiles)
	}
	if session.TotalDetections != 2 {
		t.Errorf("TotalDetections: got %d, want 2", session.TotalDetections)
	}
	if len(session.Species) != 2 {
		t.Errorf("Species count: got %d, want 2", len(session.Species))
	}

	// --- Verify progress and elapsed output ---
	if !strings.Contains(output, "elapsed") {
		t.Errorf("output missing elapsed timer.\nOutput:\n%s", output)
	}
	if !strings.Contains(output, "complete") {
		t.Errorf("output missing progress counter.\nOutput:\n%s", output)
	}
	if !strings.Contains(output, "detections") {
		t.Errorf("output missing detection count.\nOutput:\n%s", output)
	}
	if !strings.Contains(output, "species") {
		t.Errorf("output missing species count.\nOutput:\n%s", output)
	}
}

func TestBirdNetSubmitAllPartialFailure(t *testing.T) {
	// Second job fails — verify other results still collected.
	jobIDs := []string{"job-ok", "job-fail"}

	var submitMu sync.Mutex
	submitIdx := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/plugin/birda/handle") {
			submitMu.Lock()
			idx := submitIdx
			submitIdx++
			submitMu.Unlock()

			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]any{
				"job_id":  jobIDs[idx],
				"status":  "queued",
				"plugin":  "birda",
				"command": "handle",
			})
			return
		}

		if jobID, ok := strings.CutPrefix(r.URL.Path, "/job/"); ok {
			status := "succeeded"
			var result any
			if jobID == "job-fail" {
				status = "failed"
				result = nil
			} else {
				result = map[string]any{
					"status":          "ok",
					"output_path":     "/out/ok.csv",
					"detections":      []any{},
					"detection_count": 0,
					"duration_s":      30.0,
					"realtime_factor":  3.0,
				}
			}

			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"job_id": jobID,
				"status": status,
				"result": result,
			})
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Services: config.Services{
			DuctileAPIURL:   srv.URL,
			DuctileTokenEnv: "TEST_DUCTILE_TOKEN2",
		},
		Defaults: config.Defaults{BirdnetMinConf: 0.5},
		Paths: config.Paths{
			ProcessingRoot: "/mnt/user/field_Recording",
			AllowedPaths:   []string{"/Volumes/field_Recording"},
		},
	}

	os.Setenv("TEST_DUCTILE_TOKEN2", "test-token")
	defer os.Unsetenv("TEST_DUCTILE_TOKEN2")

	b := &batch.Batch{
		SessionDate: "2026-03-29",
		Stages:      batch.StageConfig{BirdNet: true},
		Pipeline:    batch.PipelineConfig{DefaultLat: -34.0, DefaultLon: 150.5},
		Files: []batch.FileEntry{
			{Path: "/Volumes/field_Recording/F3/Orig/ok.WAV", Type: "audio"},
			{Path: "/Volumes/field_Recording/F3/Orig/fail.WAV", Type: "audio"},
		},
	}

	results := pipeline.NewResults()
	bn := &BirdNet{Cfg: cfg}
	err := bn.Run(context.Background(), b, results)
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// ok.WAV should have a result, fail.WAV should not.
	if _, ok := results.Get("birdnet", "/Volumes/field_Recording/F3/Orig/ok.WAV"); !ok {
		t.Error("missing result for ok.WAV")
	}
	if _, ok := results.Get("birdnet", "/Volumes/field_Recording/F3/Orig/fail.WAV"); ok {
		t.Error("unexpected result for fail.WAV — should have been dropped")
	}

	sessVal, ok := results.Get("birdnet", "session")
	if !ok {
		t.Fatal("missing session result")
	}
	session := sessVal.(SessionBirdNetResult)
	if session.TotalFiles != 1 {
		t.Errorf("TotalFiles: got %d, want 1", session.TotalFiles)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("mustJSON: %v", err)
	}
	return json.RawMessage(data)
}

func TestBirdNetSubmitNonAudioSkipped(t *testing.T) {
	// Verify non-audio files are not submitted.
	var submitMu sync.Mutex
	submits := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/plugin/") {
			submitMu.Lock()
			submits++
			id := submits
			submitMu.Unlock()
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]any{
				"job_id": fmt.Sprintf("job-%d", id),
				"status": "queued",
			})
			return
		}
		if strings.HasPrefix(r.URL.Path, "/job/") {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]any{
				"job_id": strings.TrimPrefix(r.URL.Path, "/job/"),
				"status": "succeeded",
				"result": map[string]any{
					"status":          "ok",
					"output_path":     "/out/x.csv",
					"detections":      []any{},
					"detection_count": 0,
					"duration_s":      10.0,
					"realtime_factor":  2.0,
				},
			})
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	cfg := &config.Config{
		Services: config.Services{DuctileAPIURL: srv.URL, DuctileTokenEnv: "TEST_SKIP_TOKEN"},
		Defaults: config.Defaults{BirdnetMinConf: 0.5},
		Paths: config.Paths{
			ProcessingRoot: "/mnt/user/field_Recording",
			AllowedPaths:   []string{"/Volumes/field_Recording"},
		},
	}
	os.Setenv("TEST_SKIP_TOKEN", "tok")
	defer os.Unsetenv("TEST_SKIP_TOKEN")

	b := &batch.Batch{
		SessionDate: "2026-03-29",
		Stages:      batch.StageConfig{BirdNet: true},
		Pipeline:    batch.PipelineConfig{DefaultLat: -34.0, DefaultLon: 150.5},
		Files: []batch.FileEntry{
			{Path: "/Volumes/field_Recording/F3/Orig/a.WAV", Type: "audio"},
			{Path: "/Volumes/field_Recording/F3/Orig/b.JPG", Type: "photo"},
			{Path: "/Volumes/field_Recording/F3/Orig/c.WAV", Type: "audio"},
		},
	}

	results := pipeline.NewResults()
	bn := &BirdNet{Cfg: cfg}
	if err := bn.Run(context.Background(), b, results); err != nil {
		t.Fatalf("Run: %v", err)
	}

	submitMu.Lock()
	got := submits
	submitMu.Unlock()
	if got != 2 {
		t.Errorf("expected 2 submits (audio only), got %d", got)
	}
}
