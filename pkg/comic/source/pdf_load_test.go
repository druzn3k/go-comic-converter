package source

import (
	"context"
	"os"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
)

// TestPdfBytesSourceLoadWithImage opens sample.pdf and verifies that at least
// one image task is produced with a non-nil image.
func TestPdfBytesSourceLoadWithImage(t *testing.T) {
	t.Parallel()

	path := testdataPath("sample.pdf")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("test fixture not found:", path)
	}

	src := NewFromBytes(data, "sample.pdf", 0)

	ctx := context.Background()
	ch, count, err := src.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 page, got %d", count)
	}

	var tasks []epubimageloader.Task
	for task := range ch {
		tasks = append(tasks, task)
	}

	if len(tasks) != count {
		t.Errorf("expected %d tasks, got %d", count, len(tasks))
	}

	imageFound := false
	for i, task := range tasks {
		if task.Error != nil {
			t.Logf("task %d error (may be expected): %v", i, task.Error)
		}
		if task.Image != nil {
			imageFound = true
		}
	}
	if !imageFound {
		t.Error("no task produced a non-nil image")
	}
}

// TestPdfBytesSourceLoadEmptyPages uses the image-free PDF from
// createTestPDFBytes and verifies all tasks have Error set (CorruptedImage).
func TestPdfBytesSourceLoadEmptyPages(t *testing.T) {
	t.Parallel()

	data := createTestPDFBytes(t)
	src := NewFromBytes(data, "empty.pdf", 0)

	ctx := context.Background()
	ch, count, err := src.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 page, got %d", count)
	}

	var tasks []epubimageloader.Task
	for task := range ch {
		tasks = append(tasks, task)
	}

	if len(tasks) != count {
		t.Errorf("expected %d tasks, got %d", count, len(tasks))
	}
	for i, task := range tasks {
		if task.Error == nil {
			t.Errorf("task %d expected error for image-free PDF, got nil", i)
		}
	}
}

// TestPdfSourceLoadFromFile opens sample.pdf from disk and verifies that
// the source dispatches to pdfSource and Load returns at least one image.
func TestPdfSourceLoadFromFile(t *testing.T) {
	t.Parallel()

	path := testdataPath("sample.pdf")
	if _, err := os.Stat(path); err != nil {
		t.Skip("test fixture not found:", path)
	}

	src := New(path, 0)

	ctx := context.Background()
	ch, count, err := src.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if count < 1 {
		t.Fatalf("expected at least 1 page, got %d", count)
	}

	var tasks []epubimageloader.Task
	for task := range ch {
		tasks = append(tasks, task)
	}

	if len(tasks) != count {
		t.Errorf("expected %d tasks, got %d", count, len(tasks))
	}

	imageFound := false
	for i, task := range tasks {
		if task.Error != nil {
			t.Logf("task %d error: %v", i, task.Error)
		}
		if task.Image != nil {
			imageFound = true
		}
	}
	if !imageFound {
		t.Error("no task produced a non-nil image")
	}
}
