package version

import "strings"

// DefaultVersion is used as a fallback when BuildVersion is unset or "dev".
const DefaultVersion = "1.0.0"

// BuildVersion can be injected at compile time via:
// -ldflags "-X github.com/AobaIwaki123/taite-board/internal/version.BuildVersion=1.5.0"
var BuildVersion = ""

// Get returns the current application semantic version without a leading "v".
func Get() string {
	if v := normalize(BuildVersion); v != "" && v != "dev" {
		return v
	}
	return DefaultVersion
}

func normalize(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "v")
	return s
}
