package output

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func TestViewerTemplateParses(t *testing.T) {
	// The embedded template must parse without error.
	if viewerTemplate == "" {
		t.Fatal("viewerTemplate is empty")
	}
	if !strings.Contains(viewerTemplate, "<!DOCTYPE html>") {
		t.Fatal("viewerTemplate missing DOCTYPE")
	}
	if !strings.Contains(viewerTemplate, "{{.Title}}") {
		t.Fatal("viewerTemplate missing Title field")
	}
	if !strings.Contains(viewerTemplate, "{{range .Pages}}") {
		t.Fatal("viewerTemplate missing Pages range")
	}
}

func TestHTMLWriterInterface(t *testing.T) {
	w := HTMLWriter{}
	if w.Format() != "html" {
		t.Errorf("Format() = %q, want %q", w.Format(), "html")
	}
	if w.Extension() != ".html" {
		t.Errorf("Extension() = %q, want %q", w.Extension(), ".html")
	}
	if w.SupportsPartSplit() {
		t.Error("SupportsPartSplit() should return false")
	}
}

func TestHTMLWriterWrite(t *testing.T) {
	ctx := context.Background()

	// Create a temp directory and output path.
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test.html")

	// Create a temp ZIP storage at the path opts.ImgStorage() will use.
	storagePath := outputPath + ".tmp"
	imgWriter, err := epubzip.NewStorageImageWriter(storagePath, "jpeg")
	if err != nil {
		t.Fatalf("create storage writer: %v", err)
	}

	// Create two fake images (small JPEG-like bytes).
	fakeImage1 := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00} // JPEG header
	fakeImage2 := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00} // JPEG header

	// Add images with paths matching what EPUBImgPath() generates.
	img1 := epubimage.EPUBImage{Id: 1, Part: 0, Format: "jpeg"}
	img2 := epubimage.EPUBImage{Id: 2, Part: 0, Format: "jpeg"}

	if err := imgWriter.AddRaw(img1.EPUBImgPath(), fakeImage1); err != nil {
		t.Fatalf("add image 1: %v", err)
	}
	if err := imgWriter.AddRaw(img2.EPUBImgPath(), fakeImage2); err != nil {
		t.Fatalf("add image 2: %v", err)
	}
	if err := imgWriter.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	// Set up opts so ImgStorage() returns our temp ZIP.
	opts := epuboptions.EPUBOptions{
		Output: outputPath,
	}

	parts := []OutputPart{
		{
			Cover:      img1,
			Images:     []epubimage.EPUBImage{img2},
			PartNumber: 1,
			TotalParts: 1,
			Metadata: PartMetadata{
				Title: "Test Comic",
			},
		},
	}
	w := HTMLWriter{}
	paths, err := w.Write(ctx, parts, opts)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 output path, got %d", len(paths))
	}
	if paths[0] != outputPath {
		t.Errorf("output path = %q, want %q", paths[0], outputPath)
	}

	// Read the output HTML and verify its structure.
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	html := string(data)

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("output missing DOCTYPE")
	}
	if !strings.Contains(html, "<title>Test Comic</title>") {
		t.Error("output missing title")
	}
	// Check for the JS counter (dynamically generated, not literal "Page 1 of 2")
	if !strings.Contains(html, "' of ' + pages.length") {
		t.Error("output missing page counter logic")
	}
	// html/template escapes / as \/ in JS strings for safety (prevents </script> injection).
	// Both forms are valid JS; we check for the escaped prefix.
	if !strings.Contains(html, `data:image\/jpeg;base64,`) {
		t.Error("output missing base64 data URI")
	}

	// Verify that the base64-encoded image data is present.
	// html/template escapes / as \/ so we escape our expected encoding too.
	for _, fake := range [][]byte{fakeImage1, fakeImage2} {
		expectedEnc := base64.StdEncoding.EncodeToString(fake)
		if !strings.Contains(html, expectedEnc) && !strings.Contains(html, strings.ReplaceAll(expectedEnc, "/", `\/`)) {
			t.Error("output does not contain expected base64-encoded image data")
		}
	}
}

func TestHTMLWriterWriteEmptyParts(t *testing.T) {
	w := HTMLWriter{}
	opts := epuboptions.EPUBOptions{}
	_, err := w.Write(context.Background(), nil, opts)
	if err == nil {
		t.Error("expected error for empty parts")
	}
}

func TestHTMLWriterContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "cancel.html")

	// Create storage at the path opts.ImgStorage() will use.
	storagePath := outputPath + ".tmp"
	imgWriter, err := epubzip.NewStorageImageWriter(storagePath, "jpeg")
	if err != nil {
		t.Fatalf("create storage writer: %v", err)
	}
	img := epubimage.EPUBImage{Id: 1, Part: 0, Format: "jpeg"}
	if err := imgWriter.AddRaw(img.EPUBImgPath(), []byte{0xff, 0xd8, 0xff}); err != nil {
		t.Fatalf("add image: %v", err)
	}
	if err := imgWriter.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before write

	opts := epuboptions.EPUBOptions{
		Output: outputPath,
	}
	parts := []OutputPart{
		{
			Cover:      img,
			Images:     []epubimage.EPUBImage{},
			PartNumber: 1,
			TotalParts: 1,
		},
	}

	w := HTMLWriter{}
	_, err = w.Write(ctx, parts, opts)
	if err == nil {
		t.Error("expected context cancellation error")
	}
}
