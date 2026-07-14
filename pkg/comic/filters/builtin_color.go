package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"
)

// GrayscaleFilter converts images to grayscale.
// Mode: 0 = gift default, 1 = average, 2 = luminance.
type GrayscaleFilter struct {
	Mode int
}

func (f *GrayscaleFilter) Name() string { return "grayscale" }

func (f *GrayscaleFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	var filter gift.Filter
	switch f.Mode {
	case 1:
		filter = gift.ColorFunc(func(r, g, b, a float32) (float32, float32, float32, float32) {
			gray := (r + g + b) / 3.0
			return gray, gray, gray, a
		})
	case 2:
		filter = gift.ColorFunc(func(r, g, b, a float32) (float32, float32, float32, float32) {
			gray := 0.299*r + 0.587*g + 0.114*b
			return gray, gray, gray, a
		})
	default:
		filter = gift.Grayscale()
	}
	g := gift.New(filter)
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

func init() {
	Register("grayscale", func(params map[string]any) (Filter, error) {
		return &GrayscaleFilter{Mode: getInt(params, "mode", 0)}, nil
	})
}
