package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"
)

// PixelFilter ensures the output image is at least 1x1 pixel.
// It's always placed last in the chain to catch any 0x0 results.
type PixelFilter struct{}

func (f *PixelFilter) Name() string { return "pixel" }

func (f *PixelFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	filter := Pixel()
	g := gift.New(filter)
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

func init() {
	Register("pixel", func(params map[string]any) (Filter, error) {
		return &PixelFilter{}, nil
	})
}
