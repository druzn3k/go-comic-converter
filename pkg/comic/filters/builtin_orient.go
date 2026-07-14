package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimagefilters"
)

func init() {
	Register("rotate90", func(params map[string]any) (Filter, error) {
		return &Rotate90Filter{}, nil
	})
	Register("crop_split_double_page", func(params map[string]any) (Filter, error) {
		return &CropSplitDoublePageFilter{Right: false}, nil
	})
}

// Rotate90Filter rotates an image 90 degrees clockwise.
type Rotate90Filter struct{}

func (f *Rotate90Filter) Name() string { return "rotate90" }

func (f *Rotate90Filter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	g := gift.New(gift.Rotate90())
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

// CropSplitDoublePageFilter splits a double-page spread into left or right half.
type CropSplitDoublePageFilter struct{ Right bool }

func (f *CropSplitDoublePageFilter) Name() string { return "crop_split_double_page" }

func (f *CropSplitDoublePageFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	filter := epubimagefilters.CropSplitDoublePage(f.Right)
	g := gift.New(filter)
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}
