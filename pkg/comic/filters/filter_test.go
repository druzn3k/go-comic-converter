package filters

import (
	"context"
	"image"
	"image/color"
	"testing"
)
func TestRegistryRegisterLookup(t *testing.T) {
	fn := func(params map[string]any) (Filter, error) { return nil, nil }
	Register("test_filter", fn)
	got, ok := Lookup("test_filter")
	if !ok {
		t.Fatal("expected to find test_filter")
	}
	if got == nil {
		t.Error("expected non-nil factory")
	}
}

func TestRegistryNames(t *testing.T) {
	Register("filter_a", func(params map[string]any) (Filter, error) { return nil, nil })
	Register("filter_b", func(params map[string]any) (Filter, error) { return nil, nil })
	names := Names()
	if len(names) < 2 {
		t.Errorf("expected at least 2 names, got %d", len(names))
	}
}

func TestChainEmpty(t *testing.T) {
	c := NewChain()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	results := c.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] != img {
		t.Error("expected same image back from empty chain")
	}
}

type identityFilter struct{}

func (f *identityFilter) Name() string { return "identity" }
func (f *identityFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	return []image.Image{img}
}

func TestChainSingleFilter(t *testing.T) {
	c := NewChain(&identityFilter{})
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	results := c.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

type doubleFilter struct{}

func (f *doubleFilter) Name() string { return "double" }
func (f *doubleFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	// Split into two halves
	b := img.Bounds()
	left := image.NewRGBA(image.Rect(0, 0, b.Dx()/2, b.Dy()))
	right := image.NewRGBA(image.Rect(0, 0, b.Dx()-b.Dx()/2, b.Dy()))
	return []image.Image{left, right}
}

func TestChainFanOut(t *testing.T) {
	c := NewChain(&doubleFilter{}, &identityFilter{})
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	results := c.Apply(context.Background(), img, FilterContext{})
	if len(results) != 2 {
		t.Fatalf("expected 2 results from fan-out, got %d", len(results))
	}
}

func TestConditionalFilterPassThrough(t *testing.T) {
	cf := &ConditionalFilter{
		Filter: &identityFilter{},
		Condition: func(fctx FilterContext) bool {
			return false // never apply
		},
	}
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	results := cf.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result from passthrough, got %d", len(results))
	}
}

