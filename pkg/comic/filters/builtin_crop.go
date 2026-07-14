package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimagefilters"
)

// AutoCropFilter crops blank margins from an image.
// Left, Up, Right, Bottom control the aggressiveness of margin detection on each side.
// Limit is the maximum percentage of the image allowed to be cropped (0 = no limit).
// SkipIfLimitReached, when true, returns the uncropped image if the limit would be exceeded.
type AutoCropFilter struct {
	Left               int
	Up                 int
	Right              int
	Bottom             int
	Limit              int
	SkipIfLimitReached bool
}

func (f *AutoCropFilter) Name() string { return "auto_crop" }

func (f *AutoCropFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	filter := epubimagefilters.AutoCrop(img, img.Bounds(), f.Left, f.Up, f.Right, f.Bottom, f.Limit, f.SkipIfLimitReached)
	bounds := filter.Bounds(img.Bounds())
	if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
		return []image.Image{image.NewRGBA(image.Rect(0, 0, 1, 1))}
	}
	g := gift.New(filter)
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

func init() {
	Register("auto_crop", func(params map[string]any) (Filter, error) {
		return &AutoCropFilter{
			Left:               getInt(params, "left", 1),
			Up:                 getInt(params, "up", 1),
			Right:              getInt(params, "right", 1),
			Bottom:             getInt(params, "bottom", 3),
			Limit:              getInt(params, "limit", 0),
			SkipIfLimitReached: getBool(params, "skip_if_limit_reached", false),
		}, nil
	})
}
