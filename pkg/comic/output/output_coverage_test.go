package output

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

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

// createTestStorage creates a temp ZIP storage with a single image and returns
// a reader for it along with the storage path.
func createTestStorage(t *testing.T, format string) (epubzip.StorageImageReader, string) {
	t.Helper()
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "storage.zip")
	sw, err := epubzip.NewStorageImageWriter(path, format)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 100, 150))
	testImg := epubimage.EPUBImage{
		Id:     1,
		Part:   0,
		Format: format,
		Width:  100,
		Height: 150,
	}
	if err := sw.Add(testImg.EPUBImgPath(), img, 85); err != nil {
		sw.Close()
		t.Fatal(err)
	}
	if err := sw.Close(); err != nil {
		t.Fatal(err)
	}
	sr, err := epubzip.NewStorageImageReader(path)
	if err != nil {
		t.Fatal(err)
	}
	return sr, path
}

// --- MarshalComicInfo tests ---

func TestMarshalComicInfo(t *testing.T) {
	meta := PartMetadata{
		Title:     "My Comic",
		Author:    "John Doe",
		Publisher: "BigPub",
		Series:    "Epic Series",
		Number:    "42",
		Summary:   "A great story.",
		Genre:     "Action",
		Writer:    "",
		Manga:     "YesAndRightToLeft",
		UID:       "abc-123",
		UpdatedAt: "2025-01-01",
	}
	data, err := MarshalComicInfo(meta, 10)
	if err != nil {
		t.Fatalf("MarshalComicInfo: %v", err)
	}
	xml := string(data)

	if !strings.Contains(xml, "<?xml") {
		t.Error("missing XML declaration")
	}
	if !strings.Contains(xml, "<ComicInfo") {
		t.Error("missing ComicInfo root element")
	}
	if !strings.Contains(xml, "<Title>My Comic</Title>") {
		t.Error("missing Title")
	}
	if !strings.Contains(xml, "<Series>Epic Series</Series>") {
		t.Error("missing Series")
	}
	if !strings.Contains(xml, "<Number>42</Number>") {
		t.Error("missing Number")
	}
	if !strings.Contains(xml, "<Summary>A great story.</Summary>") {
		t.Error("missing Summary")
	}
	if !strings.Contains(xml, "<Publisher>BigPub</Publisher>") {
		t.Error("missing Publisher")
	}
	if !strings.Contains(xml, "<Genre>Action</Genre>") {
		t.Error("missing Genre")
	}
	if !strings.Contains(xml, "<PageCount>10</PageCount>") {
		t.Error("missing PageCount")
	}
	if !strings.Contains(xml, "<Writer>John Doe</Writer>") {
		t.Error("missing Writer (from Author field)")
	}
	if !strings.Contains(xml, "<Manga>YesAndRightToLeft</Manga>") {
		t.Error("missing Manga")
	}
}

func TestMarshalComicInfoEmpty(t *testing.T) {
	meta := PartMetadata{}
	data, err := MarshalComicInfo(meta, 5)
	if err != nil {
		t.Fatalf("MarshalComicInfo empty: %v", err)
	}
	xml := string(data)

	if !strings.Contains(xml, "<?xml") {
		t.Error("missing XML declaration")
	}
	if !strings.Contains(xml, "<ComicInfo") {
		t.Error("missing ComicInfo root element")
	}
	if !strings.Contains(xml, "<PageCount>5</PageCount>") {
		t.Error("missing PageCount")
	}
	// With omitempty, empty fields should not appear.
	if strings.Contains(xml, "<Title>") {
		t.Error("Title should not appear for empty metadata")
	}
	if strings.Contains(xml, "<Series>") {
		t.Error("Series should not appear for empty metadata")
	}
}

// --- CBZWriter tests ---

