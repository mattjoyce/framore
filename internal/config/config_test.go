package config

import (
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Defaults.Timezone != "Australia/Sydney" {
		t.Errorf("Timezone: got %q, want %q", cfg.Defaults.Timezone, "Australia/Sydney")
	}
	if cfg.Defaults.DefaultLat != -34.0021 {
		t.Errorf("DefaultLat: got %f, want -34.0021", cfg.Defaults.DefaultLat)
	}
	if cfg.Defaults.BirdnetMinConf != 0.6 {
		t.Errorf("BirdnetMinConf: got %f, want 0.6", cfg.Defaults.BirdnetMinConf)
	}
	if cfg.Paths.ProcessingRoot != "/mnt/user/field_Recording" {
		t.Errorf("ProcessingRoot: got %q", cfg.Paths.ProcessingRoot)
	}
	if len(cfg.Paths.AllowedPaths) != 2 {
		t.Fatalf("AllowedPaths: got %d, want 2", len(cfg.Paths.AllowedPaths))
	}
	if cfg.Paths.AllowedPaths[0] != "/Volumes/field_Recording" {
		t.Errorf("AllowedPaths[0]: got %q", cfg.Paths.AllowedPaths[0])
	}
	if cfg.Services.DuctileTokenEnv != "FRAMORE_DUCTILE_TOKEN" {
		t.Errorf("DuctileTokenEnv: got %q", cfg.Services.DuctileTokenEnv)
	}
}
