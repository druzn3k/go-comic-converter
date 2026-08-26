package filters

import (
	"fmt"
	"image"
	"reflect"
	"testing"

	"github.com/disintegration/gift"

	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// filterTypeName maps a gift filter from DefaultChain to the recipe filter name
// it corresponds to. The representative config below guarantees that color
// channel filters appear in the order contrast, brightness.
func filterTypeName(f gift.Filter, colorChanIdx int) string {
	typ := fmt.Sprintf("%T", f)
	switch typ {
	case "*gift.cropFilter":
		return "auto_crop"
	case "filters.autocontrast":
		return "auto_contrast"
	case "filters.cropSplitDoublePage":
		return "split_double_page"
	case "filters.pixel":
		return "pixel"
	case "*gift.resizeToFitFilter":
		return "resize"
	case "*gift.colorFilter":
		return "grayscale"
	case "*gift.colorchanFilter":
		if colorChanIdx == 0 {
			return "contrast"
		}
		return "brightness"
	}
	return typ
}

func TestDefaultFilterConfigsMatchesDefaultChain(t *testing.T) {
	img := epuboptions.Image{
		AutoSplitDoublePage:  true,
		KeepDoublePageIfSplit: true,
		Manga:                 true,
		Crop: epuboptions.Crop{
			Enabled:            true,
			Left:               1,
			Up:                 2,
			Right:              3,
			Bottom:             4,
			Limit:              5,
			SkipIfLimitReached: true,
		},
		AutoContrast: true,
		Contrast:     30,
		Brightness:   15,
		Resize:       true,
		View: epuboptions.View{
			Width:  1200,
			Height: 1920,
		},
		GrayScale:     true,
		GrayScaleMode: 2,
	}

	// Names produced by the options→recipe bridge.
	var gotNames []string
	for _, cfg := range DefaultFilterConfigs(img) {
		gotNames = append(gotNames, cfg.Name)
	}

	// Representative image: landscape so DefaultChain sees a double page and
	// applies the split when part > 0.
	src := image.NewRGBA(image.Rect(0, 0, 1600, 800))
	g, _, _ := DefaultChain(src, DefaultChainOpts{
		Image:                     img,
		Part:                      1,
		Right:                     true,
		SrcBounds:                 src.Bounds(),
		CurrentBounds:             src.Bounds(),
		KeepSplitDoublePageAspect: false,
	})

	colorChanIdx := 0
	var wantNames []string
	for _, f := range g.Filters {
		name := filterTypeName(f, colorChanIdx)
		if name == "contrast" {
			colorChanIdx++
		}
		wantNames = append(wantNames, name)
	}

	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Errorf("defaultFilterConfigs names do not match DefaultChain filter set\ngot:  %v\nwant: %v", gotNames, wantNames)
	}

	// Additionally verify every emitted config can be resolved to a filter.
	for _, cfg := range DefaultFilterConfigs(img) {
		factory, ok := Lookup(cfg.Name)
		if !ok {
			t.Fatalf("filter %q not registered", cfg.Name)
		}
		if _, err := factory(cfg.Params); err != nil {
			t.Fatalf("filter %q failed to build with params %v: %v", cfg.Name, cfg.Params, err)
		}
	}
}
