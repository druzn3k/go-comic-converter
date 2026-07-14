package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimagefilters"
)

func init() {
	Register("auto_contrast", func(params map[string]any) (Filter, error) {
		return &AutoContrastFilter{}, nil
	})
	Register("contrast", func(params map[string]any) (Filter, error) {
		amount := getFloat(params, "amount", 0)
		return &ContrastFilter{Amount: float32(amount)}, nil
	})
	Register("brightness", func(params map[string]any) (Filter, error) {
		amount := getFloat(params, "amount", 0)
		return &BrightnessFilter{Amount: float32(amount)}, nil
	})
}

// AutoContrastFilter wraps epubimagefilters.AutoContrast via gift.
type AutoContrastFilter struct{}

func (f *AutoContrastFilter) Name() string { return "auto_contrast" }

func (f *AutoContrastFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	g := gift.New(epubimagefilters.AutoContrast())
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

// ContrastFilter adjusts image contrast via gift.Contrast.
type ContrastFilter struct {
	Amount float32
}

func (f *ContrastFilter) Name() string { return "contrast" }

func (f *ContrastFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	g := gift.New(gift.Contrast(f.Amount))
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

// BrightnessFilter adjusts image brightness via gift.Brightness.
type BrightnessFilter struct {
	Amount float32
}

func (f *BrightnessFilter) Name() string { return "brightness" }

func (f *BrightnessFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	g := gift.New(gift.Brightness(f.Amount))
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}
