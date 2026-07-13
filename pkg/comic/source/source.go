// Package source provides Source interface implementations for loading
// images from various comic input formats (directory, CBZ/ZIP, CBR/RAR, PDF).
//
// Each Source implementation handles file discovery, decompression, and
// decoding, returning a channel of Tasks for parallel processing.
package source

import (
	"context"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimageloader"
)

// Source represents a source of comic images.
type Source interface {
	// Load returns a channel of decoded image Tasks and the total count.
	// The caller is responsible for draining the channel.
	Load(ctx context.Context) (<-chan epubimageloader.Task, int, error)

	// Name returns a human-readable identifier for this source (e.g., the file path).
	Name() string
}
// errorSource implements Source for error cases.
type errorSource struct {
	err error
}

func (e *errorSource) Load(ctx context.Context) (<-chan epubimageloader.Task, int, error) {
	return nil, 0, e.err
}

func (e *errorSource) Name() string {
	return e.err.Error()
}
