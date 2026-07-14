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
