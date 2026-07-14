package epubzip

import (
	"bytes"
	"compress/flate"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/image/webp"
)

func TestCompressImageJPEG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	result, err := CompressImage("test.jpg", "jpeg", img, 85)
	if err != nil {
		t.Fatalf("CompressImage JPEG: %v", err)
	}
	if result.Header == nil {
		t.Fatal("Header is nil")
	}
	if result.Header.Name != "test.jpg" {
		t.Errorf("expected name test.jpg, got %q", result.Header.Name)
	}

	// Data is flate-compressed; decompress first
	dr := flate.NewReader(bytes.NewReader(result.Data))
	defer dr.Close()
	decompressed, err := io.ReadAll(dr)
	if err != nil {
		t.Fatalf("decompressing JPEG data: %v", err)
	}

	decoded, err := jpeg.Decode(bytes.NewReader(decompressed))
	if err != nil {
		t.Fatalf("decoding JPEG: %v", err)
	}
	if decoded == nil {
		t.Fatal("decoded image is nil")
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("expected 100x100, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestCompressImagePNG(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	result, err := CompressImage("test.png", "png", img, 85)
	if err != nil {
		t.Fatalf("CompressImage PNG: %v", err)
	}
	if result.Header == nil {
		t.Fatal("Header is nil")
	}
	if result.Header.Name != "test.png" {
		t.Errorf("expected name test.png, got %q", result.Header.Name)
	}

	dr := flate.NewReader(bytes.NewReader(result.Data))
	defer dr.Close()
	decompressed, err := io.ReadAll(dr)
	if err != nil {
		t.Fatalf("decompressing PNG data: %v", err)
	}

	decoded, err := png.Decode(bytes.NewReader(decompressed))
	if err != nil {
		t.Fatalf("decoding PNG: %v", err)
	}
	if decoded == nil {
		t.Fatal("decoded image is nil")
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("expected 100x100, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestCompressImageWebP(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	result, err := CompressImage("test.webp", "webp", img, 85)
	if err != nil {
		t.Fatalf("CompressImage WebP: %v", err)
	}
	if result.Header == nil {
		t.Fatal("Header is nil")
	}

	dr := flate.NewReader(bytes.NewReader(result.Data))
	defer dr.Close()
	decompressed, err := io.ReadAll(dr)
	if err != nil {
		t.Fatalf("decompressing WebP data: %v", err)
	}

	decoded, err := webp.Decode(bytes.NewReader(decompressed))
	if err != nil {
		t.Fatalf("decoding WebP: %v", err)
	}
	if decoded == nil {
		t.Fatal("decoded image is nil")
	}
	bounds := decoded.Bounds()
	if bounds.Dx() != 100 || bounds.Dy() != 100 {
		t.Errorf("expected 100x100, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestCompressImageUnknownFormat(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	_, err := CompressImage("test.xyz", "bmp", img, 85)
	if err == nil {
		t.Fatal("expected error for unknown format, got nil")
	}
	if !strings.Contains(err.Error(), "unknown format") {
		t.Errorf("expected 'unknown format' in error, got: %v", err)
	}
}

func TestCompressRaw(t *testing.T) {
	original := []byte("hello world, this is test data for compress raw")
	result, err := CompressRaw("rawfile.bin", original)
	if err != nil {
		t.Fatalf("CompressRaw: %v", err)
	}
	if result.Header == nil {
		t.Fatal("Header is nil")
	}
	if result.Header.Name != "rawfile.bin" {
		t.Errorf("expected name rawfile.bin, got %q", result.Header.Name)
	}
	if result.Header.UncompressedSize64 != uint64(len(original)) {
		t.Errorf("uncompressed size: expected %d, got %d", len(original), result.Header.UncompressedSize64)
	}

	// Verify round-trip: decompress and compare
	dr := flate.NewReader(bytes.NewReader(result.Data))
	defer dr.Close()
	roundTripped, err := io.ReadAll(dr)
	if err != nil {
		t.Fatalf("decompressing raw data: %v", err)
	}
	if !bytes.Equal(original, roundTripped) {
		t.Fatal("round-trip data mismatch")
	}
}

func TestStorageWriterReader(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images.zip")

	w, err := NewStorageImageWriter(path, "jpeg")
	if err != nil {
		t.Fatalf("NewStorageImageWriter: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	err = w.Add("OEBPS/Images/image-001.jpg", img, 85)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	err = w.Close()
	if err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	r, err := NewStorageImageReader(path)
	if err != nil {
		t.Fatalf("NewStorageImageReader: %v", err)
	}
	defer r.Close()

	f := r.Get("OEBPS/Images/image-001.jpg")
	if f == nil {
		t.Fatal("Get returned nil for known path")
	}
	if f.Name != "OEBPS/Images/image-001.jpg" {
		t.Errorf("expected name OEBPS/Images/image-001.jpg, got %q", f.Name)
	}
}

func TestStorageReaderSize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "images.zip")

	w, err := NewStorageImageWriter(path, "jpeg")
	if err != nil {
		t.Fatalf("NewStorageImageWriter: %v", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	imgPath := "OEBPS/Images/image-001.jpg"
	err = w.Add(imgPath, img, 85)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	err = w.Close()
	if err != nil {
		t.Fatalf("Close writer: %v", err)
	}

	r, err := NewStorageImageReader(path)
	if err != nil {
		t.Fatalf("NewStorageImageReader: %v", err)
	}
	defer r.Close()

	if sz := r.Size(imgPath); sz == 0 {
		t.Error("Size for known path returned 0, expected >0")
	}

	if sz := r.Size("nonexistent/path.jpg"); sz != 0 {
		t.Errorf("Size for unknown path expected 0, got %d", sz)
	}
}

func TestStorageReaderNotFound(t *testing.T) {
	_, err := NewStorageImageReader("/nonexistent/file.zip")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Logf("error is not os.IsNotExist; got: %v", err)
	}
}
