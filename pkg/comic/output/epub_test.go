package output

import (
	"context"
	"testing"

	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

func TestEPUBWriterFormat(t *testing.T) {
	w := EPUBWriter{}
	if got := w.Format(); got != "epub" {
		t.Errorf("expected 'epub', got %q", got)
	}
}

func TestEPUBWriterExtension(t *testing.T) {
	w := EPUBWriter{}
	if got := w.Extension(); got != ".epub" {
		t.Errorf("expected '.epub', got %q", got)
	}
}

func TestEPUBWriterSupportsPartSplit(t *testing.T) {
	w := EPUBWriter{}
	if !w.SupportsPartSplit() {
		t.Error("expected SupportsPartSplit to be true")
	}
}

func TestEPUBWriterRegistered(t *testing.T) {
	w := Get("epub")
	if w == nil {
		t.Fatal("expected EPUB writer to be registered")
	}
	if w.Format() != "epub" {
		t.Errorf("expected format 'epub', got %q", w.Format())
	}
}

func TestEPUBWriterWriteNoInput(t *testing.T) {
	// Writing with no input should fail gracefully
	w := EPUBWriter{}
	paths, err := w.Write(context.Background(), nil, epuboptions.EPUBOptions{})
	if err == nil {
		t.Error("expected error for empty options, got nil")
	}
	if paths != nil {
		t.Errorf("expected nil paths, got %v", paths)
	}
}
