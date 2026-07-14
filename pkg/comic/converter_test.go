package comic

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

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
