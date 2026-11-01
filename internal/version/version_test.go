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
