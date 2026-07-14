package comic

import (
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
)

func TestToPartAspects(t *testing.T) {
	parts := []Part{
		{
			Cover: epubimage.EPUBImage{OriginalAspectRatio: 1.5},
			Images: []epubimage.EPUBImage{
				{OriginalAspectRatio: 0.75},
				{OriginalAspectRatio: 1.33},
			},
		},
		{
			Cover: epubimage.EPUBImage{OriginalAspectRatio: 2.0},
			Images: []epubimage.EPUBImage{
				{OriginalAspectRatio: 1.0},
			},
		},
	}

	aspects := toPartAspects(parts)
	if len(aspects) != 2 {
		t.Fatalf("expected 2 aspects, got %d", len(aspects))
	}

	if aspects[0].Cover.OriginalAspectRatio != 1.5 {
		t.Errorf("expected cover ratio 1.5, got %f", aspects[0].Cover.OriginalAspectRatio)
	}
	if len(aspects[0].Images) != 2 {
		t.Errorf("expected 2 images in part 0, got %d", len(aspects[0].Images))
	}
	if aspects[0].Images[0].OriginalAspectRatio != 0.75 {
		t.Errorf("expected image ratio 0.75, got %f", aspects[0].Images[0].OriginalAspectRatio)
	}
	if aspects[1].Cover.OriginalAspectRatio != 2.0 {
		t.Errorf("expected cover ratio 2.0, got %f", aspects[1].Cover.OriginalAspectRatio)
	}
	if len(aspects[1].Images) != 1 {
		t.Errorf("expected 1 image in part 1, got %d", len(aspects[1].Images))
	}
}

func TestToPartAspectsEmpty(t *testing.T) {
	aspects := toPartAspects([]Part{})
	if len(aspects) != 0 {
		t.Errorf("expected 0 aspects for empty parts, got %d", len(aspects))
	}
}
