// Package epub Tools to create EPUB from images.
package epub

import (
	"context"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimagepassthrough"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubtemplates"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubwriter"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// ErrImageCorrupted is returned by Write() when some images had errors
// and could not be fully processed. The EPUB is still written with placeholders.
var ErrImageCorrupted = epubwriter.ErrImageCorrupted

// Deprecated: a single-implementation interface; New returns *epub.
type EPUB interface {
	Write(ctx context.Context) error
}

type epub struct {
	epuboptions.EPUBOptions
}

// New initialize EPUB
func New(options epuboptions.EPUBOptions) EPUB {
	return &epub{EPUBOptions: options}
}

// Write creates the EPUB file(s) from the configured input.
func (e *epub) Write(ctx context.Context) error {
	var imageProcessor epubimageprocessor.EPUBImageProcessor
	if e.Image.Format == "copy" {
		imageProcessor = epubimagepassthrough.New(e.EPUBOptions)
	} else {
		imageProcessor = epubimageprocessor.New(e.EPUBOptions)
	}

	comicParts, imgStorage, err := comic.GetParts(ctx, imageProcessor, e.EPUBOptions)
	if err != nil {
		return err
	}
	if !e.Dry {
		_ = imgStorage.Close()
	}

	parts := make([]epubwriter.Part, len(comicParts))
	for i, p := range comicParts {
		parts[i] = epubwriter.Part{Cover: p.Cover, Images: p.Images}
	}

	return epubwriter.Write(ctx, parts, epubwriter.Variant{
		Extension:    ".epub",
		TextTemplate: epubtemplates.Text,
		KoboStyle:    false,
		AppleBooks:   true,
		EscapeTitle:  true,
	}, e.EPUBOptions)
}
