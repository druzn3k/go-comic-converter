// Package output defines the OutputWriter interface for producing
// output files from processed comic images. Each output format
// (EPUB, CBZ, KEPUB, HTML) implements this interface.
package output

import (
	"context"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// OutputPart represents a single processed part of a comic conversion.
// A conversion may produce multiple parts when the output is split
// by size limit (e.g., for email-sized EPUB splits).
type OutputPart struct {
	Cover       epubimage.EPUBImage
	Images      []epubimage.EPUBImage
	PartNumber  int
	TotalParts  int
	Metadata    PartMetadata
}

// PartMetadata contains metadata associated with a conversion.
type PartMetadata struct {
	Title       string
	Author      string
	Publisher   string
	Series      string
	Number      string
	Summary     string
	Genre       string
	Writer      string
	Manga       string
	UID         string
	UpdatedAt   string
	ImageConfig epuboptions.Image
}

// OutputWriter produces output files from processed image parts.
// Each format (EPUB, CBZ, KEPUB, HTML) provides its own implementation.
type OutputWriter interface {
	// Write produces one or more output files from the given parts.
	// Returns the paths of the generated files.
	Write(ctx context.Context, parts []OutputPart, opts epuboptions.EPUBOptions) ([]string, error)

	// Format returns the output format identifier (e.g., "epub", "cbz").
	Format() string

	// Extension returns the file extension for this format (e.g., ".epub", ".cbz").
	Extension() string

	// SupportsPartSplit indicates whether this format supports splitting
	// output into multiple files by size limit.
	SupportsPartSplit() bool
}

// registry maps format names to their constructors.
var registry = map[string]func() OutputWriter{}

// Register adds an output format to the global registry.
// Called by init() functions in each output implementation.
func Register(writer func() OutputWriter) {
	w := writer()
	registry[w.Format()] = writer
}

// Get returns the registered writer for the given format, or nil.
func Get(format string) OutputWriter {
	if w, ok := registry[format]; ok {
		return w()
	}
	return nil
}

// Available returns a sorted list of registered format names.
func Available() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}
