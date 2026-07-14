package filters

import (
	"context"
	"image"

	"github.com/disintegration/gift"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimagefilters"
)

// SplitDoublePageFilter splits a double-page spread into two halves.
// It returns 3 images: original, left half, right half.
// KeepOriginal controls whether the original is included (for landscape rendering).
// Manga controls ordering (right-to-left).
type SplitDoublePageFilter struct {
	KeepOriginal bool
	Manga        bool
}

func (f *SplitDoublePageFilter) Name() string { return "split_double_page" }

func (f *SplitDoublePageFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	bounds := img.Bounds()
	if bounds.Dx() <= bounds.Dy() {
		// Not a double page — return as-is
		return []image.Image{img}
	}

	leftFilter := epubimagefilters.CropSplitDoublePage(false)
	rightFilter := epubimagefilters.CropSplitDoublePage(true)

	leftDst := image.NewNRGBA64(leftFilter.Bounds(bounds))
	rightDst := image.NewNRGBA64(rightFilter.Bounds(bounds))

	gLeft := gift.New(leftFilter)
	gLeft.Draw(leftDst, img)
	gRight := gift.New(rightFilter)
	gRight.Draw(rightDst, img)

	if f.KeepOriginal {
		if f.Manga {
			return []image.Image{img, rightDst, leftDst}
		}
		return []image.Image{img, leftDst, rightDst}
	}

	if f.Manga {
		return []image.Image{rightDst, leftDst}
	}
	return []image.Image{leftDst, rightDst}
}

func init() {
	Register("split_double_page", func(params map[string]any) (Filter, error) {
		return &SplitDoublePageFilter{
			KeepOriginal: getBool(params, "keep_original", true),
			Manga:        getBool(params, "manga", false),
		}, nil
	})
}