func TestConditionalFilterApply(t *testing.T) {
	cf := &ConditionalFilter{
		Filter: &identityFilter{},
		Condition: func(fctx FilterContext) bool {
			return true // always apply
		},
	}
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	results := cf.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestChainApplyPreservesBounds(t *testing.T) {
	c := NewChain(&identityFilter{})
	img := image.NewRGBA(image.Rect(0, 0, 100, 200))
	fctx := FilterContext{
		OriginalBounds: img.Bounds(),
		IsDoublePage:   true,
	}
	if !fctx.IsDoublePage {
		t.Error("expected IsDoublePage to be true")
	}
	if fctx.OriginalBounds.Dx() != 100 {
		t.Errorf("expected OriginalBounds width 100, got %d", fctx.OriginalBounds.Dx())
	}

	results := c.Apply(context.Background(), img, fctx)
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestChainEmptyInput(t *testing.T) {
	c := NewChain()
	// nil input should not panic
	results := c.Apply(context.Background(), nil, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestContrastFiltersRegistration(t *testing.T) {
	for _, name := range []string{"auto_contrast", "contrast", "brightness"} {
		factory, ok := Lookup(name)
		if !ok {
			t.Fatalf("expected filter %q to be registered", name)
		}
		f, err := factory(map[string]any{"amount": 10.0})
		if err != nil {
			t.Fatalf("factory for %q returned error: %v", name, err)
		}
		if f.Name() != name {
			t.Errorf("expected Name()=%q, got %q", name, f.Name())
		}
	}
}

func TestAutoContrastFilterApply(t *testing.T) {
	f := &AutoContrastFilter{}
	img := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	results := f.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] == nil {
		t.Fatal("expected non-nil result image")
	}
	if results[0].Bounds() != img.Bounds() {
		t.Errorf("bounds changed: %v -> %v", img.Bounds(), results[0].Bounds())
	}
}

func TestContrastFilterApply(t *testing.T) {
	f := &ContrastFilter{Amount: 0}
	img := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	results := f.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] == nil {
		t.Fatal("expected non-nil result image")
	}
	if results[0].Bounds() != img.Bounds() {
		t.Errorf("bounds changed: %v -> %v", img.Bounds(), results[0].Bounds())
	}
}

func TestBrightnessFilterApply(t *testing.T) {
	f := &BrightnessFilter{Amount: 0}
	img := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	results := f.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] == nil {
		t.Fatal("expected non-nil result image")
	}
	if results[0].Bounds() != img.Bounds() {
		t.Errorf("bounds changed: %v -> %v", img.Bounds(), results[0].Bounds())
	}
}

func TestContrastFilterNoopPreservesPixels(t *testing.T) {
	img := image.NewNRGBA64(image.Rect(0, 0, 2, 2))
	img.SetNRGBA64(0, 0, color.NRGBA64{R: 65535, G: 0, B: 0, A: 65535})
	img.SetNRGBA64(1, 0, color.NRGBA64{R: 0, G: 65535, B: 0, A: 65535})
	img.SetNRGBA64(0, 1, color.NRGBA64{R: 0, G: 0, B: 65535, A: 65535})
	img.SetNRGBA64(1, 1, color.NRGBA64{R: 32768, G: 32768, B: 32768, A: 65535})

	for _, tc := range []struct {
		name string
		f    Filter
	}{
		{"contrast_zero", &ContrastFilter{Amount: 0}},
		{"brightness_zero", &BrightnessFilter{Amount: 0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			results := tc.f.Apply(context.Background(), img, FilterContext{})
			if len(results) != 1 {
				t.Fatalf("expected 1 result, got %d", len(results))
			}
			dst := results[0]
			for x := 0; x < 2; x++ {
				for y := 0; y < 2; y++ {
					or, og, ob, oa := img.At(x, y).RGBA()
					dr, dg, db, da := dst.At(x, y).RGBA()
					if or != dr || og != dg || ob != db || oa != da {
						t.Errorf("pixel (%d,%d) changed: src=%v dst=%v", x, y,
							[]uint32{or, og, ob, oa}, []uint32{dr, dg, db, da})
					}
				}
			}
		})
	}
}

func TestThresholdFilterRegistration(t *testing.T) {
	factory, ok := Lookup("threshold")
	if !ok {
		t.Fatal("expected filter \"threshold\" to be registered")
	}
	f, err := factory(map[string]any{"level": 128.0})
	if err != nil {
		t.Fatalf("factory for threshold returned error: %v", err)
	}
	if f.Name() != "threshold" {
		t.Errorf("expected Name()=%q, got %q", "threshold", f.Name())
	}
}

func TestThresholdFilterApply(t *testing.T) {
	f := &ThresholdFilter{Level: 128}
	img := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	results := f.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] == nil {
		t.Fatal("expected non-nil result image")
	}
	if results[0].Bounds() != img.Bounds() {
		t.Errorf("bounds changed: %v -> %v", img.Bounds(), results[0].Bounds())
	}
}

