package schedule

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"math/bits"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	visualMaxHammingDist  = 8
	visualMaxMAE16        = 7.0
	visualMaxAspectDelta  = 0.12
	maxVisualImageBytes   = 16 << 20
	visualMinLumaContrast = 2.0 // minimum [0..255] luma contrast to compute perceptual hash
)

// ImageSignature summarizes an image's aspect ratio, 16x16 luma grid, and 64-bit dHash.
type ImageSignature struct {
	Hash        uint64
	Grid        [16][16]float64
	AspectRatio float64
	Valid       bool
}

// ImageFingerprinter computes visual signatures for image URLs.
type ImageFingerprinter interface {
	Fingerprint(ctx context.Context, rawURL string) (ImageSignature, error)
}

// HTTPImageFingerprinter downloads JPEG/PNG images via HTTP and caches signatures per canonical media key.
type HTTPImageFingerprinter struct {
	Client *http.Client
	mu     sync.Mutex
	cache  map[string]cachedFingerprint
}

type cachedFingerprint struct {
	sig ImageSignature
	err error
}

// NewHTTPImageFingerprinter returns an HTTP-backed ImageFingerprinter with an in-run signature cache.
func NewHTTPImageFingerprinter(timeout time.Duration) *HTTPImageFingerprinter {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return &HTTPImageFingerprinter{
		Client: &http.Client{Timeout: timeout},
		cache:  make(map[string]cachedFingerprint),
	}
}

// Fingerprint fetches rawURL once per media key and computes its 64-bit perceptual dHash.
func (f *HTTPImageFingerprinter) Fingerprint(ctx context.Context, rawURL string) (ImageSignature, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return ImageSignature{}, fmt.Errorf("empty image url")
	}
	key, _ := imageKey(trimmed)

	f.mu.Lock()
	if cached, ok := f.cache[key]; ok {
		f.mu.Unlock()
		return cached.sig, cached.err
	}
	f.mu.Unlock()

	sig, err := f.fetchAndDecode(ctx, trimmed)

	f.mu.Lock()
	f.cache[key] = cachedFingerprint{sig: sig, err: err}
	f.mu.Unlock()
	return sig, err
}

func (f *HTTPImageFingerprinter) fetchAndDecode(ctx context.Context, rawURL string) (ImageSignature, error) {
	client := f.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return ImageSignature{}, err
	}
	req.Header.Set("User-Agent", "taite-board-image-dedupe/1.0")
	resp, err := client.Do(req)
	if err != nil {
		return ImageSignature{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ImageSignature{}, fmt.Errorf("image fetch HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxVisualImageBytes+1))
	if err != nil {
		return ImageSignature{}, err
	}
	if len(body) > maxVisualImageBytes {
		return ImageSignature{}, fmt.Errorf("image exceeds %d bytes", maxVisualImageBytes)
	}
	img, _, err := image.Decode(strings.NewReader(string(body)))
	if err != nil {
		return ImageSignature{}, err
	}
	return ComputeImageSignature(img)
}

func sampleLumaGrid(img image.Image, cols, rows int) (grid []float64, minLum, maxLum float64) {
	b := img.Bounds()
	grid = make([]float64, cols*rows)
	minLum = math.MaxFloat64
	maxLum = -1.0
	dx := float64(b.Dx()) / float64(cols)
	dy := float64(b.Dy()) / float64(rows)

	for gy := 0; gy < rows; gy++ {
		y0 := b.Min.Y + int(float64(gy)*dy)
		y1 := b.Min.Y + int(float64(gy+1)*dy)
		if y1 <= y0 {
			y1 = y0 + 1
		}
		stepY := (y1 - y0) / 8
		if stepY < 1 {
			stepY = 1
		}
		for gx := 0; gx < cols; gx++ {
			x0 := b.Min.X + int(float64(gx)*dx)
			x1 := b.Min.X + int(float64(gx+1)*dx)
			if x1 <= x0 {
				x1 = x0 + 1
			}
			stepX := (x1 - x0) / 8
			if stepX < 1 {
				stepX = 1
			}
			var sum float64
			var cnt int
			for py := y0; py < y1; py += stepY {
				for px := x0; px < x1; px += stepX {
					r, g, bl, _ := img.At(px, py).RGBA()
					sum += (0.299*float64(r) + 0.587*float64(g) + 0.114*float64(bl)) / 257.0
					cnt++
				}
			}
			avg := sum / float64(cnt)
			grid[gy*cols+gx] = avg
			if avg < minLum {
				minLum = avg
			}
			if avg > maxLum {
				maxLum = avg
			}
		}
	}
	return grid, minLum, maxLum
}

// ComputeImageSignature calculates an aspect ratio, 16x16 luma grid, and 9x8 difference hash (64-bit dHash) from img.
func ComputeImageSignature(img image.Image) (ImageSignature, error) {
	if img == nil {
		return ImageSignature{}, fmt.Errorf("nil image")
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return ImageSignature{}, fmt.Errorf("invalid image dimensions %dx%d", b.Dx(), b.Dy())
	}

	const dCols, dRows = 9, 8
	lum, minLum, maxLum := sampleLumaGrid(img, dCols, dRows)
	if maxLum-minLum < visualMinLumaContrast {
		return ImageSignature{}, fmt.Errorf("image lacks gradient contrast")
	}

	var hash uint64
	var bit uint
	for gy := 0; gy < dRows; gy++ {
		for gx := 0; gx < dCols-1; gx++ {
			if lum[gy*dCols+gx] > lum[gy*dCols+gx+1] {
				hash |= 1 << bit
			}
			bit++
		}
	}

	rawGrid16, _, _ := sampleLumaGrid(img, 16, 16)
	var grid [16][16]float64
	for gy := 0; gy < 16; gy++ {
		copy(grid[gy][:], rawGrid16[gy*16:(gy+1)*16])
	}

	return ImageSignature{
		Hash:        hash,
		Grid:        grid,
		AspectRatio: float64(b.Dx()) / float64(b.Dy()),
		Valid:       true,
	}, nil
}

func gridMAE16(a, b [16][16]float64) float64 {
	var total float64
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			total += math.Abs(a[y][x] - b[y][x])
		}
	}
	return total / 256.0
}

