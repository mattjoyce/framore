package config

import (
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	CurrentBatch string        `toml:"current_batch"`
	Defaults     Defaults      `toml:"defaults"`
	Paths        Paths         `toml:"paths"`
	Services     Services      `toml:"services"`
	Weather      WeatherConfig `toml:"weather"`
	Output       Output        `toml:"output"`
}

type Defaults struct {
	Timezone       string  `toml:"timezone"`
	DefaultLat     float64 `toml:"default_lat"`
	DefaultLon     float64 `toml:"default_lon"`
	BirdnetMinConf float64 `toml:"birdnet_min_conf"`
}

type Paths struct {
	ProcessingRoot string   `toml:"processing_root"`
	AllowedPaths   []string `toml:"allowed_paths"`
}

type Services struct {
	DuctileAPIURL   string `toml:"ductile_api_url"`
	DuctileTokenEnv string `toml:"ductile_token_env"`
	OllamaURL       string `toml:"ollama_url"`
	WhisperURL      string `toml:"whisper_url"`
}

type WeatherConfig struct {
	CacheDir        string `toml:"cache_dir"`
	CacheMaxAgeDays int    `toml:"cache_max_age_days"`
	TimeoutSeconds  int    `toml:"timeout_seconds"`
}

type Output struct {
	LogLevel string `toml:"log_level"`
}

// ConfigDir returns the framore configuration directory path.
func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "framore")
}

// ConfigPath returns the full path to the framore config file.
func ConfigPath() string {
	return filepath.Join(ConfigDir(), "config.toml")
}

// DefaultConfig returns a Config populated with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Defaults: Defaults{
			Timezone:       "Australia/Sydney",
			DefaultLat:     -34.0021,
			DefaultLon:     150.4987,
			BirdnetMinConf: 0.6,
		},
		Paths: Paths{
			ProcessingRoot: "/mnt/user/field_Recording",
			AllowedPaths: []string{
				"/Volumes/field_Recording",
				"/mnt/field_Recording",
			},
		},
		Services: Services{
			DuctileAPIURL:   "http://192.168.20.4:8888",
			DuctileTokenEnv: "FRAMORE_DUCTILE_TOKEN",
			OllamaURL:       "http://192.168.20.4:11434",
			WhisperURL:      "http://192.168.20.4:8765",
		},
		Weather: WeatherConfig{
			CacheDir:        "~/.cache/framore/weather",
			CacheMaxAgeDays: 30,
			TimeoutSeconds:  30,
		},
		Output: Output{
			LogLevel: "info",
		},
	}
}

// Load reads the config from disk. If the file does not exist, it creates one
// with default values and returns those defaults.
func Load() (*Config, error) {
	path := ConfigPath()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if saveErr := SaveConfig(cfg); saveErr != nil {
				return nil, saveErr
			}
			return cfg, nil
		}
		return nil, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Backfill any zero-value fields with defaults so configs created
	// before new fields were added still work.
	defaults := DefaultConfig()
	if cfg.Defaults.Timezone == "" {
		cfg.Defaults.Timezone = defaults.Defaults.Timezone
	}
	if cfg.Defaults.BirdnetMinConf == 0 {
		cfg.Defaults.BirdnetMinConf = defaults.Defaults.BirdnetMinConf
	}
	if cfg.Services.DuctileAPIURL == "" {
		cfg.Services.DuctileAPIURL = defaults.Services.DuctileAPIURL
	}
	if cfg.Services.DuctileTokenEnv == "" {
		cfg.Services.DuctileTokenEnv = defaults.Services.DuctileTokenEnv
	}
	if cfg.Services.OllamaURL == "" {
		cfg.Services.OllamaURL = defaults.Services.OllamaURL
	}
	if cfg.Services.WhisperURL == "" {
		cfg.Services.WhisperURL = defaults.Services.WhisperURL
	}
	if cfg.Paths.ProcessingRoot == "" {
		cfg.Paths.ProcessingRoot = defaults.Paths.ProcessingRoot
	}
	if len(cfg.Paths.AllowedPaths) == 0 {
		cfg.Paths.AllowedPaths = defaults.Paths.AllowedPaths
	}

	return &cfg, nil
}

// Save is a convenience method that writes this config to disk.
func (c *Config) Save() error {
	return SaveConfig(c)
}

// SaveConfig writes the config to disk, creating the directory if needed.
func SaveConfig(cfg *Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}

	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}

	return os.WriteFile(ConfigPath(), data, 0o600)
}
