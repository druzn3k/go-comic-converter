package comic

import (
	"context"
	"errors"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func TestConverterNew(t *testing.T) {
	opts := Options{
		Output: filepath.Join(t.TempDir(), "test.epub"),
		Image: epuboptions.Image{
			Format:    "jpeg",
			GrayScale: false,
			HasCover:  false,
			Resize:    false,
			View:      epuboptions.View{Width: 1200, Height: 1920},
		},
	}
	c := New(opts)
	if c == nil {
		t.Fatal("expected non-nil Converter")
	}
}

func TestConverterNewPassthrough(t *testing.T) {
	opts := Options{
		Output: filepath.Join(t.TempDir(), "test.epub"),
		Image: epuboptions.Image{
			Format:    "copy",
			GrayScale: false,
			HasCover:  false,
			Resize:    false,
		},
	}
	c := New(opts)
	if c == nil {
		t.Fatal("expected non-nil Converter")
	}
}

func TestConverterConvertNoInput(t *testing.T) {
	opts := Options{
		Output: filepath.Join(t.TempDir(), "test.epub"),
	}
	c := New(opts)
	err := c.Convert(context.Background())
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestErrImageCorrupted(t *testing.T) {
	if ErrImageCorrupted == nil {
		t.Fatal("ErrImageCorrupted sentinel should be non-nil")
	}
}

func TestErrImageCorruptedSentinel(t *testing.T) {
	sentinel := ErrImageCorrupted
	if !errors.Is(sentinel, ErrImageCorrupted) {
		t.Error("ErrImageCorrupted should match itself via errors.Is")
	}
}

func TestConverterFormatDetection(t *testing.T) {
	opts := Options{
		Output:       filepath.Join(t.TempDir(), "test"),
		OutputFormat: "all",
	}
	c := New(opts)
	if c.opts.OutputFormat != "all" {
		t.Errorf("expected OutputFormat 'all', got %q", c.opts.OutputFormat)
	}
}

func TestNewWithRecipe(t *testing.T) {
	opts := Options{
		Output: filepath.Join(t.TempDir(), "test.epub"),
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1920},
		},
	}
	chain := filters.NewChain()
	c := NewWithRecipe(opts, chain)
	if c == nil {
		t.Fatal("expected non-nil Converter from NewWithRecipe")
	}
	if c.chain != chain {
		t.Error("expected chain to be set on converter")
	}
}

func TestSetRecipe(t *testing.T) {
	opts := Options{
		Output: filepath.Join(t.TempDir(), "test.epub"),
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1920},
		},
	}
	c := New(opts)
	chain := filters.NewChain()
	// Must not panic
	c.SetRecipe(chain)
	if c.chain != chain {
		t.Error("expected chain to be set via SetRecipe")
	}
	// Setting nil must not panic
	c.SetRecipe(nil)
	if c.chain != nil {
		t.Error("expected chain to be nil after SetRecipe(nil)")
	}
}

func TestBuildOutputParts(t *testing.T) {
	opts := Options{
		Output:  filepath.Join(t.TempDir(), "test.epub"),
		Title:   "My Title",
		Author:  "Test Author",
		Series:  "Test Series",
		Number:  "42",
		Summary: "A summary",
		Genre:   "Action",
		Manga:   true,
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1920},
		},
	}
	c := New(opts)

	parts := []Part{
		{
			Cover:  epubimage.EPUBImage{Name: "cover.jpg", Format: "jpeg"},
			Images: []epubimage.EPUBImage{{Name: "page1.jpg", Format: "jpeg"}},
		},
		{
			Cover:  epubimage.EPUBImage{Name: "cover2.jpg", Format: "jpeg"},
			Images: []epubimage.EPUBImage{{Name: "page2.jpg", Format: "jpeg"}, {Name: "page3.jpg", Format: "jpeg"}},
		},
	}

	oparts := c.buildOutputParts(parts)
	if len(oparts) != 2 {
		t.Fatalf("expected 2 output parts, got %d", len(oparts))
	}

	md := oparts[0].Metadata
	if md.Title != "My Title" {
		t.Errorf("expected Title 'My Title', got %q", md.Title)
	}
	if md.Author != "Test Author" {
		t.Errorf("expected Author 'Test Author', got %q", md.Author)
	}
	if md.Series != "Test Series" {
		t.Errorf("expected Series 'Test Series', got %q", md.Series)
	}
	if md.Number != "42" {
		t.Errorf("expected Number '42', got %q", md.Number)
	}
	if md.Manga != "Yes" {
		t.Errorf("expected Manga 'Yes', got %q", md.Manga)
	}
	if md.Publisher != "GO Comic Converter" {
		t.Errorf("expected Publisher 'GO Comic Converter', got %q", md.Publisher)
	}
	if md.Summary != "A summary" {
		t.Errorf("expected Summary 'A summary', got %q", md.Summary)
	}
	if md.Genre != "Action" {
		t.Errorf("expected Genre 'Action', got %q", md.Genre)
	}
	if md.UID == "" {
		t.Error("expected non-empty UID")
	}

	// Verify part numbering
	if oparts[0].PartNumber != 1 {
		t.Errorf("expected PartNumber 1, got %d", oparts[0].PartNumber)
	}
	if oparts[0].TotalParts != 2 {
		t.Errorf("expected TotalParts 2, got %d", oparts[0].TotalParts)
	}
	if oparts[1].PartNumber != 2 {
		t.Errorf("expected PartNumber 2, got %d", oparts[1].PartNumber)
	}

	// Verify Manga=false maps to "No"
	opts2 := Options{
		Output: filepath.Join(t.TempDir(), "test2.epub"),
		Manga:  false,
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1920},
		},
	}
	c2 := New(opts2)
	oparts2 := c2.buildOutputParts([]Part{
		{Cover: epubimage.EPUBImage{Name: "c.jpg", Format: "jpeg"}},
	})
	if oparts2[0].Metadata.Manga != "No" {
		t.Errorf("expected Manga 'No', got %q", oparts2[0].Metadata.Manga)
	}
}

