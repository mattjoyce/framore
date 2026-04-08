package batch

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Batch struct {
	SessionDir  string             `yaml:"session_dir"`
	SessionDate string             `yaml:"session_date"`
	Stages      StageConfig        `yaml:"stages"`
	BirdNet     BirdNetConfig      `yaml:"birdnet"`
	Weather     WeatherStageConfig `yaml:"weather"`
	Pipeline    PipelineConfig     `yaml:"pipeline"`
	Files       []FileEntry        `yaml:"files"`
}

type StageConfig struct {
	EXIF       bool `yaml:"exif"`
	Weather    bool `yaml:"weather"`
	BirdNet    bool `yaml:"birdnet"`
	Transcribe bool `yaml:"transcribe"`
	Report     bool `yaml:"report"`
}

type BirdNetConfig struct {
	MinConf      float64 `yaml:"min_conf"`
	SkipExisting bool    `yaml:"skip_existing"`
}

type WeatherStageConfig struct {
	Timezone string `yaml:"timezone"`
}

type PipelineConfig struct {
	PlusCode   string  `yaml:"plus_code,omitempty"`
	DefaultLat float64 `yaml:"default_lat"`
	DefaultLon float64 `yaml:"default_lon"`
}

type FileEntry struct {
	Path  string   `yaml:"path"`
	Type  string   `yaml:"type"`
	Added string   `yaml:"added"`
	Meta  FileMeta `yaml:"meta"`
}

type FileMeta struct {
	DurationSeconds float64 `yaml:"duration_seconds,omitempty"`
	BitDepth        int     `yaml:"bit_depth,omitempty"`
	SampleRate      int     `yaml:"sample_rate,omitempty"`
	Channels        int     `yaml:"channels,omitempty"`
	SizeBytes       int64   `yaml:"size_bytes,omitempty"`
	Datetime        string  `yaml:"datetime,omitempty"`
	Lat             float64 `yaml:"lat,omitempty"`
	Lon             float64 `yaml:"lon,omitempty"`
	AltitudeM       float64 `yaml:"altitude_m,omitempty"`
	Device          string  `yaml:"device,omitempty"`
}

// Load reads a batch YAML file and unmarshals it into a Batch struct.
func Load(path string) (*Batch, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var b Batch
	if err := yaml.Unmarshal(data, &b); err != nil {
		return nil, err
	}
	return &b, nil
}

// Save marshals a Batch and writes it to the given path.
func Save(path string, b *Batch) error {
	data, err := yaml.Marshal(b)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// HasFile checks whether a file with the given absolute path already exists
// in the batch's file list.
func HasFile(b *Batch, absPath string) bool {
	for _, f := range b.Files {
		if f.Path == absPath {
			return true
		}
	}
	return false
}
