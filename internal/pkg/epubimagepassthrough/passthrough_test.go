package epubimagepassthrough

import (
	"archive/zip"
	"context"
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
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

func writeTestJPEG(t *testing.T, dir, name string) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestLoadImages(t *testing.T) {
	dir := t.TempDir()

	writeTestJPEG(t, dir, "page01.jpg")
	writeTestJPEG(t, dir, "page02.jpg")
	writeTestJPEG(t, dir, "page03.jpg")

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

	if len(images) != 3 {
		t.Fatalf("expected 3 images, got %d", len(images))
	}

	for i, img := range images {
		if img.Format != "jpeg" {
			t.Errorf("image %d: expected Format=jpeg, got %q", i, img.Format)
		}
	}

	// First image (id == 0) gets Raw decoded
	if images[0].Raw == nil {
		t.Error("expected first image to have Raw set")
	}

	// Subsequent images have Raw nil
	for i := 1; i < len(images); i++ {
		if images[i].Raw != nil {
			t.Errorf("image %d: expected Raw=nil, got non-nil", i)
		}
	}
}

func TestLoadNoImages(t *testing.T) {
	dir := t.TempDir()

	opts := epuboptions.EPUBOptions{
		Input: dir,
		Image: epuboptions.Image{
			Format: "jpeg",
			View: epuboptions.View{
				Width:  1200,
				Height: 1920,
			},
		},
		Output: filepath.Join(t.TempDir(), "out.epub"),
	}

	p := New(opts)
	_, err := p.Load(context.Background())
	if err == nil {
		t.Fatal("expected error from empty dir")
	}
	// ErrNoImagesFound from epubimageloader
	if err.Error() != "no images found" {
		t.Errorf("expected 'no images found' error, got: %v", err)
	}
}

func TestCoverTitleData(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))

	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			Quality: 80,
		},
	}

	p := New(opts)

	coverOpts := epubimageprocessor.CoverTitleDataOptions{
		Src:         src,
		Name:        "title",
		Text:        "Test Title",
		Align:       "center",
		PctWidth:    80,
		PctMargin:   5,
		MaxFontSize: 24,
		BorderSize:  2,
	}

	img, err := p.CoverTitleData(coverOpts)
	if err != nil {
		t.Fatalf("CoverTitleData() should succeed: %v", err)
	}

	if img.Data == nil {
		t.Error("expected non-nil image data")
	}
}

func TestIsSupportedImage(t *testing.T) {
	p := New(epuboptions.EPUBOptions{})

	tests := []struct {
		path     string
		expected bool
	}{
		{"image.jpg", true},
		{"image.jpeg", true},
		{"image.png", true},
		{"image.JPG", true},
		{"image.JPEG", true},
		{"image.PNG", true},
		{".hidden.jpg", false},
		{"image.gif", false},
		{"image.webp", false},
		{"image.tiff", false},
		{"noext", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := p.(ePUBImagePassthrough).isSupportedImage(tt.path)
			if result != tt.expected {
				t.Errorf("isSupportedImage(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestLoadCbz(t *testing.T) {
	dir := t.TempDir()
	cbzPath := filepath.Join(dir, "test.cbz")

	// Create a CBZ (zip) file with JPEG images
	fh, err := os.Create(cbzPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(fh)

	for _, name := range []string{"page01.jpg", "page02.jpg"} {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		img := image.NewRGBA(image.Rect(0, 0, 50, 50))
		if err := jpeg.Encode(w, img, &jpeg.Options{Quality: 80}); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	fh.Close()

	opts := epuboptions.EPUBOptions{
		Input: cbzPath,
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

	if len(images) != 2 {
		t.Fatalf("expected 2 images, got %d", len(images))
	}

	for i, img := range images {
		if img.Format != "jpeg" {
			t.Errorf("image %d: expected Format=jpeg, got %q", i, img.Format)
		}
	}
}
