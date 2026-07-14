package comic

import (
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/viewport"
)

// ImageAspect, PartAspect, ComputeAspectRatio, and ComputeViewPort
// are defined in the subpackage pkg/comic/viewport and re-exported here
// for backward compatibility with existing callers.
type ImageAspect = viewport.ImageAspect

type PartAspect = viewport.PartAspect

var ComputeAspectRatio = viewport.ComputeAspectRatio

var ComputeViewPort = viewport.ComputeViewPort

// toPartAspects converts []Part (from types.go) to []PartAspect,
// extracting only the aspect ratio data needed by viewport functions.
func toPartAspects(parts []Part) []PartAspect {
	aspects := make([]PartAspect, len(parts))
	for i, p := range parts {
		images := make([]ImageAspect, len(p.Images))
		for j, img := range p.Images {
			images[j] = ImageAspect{OriginalAspectRatio: img.OriginalAspectRatio}
		}
		aspects[i] = PartAspect{
			Cover:  ImageAspect{OriginalAspectRatio: p.Cover.OriginalAspectRatio},
			Images: images,
		}
	}
	return aspects
}
