package version

import (
	"testing"
)

func TestGet(t *testing.T) {
	v := Get()
	if v == "" {
		t.Fatalf("expected non-empty version, got empty")
	}
}

func TestGet_BuildVersionOverride(t *testing.T) {
	orig := BuildVersion
	t.Cleanup(func() { BuildVersion = orig })

	BuildVersion = "v1.5.0"
	if got := Get(); got != "1.5.0" {
		t.Fatalf("expected 1.5.0, got %q", got)
	}

	BuildVersion = "dev"
	if got := Get(); got == "" || got == "dev" {
		t.Fatalf("expected fallback when BuildVersion=dev, got %q", got)
	}
}
