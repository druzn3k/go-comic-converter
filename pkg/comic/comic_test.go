package comic

import (
	"testing"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

func TestOptionsAlias(t *testing.T) {
	var opts Options
	_ = opts
}

func TestOptionsAssignability(t *testing.T) {
	eo := epuboptions.EPUBOptions{
		Title: "test",
	}
	var opts Options = eo
	if opts.Title != "test" {
		t.Errorf("expected title 'test', got %q", opts.Title)
	}
}

func TestPartFields(t *testing.T) {
	p := Part{
		Cover:  epubimage.EPUBImage{Name: "cover.jpg"},
		Images: []epubimage.EPUBImage{{Name: "page1.jpg"}, {Name: "page2.jpg"}},
	}
	if p.Cover.Name != "cover.jpg" {
		t.Errorf("expected cover name 'cover.jpg', got %q", p.Cover.Name)
	}
	if len(p.Images) != 2 {
		t.Errorf("expected 2 images, got %d", len(p.Images))
	}
}

func TestRegistryRegisterLookup(t *testing.T) {
	r := newRegistry[string]()
	r.register("key1", "value1")

	val, ok := r.lookup("key1")
	if !ok {
		t.Fatal("expected to find key1")
	}
	if val != "value1" {
		t.Errorf("expected 'value1', got %q", val)
	}
}

func TestRegistryLookupMissing(t *testing.T) {
	r := newRegistry[string]()
	_, ok := r.lookup("nonexistent")
	if ok {
		t.Error("expected false for missing key")
	}
}

func TestRegistryNames(t *testing.T) {
	r := newRegistry[int]()
	r.register("a", 1)
	r.register("b", 2)
	r.register("c", 3)

	names := r.names()
	if len(names) != 3 {
		t.Fatalf("expected 3 names, got %d", len(names))
	}

	seen := make(map[string]bool)
	for _, n := range names {
		seen[n] = true
	}
	for _, expected := range []string{"a", "b", "c"} {
		if !seen[expected] {
			t.Errorf("expected name %q not found", expected)
		}
	}
}

func TestRegistryConcurrentSafe(t *testing.T) {
	r := newRegistry[int]()
	done := make(chan struct{})
	go func() {
		for range 100 {
			r.register("key", 0)
		}
		close(done)
	}()
	for range 100 {
		_, _ = r.lookup("key")
	}
	<-done
}
