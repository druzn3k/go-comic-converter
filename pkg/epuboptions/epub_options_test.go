package epuboptions

import (
	"testing"
)

func TestWorkersRatio(t *testing.T) {
	// 50% of 10 workers = 5
	opts := EPUBOptions{Workers: 10}
	if got := opts.WorkersRatio(50); got != 5 {
		t.Errorf("WorkersRatio(50) with Workers=10 = %d, want 5", got)
	}

	// 0% always returns at least 1
	if got := opts.WorkersRatio(0); got != 1 {
		t.Errorf("WorkersRatio(0) with Workers=10 = %d, want 1", got)
	}

	// 100% of 10 = 10
	if got := opts.WorkersRatio(100); got != 10 {
		t.Errorf("WorkersRatio(100) with Workers=10 = %d, want 10", got)
	}

	// 0% with Workers=0 also returns min 1
	opts = EPUBOptions{Workers: 0}
	if got := opts.WorkersRatio(0); got != 1 {
		t.Errorf("WorkersRatio(0) with Workers=0 = %d, want 1", got)
	}
}

func TestImgStorage(t *testing.T) {
	opts := EPUBOptions{Output: "/path/to/output.epub"}
	if got := opts.ImgStorage(); got != "/path/to/output.epub.tmp" {
		t.Errorf("ImgStorage() = %q, want %q", got, "/path/to/output.epub.tmp")
	}

	opts = EPUBOptions{Output: "mybook.epub"}
	if got := opts.ImgStorage(); got != "mybook.epub.tmp" {
		t.Errorf("ImgStorage() = %q, want %q", got, "mybook.epub.tmp")
	}
}

func TestCropDefaults(t *testing.T) {
	var c Crop
	if c.Enabled {
		t.Error("Crop.Enabled should default to false")
	}
	if c.Left != 0 {
		t.Errorf("Crop.Left should default to 0, got %d", c.Left)
	}
	if c.Up != 0 {
		t.Errorf("Crop.Up should default to 0, got %d", c.Up)
	}
	if c.Right != 0 {
		t.Errorf("Crop.Right should default to 0, got %d", c.Right)
	}
	if c.Bottom != 0 {
		t.Errorf("Crop.Bottom should default to 0, got %d", c.Bottom)
	}
	if c.Limit != 0 {
		t.Errorf("Crop.Limit should default to 0, got %d", c.Limit)
	}
	if c.SkipIfLimitReached {
		t.Error("Crop.SkipIfLimitReached should default to false")
	}
}

func TestViewDimension(t *testing.T) {
	// Landscape
	v := View{Width: 1200, Height: 800}
	if got := v.Dimension(); got != "1200x800" {
		t.Errorf("Dimension() = %q, want %q", got, "1200x800")
	}
	if got := v.Port(); got != "width=1200,height=800" {
		t.Errorf("Port() = %q, want %q", got, "width=1200,height=800")
	}

	// Portrait
	v = View{Width: 800, Height: 1200}
	if got := v.Dimension(); got != "800x1200" {
		t.Errorf("Dimension() = %q, want %q", got, "800x1200")
	}
	if got := v.Port(); got != "width=800,height=1200" {
		t.Errorf("Port() = %q, want %q", got, "width=800,height=1200")
	}

	// Zero values
	v = View{}
	if got := v.Dimension(); got != "0x0" {
		t.Errorf("Dimension() zero = %q, want %q", got, "0x0")
	}
}
