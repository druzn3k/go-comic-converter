package filters

import (
	"image"

	"github.com/disintegration/gift"

	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// DefaultChainOpts holds the options and context needed to build
// the default processing chain that matches the current transformImage.
type DefaultChainOpts struct {
	Image                     epuboptions.Image
	Part                      int
	Right                     bool
	SrcBounds                 image.Rectangle
	CurrentBounds             image.Rectangle
	KeepSplitDoublePageAspect bool
}

// DefaultChain builds a *gift.GIFT chain that reproduces the exact behavior
// of the current transformImage (processor.go:227-333).
// Returns the gift chain, final bounds, and whether the image is a double page.
func DefaultChain(src image.Image, opts DefaultChainOpts) (*gift.GIFT, image.Rectangle, bool) {
	g := gift.New()
	srcBounds := opts.SrcBounds

	// Portrait mode: split before crop
	if opts.Part > 0 && !opts.KeepSplitDoublePageAspect {
		g.Add(CropSplitDoublePage(opts.Right))
	}

	// Crop or blank detection
	if opts.Image.Crop.Enabled || opts.Image.NoBlankImage {
		f := AutoCrop(
			src,
			g.Bounds(srcBounds),
			opts.Image.Crop.Left,
			opts.Image.Crop.Up,
			opts.Image.Crop.Right,
			opts.Image.Crop.Bottom,
			opts.Image.Crop.Limit,
			opts.Image.Crop.SkipIfLimitReached,
		)
		size := f.Bounds(srcBounds)
		isBlank := size.Dx() == 0 && size.Dy() == 0
		if opts.Image.Crop.Enabled || (opts.Image.NoBlankImage && isBlank) {
			g.Add(f)
		}
	}

	// Landscape mode: crop before split
	if opts.Part > 0 && opts.KeepSplitDoublePageAspect {
		g.Add(CropSplitDoublePage(opts.Right))
	}

	dstBounds := g.Bounds(srcBounds)
	isDoublePage := opts.Part == 0 && srcBounds.Dx() > srcBounds.Dy() && dstBounds.Dx() > dstBounds.Dy()

	if opts.Image.AutoRotate && isDoublePage {
		g.Add(gift.Rotate90())
	}
	if opts.Image.AutoContrast {
		g.Add(AutoContrast())
	}
	if opts.Image.Contrast != 0 {
		g.Add(gift.Contrast(float32(opts.Image.Contrast)))
	}
	if opts.Image.Brightness != 0 {
		g.Add(gift.Brightness(float32(opts.Image.Brightness)))
	}
	if opts.Image.Resize {
		g.Add(gift.ResizeToFit(opts.Image.View.Width, opts.Image.View.Height, gift.LanczosResampling))
	}
	if opts.Image.GrayScale {
		var f gift.Filter
		switch opts.Image.GrayScaleMode {
		case 1:
			f = gift.ColorFunc(func(r0, g0, b0, a0 float32) (r, g, b, a float32) {
				y := (r0 + g0 + b0) / 3
				return y, y, y, a0
			})
		case 2:
			f = gift.ColorFunc(func(r0, g0, b0, a0 float32) (r, g, b, a float32) {
				y := 0.2126*r0 + 0.7152*g0 + 0.0722*b0
				return y, y, y, a0
			})
		default:
			f = gift.Grayscale()
		}
		g.Add(f)
	}

	g.Add(Pixel())

	return g, g.Bounds(srcBounds).Intersect(srcBounds), isDoublePage
}

// DefaultFilterConfigs returns a recipe configuration that reproduces the
// default processing chain for the given image options. It is total: every
// filter that DefaultChain may apply is represented, with the same option
// values and the same integer scale used by gift.
func DefaultFilterConfigs(img epuboptions.Image) []FilterConfig {
	var cfgs []FilterConfig
	if img.AutoSplitDoublePage {
		cfgs = append(cfgs, FilterConfig{
			Name: "split_double_page",
			Params: map[string]any{
				"keep_original": img.KeepDoublePageIfSplit,
				"manga":         img.Manga,
			},
		})
	}
	if img.Crop.Enabled || img.NoBlankImage {
		cfgs = append(cfgs, FilterConfig{
			Name: "auto_crop",
			Params: map[string]any{
				"left":                  img.Crop.Left,
				"up":                    img.Crop.Up,
				"right":                 img.Crop.Right,
				"bottom":                img.Crop.Bottom,
				"limit":                 img.Crop.Limit,
				"skip_if_limit_reached": img.Crop.SkipIfLimitReached,
			},
		})
	}
	if img.AutoContrast {
		cfgs = append(cfgs, FilterConfig{Name: "auto_contrast"})
	}
	if img.Contrast != 0 {
		cfgs = append(cfgs, FilterConfig{
			Name:   "contrast",
			Params: map[string]any{"amount": img.Contrast},
		})
	}
	if img.Brightness != 0 {
		cfgs = append(cfgs, FilterConfig{
			Name:   "brightness",
			Params: map[string]any{"amount": img.Brightness},
		})
	}
	if img.Resize {
		cfgs = append(cfgs, FilterConfig{
			Name: "resize",
			Params: map[string]any{
				"width":  img.View.Width,
				"height": img.View.Height,
			},
		})
	}
	if img.GrayScale {
		cfgs = append(cfgs, FilterConfig{
			Name:   "grayscale",
			Params: map[string]any{"mode": img.GrayScaleMode},
		})
	}
	cfgs = append(cfgs, FilterConfig{Name: "pixel"})
	return cfgs
}
