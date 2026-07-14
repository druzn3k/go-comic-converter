package viewport

import (
	"testing"

	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

func TestComputeAspectRatioSingle(t *testing.T) {
	parts := []PartAspect{
		{Cover: ImageAspect{OriginalAspectRatio: 1.5}, Images: []ImageAspect{{OriginalAspectRatio: 1.5}}},
	}
	ratio := ComputeAspectRatio(parts)
	if ratio != 1.5 {
		t.Errorf("expected 1.5, got %f", ratio)
	}
}

func TestComputeAspectRatioMultiple(t *testing.T) {
	parts := []PartAspect{
		{
			Cover:  ImageAspect{OriginalAspectRatio: 1.5},
			Images: []ImageAspect{{OriginalAspectRatio: 1.5}, {OriginalAspectRatio: 1.5}},
		},
		{
			Cover:  ImageAspect{OriginalAspectRatio: 1.0},
			Images: []ImageAspect{{OriginalAspectRatio: 1.0}},
		},
	}
	ratio := ComputeAspectRatio(parts)
	// 1.5 appears 3 times, 1.0 appears 2 times → 1.5 should win
	if ratio != 1.5 {
		t.Errorf("expected 1.5 (most common), got %f", ratio)
	}
}

func TestComputeViewPortUseDevice(t *testing.T) {
	w, h := ComputeViewPort(nil, epuboptions.View{Width: 1200, Height: 1920, AspectRatio: -1})
	if w != 1200 || h != 1920 {
		t.Errorf("expected 1200x1920, got %dx%d", w, h)
	}
}

func TestComputeViewPortUseSource(t *testing.T) {
	parts := []PartAspect{
		{Cover: ImageAspect{OriginalAspectRatio: 1.5},
			Images: []ImageAspect{{OriginalAspectRatio: 1.5}}},
	}
	// AspectRatio=0 means use source images' most common ratio
	w, h := ComputeViewPort(parts, epuboptions.View{Width: 1200, Height: 1920, AspectRatio: 0})
	// aspect ratio 1.5 = height/width = 1.5, so width = 1920/1.5 = 1280
	// but width can't exceed 1200, so width=1200, height=1200*1.5=1800
	if w != 1200 || h != 1800 {
		t.Errorf("expected 1200x1800 (1.5 ratio), got %dx%d", w, h)
	}
}

func TestComputeViewPortCustomAspect(t *testing.T) {
	w, h := ComputeViewPort(nil, epuboptions.View{Width: 1200, Height: 1920, AspectRatio: 1.6})
	// aspect ratio 1.6, width = 1920/1.6 = 1200, height = 1920
	if w != 1200 || h != 1920 {
		t.Errorf("expected 1200x1920, got %dx%d", w, h)
	}
}
