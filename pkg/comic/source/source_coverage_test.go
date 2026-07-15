package source

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
)

// helper: writes a valid JPEG to the given path using image.NewRGBA + jpeg.Encode.
func writeTestJPEG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, nil); err != nil {
		t.Fatal(err)
	}
}

// helper: creates a minimal CBZ file containing a single JPEG, returns path.
func createTestCBZ(t *testing.T, dir string) string {
	t.Helper()
	path := filepath.Join(dir, "test.cbz")
	zf, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(zf)

	// Create a JPEG entry
	jf, err := w.Create("page001.jpg")
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	if err := jpeg.Encode(jf, img, nil); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDispatchCBZ(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cbz")
	// Create empty file so os.Stat succeeds
	if err := os.WriteFile(path, []byte{0}, 0644); err != nil {
		t.Fatal(err)
	}
	s := NewWithOpts(path, 0, true)
	if _, ok := s.(*cbzSource); !ok {
		t.Errorf("expected *cbzSource, got %T", s)
	}
}

func TestDispatchZip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.zip")
	if err := os.WriteFile(path, []byte{0}, 0644); err != nil {
		t.Fatal(err)
	}
	s := NewWithOpts(path, 0, true)
	if _, ok := s.(*cbzSource); !ok {
		t.Errorf("expected *cbzSource, got %T", s)
	}
}

func TestDispatchCBR(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.cbr")
	if err := os.WriteFile(path, []byte{0}, 0644); err != nil {
		t.Fatal(err)
	}
	s := NewWithOpts(path, 0, true)
	if _, ok := s.(*cbrSource); !ok {
		t.Errorf("expected *cbrSource, got %T", s)
	}
}

func TestDispatchRAR(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.rar")
	if err := os.WriteFile(path, []byte{0}, 0644); err != nil {
		t.Fatal(err)
	}
	s := NewWithOpts(path, 0, true)
	if _, ok := s.(*cbrSource); !ok {
		t.Errorf("expected *cbrSource, got %T", s)
	}
}

func TestDispatchPDF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pdf")
	if err := os.WriteFile(path, []byte{0}, 0644); err != nil {
		t.Fatal(err)
	}
	s := NewWithOpts(path, 0, true)
	if _, ok := s.(*pdfSource); !ok {
		t.Errorf("expected *pdfSource, got %T", s)
	}
}

func TestDispatchDir(t *testing.T) {
	dir := t.TempDir()
	s := NewWithOpts(dir, 0, true)
	if _, ok := s.(*dirSource); !ok {
		t.Errorf("expected *dirSource, got %T", s)
	}
}

func TestDispatchUnknown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.xyz")
	if err := os.WriteFile(path, []byte{0}, 0644); err != nil {
		t.Fatal(err)
	}
	s := NewWithOpts(path, 0, true)
	es, ok := s.(*errorSource)
	if !ok {
		t.Fatalf("expected *errorSource, got %T", s)
	}
	if es.err == nil {
		t.Error("expected non-nil error")
	}
}

func TestCBZSourceLoad(t *testing.T) {
	dir := t.TempDir()
	cbzPath := createTestCBZ(t, dir)

	s := NewWithOpts(cbzPath, 0, true)
	cbz, ok := s.(*cbzSource)
	if !ok {
		t.Fatalf("expected *cbzSource, got %T", s)
	}

	ctx := context.Background()
	tasks, count, err := cbz.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 image, got %d", count)
	}

	got := 0
	for task := range tasks {
		got++
		if task.Id < 0 {
			t.Error("task has negative Id")
		}
		if task.Name == "" {
			t.Error("task has empty Name")
		}
	}
	if got != count {
		t.Errorf("received %d tasks, expected %d", got, count)
	}
}

func TestDirSourceLoad(t *testing.T) {
	dir := t.TempDir()
	writeTestJPEG(t, filepath.Join(dir, "page001.jpg"))
	writeTestJPEG(t, filepath.Join(dir, "page002.jpg"))

	s := NewWithOpts(dir, 0, true)
	ds, ok := s.(*dirSource)
	if !ok {
		t.Fatalf("expected *dirSource, got %T", s)
	}

	ctx := context.Background()
	tasks, count, err := ds.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 images, got %d", count)
	}

	got := 0
	for task := range tasks {
		got++
		if task.Id < 0 {
			t.Error("task has negative Id")
		}
		if task.Name == "" {
			t.Error("task has empty Name")
		}
	}
	if got != count {
		t.Errorf("received %d tasks, expected %d", got, count)
	}
}