func TestThresholdFilterBinarization(t *testing.T) {
	// White pixel (above threshold) -> white
	// Black pixel (below threshold) -> black
	img := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	img.SetNRGBA64(0, 0, color.NRGBA64{R: 65535, G: 65535, B: 65535, A: 65535}) // white
	img.SetNRGBA64(1, 0, color.NRGBA64{R: 0, G: 0, B: 0, A: 65535})               // black

	f := &ThresholdFilter{Level: 128}
	results := f.Apply(context.Background(), img, FilterContext{})
	dst := results[0]

	// White pixel should stay white
	r, g, b, _ := dst.At(0, 0).RGBA()
	if r != 65535 || g != 65535 || b != 65535 {
		t.Errorf("white pixel expected (65535,65535,65535), got (%d,%d,%d)", r, g, b)
	}

	// Black pixel should stay black
	r, g, b, _ = dst.At(1, 0).RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("black pixel expected (0,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestThresholdFilterGrayBinarization(t *testing.T) {
	// Gray pixel > level -> white; gray pixel < level -> black
	img := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	img.SetNRGBA64(0, 0, color.NRGBA64{R: 40000, G: 40000, B: 40000, A: 65535}) // ~156
	img.SetNRGBA64(1, 0, color.NRGBA64{R: 20000, G: 20000, B: 20000, A: 65535}) // ~78

	f := &ThresholdFilter{Level: 128}
	results := f.Apply(context.Background(), img, FilterContext{})
	dst := results[0]

	// Pixel at ~156 should become white
	r, g, b, _ := dst.At(0, 0).RGBA()
	if r != 65535 || g != 65535 || b != 65535 {
		t.Errorf("gray-above expected white (65535,65535,65535), got (%d,%d,%d)", r, g, b)
	}

	// Pixel at ~78 should become black
	r, g, b, _ = dst.At(1, 0).RGBA()
	if r != 0 || g != 0 || b != 0 {
		t.Errorf("gray-below expected black (0,0,0), got (%d,%d,%d)", r, g, b)
	}
}

func TestDuotoneFilterRegistration(t *testing.T) {
	factory, ok := Lookup("duotone")
	if !ok {
		t.Fatal("expected filter \"duotone\" to be registered")
	}
	f, err := factory(map[string]any{"dark": "000", "light": "FFF"})
	if err != nil {
		t.Fatalf("factory for duotone returned error: %v", err)
	}
	if f.Name() != "duotone" {
		t.Errorf("expected Name()=%q, got %q", "duotone", f.Name())
	}
}

func TestDuotoneFilterApply(t *testing.T) {
	f := &DuotoneFilter{Dark: "000", Light: "FFF"}
	img := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	results := f.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] == nil {
		t.Fatal("expected non-nil result image")
	}
	if results[0].Bounds() != img.Bounds() {
		t.Errorf("bounds changed: %v -> %v", img.Bounds(), results[0].Bounds())
	}
}

func TestDuotoneFilterSepia(t *testing.T) {
	img := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	img.SetNRGBA64(0, 0, color.NRGBA64{R: 65535, G: 65535, B: 65535, A: 65535}) // white
	img.SetNRGBA64(1, 0, color.NRGBA64{R: 0, G: 0, B: 0, A: 65535})               // black

	f := &DuotoneFilter{Dark: "222", Light: "EEE"}
	results := f.Apply(context.Background(), img, FilterContext{})
	dst := results[0]

	// Light pixel (>0.5 luminance) should map to light color (0xEE = 238)
	r, g, b, _ := dst.At(0, 0).RGBA()
	expectedLight := uint32(0xEE * 257)
	if r != expectedLight || g != expectedLight || b != expectedLight {
		t.Errorf("light pixel expected (%d,%d,%d), got (%d,%d,%d)", expectedLight, expectedLight, expectedLight, r, g, b)
	}

	// Dark pixel (<=0.5 luminance) should map to dark color (0x22 = 34)
	r, g, b, _ = dst.At(1, 0).RGBA()
	expectedDark := uint32(0x22 * 257)
	if r != expectedDark || g != expectedDark || b != expectedDark {
		t.Errorf("dark pixel expected (%d,%d,%d), got (%d,%d,%d)", expectedDark, expectedDark, expectedDark, r, g, b)
	}
}

