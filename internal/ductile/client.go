package ductile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client talks to the Ductile REST API (Bearer token auth).
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		BaseURL: baseURL,
		Token:   token,
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// SubmitResponse is the 202 Accepted response from POST /plugin/{name}/{command}.
type SubmitResponse struct {
	JobID   string `json:"job_id"`
	Status  string `json:"status"`
	Plugin  string `json:"plugin"`
	Command string `json:"command"`
}

// JobResponse is the response from GET /job/{id}.
type JobResponse struct {
	JobID       string          `json:"job_id"`
	Status      string          `json:"status"`
	Plugin      string          `json:"plugin"`
	Command     string          `json:"command"`
	CreatedAt   string          `json:"created_at"`
	StartedAt   string          `json:"started_at"`
	CompletedAt string          `json:"completed_at"`
	Result      json.RawMessage `json:"result"`
}

// Submit sends a payload to POST /plugin/{plugin}/{command} and returns the job ID.
func (c *Client) Submit(ctx context.Context, plugin, command string, payload any) (*SubmitResponse, error) {
	body := map[string]any{"payload": payload}
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	url := fmt.Sprintf("%s/plugin/%s/%s", c.BaseURL, plugin, command)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http post: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ductile returned %d: %s", resp.StatusCode, string(respBody))
	}

	var sr SubmitResponse
	if err := json.Unmarshal(respBody, &sr); err != nil {
		return nil, fmt.Errorf("parse submit response: %w", err)
	}
	return &sr, nil
}

// GetJob retrieves the current status of a job.
func (c *Client) GetJob(ctx context.Context, jobID string) (*JobResponse, error) {
	url := fmt.Sprintf("%s/job/%s", c.BaseURL, jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ductile returned %d: %s", resp.StatusCode, string(respBody))
	}

	var jr JobResponse
	if err := json.Unmarshal(respBody, &jr); err != nil {
		return nil, fmt.Errorf("parse job response: %w", err)
	}
	return &jr, nil
}

// ListJobsResponse is the response from GET /jobs.
type ListJobsResponse struct {
	Jobs []JobSummary `json:"jobs"`
}

// JobSummary is one entry in the /jobs listing.
type JobSummary struct {
	JobID       string `json:"job_id"`
	Plugin      string `json:"plugin"`
	Command     string `json:"command"`
	Status      string `json:"status"`
	CreatedAt   string `json:"created_at"`
	StartedAt   string `json:"started_at"`
	CompletedAt string `json:"completed_at"`
	Attempt     int    `json:"attempt"`
}

// ListJobs queries GET /jobs with optional plugin filter and limit.
func (c *Client) ListJobs(ctx context.Context, plugin string, limit int) (*ListJobsResponse, error) {
	url := fmt.Sprintf("%s/jobs?limit=%d", c.BaseURL, limit)
	if plugin != "" {
		url += "&plugin=" + plugin
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ductile returned %d: %s", resp.StatusCode, string(respBody))
	}

	var lr ListJobsResponse
	if err := json.Unmarshal(respBody, &lr); err != nil {
		return nil, fmt.Errorf("parse jobs response: %w", err)
	}
	return &lr, nil
}

// WaitForJob polls until the job reaches a terminal state. Returns the final job response.
func (c *Client) WaitForJob(ctx context.Context, jobID string, pollInterval time.Duration) (*JobResponse, error) {
	for {
		job, err := c.GetJob(ctx, jobID)
		if err != nil {
			return nil, err
		}

		switch job.Status {
		case "succeeded", "failed", "dead", "timed_out":
			return job, nil
		case "queued", "running":
			// keep polling
		default:
			return job, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(pollInterval):
		}
	}
}