func TestErrorSourceLoad(t *testing.T) {
	testErr := errors.New("test error")
	es := &errorSource{err: testErr}

	ctx := context.Background()
	tasks, count, err := es.Load(ctx)
	if tasks != nil {
		t.Error("expected nil tasks channel")
	}
	if count != 0 {
		t.Errorf("expected 0 count, got %d", count)
	}
	if err != testErr {
		t.Errorf("expected %v, got %v", testErr, err)
	}
}

func TestErrorSourceName(t *testing.T) {
	testErr := errors.New("test error message")
	es := &errorSource{err: testErr}

	name := es.Name()
	if name != testErr.Error() {
		t.Errorf("expected %q, got %q", testErr.Error(), name)
	}

	// Also test via dispatch for a nonexistent path
	s := NewWithOpts("/nonexistent/path/does/not/exist", 0, true)
	if s.Name() == "" {
		t.Error("expected non-empty Name for error source")
	}
}

func TestDecodeWorkers(t *testing.T) {
	n := decodeWorkers()
	if n < 1 {
		t.Errorf("decodeWorkers() = %d, want >= 1", n)
	}
}

func TestMaxProcs(t *testing.T) {
	n := maxProcs()
	if n < 1 {
		t.Errorf("maxProcs() = %d, want >= 1", n)
	}
}

func TestNew(t *testing.T) {
	// Test that New calls NewWithOpts with includeWebpTiff=true
	dir := t.TempDir()
	s := New(dir, 0)
	ds, ok := s.(*dirSource)
	if !ok {
		t.Fatalf("expected *dirSource, got %T", s)
	}
	if !ds.includeWebpTiff {
		t.Error("New() should set includeWebpTiff=true")
	}

	// Test New with nonexistent path returns errorSource
	s = New("/nonexistent/path", 0)
	if _, ok := s.(*errorSource); !ok {
		t.Errorf("expected *errorSource for nonexistent path, got %T", s)
	}
}

