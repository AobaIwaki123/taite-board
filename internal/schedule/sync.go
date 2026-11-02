package schedule

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// DefaultSyncTargets returns the standard target file paths relative to rootDir.
func DefaultSyncTargets(rootDir string) []string {
	return []string{
		filepath.Join(rootDir, "apps", "web", "public", "schedule.json"),
		filepath.Join(rootDir, "apps", "web", "public", "mock_schedule.json"),
		filepath.Join(rootDir, "data", "groups", "ribonclock", "schedule.json"),
		filepath.Join(rootDir, "apps", "web", "public", "data", "groups", "ribonclock", "schedule.json"),
	}
}

// Sync copies sourcePath to all targetPaths and verifies SHA256 checksums match.
func Sync(sourcePath string, targetPaths ...string) (string, error) {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", fmt.Errorf("reading source file %s: %w", sourcePath, err)
	}

	srcHashBytes := sha256.Sum256(data)
	srcHash := hex.EncodeToString(srcHashBytes[:])

	for _, target := range targetPaths {
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return "", fmt.Errorf("creating dir for %s: %w", target, err)
		}

		// Write only if different to avoid touching mtime unnecessarily
		existing, err := os.ReadFile(target)
		if err == nil && bytes.Equal(existing, data) {
			continue
		}

		if err := os.WriteFile(target, data, 0644); err != nil {
			return "", fmt.Errorf("writing target file %s: %w", target, err)
		}

		// Verify target written matches source hash
		targetData, err := os.ReadFile(target)
		if err != nil {
			return "", fmt.Errorf("re-reading target file %s: %w", target, err)
		}
		targetHashBytes := sha256.Sum256(targetData)
		if targetHashBytes != srcHashBytes {
			return "", fmt.Errorf("checksum mismatch on target %s", target)
		}
	}

	return srcHash, nil
}
