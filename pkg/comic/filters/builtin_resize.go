package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"
)

// ResizeFilter resizes images using the gift library.
// Mode: 0 = fit (default), 1 = fill, 2 = stretch.
type ResizeFilter struct {
	Width  int
	Height int
	Mode   int
}

func (f *ResizeFilter) Name() string { return "resize" }

func (f *ResizeFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	var filter gift.Filter
	switch f.Mode {
	case 1:
		filter = gift.ResizeToFill(f.Width, f.Height, gift.LanczosResampling, gift.CenterAnchor)
	case 2:
		filter = gift.Resize(f.Width, f.Height, gift.LanczosResampling)
	default:
		filter = gift.ResizeToFit(f.Width, f.Height, gift.LanczosResampling)
	}
	g := gift.New(filter)
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

func init() {
	Register("resize", func(params map[string]any) (Filter, error) {
		return &ResizeFilter{
			Width:  getInt(params, "width", 1200),
			Height: getInt(params, "height", 1920),
			Mode:   getInt(params, "mode", 0),
		}, nil
	})
}