func TestCBZWriterWrite(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test.cbz")

	// Create temp ZIP storage at the path opts.ImgStorage() will use.
	storagePath := outputPath + ".tmp"
	sw, err := epubzip.NewStorageImageWriter(storagePath, "jpeg")
	if err != nil {
		t.Fatalf("create storage writer: %v", err)
	}

	// Add cover and page images matching EPUBImgPath paths.
	cover := epubimage.EPUBImage{Id: 0, Part: 0, Format: "jpeg"}
	page1 := epubimage.EPUBImage{Id: 1, Part: 0, Format: "jpeg"}
	fakeJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}

	if err := sw.AddRaw(cover.EPUBImgPath(), fakeJPEG); err != nil {
		t.Fatalf("add cover: %v", err)
	}
	if err := sw.AddRaw(page1.EPUBImgPath(), fakeJPEG); err != nil {
		t.Fatalf("add page1: %v", err)
	}
	if err := sw.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	opts := epuboptions.EPUBOptions{
		Output: outputPath,
	}
	parts := []OutputPart{
		{
			Cover:      cover,
			Images:     []epubimage.EPUBImage{page1},
			PartNumber: 1,
			TotalParts: 1,
			Metadata: PartMetadata{
				Title:  "Test Comic",
				Series: "Test Series",
			},
		},
	}

	w := CBZWriter{}
	paths, err := w.Write(ctx, parts, opts)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 output path, got %d", len(paths))
	}
	if paths[0] != outputPath {
		t.Errorf("output path = %q, want %q", paths[0], outputPath)
	}

	// Verify output is a valid ZIP.
	zr, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("open resulting CBZ: %v", err)
	}
	defer zr.Close()

	if len(zr.File) < 3 {
		t.Fatalf("expected at least 3 entries (ComicInfo + cover + page), got %d", len(zr.File))
	}

	// First entry must be ComicInfo.xml.
	if zr.File[0].Name != "ComicInfo.xml" {
		t.Errorf("first entry = %q, want ComicInfo.xml", zr.File[0].Name)
	}

	// Read ComicInfo.xml to verify content.
	rc, err := zr.File[0].Open()
	if err != nil {
		t.Fatalf("open ComicInfo.xml: %v", err)
	}
	defer rc.Close()
	comicInfoData := make([]byte, 1024)
	n, _ := rc.Read(comicInfoData)
	ciXML := string(comicInfoData[:n])
	if !strings.Contains(ciXML, "<Title>Test Comic</Title>") {
		t.Error("ComicInfo.xml missing Title")
	}
	if !strings.Contains(ciXML, "<Series>Test Series</Series>") {
		t.Error("ComicInfo.xml missing Series")
	}

	// Verify image entries exist with expected naming.
	foundCover := false
	foundPage := false
	for _, f := range zr.File[1:] {
		if strings.Contains(f.Name, cover.ImgKey()) {
			foundCover = true
		}
		if strings.Contains(f.Name, page1.ImgKey()) {
			foundPage = true
		}
	}
	if !foundCover {
		t.Error("cover image not found in CBZ")
	}
	if !foundPage {
		t.Error("page image not found in CBZ")
	}
}

func TestCBZWriterWriteNoMetadata(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test-nometa.cbz")

	// Create temp ZIP storage at the path opts.ImgStorage() will use.
	storagePath := outputPath + ".tmp"
	sw, err := epubzip.NewStorageImageWriter(storagePath, "jpeg")
	if err != nil {
		t.Fatalf("create storage writer: %v", err)
	}

	page1 := epubimage.EPUBImage{Id: 1, Part: 0, Format: "jpeg"}
	fakeJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}

	if err := sw.AddRaw(page1.EPUBImgPath(), fakeJPEG); err != nil {
		t.Fatalf("add page1: %v", err)
	}
	if err := sw.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	opts := epuboptions.EPUBOptions{
		Output: outputPath,
	}
	parts := []OutputPart{
		{
			Cover:      page1,
			Images:     []epubimage.EPUBImage{},
			PartNumber: 1,
			TotalParts: 1,
			Metadata: PartMetadata{
				Title:  "",
				Series: "",
			},
		},
	}

	w := CBZWriter{}
	paths, err := w.Write(ctx, parts, opts)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 output path, got %d", len(paths))
	}

	// Verify output is a valid ZIP.
	zr, err := zip.OpenReader(outputPath)
	if err != nil {
		t.Fatalf("open resulting CBZ: %v", err)
	}
	defer zr.Close()

	if len(zr.File) == 0 {
		t.Fatal("expected at least 1 entry in CBZ")
	}

	// No ComicInfo.xml entry should exist.
	for _, f := range zr.File {
		if f.Name == "ComicInfo.xml" {
			t.Error("ComicInfo.xml should not be present when metadata is empty")
		}
	}
}

func TestCBZWriterWriteEmptyParts(t *testing.T) {
	w := CBZWriter{}
	opts := epuboptions.EPUBOptions{}
	_, err := w.Write(context.Background(), nil, opts)
	if err == nil {
		t.Error("expected error for empty parts")
	}
}

// --- Registry tests ---

func TestGetFormat(t *testing.T) {
	tests := []struct {
		format string
		want   bool // non-nil?
	}{
		{"cbz", true},
		{"kepub", true},
		{"html", true},
		{"unknown", false},
	}
	for _, tt := range tests {
		w := Get(tt.format)
		if (w != nil) != tt.want {
			t.Errorf("Get(%q) = %v, want non-nil=%v", tt.format, w, tt.want)
		}
	}
}

