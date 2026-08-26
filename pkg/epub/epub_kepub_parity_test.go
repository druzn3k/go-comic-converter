package epub

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/output"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// createParityInput creates a temp directory with two small JPEGs.
func createParityInput(t *testing.T) string {
	t.Helper()
	srcDir := t.TempDir()
	for _, name := range []string{"page01.jpg", "page02.jpg"} {
		img := image.NewRGBA(image.Rect(0, 0, 20, 30))
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
			t.Fatalf("encode test JPEG: %v", err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, name), buf.Bytes(), 0644); err != nil {
			t.Fatalf("write test JPEG: %v", err)
		}
	}
	return srcDir
}

// listEntryNames returns the sorted names of all entries in the ZIP at path.
func listEntryNames(t *testing.T, path string) []string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer zr.Close()
	names := make([]string, 0, len(zr.File))
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

// readEntry reads a single entry from the ZIP at path.
func readEntry(t *testing.T, path, name string) []byte {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			defer rc.Close()
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			return buf.Bytes()
		}
	}
	t.Fatalf("entry %s not found in %s", name, path)
	return nil
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// findPageEntry returns the first OEBPS/Text/page_*.xhtml entry.
func findPageEntry(t *testing.T, names []string) string {
	t.Helper()
	for _, n := range names {
		if strings.HasPrefix(n, "OEBPS/Text/page_") && strings.HasSuffix(n, ".xhtml") {
			return n
		}
	}
	t.Fatal("no page entry found")
	return ""
}

// TestEPUBKEPUBParity converts the same input to both EPUB and KEPUB and
// asserts the only documented deltas are present.
func TestEPUBKEPUBParity(t *testing.T) {
	ctx := context.Background()
	srcDir := createParityInput(t)
	tmpDir := t.TempDir()

	// Use a title with an ampersand to exercise EscapeTitle difference.
	title := "Rocket & Groot"

	// Produce EPUB via pkg/epub.
	epubPath := filepath.Join(tmpDir, "comic.epub")
	opts := epuboptions.EPUBOptions{
		Input:     srcDir,
		Output:    epubPath,
		Title:     title,
		TitlePage: 1,
		Image: epuboptions.Image{
			Format: "jpeg",
			View: epuboptions.View{
				Width:  600,
				Height: 800,
			},
		},
	}
	if err := New(opts).Write(ctx); err != nil {
		t.Fatalf("write EPUB: %v", err)
	}

	// Produce KEPUB from the same input via comic.GetParts + KEPUBWriter.
	kepubBase := filepath.Join(tmpDir, "comic")
	kepubPath := kepubBase + ".kepub.epub"
	kepubOpts := epuboptions.EPUBOptions{
		Input:     srcDir,
		Output:    kepubBase,
		Title:     title,
		TitlePage: 1,
		Image: epuboptions.Image{
			Format: "jpeg",
			View: epuboptions.View{
				Width:  600,
				Height: 800,
			},
		},
	}

	proc := epubimageprocessor.New(kepubOpts)
	comicParts, imgStorage, err := comic.GetParts(ctx, proc, kepubOpts)
	if err != nil {
		t.Fatalf("get parts: %v", err)
	}
	defer func() {
		_ = imgStorage.Close()
		_ = imgStorage.Remove()
	}()

	outputParts := make([]output.OutputPart, len(comicParts))
	for i, p := range comicParts {
		outputParts[i] = output.OutputPart{
			Cover:      p.Cover,
			Images:     p.Images,
			PartNumber: i + 1,
			TotalParts: len(comicParts),
		}
	}

	w := output.KEPUBWriter{}
	if _, err := w.Write(ctx, outputParts, kepubOpts); err != nil {
		t.Fatalf("write KEPUB: %v", err)
	}

	epubNames := listEntryNames(t, epubPath)
	kepubNames := listEntryNames(t, kepubPath)

	// EPUB must contain Apple Books metadata; KEPUB must not.
	if !contains(epubNames, "META-INF/com.apple.ibooks.display-options.xml") {
		t.Error("EPUB missing Apple Books display options")
	}
	if contains(kepubNames, "META-INF/com.apple.ibooks.display-options.xml") {
		t.Error("KEPUB should not contain Apple Books display options")
	}

	// After removing the Apple Books entry, the entry name sets should match
	// (both use the same EPUB ZIP layout: OEBPS/Text, OEBPS/Images, etc.).
	epubSet := make(map[string]bool)
	for _, n := range epubNames {
		if n != "META-INF/com.apple.ibooks.display-options.xml" {
			epubSet[n] = true
		}
	}
	kepubSet := make(map[string]bool)
	for _, n := range kepubNames {
		kepubSet[n] = true
	}

	for n := range epubSet {
		if !kepubSet[n] {
			t.Errorf("entry %q present in EPUB but missing in KEPUB", n)
		}
	}
	for n := range kepubSet {
		if !epubSet[n] {
			t.Errorf("entry %q present in KEPUB but missing in EPUB", n)
		}
	}

	// Find a non-cover page entry to compare templates.
	kepubPageName := findPageEntry(t, kepubNames)
	kepubText := string(readEntry(t, kepubPath, kepubPageName))
	if !strings.Contains(kepubText, `<div class="kobolink">`) {
		t.Error("KEPUB page missing kobolink wrapper")
	}

	epubPageName := findPageEntry(t, epubNames)
	epubText := string(readEntry(t, epubPath, epubPageName))
	if strings.Contains(epubText, `<div class="kobolink">`) {
		t.Error("EPUB page should not contain kobolink wrapper")
	}

	// KEPUB content.opf contains kobo-style meta; EPUB does not.
	kepubOPF := string(readEntry(t, kepubPath, "OEBPS/content.opf"))
	if !strings.Contains(kepubOPF, `name="kobo-style"`) {
		t.Error("KEPUB content.opf missing kobo-style meta")
	}
	epubOPF := string(readEntry(t, epubPath, "OEBPS/content.opf"))
	if strings.Contains(epubOPF, `name="kobo-style"`) {
		t.Error("EPUB content.opf should not contain kobo-style meta")
	}

	// EPUB title page applies html.EscapeString to the title before the template
	// applies xmlEscape, so an ampersand is double-escaped. KEPUB does not
	// pre-escape, so the template's xmlEscape produces a single escape.
	epubTitlePage := string(readEntry(t, epubPath, "OEBPS/Text/title.xhtml"))
	if !strings.Contains(epubTitlePage, "Rocket &amp;amp; Groot") {
		t.Errorf("EPUB title page should double-escape ampersand; got %s", epubTitlePage)
	}
	kepubTitlePage := string(readEntry(t, kepubPath, "OEBPS/Text/title.xhtml"))
	if !strings.Contains(kepubTitlePage, "Rocket &amp; Groot") {
		t.Errorf("KEPUB title page should single-escape ampersand; got %s", kepubTitlePage)
	}
}
