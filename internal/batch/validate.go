package batch

import (
	"fmt"
	"strings"

	"github.com/mattjoyce/framore/internal/config"
)

// CheckAllowedPath verifies that absPath falls under one of the configured
// allowed path mappings and returns the NAS-translated equivalent. If the path
// is not under any allowed mapping, an error is returned.
func CheckAllowedPath(absPath string, cfg *config.Config) (string, error) {
	for _, mapping := range cfg.Paths.AllowedPaths {
		if strings.HasPrefix(absPath, mapping.Mac) {
			nasPath := strings.Replace(absPath, mapping.Mac, mapping.NAS, 1)
			return nasPath, nil
		}
	}

	var allowed []string
	for _, mapping := range cfg.Paths.AllowedPaths {
		allowed = append(allowed, mapping.Mac)
	}

	return "", fmt.Errorf(
		"path %q is not under any allowed path; allowed prefixes: %s\nadd a path mapping in %s under [paths.allowed_paths]",
		absPath,
		strings.Join(allowed, ", "),
		config.ConfigPath(),
	)
}
