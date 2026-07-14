package output

import (
	"context"

	"github.com/celogeek/go-comic-converter/v3/pkg/epub"
	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

func init() {
	Register(func() OutputWriter { return EPUBWriter{} })
}

// EPUBWriter produces EPUB files using the existing epub package.
// It implements the OutputWriter interface so EPUB can be selected
// via -output-format and included in "all" format orchestrations.
type EPUBWriter struct{}

func (w EPUBWriter) Format() string          { return "epub" }
func (w EPUBWriter) Extension() string       { return ".epub" }
func (w EPUBWriter) SupportsPartSplit() bool { return true }

// Write produces the EPUB file by delegating to the existing epub package.
// Parts are ignored — the epub package re-loads them from the image processor.
func (w EPUBWriter) Write(ctx context.Context, parts []OutputPart, opts epuboptions.EPUBOptions) ([]string, error) {
	e := epub.New(opts)
	if err := e.Write(ctx); err != nil {
		return nil, err
	}
	return []string{opts.Output}, nil
}
