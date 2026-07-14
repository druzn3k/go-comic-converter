package source

import (
	"archive/zip"
	"context"
	"errors"
	"image"
	"image/jpeg"
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