// IsVisuallyDuplicate returns true when two valid signatures share aspect ratio, dHash, and 16x16 luma grid within thresholds.
func IsVisuallyDuplicate(a, b ImageSignature) bool {
	if !a.Valid || !b.Valid {
		return false
	}
	if math.Abs(a.AspectRatio-b.AspectRatio) > visualMaxAspectDelta {
		return false
	}
	if bits.OnesCount64(a.Hash^b.Hash) > visualMaxHammingDist {
		return false
	}
	return gridMAE16(a.Grid, b.Grid) <= visualMaxMAE16
}

type keptVisualImage struct {
	entry map[string]interface{}
	url   string
	sig   ImageSignature
}

// imageSourceRank prefers original high-resolution X media over TimeTree re-uploads.
func imageSourceRank(rawURL string) int {
	lower := strings.ToLower(strings.TrimSpace(rawURL))
	switch {
	case strings.Contains(lower, "pbs.twimg.com"):
		return 3
	case strings.Contains(lower, "attachments.timetreeapp.com"):
		return 1
	default:
		return 2
	}
}

// VisualDedupeSession removes exact URL duplicates and visually identical images (e.g., X vs TimeTree re-uploads)
// from a session map while preserving the higher-priority source URL (X > generic > TimeTree) and keeping image_url consistent.
// Any network or decode failure fails open so unreachable images are never mistakenly deleted.
func VisualDedupeSession(ctx context.Context, sMap map[string]interface{}, fp ImageFingerprinter) (removed int, changed bool) {
	beforeLen := 0
	if imgs, ok := sMap["images"].([]interface{}); ok {
		beforeLen = len(imgs)
	}
	if dedupeImages(sMap) {
		changed = true
	}
	imgs, _ := sMap["images"].([]interface{})
	if fp == nil || len(imgs) <= 1 {
		if afterLen := len(imgs); afterLen < beforeLen {
			removed = beforeLen - afterLen
		}
		return removed, changed
	}

	kept := make([]keptVisualImage, 0, len(imgs))
	out := make([]interface{}, 0, len(imgs))
	droppedURLs := make(map[string]string)

	for _, rawImg := range imgs {
		im, _ := rawImg.(map[string]interface{})
		if im == nil {
			changed = true
			continue
		}
		u := strings.TrimSpace(stringField(im, "url"))
		if u == "" {
			out = append(out, im)
			continue
		}
		sig, err := fp.Fingerprint(ctx, u)
		if err != nil || !sig.Valid {
			kept = append(kept, keptVisualImage{entry: im, url: u})
			out = append(out, im)
			continue
		}
		dupIdx := -1
		for idx, k := range kept {
			if IsVisuallyDuplicate(k.sig, sig) {
				dupIdx = idx
				break
			}
		}
		if dupIdx >= 0 {
			existing := &kept[dupIdx]
			if imageSourceRank(u) > imageSourceRank(existing.url) {
				oldURL := existing.url
				existing.entry["url"] = u
				existing.url = u
				existing.sig = sig
				droppedURLs[oldURL] = u
			} else {
				droppedURLs[u] = existing.url
			}
			removed++
			changed = true
			continue
		}
		kept = append(kept, keptVisualImage{entry: im, url: u, sig: sig})
		out = append(out, im)
	}

	if changed {
		sMap["images"] = out
		curImgURL := strings.TrimSpace(stringField(sMap, "image_url"))
		if curImgURL != "" {
			for step := 0; step < 4; step++ {
				if replacement, dropped := droppedURLs[curImgURL]; dropped && replacement != curImgURL {
					curImgURL = replacement
					continue
				}
				break
			}
			sMap["image_url"] = curImgURL
		}
		if stringField(sMap, "image_url") == "" && len(kept) > 0 {
			sMap["image_url"] = kept[0].url
		}
	}
	return removed, changed
}

