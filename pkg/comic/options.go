// Package comic provides the foundational types and algorithms for the
// go-comic-converter pipeline. It is designed to be imported by both the
// CLI tool and external Go programs.
package comic

import "github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"

// Options is the public alias for the options type.
// It re-exports epuboptions.EPUBOptions to provide a stable import path
// independent of the output format.
type Options = epuboptions.EPUBOptions
