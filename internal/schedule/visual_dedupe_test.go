package schedule

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"testing"
)

type stubFingerprinter struct {
	sigs map[string]ImageSignature
	errs map[string]error
}

func (s *stubFingerprinter) Fingerprint(_ context.Context, rawURL string) (ImageSignature, error) {
	if err, ok := s.errs[rawURL]; ok {
		return ImageSignature{}, err
	}
	if sig, ok := s.sigs[rawURL]; ok {
		return sig, nil
	}
	return ImageSignature{}, errors.New("not found")
}

func makeGradientImage(w, h int, invert bool) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			v := uint8((x*200)/w + (y*55)/h)
			if invert {
				v = 255 - v
			}
			img.Set(x, y, color.RGBA{R: v, G: v, B: v, A: 255})
		}
	}
	return img
}

func TestComputeImageSignatureMatchesResizedCopy(t *testing.T) {
	large := makeGradientImage(1532, 2048, false)
	resized := makeGradientImage(766, 1024, false)
	different := makeGradientImage(1532, 2048, true)

	sigLarge, err := ComputeImageSignature(large)
	if err != nil {
		t.Fatalf("large: %v", err)
	}
	sigResized, err := ComputeImageSignature(resized)
	if err != nil {
		t.Fatalf("resized: %v", err)
	}
	sigDiff, err := ComputeImageSignature(different)
	if err != nil {
		t.Fatalf("diff: %v", err)
	}

	if !IsVisuallyDuplicate(sigLarge, sigResized) {
		t.Fatalf("expected resized copy to match: %+v vs %+v", sigLarge, sigResized)
	}
	if IsVisuallyDuplicate(sigLarge, sigDiff) {
		t.Fatalf("expected inverted image not to match: %+v vs %+v", sigLarge, sigDiff)
	}
	// Shared blue frame + distinct center content (modeled on flyer vs timetable) must not collide.
	sigDistinctFrame := sigLarge
	for y := 4; y < 12; y++ {
		for x := 4; x < 12; x++ {
			sigDistinctFrame.Grid[y][x] += 40
		}
	}
	if IsVisuallyDuplicate(sigLarge, sigDistinctFrame) {
		t.Fatalf("expected distinct center content with similar border not to match")
	}
}

func TestVisualDedupeScheduleBytesRemovesCrossSourceDuplicateAndKeepsDistinct(t *testing.T) {
	raw := []byte(`{
  "schema_version": 1,
  "timezone": "Asia/Tokyo",
  "sessions": [
    {
      "id": "20261129-1",
      "image_url": "https://attachments.timetreeapp.com/dup-flyer.jpg",
      "images": [
        {"type": "flyer", "label": "告知画像", "url": "https://pbs.twimg.com/media/flyer.jpg?name=large"},
        {"type": "flyer", "label": "告知画像", "url": "https://attachments.timetreeapp.com/dup-flyer.jpg"},
        {"type": "flyer", "label": "告知画像", "url": "https://attachments.timetreeapp.com/timetable.jpg"},
        {"type": "flyer", "label": "告知画像", "url": "https://attachments.timetreeapp.com/unreachable.jpg"}
      ]
    }
  ]
}`)

	fp := &stubFingerprinter{
		sigs: map[string]ImageSignature{
			"https://pbs.twimg.com/media/flyer.jpg?name=large":    {Hash: 0xad0ca323bfcb4f1f, AspectRatio: 0.748, Valid: true},
			"https://attachments.timetreeapp.com/dup-flyer.jpg":   {Hash: 0x2d0ca3a2bbcbd71f, AspectRatio: 0.748, Valid: true}, // hamming=7
			"https://attachments.timetreeapp.com/timetable.jpg":   {Hash: 0xa5eaeaeaebcb5d1f, AspectRatio: 0.748, Valid: true}, // hamming=18
		},
		errs: map[string]error{
			"https://attachments.timetreeapp.com/unreachable.jpg": errors.New("timeout"),
		},
	}

	out, removed, err := VisualDedupeScheduleBytes(context.Background(), raw, fp)
	if err != nil {
		t.Fatalf("VisualDedupeScheduleBytes: %v", err)
	}
	if removed != 1 {
		t.Fatalf("expected 1 removed duplicate, got %d", removed)
	}

	var doc struct {
		Sessions []struct {
			ImageURL string `json:"image_url"`
			Images   []struct {
				URL string `json:"url"`
			} `json:"images"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(out, &doc); err != nil {
		t.Fatal(err)
	}
	got := doc.Sessions[0]
	if len(got.Images) != 3 {
		t.Fatalf("expected 3 images (X flyer + timetable + fail-open unreachable), got %d: %+v", len(got.Images), got.Images)
	}
	if got.ImageURL != "https://pbs.twimg.com/media/flyer.jpg?name=large" {
		t.Fatalf("expected image_url updated to kept representative, got %s", got.ImageURL)
	}
}
