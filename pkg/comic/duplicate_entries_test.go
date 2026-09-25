package comic

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func testPNGBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{0, 128, 255, 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// A zip may repeat an entry name. Ids are derived per name, so keeping both
// entries gave two images the same id and the writer segfaulted on the cover.
func TestConvertDuplicateEntryNamesDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	input := filepath.Join(dir, "dupe.cbz")
	f, err := os.Create(input)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	data := testPNGBytes(t)
	for range 2 {
		w, err := zw.Create("a.png")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	opts := epuboptions.EPUBOptions{
		Input:        input,
		Output:       filepath.Join(dir, "out.epub"),
		OutputFormat: "epub",
		Image: epuboptions.Image{
			Format:   "jpeg",
			HasCover: true,
			View:     epuboptions.View{Width: 600, Height: 800},
		},
	}
	if err := New(opts).Convert(context.Background()); err != nil {
		t.Fatalf("Convert: %v", err)
	}
	fi, err := os.Stat(opts.Output)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if fi.Size() == 0 {
		t.Error("output is empty")
	}
}
