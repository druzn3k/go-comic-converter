package source

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// New creates the appropriate Source implementation based on the input path,
// with webp/tiff support enabled (processor mode).
func New(input string, sortMode int) Source {
	return NewWithOpts(input, sortMode, true)
}

// NewWithOpts creates a Source with explicit webp/tiff support.
// Processor mode (includeWebpTiff=true) accepts .webp and .tiff;
// passthrough mode (includeWebpTiff=false) accepts only .jpg/.jpeg/.png.
func NewWithOpts(input string, sortMode int, includeWebpTiff bool) Source {
	fi, err := os.Stat(input)
	if err != nil {
		return &errorSource{err: err}
	}
	if fi.IsDir() {
		return &dirSource{input: input, sortMode: sortMode, includeWebpTiff: includeWebpTiff}
	}
	switch ext := strings.ToLower(filepath.Ext(input)); ext {
	case ".cbz", ".zip":
		return &cbzSource{input: input, sortMode: sortMode, includeWebpTiff: includeWebpTiff}
	case ".cbr", ".rar":
		return &cbrSource{input: input, sortMode: sortMode, includeWebpTiff: includeWebpTiff}
	case ".pdf":
		return &pdfSource{input: input, sortMode: sortMode}
	default:
		return &errorSource{err: fmt.Errorf("unknown format: %s", ext)}
	}
}

// maxProcs returns GOMAXPROCS, minimum 1.
func maxProcs() int {
	n := runtime.GOMAXPROCS(0)
	if n < 1 {
		n = 1
	}
	return n
}

// decodeWorkers returns the number of parallel decode goroutines as 50% of GOMAXPROCS
// (minimum 1), matching the original WorkersRatio(50) behavior.
func decodeWorkers() int {
	n := maxProcs()
	n = n * 50 / 100
	if n < 1 {
		n = 1
	}
	return n
}
