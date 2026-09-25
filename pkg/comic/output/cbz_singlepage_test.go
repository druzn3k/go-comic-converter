package output

import (
	"archive/zip"
	"context"
	"path/filepath"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// A single-page comic keeps its only page as cover and page at once (see
// comic.GetParts), so the CBZ must still contain it exactly once instead of
// writing the same name twice.
func TestCBZWriterSinglePageNoDuplicateEntry(t *testing.T) {
	ctx := context.Background()
	outputPath := filepath.Join(t.TempDir(), "single.cbz")

	sw, err := epubzip.NewStorageImageWriter(outputPath+".tmp", "jpeg")
	if err != nil {
		t.Fatalf("create storage writer: %v", err)
	}
	page := epubimage.EPUBImage{Id: 0, Part: 0, Format: "jpeg"}
	fakeJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	if err := sw.AddRaw(page.StorageKey(), fakeJPEG); err != nil {
		t.Fatalf("add page: %v", err)
	}
	if err := sw.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	parts := []OutputPart{{
		Cover:      page,
		Images:     []epubimage.EPUBImage{page},
		PartNumber: 1,
		TotalParts: 1,
		Metadata:   PartMetadata{Title: "Single Page"},
	}}
	if _, err := (CBZWriter{}).Write(ctx, parts, epuboptions.EPUBOptions{Output: outputPath}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	zr, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("open resulting CBZ: %v", err)
	}
	defer zr.Close()

	if len(zr.File) != 2 {
		t.Errorf("got %d entries, want 2 (ComicInfo.xml + the single page)", len(zr.File))
	}
	if zr.File[0].Name != "ComicInfo.xml" {
		t.Errorf("first entry = %q, want ComicInfo.xml", zr.File[0].Name)
	}
	counts := map[string]int{}
	for _, f := range zr.File {
		counts[f.Name]++
	}
	for name, n := range counts {
		if n != 1 {
			t.Errorf("entry %q written %d times, want 1", name, n)
		}
	}
}
