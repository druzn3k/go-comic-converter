package epubimageprocessor

import (
	"context"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimageloader"
	"github.com/celogeek/go-comic-converter/v3/pkg/comic/source"
)

func (e ePUBImageProcessor) load(ctx context.Context) (totalImages int, output <-chan epubimageloader.Task, err error) {
	ch, count, loadErr := source.NewWithOpts(e.Input, e.SortPathMode, true).Load(ctx)
	return count, ch, loadErr
}
