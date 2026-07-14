package comic

import (
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

// Part is a format-agnostic grouping of processed images.
// It is produced by the part-splitting logic and consumed by output writers.
type Part struct {
	Cover  epubimage.EPUBImage
	Images []epubimage.EPUBImage
}

// PartMetadata holds the metadata for an output part.
type PartMetadata struct {
	Title       string
	Author      string
	Publisher   string
	UID         string
	UpdatedAt   string
	ImageConfig epuboptions.Image
}

// OutputPart is the full data bundle passed to an OutputWriter.
type OutputPart struct {
	PartNumber int
	TotalParts int
	Part       Part
	Metadata   PartMetadata
}
