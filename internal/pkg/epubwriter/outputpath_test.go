package epubwriter_test

import (
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubwriter"
)

// Part suffixes must match the naming the cbz and html writers use, otherwise the
// same conversion produced "in Part 1 of 2.cbz" next to "inPart 1 of 2.epub".
func TestOutputPathPartNaming(t *testing.T) {
	for _, tc := range []struct {
		name      string
		output    string
		extension string
		current   int
		total     int
		want      string
	}{
		{"single part keeps the plain name", "/tmp/in.epub", ".epub", 1, 1, "/tmp/in.epub"},
		{"epub first part", "/tmp/in.epub", ".epub", 1, 2, "/tmp/in Part 1 of 2.epub"},
		{"epub second part", "/tmp/in.epub", ".epub", 2, 2, "/tmp/in Part 2 of 2.epub"},
		{"kepub part", "/tmp/in.kepub.epub", ".kepub.epub", 2, 3, "/tmp/in Part 2 of 3.kepub.epub"},
		{"ten parts stay unpadded like cbz", "/tmp/in.epub", ".epub", 1, 12, "/tmp/in Part 1 of 12.epub"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := epubwriter.OutputPath(tc.output, tc.extension, tc.current, tc.total)
			if got != tc.want {
				t.Errorf("OutputPath(%q, %q, %d, %d) = %q, want %q",
					tc.output, tc.extension, tc.current, tc.total, got, tc.want)
			}
		})
	}
}
