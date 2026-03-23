package imaging

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// CaptureScreenshot takes a macOS screenshot and returns a JPEG path.
// The caller is responsible for removing the returned file.
func CaptureScreenshot(quality int) (string, func(), error) {
	tmpFile := filepath.Join(os.TempDir(), fmt.Sprintf("claude-screenshot-%d.png", time.Now().UnixMilli()))

	cmd := exec.Command("/usr/sbin/screencapture", "-x", tmpFile)
	if err := cmd.Run(); err != nil {
		return "", nil, fmt.Errorf("screencapture: %w", err)
	}

	jpegFile := strings.TrimSuffix(tmpFile, ".png") + ".jpg"

	if err := pngToJPEG(tmpFile, jpegFile, quality); err != nil {
		os.Remove(tmpFile)
		return "", nil, fmt.Errorf("jpeg conversion: %w", err)
	}
	os.Remove(tmpFile)

	cleanup := func() { os.Remove(jpegFile) }
	return jpegFile, cleanup, nil
}

func pngToJPEG(pngPath, jpegPath string, quality int) error {
	src, err := os.Open(pngPath)
	if err != nil {
		return fmt.Errorf("open png: %w", err)
	}
	defer src.Close()

	img, err := png.Decode(src)
	if err != nil {
		return fmt.Errorf("decode png: %w", err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	log.Printf("[screenshot] original resolution: %dx%d", w, h)
	if w > 2560 {
		img = scaleDown(img, 2560)
		bounds = img.Bounds()
		log.Printf("[screenshot] scaled to: %dx%d", bounds.Dx(), bounds.Dy())
	}

	dst, err := os.Create(jpegPath)
	if err != nil {
		return fmt.Errorf("create jpeg: %w", err)
	}
	defer dst.Close()

	return jpeg.Encode(dst, img, &jpeg.Options{Quality: quality})
}

func scaleDown(img image.Image, maxWidth int) image.Image {
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	newW := maxWidth
	newH := h * maxWidth / w

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	for y := 0; y < newH; y++ {
		srcY := y * h / newH + bounds.Min.Y
		for x := 0; x < newW; x++ {
			srcX := x * w / newW + bounds.Min.X
			dst.Set(x, y, img.At(srcX, srcY))
		}
	}
	return dst
}
