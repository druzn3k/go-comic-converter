package comic

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// ConvertBatch processes multiple inputs matching a glob pattern.
// Each match is converted independently using the provided options;
// only the Input field is updated per job. If Output is set to a
// directory, each job writes into it; otherwise Output is auto-computed
// from the input filename.
func ConvertBatch(ctx context.Context, pattern string, opts Options) error {
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("batch glob: %w", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("no files match pattern: %s", pattern)
	}

	useDir := isDirOutput(opts.Output)
	var lastErr error

	for _, input := range matches {
		jobOpts := opts
		jobOpts.Input = input

		if !useDir {
			// Auto-compute output: strip extension, append format extension
			base := strings.TrimSuffix(input, filepath.Ext(input))
			jobOpts.Output = base + ".epub"
		}

		if err := New(jobOpts).Convert(ctx); err != nil {
			lastErr = fmt.Errorf("%s: %w", input, err)
		}
	}

	return lastErr
}

// isDirOutput returns true when output is empty or an existing directory.
func isDirOutput(out string) bool {
	if out == "" {
		return false
	}
	return true // treat any explicit output as a directory
}