// VisualDedupeScheduleBytes runs VisualDedupeSession across every session in raw schedule JSON.
// Only sessions whose images change are re-serialized; root fields outside modified sessions stay intact.
func VisualDedupeScheduleBytes(ctx context.Context, raw []byte, fp ImageFingerprinter) ([]byte, int, error) {
	return visualDedupeScheduleBytesFiltered(ctx, raw, "", fp)
}

// VisualDedupeSessionByIDBytes runs VisualDedupeSession only on the specified sessionID
// so interactive intake endpoints do not re-download unrelated sessions' images.
func VisualDedupeSessionByIDBytes(ctx context.Context, raw []byte, sessionID string, fp ImageFingerprinter) ([]byte, int, error) {
	if strings.TrimSpace(sessionID) == "" {
		return raw, 0, nil
	}
	return visualDedupeScheduleBytesFiltered(ctx, raw, strings.TrimSpace(sessionID), fp)
}

func visualDedupeScheduleBytesFiltered(ctx context.Context, raw []byte, targetSessionID string, fp ImageFingerprinter) ([]byte, int, error) {
	if fp == nil {
		return raw, 0, nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, 0, fmt.Errorf("unmarshaling schedule json: %w", err)
	}
	rawSessionsBytes, hasSessions := root["sessions"]
	if !hasSessions {
		return raw, 0, nil
	}
	var rawSessions []json.RawMessage
	if err := json.Unmarshal(rawSessionsBytes, &rawSessions); err != nil {
		return nil, 0, fmt.Errorf("unmarshaling sessions: %w", err)
	}

	totalRemoved := 0
	anyChanged := false
	for i, sBytes := range rawSessions {
		var sMap map[string]interface{}
		if err := json.Unmarshal(sBytes, &sMap); err != nil {
			continue
		}
		if targetSessionID != "" && strings.TrimSpace(stringField(sMap, "id")) != targetSessionID {
			continue
		}
		removed, changed := VisualDedupeSession(ctx, sMap, fp)
		if changed {
			updatedSession, err := json.Marshal(sMap)
			if err != nil {
				return nil, 0, fmt.Errorf("marshaling deduped session %d: %w", i, err)
			}
			rawSessions[i] = json.RawMessage(updatedSession)
			totalRemoved += removed
			anyChanged = true
		}
	}
	if !anyChanged {
		return raw, 0, nil
	}

	updatedSessionsBytes, err := json.Marshal(rawSessions)
	if err != nil {
		return nil, 0, fmt.Errorf("marshaling sessions: %w", err)
	}
	root["sessions"] = json.RawMessage(updatedSessionsBytes)
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, 0, err
	}
	return out, totalRemoved, nil
}
