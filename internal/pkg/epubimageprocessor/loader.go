package epubimageprocessor

import (
	"context"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/source"
)

func (e ePUBImageProcessor) load(ctx context.Context) (totalImages int, output <-chan epubimageloader.Task, err error) {
	var src source.Source
	if e.testSrc != nil {
		src = e.testSrc
	} else {
		src = source.NewWithOpts(e.Input, e.SortPathMode, true)
	}
	ch, count, loadErr := src.Load(ctx)
	return count, ch, loadErr
}
