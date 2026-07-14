package filters

import (
	"context"
	"image"
)

// Chain is an ordered list of filters applied sequentially.
// It handles multi-output fan-out: if a filter returns N images,
// subsequent filters run on each of the N images independently.
type Chain struct {
	filters []Filter
}

// NewChain creates a new Chain with the given filters.
func NewChain(filters ...Filter) *Chain {
	return &Chain{filters: filters}
}

// Add appends a filter to the chain.
func (c *Chain) Add(f Filter) {
	c.filters = append(c.filters, f)
}

// Apply runs all filters in sequence, handling multi-output fan-out.
func (c *Chain) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	results := []image.Image{img}
	for _, f := range c.filters {
		var next []image.Image
		for _, r := range results {
			applied := f.Apply(ctx, r, fctx)
			next = append(next, applied...)
		}
		results = next
		if len(results) == 0 {
			return nil
		}
	}
	return results
}

// Len returns the number of filters in the chain.
func (c *Chain) Len() int {
	return len(c.filters)
}
