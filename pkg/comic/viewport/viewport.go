// Package viewport provides aspect ratio computation and viewport sizing
// for comic-to-ebook conversion. It is separate from pkg/comic to avoid
// circular dependencies with output writers that also need these functions.
package viewport

import (
	"math"

	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// ImageAspect carries the aspect ratio of a single image.
type ImageAspect struct {
	OriginalAspectRatio float64
}

// PartAspect carries the aspect ratios of a group of images,
// representing either a single output file or a split part.
type PartAspect struct {
	Cover  ImageAspect
	Images []ImageAspect
}

// ComputeAspectRatio finds the most common aspect ratio across all images
// in the given parts. This is used to determine the optimal viewport
// dimensions for the output.
func ComputeAspectRatio(parts []PartAspect) float64 {
	var (
		bestAspectRatio      float64
		bestAspectRatioCount int
		aspectRatio          = map[float64]int{}
	)

	trunc := func(v float64) float64 {
		return float64(math.Round(v*10000)) / 10000
	}

	for _, p := range parts {
		aspectRatio[trunc(p.Cover.OriginalAspectRatio)]++
		for _, i := range p.Images {
			aspectRatio[trunc(i.OriginalAspectRatio)]++
		}
	}

	for k, v := range aspectRatio {
		if v > bestAspectRatioCount {
			bestAspectRatio, bestAspectRatioCount = k, v
		}
	}

	return bestAspectRatio
}

// ComputeViewPort computes the optimal viewport dimensions for the output
// based on the aspect ratios of the images and the configured view settings.
// If AspectRatio is -1, the device dimensions are kept unchanged.
// If AspectRatio is 0, the most common image aspect ratio is used.
func ComputeViewPort(parts []PartAspect, view epuboptions.View) (width, height int) {
	if view.AspectRatio == -1 {
		return view.Width, view.Height
	}

	bestAspectRatio := view.AspectRatio
	if bestAspectRatio == 0 {
		bestAspectRatio = ComputeAspectRatio(parts)
	}

	viewWidth := int(float64(view.Height) / bestAspectRatio)
	viewHeight := int(float64(view.Width) * bestAspectRatio)
	if viewWidth > view.Width {
		return view.Width, viewHeight
	}
	return viewWidth, view.Height
}
