package output

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubtemplates"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubwriter"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/utils"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func init() {
	Register(func() OutputWriter { return KEPUBWriter{} })
}

// KEPUBWriter produces KEPUB files (Kobo-enhanced EPUB) from processed comic images.
type KEPUBWriter struct {
	chain *filters.Chain
}

func (w KEPUBWriter) Format() string          { return "kepub" }
func (w KEPUBWriter) Extension() string       { return ".kepub.epub" }
func (w KEPUBWriter) SupportsPartSplit() bool { return true }

// SetRecipe stores the recipe chain for recipe-based KEPUB output.
func (w *KEPUBWriter) SetRecipe(chain *filters.Chain) {
	w.chain = chain
}

var kepubVariant = epubwriter.Variant{
	Extension:    ".kepub.epub",
	TextTemplate: epubtemplates.KepubText,
	KoboStyle:    true,
	AppleBooks:   false,
	EscapeTitle:  false,
}

// Write produces KEPUB file(s) from the already-split parts.
// It does not re-load or re-split images; it uses the passed parts directly.
func (w KEPUBWriter) Write(ctx context.Context, parts []OutputPart, opts epuboptions.EPUBOptions) ([]string, error) {
	kepubParts := make([]epubwriter.Part, len(parts))
	for i, p := range parts {
		kepubParts[i] = epubwriter.Part{Cover: p.Cover, Images: p.Images}
	}

	totalParts := len(parts)
	outputPaths := make([]string, 0, totalParts)
	for i := range parts {
		outputPaths = append(outputPaths, kepubPartOutputPath(opts.Output, kepubVariant.Extension, i+1, totalParts))
	}

	err := epubwriter.Write(ctx, kepubParts, kepubVariant, opts)
	if opts.Dry {
		return nil, err
	}
	return outputPaths, err
}

func kepubPartOutputPath(output, extension string, currentPart, totalParts int) string {
	base := output
	if strings.HasSuffix(base, extension) {
		base = base[:len(base)-len(extension)]
	} else {
		ext := filepath.Ext(base)
		base = base[:len(base)-len(ext)]
	}

	suffix := ""
	if totalParts > 1 {
		fmtLen := utils.FormatNumberOfDigits(totalParts)
		fmtPart := "Part " + fmtLen + " of " + fmtLen
		suffix = fmt.Sprintf(fmtPart, currentPart, totalParts)
	}
	return base + suffix + extension
}
