package comic

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// A single-page source must still produce one part. With HasCover the only page
// becomes the cover, so the image list is empty — that must not yield a book
// with zero parts, which would write no output at all while reporting success.
func TestGetPartsSinglePageWithCover(t *testing.T) {
	opts := epuboptions.EPUBOptions{
		Input:  filepath.Join("source", "testdata", "sample.pdf"),
		Output: filepath.Join(t.TempDir(), "comic.epub"),
		Image: epuboptions.Image{
			Format:   "jpeg",
			HasCover: true,
			View:     epuboptions.View{Width: 600, Height: 800},
		},
	}

	parts, storage, err := GetParts(context.Background(), epubimageprocessor.New(opts), opts)
	if err != nil {
		t.Fatalf("GetParts: %v", err)
	}
	defer func() { _ = storage.Close() }()

	if len(parts) != 1 {
		t.Fatalf("got %d parts, want 1", len(parts))
	}
	if parts[0].Cover.Name == "" {
		t.Error("single part has no cover")
	}
	// The only page stays in the reading flow: a part with no images is not
	// renderable by the writers (empty spine item list).
	if len(parts[0].Images) != 1 {
		t.Errorf("got %d images in the part, want 1", len(parts[0].Images))
	}
}