func TestHasCorruptedImages(t *testing.T) {
	errSentinel := errors.New("corrupt")

	// Clean parts — no errors
	clean := []Part{
		{
			Cover:  epubimage.EPUBImage{Name: "cover.jpg", Error: nil},
			Images: []epubimage.EPUBImage{{Name: "p1.jpg", Error: nil}},
		},
	}
	if hasCorruptedImages(clean) {
		t.Error("expected false for clean parts")
	}

	// Cover has error
	coverBad := []Part{
		{
			Cover:  epubimage.EPUBImage{Name: "cover.jpg", Error: errSentinel},
			Images: []epubimage.EPUBImage{{Name: "p1.jpg", Error: nil}},
		},
	}
	if !hasCorruptedImages(coverBad) {
		t.Error("expected true when cover has error")
	}

	// One image has error
	imgBad := []Part{
		{
			Cover: epubimage.EPUBImage{Name: "cover.jpg", Error: nil},
			Images: []epubimage.EPUBImage{
				{Name: "p1.jpg", Error: nil},
				{Name: "p2.jpg", Error: errSentinel},
			},
		},
	}
	if !hasCorruptedImages(imgBad) {
		t.Error("expected true when an image has error")
	}

	// Empty parts
	if hasCorruptedImages([]Part{}) {
		t.Error("expected false for empty parts")
	}
}

func TestConvertDry(t *testing.T) {
	dir := t.TempDir()
	// Create a small JPEG in the temp dir
	if err := writeTestJPEG(filepath.Join(dir, "page01.jpg"), 100, 100); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Input:        dir,
		Output:       filepath.Join(t.TempDir(), "test"),
		OutputFormat: "kepub",
		Dry:          true,
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1920},
		},
	}
	c := New(opts)
	if err := c.Convert(context.Background()); err != nil {
		t.Fatalf("Convert Dry failed: %v", err)
	}
}

func TestConvertEPUBDirect(t *testing.T) {
	dir := t.TempDir()
	if err := writeTestJPEG(filepath.Join(dir, "page01.jpg"), 100, 100); err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Input:        dir,
		Output:       filepath.Join(t.TempDir(), "test.epub"),
		OutputFormat: "epub",
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1920},
		},
	}
	c := New(opts)
	err := c.Convert(context.Background())
	if err == nil {
		t.Error("expected error for direct EPUB format")
	}
}

func TestConvertProgressCallback(t *testing.T) {
	dir := t.TempDir()
	if err := writeTestJPEG(filepath.Join(dir, "page01.jpg"), 100, 100); err != nil {
		t.Fatal(err)
	}

	var milestones []string
	opts := Options{
		Input:        dir,
		Output:       filepath.Join(t.TempDir(), "test"),
		OutputFormat: "kepub",
		Dry:          true,
		Image: epuboptions.Image{
			Format: "jpeg",
			View:   epuboptions.View{Width: 1200, Height: 1920},
		},
	}
	c := New(opts)
	c.ProgressCallback = func(s string) {
		milestones = append(milestones, s)
	}
	if err := c.Convert(context.Background()); err != nil {
		t.Fatalf("Convert with callback failed: %v", err)
	}
	if len(milestones) < 1 {
		t.Error("expected at least one progress milestone")
	}
}

// writeTestJPEG creates a minimal JPEG file for testing.
func writeTestJPEG(path string, width, height int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 80})
}