func TestAvailable(t *testing.T) {
	formats := Available()
	found := make(map[string]bool)
	for _, f := range formats {
		found[f] = true
	}
	for _, want := range []string{"cbz", "kepub", "html"} {
		if !found[want] {
			t.Errorf("Available() missing %q", want)
		}
	}
}

func TestRegisterGet(t *testing.T) {
	// Register a custom writer.
	customFormat := "testcustom"
	customExt := ".test"

	custom := func() OutputWriter {
		return testWriter{format: customFormat, ext: customExt}
	}
	Register(custom)

	w := Get(customFormat)
	if w == nil {
		t.Fatal("Get returned nil for registered custom format")
	}
	if w.Format() != customFormat {
		t.Errorf("Format() = %q, want %q", w.Format(), customFormat)
	}
	if w.Extension() != customExt {
		t.Errorf("Extension() = %q, want %q", w.Extension(), customExt)
	}
}

// testWriter is a minimal OutputWriter for testing registration.
type testWriter struct {
	format string
	ext    string
}

func (w testWriter) Format() string                       { return w.format }
func (w testWriter) Extension() string                    { return w.ext }
func (w testWriter) SupportsPartSplit() bool              { return false }
func (w testWriter) Write(_ context.Context, _ []OutputPart, _ epuboptions.EPUBOptions) ([]string, error) {
	return nil, nil
}

// --- CBZWriter interface checks ---

func TestCBZWriterInterface(t *testing.T) {
	w := CBZWriter{}
	if w.Format() != "cbz" {
		t.Errorf("Format() = %q, want %q", w.Format(), "cbz")
	}
	if w.Extension() != ".cbz" {
		t.Errorf("Extension() = %q, want %q", w.Extension(), ".cbz")
	}
	if w.SupportsPartSplit() {
		t.Error("SupportsPartSplit() should return false")
	}
}

// TestCBZWriterMultiPart covers the multi-part path of partOutputPath.
func TestCBZWriterMultiPart(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "comic.cbz")

	storagePath := outputPath + ".tmp"
	sw, err := epubzip.NewStorageImageWriter(storagePath, "jpeg")
	if err != nil {
		t.Fatalf("create storage writer: %v", err)
	}

	fakeJPEG := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}
	cover := epubimage.EPUBImage{Id: 0, Part: 0, Format: "jpeg"}
	page1 := epubimage.EPUBImage{Id: 1, Part: 0, Format: "jpeg"}
	if err := sw.AddRaw(cover.EPUBImgPath(), fakeJPEG); err != nil {
		t.Fatalf("add cover: %v", err)
	}
	if err := sw.AddRaw(page1.EPUBImgPath(), fakeJPEG); err != nil {
		t.Fatalf("add page1: %v", err)
	}
	if err := sw.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	opts := epuboptions.EPUBOptions{
		Output: outputPath,
	}
	parts := []OutputPart{
		{
			Cover:      cover,
			Images:     []epubimage.EPUBImage{page1},
			PartNumber: 1,
			TotalParts: 2,
			Metadata: PartMetadata{
				Title:  "Multi Part Comic",
				Series: "Multi Series",
			},
		},
	}

	w := CBZWriter{}
	paths, err := w.Write(ctx, parts, opts)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 output path, got %d", len(paths))
	}
	// For multi-part, the path should include "Part 1 of 2"
	expectedSuffix := "Part 1 of 2.cbz"
	if !strings.HasSuffix(paths[0], expectedSuffix) {
		t.Errorf("output path = %q, want suffix %q", paths[0], expectedSuffix)
	}

	// Verify output is a valid ZIP.
	zr, err := zip.OpenReader(paths[0])
	if err != nil {
		t.Fatalf("open multi-part CBZ: %v", err)
	}
	defer zr.Close()
	if len(zr.File) == 0 {
		t.Fatal("expected entries in multi-part CBZ")
	}
}

// --- xmlEscape tests ---

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"plain text", "plain text"},
		{"a & b", "a &amp; b"},
		{"a < b", "a &lt; b"},
		{"a > b", "a &gt; b"},
		{`he said "hello"`, "he said &quot;hello&quot;"},
		{"it's", "it&apos;s"},
		{"<tag attr=\"val\">&amp;</tag>", "&lt;tag attr=&quot;val&quot;&gt;&amp;amp;&lt;/tag&gt;"},
	}
	for _, tt := range tests {
		got := xmlEscape(tt.input)
		if got != tt.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- KEPUB helper function tests ---

func TestComputeViewPort(t *testing.T) {
	w := KEPUBWriter{}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			View: epuboptions.View{
				Width:  1000,
				Height: 800,
			},
		},
	}

	parts := []kepubPart{
		{
			Cover: epubimage.EPUBImage{OriginalAspectRatio: 0.75},
			Images: []epubimage.EPUBImage{
				{OriginalAspectRatio: 0.75},
				{OriginalAspectRatio: 0.75},
			},
		},
	}

	width, height := w.computeViewPort(parts, opts)
	if width <= 0 || height <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", width, height)
	}
	// With default AspectRatio=0, common aspect ratio 0.75 used.
	// viewWidth = height / 0.75 = 800/0.75 = 1066; since 1066 > 1000, use Width=1000
	// viewHeight = 1000 * 0.75 = 750
	if width != 1000 || height != 750 {
		t.Errorf("expected 1000x750, got %dx%d", width, height)
	}
}

