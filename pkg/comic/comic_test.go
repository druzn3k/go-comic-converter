package comic

import (
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
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
