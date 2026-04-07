package batch

import (
	"testing"

	"github.com/mattjoyce/framore/internal/config"
)

func TestCheckAllowedPath_Valid(t *testing.T) {
	cfg := &config.Config{
		Paths: config.Paths{
			ProcessingRoot: "/mnt/user/field_Recording",
			AllowedPaths:   []string{"/Volumes/field_Recording", "/mnt/field_Recording"},
		},
	}

	// Mac path
	nasPath, err := CheckAllowedPath("/Volumes/field_Recording/F3/Orig/test.WAV", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/mnt/user/field_Recording/F3/Orig/test.WAV"
	if nasPath != want {
		t.Errorf("mac: got %q, want %q", nasPath, want)
	}

	// Linux path
	nasPath, err = CheckAllowedPath("/mnt/field_Recording/F3/Orig/test.WAV", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if nasPath != want {
		t.Errorf("linux: got %q, want %q", nasPath, want)
	}
}

func TestCheckAllowedPath_Rejected(t *testing.T) {
	cfg := &config.Config{
		Paths: config.Paths{
			ProcessingRoot: "/mnt/user/field_Recording",
			AllowedPaths:   []string{"/Volumes/field_Recording"},
		},
	}

	_, err := CheckAllowedPath("/Users/matt/Downloads/test.wav", cfg)
	if err == nil {
		t.Fatal("expected error for path outside allowed_paths")
	}
}
