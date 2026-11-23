package version

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultVersion is used as a fallback if neither BuildVersion nor a VERSION file is available.
const DefaultVersion = "1.0.0"

// BuildVersion can be injected at compile time via:
// -ldflags "-X github.com/AobaIwaki123/taite-board/internal/version.BuildVersion=1.5.0"
var BuildVersion = ""

// Get returns the current application semantic version without a leading "v".
func Get() string {
	if v := normalize(BuildVersion); v != "" && v != "dev" {
		return v
	}

	dir, err := os.Getwd()
	if err == nil {
		for i := 0; i < 5; i++ {
			vPath := filepath.Join(dir, "VERSION")
			if data, err := os.ReadFile(vPath); err == nil {
				if v := normalize(string(data)); v != "" {
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

func normalize(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "v")
	return s
}
