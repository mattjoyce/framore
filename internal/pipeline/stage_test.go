package pipeline

import (
	"testing"
)

func TestResults_SetGet(t *testing.T) {
	r := NewResults()

	r.Set("weather", "session", "sunny")
	val, ok := r.Get("weather", "session")
	if !ok {
		t.Fatal("expected Get to find value")
	}
	if val != "sunny" {
		t.Errorf("got %v, want 'sunny'", val)
	}

	_, ok = r.Get("weather", "missing")
	if ok {
		t.Error("expected Get to return false for missing key")
	}

	_, ok = r.Get("missing", "session")
	if ok {
		t.Error("expected Get to return false for missing stage")
	}
}

func TestResults_AllForStage(t *testing.T) {
	r := NewResults()

	r.Set("birdnet", "/a.wav", "result-a")
	r.Set("birdnet", "/b.wav", "result-b")

	all := r.AllForStage("birdnet")
	if len(all) != 2 {
		t.Fatalf("got %d entries, want 2", len(all))
	}

	empty := r.AllForStage("missing")
	if empty != nil {
		t.Error("expected nil for missing stage")
	}
}
