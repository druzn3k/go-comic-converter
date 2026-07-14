package filters

import (
	"context"
	"image"
	"image/color"
	"strconv"

	"github.com/disintegration/gift"
)

// DuotoneFilter maps an image to a 2-color palette using luminance thresholding.
// Dark and Light are hex RGB strings (3 or 6 chars, e.g. "000", "FFF").
type DuotoneFilter struct {
	Dark  string
	Light string
}

func (f *DuotoneFilter) Name() string { return "duotone" }

func (f *DuotoneFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	dark := parseHexColor(f.Dark, 0, 0, 0)
	light := parseHexColor(f.Light, 255, 255, 255)
	filter := gift.ColorFunc(func(r, g, b, a float32) (float32, float32, float32, float32) {
		lum := (r + g + b) / 3
		threshold := float32(0.5)
		if lum > threshold {
			return float32(light.R) / 255, float32(light.G) / 255, float32(light.B) / 255, a
		}
		return float32(dark.R) / 255, float32(dark.G) / 255, float32(dark.B) / 255, a
	})
	g := gift.New(filter)
	dst := image.NewNRGBA64(g.Bounds(img.Bounds()))
	g.Draw(dst, img)
	return []image.Image{dst}
}

// parseHexColor parses a 3- or 6-char hex RGB string. Returns the default color on failure.
func parseHexColor(s string, defaultR, defaultG, defaultB uint8) color.RGBA {
	if s == "" {
		return color.RGBA{R: defaultR, G: defaultG, B: defaultB, A: 255}
	}
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return color.RGBA{R: defaultR, G: defaultG, B: defaultB, A: 255}
	}
	v, err := strconv.ParseUint(s, 16, 24)
	if err != nil {
		return color.RGBA{R: defaultR, G: defaultG, B: defaultB, A: 255}
	}
	return color.RGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

func init() {
	Register("duotone", func(params map[string]any) (Filter, error) {
		return &DuotoneFilter{
			Dark:  getString(params, "dark", "000"),
			Light: getString(params, "light", "FFF"),
		}, nil
	})
}
