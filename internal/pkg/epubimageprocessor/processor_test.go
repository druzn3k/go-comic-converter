package epubimageprocessor

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

// createTestJPEG creates a JPEG image of the given dimensions.
func createTestJPEG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDecodeBounded(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		height  int
		maxDim  int
		wantErr bool
	}{
		{"normal size", 100, 100, 10000, false},
		{"at boundary", 100, 100, 100, false},
		{"exceeds width", 101, 100, 100, true},
		{"exceeds height", 100, 101, 100, true},
		{"exceeds both", 500, 500, 100, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := createTestJPEG(t, tt.width, tt.height)
			_, _, err := decodeBounded(bytes.NewReader(data), tt.maxDim)
			if tt.wantErr && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestDecodeBoundedTruncated(t *testing.T) {
	_, _, err := decodeBounded(bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xe0}), 10000)
	if err == nil {
		t.Error("expected error for truncated JPEG")
	}
}

func TestDecodeBoundedEmpty(t *testing.T) {
	_, _, err := decodeBounded(bytes.NewReader([]byte{}), 10000)
	if err == nil {
		t.Error("expected error for empty input")
	}
}

func TestLoadDryMode(t *testing.T) {
	dir := t.TempDir()
	data := createTestJPEG(t, 10, 10)
	imgPath := filepath.Join(dir, "test.jpg")
	if err := os.WriteFile(imgPath, data, 0644); err != nil {
		t.Fatal(err)
	}

	opts := epuboptions.EPUBOptions{
		Input: dir,
		Image: epuboptions.Image{
			Format: "jpeg",
		},
		Dry: true,
	}

	p := New(opts)
	images, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("dry Load() should succeed: %v", err)
	}
	if len(images) == 0 {
		t.Fatal("expected at least 1 image in dry mode")
	}
	if images[0].Name != "test.jpg" {
		t.Errorf("expected test.jpg, got %s", images[0].Name)
	}
}

func TestLoadDirSkipsSymlinks(t *testing.T) {
	dir := t.TempDir()

	// Create a real image file
	data := createTestJPEG(t, 10, 10)
	realImg := filepath.Join(dir, "real.jpg")
	if err := os.WriteFile(realImg, data, 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to it
	symImg := filepath.Join(dir, "link.jpg")
	if err := os.Symlink(realImg, symImg); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to something outside the dir
	outsideSym := filepath.Join(dir, "outside.jpg")
	if err := os.Symlink("/etc/hostname", outsideSym); err != nil {
		t.Fatal(err)
	}

	opts := epuboptions.EPUBOptions{
		Input: dir,
		Image: epuboptions.Image{
			Format: "jpeg",
		},
		Dry: true,
	}

	p := New(opts)
	images, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() should succeed: %v", err)
	}

	// Should only have 1 real image, not the 2 symlinks
	if len(images) != 1 {
		t.Errorf("expected 1 real image (symlinks excluded), got %d", len(images))
		for _, img := range images {
			t.Logf("  image: %s", img.Name)
		}
	}
	if images[0].Name != "real.jpg" {
		t.Errorf("expected real.jpg, got %s", images[0].Name)
	}
}
