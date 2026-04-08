package ductile

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSubmit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if r.URL.Path != "/plugin/birda/handle" {
			t.Errorf("path: got %s, want /plugin/birda/handle", r.URL.Path)
		}
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("Authorization: got %q", auth)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type: got %q", ct)
		}

		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("unmarshal body: %v", err)
		}
		if _, ok := req["payload"]; !ok {
			t.Error("missing payload wrapper in request body")
		}

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(SubmitResponse{
			JobID:   "job-123",
			Status:  "queued",
			Plugin:  "birda",
			Command: "handle",
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	sr, err := client.Submit(context.Background(), "birda", "handle", map[string]any{
		"wav_path": "/mnt/user/test.WAV",
		"lat":      -34.0,
		"lon":      150.5,
	})
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if sr.JobID != "job-123" {
		t.Errorf("JobID: got %q, want job-123", sr.JobID)
	}
}

func TestGetJob(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/job/job-123" {
			t.Errorf("path: got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id":  "job-123",
			"status":  "succeeded",
			"plugin":  "birda",
			"command": "handle",
			"result":  map[string]any{"status": "ok", "detection_count": 5},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	job, err := client.GetJob(context.Background(), "job-123")
	if err != nil {
		t.Fatalf("GetJob: %v", err)
	}
	if job.Status != "succeeded" {
		t.Errorf("Status: got %q", job.Status)
	}
}

func TestWaitForJob(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		status := "running"
		if callCount >= 2 {
			status = "succeeded"
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"job_id": "job-456",
			"status": status,
			"result": map[string]any{"status": "ok"},
		})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-token")
	job, err := client.WaitForJob(context.Background(), "job-456", 10*time.Millisecond)
	if err != nil {
		t.Fatalf("WaitForJob: %v", err)
	}
	if job.Status != "succeeded" {
		t.Errorf("Status: got %q", job.Status)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 polls, got %d", callCount)
	}
}
