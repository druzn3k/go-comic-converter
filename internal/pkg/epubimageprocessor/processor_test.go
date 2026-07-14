package epubimageprocessor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/source"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
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
			_, _, err := epubimageloader.DecodeBounded(bytes.NewReader(data), tt.maxDim)
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
	_, _, err := epubimageloader.DecodeBounded(bytes.NewReader([]byte{0xff, 0xd8, 0xff, 0xe0}), 10000)
	if err == nil {
		t.Error("expected error for truncated JPEG")
	}
}

func TestDecodeBoundedEmpty(t *testing.T) {
	_, _, err := epubimageloader.DecodeBounded(bytes.NewReader([]byte{}), 10000)
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

// ---- Mock source + helpers for processor tests ----

type mockSource struct {
	tasks []epubimageloader.Task
	name  string
}

func (m *mockSource) Load(ctx context.Context) (<-chan epubimageloader.Task, int, error) {
	ch := make(chan epubimageloader.Task, len(m.tasks))
	go func() {
		defer close(ch)
		for _, t := range m.tasks {
			select {
			case ch <- t:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, len(m.tasks), nil
}

func (m *mockSource) Name() string { return m.name }

func makeTask(id int, w, h int) epubimageloader.Task {
	return epubimageloader.Task{
		Id:    id,
		Path:  "/test",
		Name:  fmt.Sprintf("page%d.jpg", id),
		Image: image.NewRGBA(image.Rect(0, 0, w, h)),
	}
}

func newTestProcessor(t *testing.T, opts epuboptions.EPUBOptions, src source.Source) *ePUBImageProcessor {
	t.Helper()
	if opts.Output == "" {
		opts.Output = filepath.Join(t.TempDir(), "output")
	}
	if opts.Workers == 0 {
		opts.Workers = 1
	}
	p := &ePUBImageProcessor{EPUBOptions: opts}
	p.SetTestSource(src)
	return p
}

func TestLoadSingleImage(t *testing.T) {
	src := &mockSource{
		name:  "test-source",
		tasks: []epubimageloader.Task{makeTask(1, 400, 600)},
	}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1600},
		},
	}
	p := newTestProcessor(t, opts, src)
	images, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0].Id != 1 {
		t.Errorf("expected Id=1, got %d", images[0].Id)
	}
	if images[0].Format != "jpeg" {
		t.Errorf("expected Format=jpeg, got %s", images[0].Format)
	}
}

func TestLoadDoublePageAutoSplit(t *testing.T) {
	src := &mockSource{
		name:  "test-source",
		tasks: []epubimageloader.Task{makeTask(1, 800, 600)},
	}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			Format:                    "jpeg",
			AutoSplitDoublePage:       true,
			KeepDoublePageIfSplit:     true,
			View:                      epuboptions.View{Width: 1200, Height: 1600},
		},
	}
	p := newTestProcessor(t, opts, src)
	images, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	// 3 outputs: original double page (part=0) + left split (part=1) + right split (part=2)
	if len(images) != 3 {
		t.Fatalf("expected 3 images (original + 2 splits), got %d", len(images))
	}
	// Verify part numbering: 0, 1, 2
	parts := make(map[int]bool)
	for _, img := range images {
		parts[img.Part] = true
	}
	if !parts[0] || !parts[1] || !parts[2] {
		t.Errorf("expected parts [0,1,2], got %v", parts)
	}
}

func TestLoadDoublePageNoSplit(t *testing.T) {
	src := &mockSource{
		name:  "test-source",
		tasks: []epubimageloader.Task{makeTask(1, 800, 600)},
	}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			Format:              "jpeg",
			AutoSplitDoublePage: false,
			View:                epuboptions.View{Width: 1200, Height: 1600},
		},
	}
	p := newTestProcessor(t, opts, src)
	images, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image (no split), got %d", len(images))
	}
	if images[0].Part != 0 {
		t.Errorf("expected part=0, got %d", images[0].Part)
	}
}

func TestLoadCorruptedImage(t *testing.T) {
	src := &mockSource{
		name: "test-source",
		tasks: []epubimageloader.Task{{
			Id:    1,
			Path:  "/test",
			Name:  "corrupted.jpg",
			Image: image.NewRGBA(image.Rect(0, 0, 1, 1)),
			Error: fmt.Errorf("simulated decode error"),
		}},
	}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1600},
		},
	}
	p := newTestProcessor(t, opts, src)
	images, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() should not fail on corrupted images: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image (placeholder), got %d", len(images))
	}
	if images[0].Error == nil {
		t.Error("expected error to be preserved on corrupted image")
	}
}

func TestLoadContextCancel(t *testing.T) {
	src := &mockSource{
		name:  "test-source",
		tasks: []epubimageloader.Task{makeTask(1, 400, 600)},
	}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1600},
		},
	}
	p := newTestProcessor(t, opts, src)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := p.Load(ctx)
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
}

func TestLoadNoImages(t *testing.T) {
	src := &mockSource{
		name:  "test-source",
		tasks: nil,
	}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			Format: "jpeg",
		},
	}
	p := newTestProcessor(t, opts, src)
	_, err := p.Load(context.Background())
	if err == nil {
		t.Fatal("expected error for no images")
	}
	if !errors.Is(err, epubimageloader.ErrNoImagesFound) {
		t.Errorf("expected ErrNoImagesFound, got: %v", err)
	}
}

func TestSetRecipe(t *testing.T) {
	src := &mockSource{
		name:  "test-source",
		tasks: []epubimageloader.Task{makeTask(1, 400, 600)},
	}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1600},
		},
	}
	p := newTestProcessor(t, opts, src)
	// An empty chain is the simplest recipe — it returns the image unchanged
	chain := filters.NewChain()
	p.SetRecipe(chain)

	images, err := p.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() with recipe chain failed: %v", err)
	}
	if len(images) != 1 {
		t.Fatalf("expected 1 image, got %d", len(images))
	}
	if images[0].Id != 1 {
		t.Errorf("expected Id=1, got %d", images[0].Id)
	}
}
