package ductile

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSign(t *testing.T) {
	c := NewClient("http://example.com", "test-secret")
	sig := c.Sign([]byte(`{"payload":{"wav_path":"/test.WAV"}}`))

	if sig[:7] != "sha256=" {
		t.Errorf("signature should start with 'sha256=', got %q", sig[:7])
	}
	if len(sig) != 7+64 { // sha256= + 64 hex chars
		t.Errorf("signature length: got %d, want 71", len(sig))
	}
}

func TestPost(t *testing.T) {
	type testPayload struct {
		Msg string `json:"msg"`
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type: got %q", ct)
		}
		sig := r.Header.Get("X-Ductile-Signature-256")
		if sig == "" {
			t.Error("missing X-Ductile-Signature-256 header")
		}
		if sig[:7] != "sha256=" {
			t.Errorf("signature prefix: got %q", sig[:7])
		}

		body, _ := io.ReadAll(r.Body)
		var p testPayload
		if err := json.Unmarshal(body, &p); err != nil {
			t.Errorf("unmarshal body: %v", err)
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "test-secret")
	resp, err := client.Post(context.Background(), testPayload{Msg: "hello"})
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if string(resp) != `{"status":"ok"}` {
		t.Errorf("response: got %q", string(resp))
	}
}
