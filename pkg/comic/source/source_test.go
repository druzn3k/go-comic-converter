package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func createTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	// Create a valid JPEG (minimal)
	data := []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01}
	if err := os.WriteFile(filepath.Join(dir, "page001.jpg"), data, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "page002.jpg"), data, 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestDirSource(t *testing.T) {
	dir := createTestDir(t)

	s := &dirSource{input: dir, sortMode: 1}
	tasks, count, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 images, got %d", count)
	}

	got := 0
	for task := range tasks {
		got++
		if task.Name == "" {
			t.Error("task has empty Name")
		}
		if task.Id < 0 {
			t.Error("task has negative Id")
		}
		_ = task // task.Image may be nil for truncated test data
	}
	if got != count {
		t.Errorf("received %d tasks, expected %d", got, count)
	}
}

func TestDirSourceSkipsSymlinks(t *testing.T) {
	dir := createTestDir(t)

	symPath := filepath.Join(dir, "link.jpg")
	if err := os.Symlink(filepath.Join(dir, "page001.jpg"), symPath); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	s := &dirSource{input: dir, sortMode: 1}
	_, count, err := s.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 images (symlink excluded), got %d", count)
	}
}

func TestDispatch(t *testing.T) {
	// Directory
	dir := createTestDir(t)
	s := New(dir, 1)
	if s == nil {
		t.Fatal("New(dir) returned nil")
	}
	if s.Name() != dir {
		t.Errorf("expected name %q, got %q", dir, s.Name())
	}

	// Unknown format
	s = New("/tmp/test.unknown", 1)
	if s == nil {
		t.Fatal("New(unknown) returned nil")
	}
	_, _, err := s.Load(context.Background())
	if err == nil {
		t.Error("expected error for unknown format")
	}
}

func TestSourceContextCancellation(t *testing.T) {
	dir := createTestDir(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	s := &dirSource{input: dir, sortMode: 1}
	_, _, err := s.Load(ctx)
	if err != nil {
		t.Logf("Load with cancelled context: %v", err)
	}
}
