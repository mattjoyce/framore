package batch

import (
	"fmt"
	"strings"

	"github.com/mattjoyce/framore/internal/config"
)

// CheckAllowedPath verifies that absPath falls under one of the configured
// allowed paths and returns the processing (NAS) equivalent by replacing the
// matching local prefix with processing_root.
func CheckAllowedPath(absPath string, cfg *config.Config) (string, error) {
	for _, prefix := range cfg.Paths.AllowedPaths {
		if strings.HasPrefix(absPath, prefix) {
			remotePath := strings.Replace(absPath, prefix, cfg.Paths.ProcessingRoot, 1)
			return remotePath, nil
		}
	}

	return "", fmt.Errorf(
		"path %q is not under any allowed path; allowed prefixes: %s\nadd a path in %s under [paths] allowed_paths",
		absPath,
		strings.Join(cfg.Paths.AllowedPaths, ", "),
		config.ConfigPath(),
	)
}
