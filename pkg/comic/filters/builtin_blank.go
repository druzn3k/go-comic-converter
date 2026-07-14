package filters

import (
	"context"
	"image"
)

// PixelFilter ensures the output image is at least 1x1 pixel.
// It's always placed last in the chain to catch any 0x0 results.
type PixelFilter struct{}

func (f *PixelFilter) Name() string { return "pixel" }

func (f *PixelFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	bounds := img.Bounds()
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return []image.Image{image.NewRGBA(image.Rect(0, 0, 1, 1))}
	}
	return []image.Image{img}
}

func init() {
	Register("pixel", func(params map[string]any) (Filter, error) {
		return &PixelFilter{}, nil
	})
}
