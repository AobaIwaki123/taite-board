package candidate

import (
	"testing"
)

func TestComputeRawFingerprint(t *testing.T) {
	fp1 := ComputeRawFingerprint("x", "12345", "明日のタイテです", []string{"https://pbs.twimg.com/media/img1.jpg"})
	fp2 := ComputeRawFingerprint("x", "12345", "  明日のタイテです  ", []string{"https://pbs.twimg.com/media/img1.jpg"})
	fp3 := ComputeRawFingerprint("x", "12345", "明日のタイテです", []string{"https://pbs.twimg.com/media/img2.jpg"})

	if fp1 != fp2 {
		t.Errorf("expected whitespace normalization to yield identical fingerprints, got %s != %s", fp1, fp2)
	}

	if fp1 == fp3 {
		t.Errorf("expected differing image list to produce different fingerprints, got %s == %s", fp1, fp3)
	}

	if len(fp1) != 64 {
		t.Errorf("expected 64-character sha256 hex string, got len %d", len(fp1))
	}
}

func TestComputeParseCacheKey(t *testing.T) {
	fp := ComputeRawFingerprint("x", "100", "test", nil)
	key1 := ComputeParseCacheKey(fp, 1, "2026-10-30.1", "gemini-2.5-flash", "v1")
	key2 := ComputeParseCacheKey(fp, 1, "2026-10-30.1", "gemini-2.5-flash", "v1")
	key3 := ComputeParseCacheKey(fp, 1, "2026-10-30.2", "gemini-2.5-flash", "v1")

	if key1 != key2 {
		t.Errorf("expected identical inputs to yield identical cache keys")
	}
	if key1 == key3 {
		t.Errorf("expected differing parser versions to invalidate cache key")
	}
}
