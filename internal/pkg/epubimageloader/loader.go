// Package epubimageloader provides shared types and helpers for loading
// images from various sources (directory, CBZ/ZIP, CBR/RAR, PDF).
// Both epubimageprocessor and epubimagepassthrough use this package
// to avoid code duplication in file discovery and image decoding.
package epubimageloader

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"path/filepath"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/font/gofont/gomonobold"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// maxImageDim is the maximum allowed pixel dimension for decoded images,
// protecting against decompression bomb attacks.
const MaxImageDim = 20000

// ErrNoImagesFound is returned when no supported images are found in the input.
var ErrNoImagesFound = errors.New("no images found")

// Task represents a single image to be processed.
type Task struct {
	Id    int
	Image image.Image
	Path  string
	Name  string
	Error error
}

// DecodeBounded decodes an image but rejects dimensions exceeding maxDim
// to prevent decompression bomb attacks.
func DecodeBounded(r io.Reader, maxDim int) (image.Image, string, error) {
	buf, err := io.ReadAll(r)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read image data: %w", err)
	}
	if len(buf) == 0 {
		return nil, "", fmt.Errorf("empty image data")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(buf))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image config: %w", err)
	}

	if cfg.Width > maxDim || cfg.Height > maxDim {
		return nil, "", fmt.Errorf("image too large: %dx%d (max %d)", cfg.Width, cfg.Height, maxDim)
	}

	img, _, err := image.Decode(bytes.NewReader(buf))
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}
	return img, format, nil
}

// IsSupportedImage checks whether the file at path has a supported image extension.
// When includeWebpTiff is true, .webp and .tiff are also accepted (processor mode);
// when false, only .jpg/.jpeg/.png are accepted (passthrough mode).
func IsSupportedImage(path string, includeWebpTiff bool) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png":
		return !strings.HasPrefix(filepath.Base(path), ".")
	case ".webp", ".tiff":
		return includeWebpTiff && !strings.HasPrefix(filepath.Base(path), ".")
	}
	return false
}

// CorruptedImage creates a placeholder image showing the file name and
// a "is corrupted!" message, used when image decoding fails.
func CorruptedImage(path, name string) image.Image {
	var w, h float64 = 1200, 1920
	f, _ := truetype.Parse(gomonobold.TTF)
	face := truetype.NewFace(f, &truetype.Options{Size: 64, DPI: 72})
	txt := name
	if path != "" {
		txt += "\nin " + filepath.Clean(path)
	}
	txt += "\nis corrupted!"

	g := gg.NewContext(int(w), int(h))
	g.SetColor(color.White)
	g.Clear()
	g.SetColor(color.Black)
	g.DrawRoundedRectangle(0, 0, w, h, 0.5)
	g.SetLineWidth(6)
	g.Stroke()
	g.DrawRoundedRectangle(0, 0, 480, 640, 0.5)
	g.SetFontFace(face)
	g.DrawStringWrapped(txt, w/2, h/2, 0.5, 0.5, 640, 1.5, gg.AlignCenter)
	return g.Image()
}
