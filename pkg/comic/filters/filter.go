// Package filters provides a composable filter chain for image processing.
package filters

import (
	"context"
	"image"
	"sync"

	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// FilterContext provides per-image context to filters.
type FilterContext struct {
	Part           int
	Right          bool
	ImageOptions   epuboptions.Image
	IsDoublePage   bool
	OriginalBounds image.Rectangle
}

// Filter processes a single image, returning 1 or more output images.
type Filter interface {
	Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image
	Name() string
}

// FilterFactory creates a Filter from parameters.
type FilterFactory func(params map[string]any) (Filter, error)

var (
	mu      sync.RWMutex
	entries = map[string]FilterFactory{}
)

// Register adds a filter factory to the global registry.
func Register(name string, factory FilterFactory) {
	mu.Lock()
	entries[name] = factory
	mu.Unlock()
}

// Lookup returns a filter factory by name.
func Lookup(name string) (FilterFactory, bool) {
	mu.RLock()
	f, ok := entries[name]
	mu.RUnlock()
	return f, ok
}

// Names returns all registered filter names.
func Names() []string {
	mu.RLock()
	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	mu.RUnlock()
	return names
}

// ConditionalFilter wraps a Filter with a per-image condition.
type ConditionalFilter struct {
	Filter
	Condition func(fctx FilterContext) bool
}

func (c *ConditionalFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	if !c.Condition(fctx) {
		return []image.Image{img}
	}
	return c.Filter.Apply(ctx, img, fctx)
}