func TestParseHexColor(t *testing.T) {
	tests := []struct {
		input    string
		defR     uint8
		defG     uint8
		defB     uint8
		expected color.RGBA
	}{
		{"FFF", 0, 0, 0, color.RGBA{255, 255, 255, 255}},
		{"000", 255, 255, 255, color.RGBA{0, 0, 0, 255}},
		{"ABC", 0, 0, 0, color.RGBA{0xAA, 0xBB, 0xCC, 255}},
		{"abcdef", 0, 0, 0, color.RGBA{0xAB, 0xCD, 0xEF, 255}},
		{"", 10, 20, 30, color.RGBA{10, 20, 30, 255}},
		{"xyz", 10, 20, 30, color.RGBA{10, 20, 30, 255}},
		{"12345", 0, 0, 0, color.RGBA{0, 0, 0, 255}},
	}
	for _, tt := range tests {
		result := parseHexColor(tt.input, tt.defR, tt.defG, tt.defB)
		if result != tt.expected {
			t.Errorf("parseHexColor(%q, %d, %d, %d) = %v, want %v",
				tt.input, tt.defR, tt.defG, tt.defB, result, tt.expected)
		}
	}
}

func TestSharpenFilterRegistration(t *testing.T) {
	factory, ok := Lookup("sharpen")
	if !ok {
		t.Fatal("expected filter \"sharpen\" to be registered")
	}
	f, err := factory(map[string]any{})
	if err != nil {
		t.Fatalf("factory for sharpen returned error: %v", err)
	}
	if f.Name() != "sharpen" {
		t.Errorf("expected Name()=%q, got %q", "sharpen", f.Name())
	}
}

func TestSharpenFilterApply(t *testing.T) {
	f := &SharpenFilter{}
	img := image.NewNRGBA64(image.Rect(0, 0, 4, 4))
	results := f.Apply(context.Background(), img, FilterContext{})
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0] == nil {
		t.Fatal("expected non-nil result image")
	}
	if results[0].Bounds() != img.Bounds() {
		t.Errorf("bounds changed: %v -> %v", img.Bounds(), results[0].Bounds())
	}
}

func TestSharpenFilterPreservesUniform(t *testing.T) {
	// A uniform image should remain unchanged by sharpening
	img := image.NewNRGBA64(image.Rect(0, 0, 3, 3))
	for x := range 3 {
		for y := range 3 {
			img.SetNRGBA64(x, y, color.NRGBA64{R: 32768, G: 32768, B: 32768, A: 65535})
		}
	}

	f := &SharpenFilter{}
	results := f.Apply(context.Background(), img, FilterContext{})
	dst := results[0]

	for x := range 3 {
		for y := range 3 {
			or, og, ob, oa := img.At(x, y).RGBA()
			dr, dg, db, da := dst.At(x, y).RGBA()
			if or != dr || og != dg || ob != db || oa != da {
				t.Errorf("pixel (%d,%d) changed: src=%v dst=%v", x, y,
					[]uint32{or, og, ob, oa}, []uint32{dr, dg, db, da})
			}
		}
	}
}

func TestNewFiltersRegistration(t *testing.T) {
	for _, name := range []string{"threshold", "duotone", "sharpen"} {
		_, ok := Lookup(name)
		if !ok {
			t.Errorf("expected filter %q to be registered", name)
		}
	}
}

func TestThresholdDefaultLevel(t *testing.T) {
	f := &ThresholdFilter{} // Level defaults to 0, Apply uses 128
	img := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	img.SetNRGBA64(0, 0, color.NRGBA64{R: 40000, G: 40000, B: 40000, A: 65535}) // ~156 -> white
	img.SetNRGBA64(1, 0, color.NRGBA64{R: 20000, G: 20000, B: 20000, A: 65535}) // ~78 -> black

	results := f.Apply(context.Background(), img, FilterContext{})
	dst := results[0]

	r, _, _, _ := dst.At(0, 0).RGBA()
	if r != 65535 {
		t.Errorf("with default level, ~156 should be white, got r=%d", r)
	}
	r, _, _, _ = dst.At(1, 0).RGBA()
	if r != 0 {
		t.Errorf("with default level, ~78 should be black, got r=%d", r)
	}
}
