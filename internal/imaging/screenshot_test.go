package imaging

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func TestScaleDown(t *testing.T) {
	// Create a 100x50 test image
	src := image.NewRGBA(image.Rect(0, 0, 100, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 100; x++ {
			src.Set(x, y, color.RGBA{uint8(x), uint8(y), 128, 255})
		}
	}

	// Scale down to maxWidth 50
	result := scaleDown(src, 50)
	bounds := result.Bounds()

	if bounds.Dx() != 50 {
		t.Errorf("expected width 50, got %d", bounds.Dx())
	}
	if bounds.Dy() != 25 {
		t.Errorf("expected height 25, got %d", bounds.Dy())
	}
}

func TestScaleDownPreservesAspectRatio(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 3840, 2160))
	result := scaleDown(src, 2560)
	bounds := result.Bounds()

	expectedH := 2160 * 2560 / 3840
	if bounds.Dx() != 2560 {
		t.Errorf("expected width 2560, got %d", bounds.Dx())
	}
	if bounds.Dy() != expectedH {
		t.Errorf("expected height %d, got %d", expectedH, bounds.Dy())
	}
}

func TestScaleDownSmallImage(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 5))
	result := scaleDown(src, 5)
	bounds := result.Bounds()

	if bounds.Dx() != 5 {
		t.Errorf("expected width 5, got %d", bounds.Dx())
	}
	if bounds.Dy() != 2 {
		t.Errorf("expected height 2 (truncated), got %d", bounds.Dy())
	}
}

func TestPngToJPEG(t *testing.T) {
	// Create a temporary PNG file
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "test.png")
	jpegPath := filepath.Join(tmpDir, "test.jpg")

	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{255, 0, 0, 255})
		}
	}

	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatalf("failed to create png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("failed to encode png: %v", err)
	}
	f.Close()

	err = pngToJPEG(pngPath, jpegPath, 70)
	if err != nil {
		t.Fatalf("pngToJPEG failed: %v", err)
	}

	// Verify JPEG exists and has content
	info, err := os.Stat(jpegPath)
	if err != nil {
		t.Fatalf("jpeg file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("jpeg file is empty")
	}
}

func TestPngToJPEGWithLargeImage(t *testing.T) {
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "large.png")
	jpegPath := filepath.Join(tmpDir, "large.jpg")

	// Create a wide image that triggers scaling
	img := image.NewRGBA(image.Rect(0, 0, 3000, 1000))

	f, err := os.Create(pngPath)
	if err != nil {
		t.Fatalf("failed to create png: %v", err)
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		t.Fatalf("failed to encode png: %v", err)
	}
	f.Close()

	err = pngToJPEG(pngPath, jpegPath, 70)
	if err != nil {
		t.Fatalf("pngToJPEG failed: %v", err)
	}

	info, err := os.Stat(jpegPath)
	if err != nil {
		t.Fatalf("jpeg file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Error("jpeg file is empty")
	}
}

func TestPngToJPEGInvalidPath(t *testing.T) {
	err := pngToJPEG("/nonexistent/path.png", "/tmp/out.jpg", 70)
	if err == nil {
		t.Error("expected error for invalid png path")
	}
}

func TestPngToJPEGInvalidPng(t *testing.T) {
	tmpDir := t.TempDir()
	badPng := filepath.Join(tmpDir, "bad.png")
	if err := os.WriteFile(badPng, []byte("not a png"), 0644); err != nil {
		t.Fatalf("failed to write bad file: %v", err)
	}

	err := pngToJPEG(badPng, filepath.Join(tmpDir, "out.jpg"), 70)
	if err == nil {
		t.Error("expected error for invalid png content")
	}
}
