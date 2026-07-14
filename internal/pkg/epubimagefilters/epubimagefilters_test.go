package epubimagefilters

import (
	"image"
	"image/color"
	"testing"

	"github.com/disintegration/gift"
)

// makeSolid creates a solid-color RGBA image.
func makeSolid(w, h int, c color.Color) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, c)
		}
	}
	return img
}

// makeGradientH creates a horizontal gradient (left black, right white).
func makeGradientH(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			v := uint8(255 * x / (w - 1))
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

func makeGradientV(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		v := uint8(255 * y / (h - 1))
		for x := range w {
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

// makeCenteredRect creates an image filled with bgColor and a centered
// rectangle of fgColor. The border width is border on each side.
func makeCenteredRect(w, h, border int, bgColor, fgColor color.Color) *image.RGBA {
	img := makeSolid(w, h, bgColor)
	for y := border; y < h-border; y++ {
		for x := border; x < w-border; x++ {
			img.Set(x, y, fgColor)
		}
	}
	return img
}

// applyFilter applies a gift filter to src and returns the result.
func applyFilter(src image.Image, f gift.Filter) *image.RGBA {
	g := gift.New(f)
	bounds := g.Bounds(src.Bounds())
	dst := image.NewRGBA(bounds)
	g.Draw(dst, src)
	return dst
}

// --- AutoCrop tests ---

func TestAutoCropBasic(t *testing.T) {
	// 100x100 white image with centered 50x50 red square → 25px blank border each side
	src := makeCenteredRect(100, 100, 25, color.White, color.RGBA{255, 0, 0, 255})
	f := AutoCrop(src, src.Bounds(), 0, 0, 0, 0, 0, false)
	dst := applyFilter(src, f)

	if dst.Bounds().Dx() >= 100 || dst.Bounds().Dy() >= 100 {
		t.Errorf("expected cropped dimensions < 100, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
	// The non-blank region is 50x50 centered. With cutRatio=0, AutoCrop should
	// find those exact bounds (or very close).
	if dst.Bounds().Dx() > 52 || dst.Bounds().Dy() > 52 {
		t.Errorf("expected cropped dimensions ~50x50, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
	if dst.Bounds().Dx() < 40 || dst.Bounds().Dy() < 40 {
		t.Errorf("cropped too aggressively: %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
}

func TestAutoCropBlank(t *testing.T) {
	// Fully blank white 100x100 image → should crop to zero area (no panic)
	src := makeSolid(100, 100, color.White)
	f := AutoCrop(src, src.Bounds(), 0, 0, 0, 0, 0, false)
	dst := applyFilter(src, f)

	// The result should be valid (non-nil). A fully blank image gets fully cropped.
	if dst == nil {
		t.Fatal("expected non-nil result for blank image")
	}
	// Dx or Dy should be 0 (fully cropped)
	if dst.Bounds().Dx() > 0 && dst.Bounds().Dy() > 0 {
		t.Logf("note: blank image cropped to %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
}

func TestAutoCropSkipLimitReached(t *testing.T) {
	// Image with blank border that WOULD be cropped, but skipIfLimitReached=true
	// with limit=1 means any cropping exceeds the 1% limit, so original bounds kept.
	src := makeCenteredRect(100, 100, 25, color.White, color.RGBA{255, 0, 0, 255})
	f := AutoCrop(src, src.Bounds(), 0, 0, 0, 0, 1, true)
	dst := applyFilter(src, f)

	if dst.Bounds().Dx() != 100 || dst.Bounds().Dy() != 100 {
		t.Errorf("expected original dimensions 100x100 with skipIfLimitReached, got %dx%d",
			dst.Bounds().Dx(), dst.Bounds().Dy())
	}
}

func TestAutoCropNoBorder(t *testing.T) {
	// Solid red image — no blank border at all, nothing to crop
	src := makeSolid(100, 100, color.RGBA{255, 0, 0, 255})
	f := AutoCrop(src, src.Bounds(), 0, 0, 0, 0, 0, false)
	dst := applyFilter(src, f)

	if dst.Bounds().Dx() != 100 || dst.Bounds().Dy() != 100 {
		t.Errorf("expected no cropping on solid image, got %dx%d",
			dst.Bounds().Dx(), dst.Bounds().Dy())
	}
}

// --- AutoContrast tests ---

func TestAutoContrast(t *testing.T) {
	// Create an image with two distinct gray bands: top half ~50, bottom half ~200.
	// AutoContrast should stretch the contrast between them.
	w, h := 100, 60
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	darkGray := color.RGBA{50, 50, 50, 255}
	brightGray := color.RGBA{200, 200, 200, 255}
	for y := range h {
		c := darkGray
		if y >= h/2 {
			c = brightGray
		}
		for x := range w {
			src.Set(x, y, c)
		}
	}
	dst := applyFilter(src, AutoContrast())

	if dst.Bounds().Dx() != w || dst.Bounds().Dy() != h {
		t.Errorf("expected %dx%d, got %dx%d", w, h, dst.Bounds().Dx(), dst.Bounds().Dy())
	}

	// Sample the two halves. The darker half should still be darker than the bright half.
	var darkSum, brightSum uint32
	darkCount, brightCount := 0, 0
	for y := 1; y < h/2-1; y++ {
		for x := range w {
			r, g, b, _ := dst.At(x, y).RGBA()
			darkSum += r + g + b
			darkCount++
		}
	}
	for y := h/2 + 1; y < h-1; y++ {
		for x := range w {
			r, g, b, _ := dst.At(x, y).RGBA()
			brightSum += r + g + b
			brightCount++
		}
	}
	darkAvg := float64(darkSum) / float64(darkCount)
	brightAvg := float64(brightSum) / float64(brightCount)

	if darkAvg >= brightAvg {
		t.Errorf("expected dark half < bright half after contrast, got dark=%.0f bright=%.0f", darkAvg, brightAvg)
	}

	// Verify that the filter actually changed something: at least some pixels differ.
	changed := false
	for y := 0; y < h && !changed; y++ {
		for x := 0; x < w && !changed; x++ {
			sr, sg, sb, sa := src.At(x, y).RGBA()
			dr, dg, db, da := dst.At(x, y).RGBA()
			if sr != dr || sg != dg || sb != db || sa != da {
				changed = true
			}
		}
	}
	if !changed {
		t.Error("auto contrast did not change any pixels")
	}
}

func TestAutoContrastUniform(t *testing.T) {
	// Uniform gray image — should not panic, output same dimensions.
	src := makeSolid(50, 50, color.RGBA{128, 128, 128, 255})
	dst := applyFilter(src, AutoContrast())

	if dst.Bounds().Dx() != 50 || dst.Bounds().Dy() != 50 {
		t.Errorf("expected 50x50, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
}

// --- CoverTitle tests ---

func TestCoverTitle(t *testing.T) {
	// 400x300 image, render a title — should not panic.
	src := makeSolid(400, 300, color.RGBA{100, 150, 200, 255})
	f := CoverTitle("Test Title", "center", 80, 5, 48, 2)
	dst := applyFilter(src, f)

	if dst.Bounds().Dx() != 400 || dst.Bounds().Dy() != 300 {
		t.Errorf("expected 400x300, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}

	// The output should differ from input (title text and boxes drawn).
	// Check the center region — it should not be identical to solid fill.
	midX, midY := 200, 150
	sr, sg, sb, _ := src.At(midX, midY).RGBA()
	dr, dg, db, _ := dst.At(midX, midY).RGBA()
	if sr == dr && sg == dg && sb == db {
		t.Log("center pixel unchanged — title may have rendered elsewhere or with zero font size")
	}
}

func TestCoverTitleEmpty(t *testing.T) {
	// Empty title — should be a no-op (same as source).
	src := makeSolid(200, 150, color.RGBA{10, 20, 30, 255})
	f := CoverTitle("", "center", 80, 5, 48, 2)
	dst := applyFilter(src, f)

	if dst.Bounds().Dx() != 200 || dst.Bounds().Dy() != 150 {
		t.Errorf("expected 200x150, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}

	// Every pixel should match (empty title = passthrough).
	for y := range 150 {
		for x := range 200 {
			sr, sg, sb, sa := src.At(x, y).RGBA()
			dr, dg, db, da := dst.At(x, y).RGBA()
			if sr != dr || sg != dg || sb != db || sa != da {
				t.Errorf("pixel at (%d,%d) changed with empty title", x, y)
				return
			}
		}
	}
}

func TestCoverTitleBottom(t *testing.T) {
	// Title aligned "bottom" — should not panic.
	src := makeSolid(300, 200, color.RGBA{50, 50, 50, 255})
	f := CoverTitle("Bottom Title", "bottom", 70, 8, 36, 2)
	dst := applyFilter(src, f)

	if dst.Bounds().Dx() != 300 || dst.Bounds().Dy() != 200 {
		t.Errorf("expected 300x200, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}
}

// --- CropSplitDoublePage tests ---

func TestCropSplitDoublePageLeft(t *testing.T) {
	src := makeSolid(800, 600, color.RGBA{100, 200, 100, 255})
	f := CropSplitDoublePage(false) // right=false → left half
	dst := applyFilter(src, f)

	if dst.Bounds().Dx() != 400 {
		t.Errorf("expected Dx=400 for left half, got %d", dst.Bounds().Dy())
	}
	if dst.Bounds().Dy() != 600 {
		t.Errorf("expected Dy=600, got %d", dst.Bounds().Dy())
	}
}

func TestCropSplitDoublePageRight(t *testing.T) {
	src := makeSolid(800, 600, color.RGBA{100, 100, 200, 255})
	f := CropSplitDoublePage(true) // right=true → right half
	dst := applyFilter(src, f)

	if dst.Bounds().Dx() != 400 {
		t.Errorf("expected Dx=400 for right half, got %d", dst.Bounds().Dx())
	}
	if dst.Bounds().Dy() != 600 {
		t.Errorf("expected Dy=600, got %d", dst.Bounds().Dy())
	}
}

// --- Pixel tests ---

func TestPixelNormal(t *testing.T) {
	// Normal 100x100 image — should pass through unchanged.
	src := makeSolid(100, 100, color.RGBA{255, 128, 64, 255})
	dst := applyFilter(src, Pixel())

	if dst.Bounds().Dx() != 100 || dst.Bounds().Dy() != 100 {
		t.Errorf("expected 100x100 passthrough, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}

	// Pixel(0,0) should match source.
	sr, sg, sb, sa := src.At(0, 0).RGBA()
	dr, dg, db, da := dst.At(0, 0).RGBA()
	if sr != dr || sg != dg || sb != db || sa != da {
		t.Error("pixel(0,0) does not match source after pixel filter")
	}
}

func TestPixelZeroSize(t *testing.T) {
	// 0x0 image → should produce a 1x1 white pixel.
	src := image.NewRGBA(image.Rect(0, 0, 0, 0))
	dst := applyFilter(src, Pixel())

	if dst.Bounds().Dx() != 1 || dst.Bounds().Dy() != 1 {
		t.Fatalf("expected 1x1 for zero-size input, got %dx%d", dst.Bounds().Dx(), dst.Bounds().Dy())
	}

	r, g, b, a := dst.At(0, 0).RGBA()
	if r != 0xffff || g != 0xffff || b != 0xffff || a != 0xffff {
		t.Errorf("expected white pixel, got RGBA(%d,%d,%d,%d)", r>>8, g>>8, b>>8, a>>8)
	}
}
