package output

import (
	"context"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubtemplates"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubwriter"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func init() {
	Register(func() OutputWriter { return &EPUBWriter{} })
}

// EPUBWriter produces standard EPUB files via the shared EPUB-family writer.
// It uses the already-split parts passed by the converter so that size-limit
// splitting and recipe-based processing are honored.
type EPUBWriter struct {
	chain *filters.Chain
}

func (w *EPUBWriter) Format() string          { return "epub" }
func (w *EPUBWriter) Extension() string       { return ".epub" }
func (w *EPUBWriter) SupportsPartSplit() bool { return true }

// SetRecipe stores the recipe chain. The chain is applied by the converter
// before parts reach this writer, so no additional processing happens here.
func (w *EPUBWriter) SetRecipe(chain *filters.Chain) {
	w.chain = chain
}

// Write creates the EPUB file(s) from the passed, already-processed parts.
func (w *EPUBWriter) Write(ctx context.Context, parts []OutputPart, opts epuboptions.EPUBOptions) ([]string, error) {
	epubParts := make([]epubwriter.Part, len(parts))
	for i, p := range parts {
		epubParts[i] = epubwriter.Part{Cover: p.Cover, Images: p.Images}
	}

	if err := epubwriter.Write(ctx, epubParts, epubwriter.Variant{
		Extension:    ".epub",
		TextTemplate: epubtemplates.Text,
		KoboStyle:    false,
		AppleBooks:   true,
		EscapeTitle:  true,
	}, opts); err != nil {
		return nil, err
	}

	if opts.Dry {
		return nil, nil
	}
	return []string{opts.Output}, nil
}
