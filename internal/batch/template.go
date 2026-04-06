package batch

import (
	"fmt"

	"github.com/mattjoyce/framore/internal/config"
)

// DefaultBatchYAML returns a YAML template string pre-populated with defaults
// from the global config.
func DefaultBatchYAML(cfg *config.Config) string {
	return fmt.Sprintf(`session_dir: ""
session_date: ""
stages:
  exif: true
  weather: true
  birdnet: true
  transcribe: false
  report: true
birdnet:
  min_conf: %.1f
weather:
  timezone: "%s"
pipeline:
  default_lat: %.4f
  default_lon: %.4f
files: []
`,
		cfg.Defaults.BirdnetMinConf,
		cfg.Defaults.Timezone,
		cfg.Defaults.DefaultLat,
		cfg.Defaults.DefaultLon,
	)
}
