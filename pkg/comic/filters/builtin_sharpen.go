package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"
)

// SharpenFilter applies a Laplacian sharpening kernel.
type SharpenFilter struct{}

func (f *SharpenFilter) Name() string { return "sharpen" }

func (f *SharpenFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	g := gift.New(gift.Convolution(
		[]float32{0, -1, 0, -1, 5, -1, 0, -1, 0},
		true, false, false, 0,
	))
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

func init() {
	Register("sharpen", func(params map[string]any) (Filter, error) {
		return &SharpenFilter{}, nil
	})
}
