package batch

import (
	"testing"

	"github.com/mattjoyce/framore/internal/config"
)

func TestCheckAllowedPath_Valid(t *testing.T) {
	cfg := &config.Config{
		Paths: config.Paths{
			AllowedPaths: []config.PathMapping{
				{Mac: "/Volumes/field_Recording", NAS: "/mnt/field_Recording"},
			},
		},
	}

	nasPath, err := CheckAllowedPath("/Volumes/field_Recording/F3/Orig/test.WAV", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/mnt/field_Recording/F3/Orig/test.WAV"
	if nasPath != want {
		t.Errorf("got %q, want %q", nasPath, want)
	}
}

func TestCheckAllowedPath_Rejected(t *testing.T) {
	cfg := &config.Config{
		Paths: config.Paths{
			AllowedPaths: []config.PathMapping{
				{Mac: "/Volumes/field_Recording", NAS: "/mnt/field_Recording"},
			},
		},
	}

	_, err := CheckAllowedPath("/Users/matt/Downloads/test.wav", cfg)
	if err == nil {
		t.Fatal("expected error for path outside allowed_paths")
	}
}
