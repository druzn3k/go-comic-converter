package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"
)

// ThresholdFilter binarizes an image: pixels above the level become white, below become black.
// Level is in the range 0-255, default 128.
type ThresholdFilter struct {
	Level float32
}

func (f *ThresholdFilter) Name() string { return "threshold" }

func (f *ThresholdFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	level := f.Level
	if level == 0 {
		level = 128
	}
	normalizedLevel := level / 255.0
	filter := gift.ColorFunc(func(r, g, b, a float32) (float32, float32, float32, float32) {
		lum := 0.299*r + 0.587*g + 0.114*b
		if lum > normalizedLevel {
			return 1, 1, 1, a
		}
		return 0, 0, 0, a
	})
	g := gift.New(filter)
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

func init() {
	Register("threshold", func(params map[string]any) (Filter, error) {
		return &ThresholdFilter{Level: float32(getFloat(params, "level", 128))}, nil
	})
}
