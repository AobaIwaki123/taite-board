package version

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultVersion is used as a fallback if the VERSION file cannot be found.
const DefaultVersion = "1.0.0"

// Get returns the current application semantic version.
func Get() string {
	dir, err := os.Getwd()
	if err == nil {
		for i := 0; i < 5; i++ {
			vPath := filepath.Join(dir, "VERSION")
			if data, err := os.ReadFile(vPath); err == nil {
				v := strings.TrimSpace(string(data))
				if v != "" {
					return v
				}
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	return DefaultVersion
}
