package comic

import (
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
)

// Part is a format-agnostic grouping of processed images.
// It is produced by the part-splitting logic and consumed by output writers.
type Part struct {
	Cover  epubimage.EPUBImage
	Images []epubimage.EPUBImage
}

// PartMetadata and OutputPart are defined in pkg/comic/output
// since they carry the output-format-specific metadata.
