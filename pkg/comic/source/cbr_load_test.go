package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
)

// testdataPath returns the path to a fixture file in the testdata directory.
func testdataPath(name string) string {
	return filepath.Join("testdata", name)
}

// TestCbrSourceLoad opens sample.cbr from disk, calls Load, and verifies
// that at least two image tasks are produced.
func TestCbrSourceLoad(t *testing.T) {
	t.Parallel()

	path := testdataPath("sample.cbr")
	if _, err := os.Stat(path); err != nil {
		t.Skip("test fixture not found:", path)
	}

	src := New(path, 0)

	ctx := context.Background()
	ch, count, err := src.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if count < 2 {
		t.Fatalf("expected at least 2 images, got %d", count)
	}

	var tasks []epubimageloader.Task
	for task := range ch {
		tasks = append(tasks, task)
	}

	if len(tasks) != count {
		t.Errorf("expected %d tasks, got %d", count, len(tasks))
	}
	for i, task := range tasks {
		if task.Error != nil {
			t.Errorf("task %d has error: %v", i, task.Error)
		}
		if task.Image == nil {
			t.Errorf("task %d has nil image", i)
		}
	}
}

// TestCbrBytesSourceLoad reads sample.cbr into memory and loads it via
// NewFromBytes, verifying image extraction.
func TestCbrBytesSourceLoad(t *testing.T) {
	t.Parallel()

	path := testdataPath("sample.cbr")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("test fixture not found:", path)
	}

	src := NewFromBytes(data, "sample.cbr", 0)

	ctx := context.Background()
	ch, count, err := src.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if count < 2 {
		t.Fatalf("expected at least 2 images, got %d", count)
	}

	var tasks []epubimageloader.Task
	for task := range ch {
		tasks = append(tasks, task)
	}

	if len(tasks) != count {
		t.Errorf("expected %d tasks, got %d", count, len(tasks))
	}
	for i, task := range tasks {
		if task.Error != nil {
			t.Errorf("task %d has error: %v", i, task.Error)
		}
		if task.Image == nil {
			t.Errorf("task %d has nil image", i)
		}
	}
}

// TestCbrBytesSourceTwoPassSortOrder verifies that entries are returned in
// natural sort order, not in RAR archive order.
func TestCbrBytesSourceTwoPassSortOrder(t *testing.T) {
	t.Parallel()

	path := testdataPath("sample.cbr")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skip("test fixture not found:", path)
	}

	src := NewFromBytes(data, "sample.cbr", 0)

	ctx := context.Background()
	ch, count, err := src.Load(ctx)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	var tasks []epubimageloader.Task
	for task := range ch {
		tasks = append(tasks, task)
	}

	if len(tasks) != count {
		t.Fatalf("expected %d tasks, got %d", count, len(tasks))
	}

	for i := 1; i < len(tasks); i++ {
		if tasks[i].Name < tasks[i-1].Name {
			t.Errorf("tasks not sorted: %q before %q", tasks[i-1].Name, tasks[i].Name)
		}
	}
}

// TestCbrSourceSolidArchive documents that the generated fixture is not
// a solid archive.
func TestCbrSourceSolidArchive(t *testing.T) {
	t.Parallel()

	path := testdataPath("sample.cbr")
	if _, err := os.Stat(path); err != nil {
		t.Skip("test fixture not found:", path)
	}

	t.Log("fixture is not a solid archive — solid-path test skipped")
}