func TestComputeViewPortAspectRatioMinusOne(t *testing.T) {
	w := KEPUBWriter{}
	opts := epuboptions.EPUBOptions{
		Image: epuboptions.Image{
			View: epuboptions.View{
				Width:       1024,
				Height:      768,
				AspectRatio: -1,
			},
		},
	}

	parts := []kepubPart{
		{
			Cover:  epubimage.EPUBImage{OriginalAspectRatio: 2.0},
			Images: []epubimage.EPUBImage{},
		},
	}

	width, height := w.computeViewPort(parts, opts)
	// AspectRatio = -1 means keep device dimensions unchanged.
	if width != 1024 || height != 768 {
		t.Errorf("expected 1024x768, got %dx%d", width, height)
	}
}

func TestGetTree(t *testing.T) {
	w := KEPUBWriter{}
	images := []epubimage.EPUBImage{
		{Path: "ch1", Name: "page01.jpg"},
		{Path: "ch1", Name: "page02.jpg"},
		{Path: "ch2", Name: "page03.jpg"},
	}

	tree := w.getTree(images, false)
	if tree == "" {
		t.Error("getTree returned empty string")
	}
	if !strings.Contains(tree, "ch1") {
		t.Error("getTree missing ch1")
	}
	if !strings.Contains(tree, "ch2") {
		t.Error("getTree missing ch2")
	}
	if !strings.Contains(tree, "page01.jpg") {
		t.Error("getTree missing page01.jpg")
	}
}

func TestGetTreeSkipFiles(t *testing.T) {
	w := KEPUBWriter{}
	images := []epubimage.EPUBImage{
		{Path: "ch1", Name: "page01.jpg"},
		{Path: "ch1", Name: "page02.jpg"},
	}

	tree := w.getTree(images, true)
	if tree == "" {
		t.Error("getTree returned empty string")
	}
	if !strings.Contains(tree, "ch1") {
		t.Error("getTree missing ch1")
	}
	if strings.Contains(tree, "page01.jpg") {
		t.Error("getTree should not contain filenames when skipFiles=true")
	}
}

// --- KEPUB Dry Run tests ---

func TestKEPUBWriterDryRun(t *testing.T) {
	ctx := context.Background()

	// Create a temp directory with a valid JPEG file.
	srcDir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test JPEG: %v", err)
	}
	jpegPath := filepath.Join(srcDir, "page01.jpg")
	if err := os.WriteFile(jpegPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write test JPEG: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "output.kepub.epub")

	opts := epuboptions.EPUBOptions{
		Input:  srcDir,
		Output: outputPath,
		Dry:    true,
		Image: epuboptions.Image{
			Format: "jpeg",
			View: epuboptions.View{
				Width:  600,
				Height: 800,
			},
		},
	}

	w := KEPUBWriter{}
	paths, err := w.Write(ctx, []OutputPart{}, opts)
	if err != nil {
		t.Fatalf("KEPUB dry run Write: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil paths for dry run, got %v", paths)
	}
}

func TestKEPUBWriterDryRunVerbose(t *testing.T) {
	ctx := context.Background()

	srcDir := t.TempDir()
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, nil); err != nil {
		t.Fatalf("encode test JPEG: %v", err)
	}
	jpegPath := filepath.Join(srcDir, "page01.jpg")
	if err := os.WriteFile(jpegPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write test JPEG: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "output-verbose.kepub.epub")

	opts := epuboptions.EPUBOptions{
		Input:      srcDir,
		Output:     outputPath,
		Dry:        true,
		DryVerbose: true,
		Quiet:      true, // suppress printf output
		Image: epuboptions.Image{
			Format:    "jpeg",
			HasCover:  true,
			View: epuboptions.View{
				Width:  600,
				Height: 800,
			},
		},
	}

	w := KEPUBWriter{}
	paths, err := w.Write(ctx, []OutputPart{}, opts)
	if err != nil {
		t.Fatalf("KEPUB dry run verbose Write: %v", err)
	}
	if paths != nil {
		t.Errorf("expected nil paths for dry run, got %v", paths)
	}
}
