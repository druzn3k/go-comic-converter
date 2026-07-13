package epubimagepassthrough

import (
	"context"
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

func createTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 5, 5))
	for x := range 5 {
		for y := range 5 {
			img.Set(x, y, color.RGBA{uint8(x * 50), uint8(y * 50), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPassthroughLoadDirSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()

	data := createTestJPEG(t)
	realImg := filepath.Join(dir, "real.jpg")
	if err := os.WriteFile(realImg, data, 0644); err != nil {
		t.Fatal(err)
	}

	symImg := filepath.Join(dir, "link.jpg")
	if err := os.Symlink(realImg, symImg); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	outsideSym := filepath.Join(dir, "outside.jpg")
	if err := os.Symlink("/etc/hostname", outsideSym); err != nil {
		t.Skipf("symlink not supported: %v", err)
	}

	opts := epuboptions.EPUBOptions{
		Input: dir,
		Image: epuboptions.Image{
			Format: "copy",
			View: epuboptions.View{
				Width:  1200,
				Height: 1920,
			},
		},
		Output: filepath.Join(t.TempDir(), "out.epub"),
	}

	p := New(opts)
	images, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() should succeed: %v", err)
	}

	if len(images) != 1 {
		t.Errorf("expected 1 real image (symlinks excluded), got %d", len(images))
		for _, img := range images {
			t.Logf("  image: %s", img.Name)
		}
	}
	if len(images) > 0 && images[0].Name != "real.jpg" {
		t.Errorf("expected real.jpg, got %s", images[0].Name)
	}
}
