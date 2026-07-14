// Package comic provides a library-first API for converting comic archives
// (CBZ, CBR, directories, PDF) to e-reader formats (EPUB, KEPUB, CBZ, HTML).
package comic

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gofrs/uuid/v5"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimagepassthrough"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/celogeek/go-comic-converter/v3/pkg/comic/output"
)

// ErrImageCorrupted is returned by Convert() when some images had errors
// and could not be fully processed. The output is still written with placeholders.
var ErrImageCorrupted = errors.New("one or more images are corrupted")

// Converter is the primary conversion engine.
// It orchestrates source loading, image processing, and output writing
// for non-EPUB formats (KEPUB, CBZ, HTML). EPUB output uses pkg/epub.
type Converter struct {
	opts Options
	proc epubimageprocessor.EPUBImageProcessor
}

// New creates a Converter with the given options.
func New(opts Options) *Converter {
	var proc epubimageprocessor.EPUBImageProcessor
	if opts.Image.Format == "copy" {
		proc = epubimagepassthrough.New(opts)
	} else {
		proc = epubimageprocessor.New(opts)
	}
	return &Converter{opts: opts, proc: proc}
}

// Convert runs the full pipeline: load → process → output.
// It dispatches to the registered OutputWriter for the configured format.
// EPUB output is handled by pkg/epub; use comic.New().Convert() for
// "kepub", "cbz", "html", or "all".
func (c *Converter) Convert(ctx context.Context) error {
	parts, imgStorage, err := GetParts(ctx, c.proc, c.opts)
	if err != nil {
		return err
	}

	if c.opts.Dry {
		return nil
	}
	defer func() {
		_ = imgStorage.Close()
		_ = imgStorage.Remove()
	}()

	format := c.opts.OutputFormat
	if format == "" || format == "epub" {
		return fmt.Errorf("EPUB output requires pkg/epub; use comic.Converter for non-EPUB formats (kepub, cbz, html)")
	}

	if format == "all" {
		return c.writeAll(ctx, parts, imgStorage)
	}

	outputParts := c.buildOutputParts(parts)
	writer := output.Get(format)
	if writer == nil {
		return fmt.Errorf("unknown output format: %s", format)
	}
	_, err = writer.Write(ctx, outputParts, c.opts)

	if hasCorruptedImages(parts) && err == nil {
		err = ErrImageCorrupted
	}
	return err
}

// writeAll produces all registered output formats.
func (c *Converter) writeAll(ctx context.Context, parts []Part, imgStorage epubzip.StorageImageReader) error {
	var firstErr error
	for _, format := range output.Available() {
		outputParts := c.buildOutputParts(parts)
		writer := output.Get(format)
		if writer == nil {
			continue
		}
		_, err := writer.Write(ctx, outputParts, c.opts)
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if hasCorruptedImages(parts) && firstErr == nil {
		firstErr = ErrImageCorrupted
	}
	return firstErr
}

// buildOutputParts converts []Part to []output.OutputPart with metadata.
func (c *Converter) buildOutputParts(parts []Part) []output.OutputPart {
	u, _ := uuid.NewV4()

	metadata := output.PartMetadata{
		Title:       c.opts.Title,
		Author:      c.opts.Author,
		Publisher:   "GO Comic Converter",
		UID:         u.String(),
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		ImageConfig: c.opts.Image,
	}

	oparts := make([]output.OutputPart, len(parts))
	for i, p := range parts {
		oparts[i] = output.OutputPart{
			PartNumber: i + 1,
			TotalParts: len(parts),
			Cover:      p.Cover,
			Images:     p.Images,
			Metadata:   metadata,
		}
	}
	return oparts
}

// hasCorruptedImages checks if any image in parts has an error.
func hasCorruptedImages(parts []Part) bool {
	for _, p := range parts {
		if p.Cover.Error != nil {
			return true
		}
		for _, img := range p.Images {
			if img.Error != nil {
				return true
			}
		}
	}
	return false
}