func TestIsSupportedImage(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{"jpg", "test.jpg", true},
		{"jpeg", "test.jpeg", true},
		{"png", "test.png", true},
		{"webp", "test.webp", true},
		{"tiff", "test.tiff", true},
		{"gif", "test.gif", false},
		{"txt", "test.txt", false},
		{"hidden_jpg", ".hidden.jpg", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSupportedImage(tt.path)
			if got != tt.want {
				t.Errorf("isSupportedImage(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestNumberOfDigitsFmt(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "%01d"},
		{9, "%01d"},
		{10, "%02d"},
		{99, "%02d"},
		{100, "%03d"},
		{999, "%03d"},
		{1000, "%04d"},
	}
	for _, tt := range tests {
		got := numberOfDigitsFmt(tt.n)
		if got != tt.want {
			t.Errorf("numberOfDigitsFmt(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}

func TestSourceName(t *testing.T) {
	// dirSource Name
	ds := &dirSource{input: "/some/dir"}
	if ds.Name() != "/some/dir" {
		t.Errorf("dirSource.Name() = %q, want %q", ds.Name(), "/some/dir")
	}

	// cbzSource Name
	cz := &cbzSource{input: "/some/file.cbz"}
	if cz.Name() != "/some/file.cbz" {
		t.Errorf("cbzSource.Name() = %q, want %q", cz.Name(), "/some/file.cbz")
	}

	// cbrSource Name
	cr := &cbrSource{input: "/some/file.cbr"}
	if cr.Name() != "/some/file.cbr" {
		t.Errorf("cbrSource.Name() = %q, want %q", cr.Name(), "/some/file.cbr")
	}

	// pdfSource Name
	ps := &pdfSource{input: "/some/file.pdf"}
	if ps.Name() != "/some/file.pdf" {
		t.Errorf("pdfSource.Name() = %q, want %q", ps.Name(), "/some/file.pdf")
	}
}

func TestErrorSourceDoesNotImplementSource(t *testing.T) {
	// Ensure errorSource satisfies the Source interface
	var _ Source = (*errorSource)(nil)
}

func TestSourceTypeIdentity(t *testing.T) {
	// Verify each source type implements Source
	var _ Source = (*cbzSource)(nil)
	var _ Source = (*cbrSource)(nil)
	var _ Source = (*dirSource)(nil)
	var _ Source = (*pdfSource)(nil)
}

func TestLoadWithCancelledContext(t *testing.T) {
	dir := t.TempDir()
	writeTestJPEG(t, filepath.Join(dir, "page001.jpg"))

	s := NewWithOpts(dir, 0, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Some Load implementations handle cancelled context gracefully
	_, _, err := s.Load(ctx)
	// Just validate it doesn't panic; error is acceptable
	_ = err
}

func TestNoImagesFound(t *testing.T) {
	// Test that an error is returned when no images are found in a directory
	dir := t.TempDir()
	s := NewWithOpts(dir, 0, true)
	_, _, err := s.Load(context.Background())
	if err != epubimageloader.ErrNoImagesFound {
		t.Errorf("expected ErrNoImagesFound, got %v", err)
	}
}

func TestLoadWithValidCBZ(t *testing.T) {
	dir := t.TempDir()
	cbzPath := createTestCBZ(t, dir)

	s := NewWithOpts(cbzPath, 0, true)
	ctx := context.Background()
	tasks, count, err := s.Load(ctx)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 image, got %d", count)
	}

	for task := range tasks {
		if task.Error != nil {
			t.Errorf("task has error: %v", task.Error)
		}
		if task.Image == nil {
			t.Error("task.Image is nil")
		}
	}
}

// helper: creates a CBZ in memory (as bytes) containing a single JPEG.
func createTestCBZBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	jf, err := w.Create("page001.jpg")
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	if err := jpeg.Encode(jf, img, nil); err != nil {
		t.Fatal(err)
	}

	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// helper: creates a minimal valid PDF in memory (no images, but structurally valid).
func createTestPDFBytes(t *testing.T) []byte {
	t.Helper()
	// Build PDF with exact byte offsets for xref table.
	var pdf bytes.Buffer

	// Object 1 (Catalog)
	pdf.WriteString("%PDF-1.4\n")
	obj1 := pdf.Len()
	pdf.WriteString("1 0 obj\n")
	pdf.WriteString("<< /Type /Catalog /Pages 2 0 R >>\n")
	pdf.WriteString("endobj\n")

	// Object 2 (Pages)
	obj2 := pdf.Len()
	pdf.WriteString("2 0 obj\n")
	pdf.WriteString("<< /Type /Pages /Kids [3 0 R] /Count 1 >>\n")
	pdf.WriteString("endobj\n")

	// Object 3 (Page)
	obj3 := pdf.Len()
	pdf.WriteString("3 0 obj\n")
	pdf.WriteString("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>\n")
	pdf.WriteString("endobj\n")

	// xref table
	xref := pdf.Len()
	pdf.WriteString("xref\n")
	pdf.WriteString("0 4\n")
	pdf.WriteString("0000000000 65535 f \n")
	fmt.Fprintf(&pdf, "%010d 00000 n \n", obj1)
	fmt.Fprintf(&pdf, "%010d 00000 n \n", obj2)
	fmt.Fprintf(&pdf, "%010d 00000 n \n", obj3)
	pdf.WriteString("trailer\n")
	pdf.WriteString("<< /Size 4 /Root 1 0 R >>\n")
	pdf.WriteString("startxref\n")
	fmt.Fprintf(&pdf, "%d\n", xref)
	pdf.WriteString("%%EOF\n")

	return pdf.Bytes()
}

func TestNewFromBytesZip(t *testing.T) {
	data := createTestCBZBytes(t)
	s := NewFromBytes(data, "test.zip", 0)
	if _, ok := s.(*cbzBytesSource); !ok {
		t.Errorf("expected *cbzBytesSource, got %T", s)
	}
}

func TestNewFromBytesCBR(t *testing.T) {
	s := NewFromBytes([]byte{0}, "test.cbr", 0)
	if _, ok := s.(*cbrBytesSource); !ok {
		t.Errorf("expected *cbrBytesSource, got %T", s)
	}
}

func TestNewFromBytesRAR(t *testing.T) {
	s := NewFromBytes([]byte{0}, "test.rar", 0)
	if _, ok := s.(*cbrBytesSource); !ok {
		t.Errorf("expected *cbrBytesSource, got %T", s)
	}
}

func TestNewFromBytesPDF(t *testing.T) {
	data := createTestPDFBytes(t)
	s := NewFromBytes(data, "test.pdf", 0)
	if _, ok := s.(*pdfBytesSource); !ok {
		t.Errorf("expected *pdfBytesSource, got %T", s)
	}
}

func TestNewFromBytesUnknown(t *testing.T) {
	s := NewFromBytes([]byte{0}, "test.xyz", 0)
	if _, ok := s.(*errorSource); !ok {
		t.Errorf("expected *errorSource, got %T", s)
	}
}

func TestNewFromBytesWithOptsNoWebPTiff(t *testing.T) {
	data := createTestCBZBytes(t)
	s := NewFromBytesWithOpts(data, "test.cbz", 0, false)
	cbz, ok := s.(*cbzBytesSource)
	if !ok {
		t.Fatalf("expected *cbzBytesSource, got %T", s)
	}
	if cbz.includeWebpTiff {
		t.Error("expected includeWebpTiff=false")
	}
}

func TestCBZBytesSourceName(t *testing.T) {
	data := createTestCBZBytes(t)
	s := NewFromBytes(data, "my-archive.cbz", 0)
	if got := s.Name(); got != "my-archive.cbz" {
		t.Errorf("Name() = %q, want %q", got, "my-archive.cbz")
	}
}

func TestPDFBytesSourceName(t *testing.T) {
	data := createTestPDFBytes(t)
	s := NewFromBytes(data, "my-doc.pdf", 0)
	if got := s.Name(); got != "my-doc.pdf" {
		t.Errorf("Name() = %q, want %q", got, "my-doc.pdf")
	}
}

func TestCBRBytesSourceName(t *testing.T) {
	s := NewFromBytes([]byte{0}, "archive.cbr", 0)
	if got := s.Name(); got != "archive.cbr" {
		t.Errorf("Name() = %q, want %q", got, "archive.cbr")
	}
}

func TestCBZBytesSourceLoad(t *testing.T) {
	data := createTestCBZBytes(t)
	s := NewFromBytes(data, "test.cbz", 0)

	tasks, count, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 image, got %d", count)
	}

	for task := range tasks {
		if task.Error != nil {
			t.Errorf("task has error: %v", task.Error)
		}
		if task.Image == nil {
			t.Error("task.Image is nil")
		}
	}
}

func TestCBZBytesSourceLoadNoImages(t *testing.T) {
	// Create a CBZ with no image files (only a text file)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	tf, err := w.Create("readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	io.WriteString(tf, "not an image")
	w.Close()

	s := NewFromBytes(buf.Bytes(), "empty.cbz", 0)
	_, _, err = s.Load(context.Background())
	if err != epubimageloader.ErrNoImagesFound {
		t.Errorf("expected ErrNoImagesFound, got %v", err)
	}
}

func TestCBZBytesSourceLoadContextCancellation(t *testing.T) {
	data := createTestCBZBytes(t)
	s := NewFromBytes(data, "test.cbz", 0)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	tasks, _, err := s.Load(ctx)
	if err != nil {
		t.Fatalf("Load() should not error on cancelled context: %v", err)
	}

	// Drain the channel — tasks should be empty or stop early
	var got int
	for range tasks {
		got++
	}
	// With immediate cancellation, we may get 0 or 1 tasks depending on timing
	if got > 1 {
		t.Errorf("expected at most 1 task with cancelled context, got %d", got)
	}
}

func TestPDFBytesSourceLoad(t *testing.T) {
	data := createTestPDFBytes(t)
	s := NewFromBytes(data, "test.pdf", 0)

	tasks, count, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 page, got %d", count)
	}

	for task := range tasks {
		// PDF has no actual image data, so extraction will produce a corrupted image
		if task.Error == nil && task.Image == nil {
			t.Error("task should have error or an image")
		}
		_ = task
	}
}

func TestCBRBytesSourceLoadInvalidData(t *testing.T) {
	// RAR data must start with valid RAR signature; garbage data should error
	s := NewFromBytes([]byte("not a rar file"), "test.cbr", 0)
	_, _, err := s.Load(context.Background())
	if err == nil {
		t.Error("expected error for invalid RAR data")
	}
}
