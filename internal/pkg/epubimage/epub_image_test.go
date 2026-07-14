package epubimage

import (
	"strings"
	"testing"
)

func TestEPUBImage_ImgKey(t *testing.T) {
	img := EPUBImage{Id: 1, Part: 0, Format: "jpeg"}
	if got := img.ImgKey(); got != "img_1_p0" {
		t.Errorf("ImgKey() = %q, want %q", got, "img_1_p0")
	}

	// Part negative
	img = EPUBImage{Id: 2, Part: -1}
	if got := img.ImgKey(); got != "img_2_p-1" {
		t.Errorf("ImgKey() part negative = %q, want %q", got, "img_2_p-1")
	}
}

func TestEPUBImage_ImgPath(t *testing.T) {
	img := EPUBImage{Id: 1, Part: 0, Format: "jpeg"}
	if got := img.ImgPath(); got != "Images/img_1_p0.jpeg" {
		t.Errorf("ImgPath() = %q, want %q", got, "Images/img_1_p0.jpeg")
	}

	// Format empty
	img = EPUBImage{Id: 1, Part: 0, Format: ""}
	if got := img.ImgPath(); got != "Images/img_1_p0." {
		t.Errorf("ImgPath() empty format = %q, want %q", got, "Images/img_1_p0.")
	}
}

func TestEPUBImage_EPUBImgPath(t *testing.T) {
	img := EPUBImage{Id: 1, Part: 0, Format: "jpeg"}
	want := "OEBPS/Images/img_1_p0.jpeg"
	if got := img.EPUBImgPath(); got != want {
		t.Errorf("EPUBImgPath() = %q, want %q", got, want)
	}
}

func TestEPUBImage_MediaType(t *testing.T) {
	img := EPUBImage{Format: "jpeg"}
	if got := img.MediaType(); got != "image/jpeg" {
		t.Errorf("MediaType() = %q, want %q", got, "image/jpeg")
	}

	img = EPUBImage{Format: "png"}
	if got := img.MediaType(); got != "image/png" {
		t.Errorf("MediaType() = %q, want %q", got, "image/png")
	}

	// Empty format
	img = EPUBImage{Format: ""}
	if got := img.MediaType(); got != "image/" {
		t.Errorf("MediaType() empty format = %q, want %q", got, "image/")
	}
}

func TestEPUBImage_SpaceKey(t *testing.T) {
	img := EPUBImage{Id: 5}
	if got := img.SpaceKey(); got != "space_5" {
		t.Errorf("SpaceKey() = %q, want %q", got, "space_5")
	}
}

func TestEPUBImage_SpacePath(t *testing.T) {
	img := EPUBImage{Id: 5}
	if got := img.SpacePath(); got != "Text/space_5.xhtml" {
		t.Errorf("SpacePath() = %q, want %q", got, "Text/space_5.xhtml")
	}
}

func TestEPUBImage_EPUBSpacePath(t *testing.T) {
	img := EPUBImage{Id: 5}
	if got := img.EPUBSpacePath(); got != "OEBPS/Text/space_5.xhtml" {
		t.Errorf("EPUBSpacePath() = %q, want %q", got, "OEBPS/Text/space_5.xhtml")
	}
}

func TestEPUBImage_PageKey(t *testing.T) {
	img := EPUBImage{Id: 3, Part: 1}
	if got := img.PageKey(); got != "page_3_p1" {
		t.Errorf("PageKey() = %q, want %q", got, "page_3_p1")
	}
}

func TestEPUBImage_PagePath(t *testing.T) {
	img := EPUBImage{Id: 3, Part: 1}
	if got := img.PagePath(); got != "Text/page_3_p1.xhtml" {
		t.Errorf("PagePath() = %q, want %q", got, "Text/page_3_p1.xhtml")
	}
}

func TestEPUBImage_EPUBPagePath(t *testing.T) {
	img := EPUBImage{Id: 3, Part: 1}
	if got := img.EPUBPagePath(); got != "OEBPS/Text/page_3_p1.xhtml" {
		t.Errorf("EPUBPagePath() = %q, want %q", got, "OEBPS/Text/page_3_p1.xhtml")
	}
}

func TestEPUBImage_RelSize(t *testing.T) {
	tests := []struct {
		name          string
		img           EPUBImage
		viewWidth     int
		viewHeight    int
		wantWidth     int
		wantHeight    int
	}{
		{
			name:          "wider image scales to view width",
			img:           EPUBImage{Width: 2000, Height: 1000},
			viewWidth:     1000,
			viewHeight:    800,
			wantWidth:     1000,
			wantHeight:    500,
		},
		{
			name:          "taller image scales to view height",
			img:           EPUBImage{Width: 1000, Height: 2000},
			viewWidth:     1000,
			viewHeight:    800,
			wantWidth:     400,
			wantHeight:    800,
		},
		{
			name:          "zero view dimensions return zero",
			img:           EPUBImage{Width: 100, Height: 100},
			viewWidth:     0,
			viewHeight:    0,
			wantWidth:     0,
			wantHeight:    0,
		},
		{
			name:          "zero source dimensions return zero",
			img:           EPUBImage{Width: 0, Height: 0},
			viewWidth:     1000,
			viewHeight:    800,
			wantWidth:     0,
			wantHeight:    0,
		},
		{
			name:          "perfect square scales to fill taller viewport",
			img:           EPUBImage{Width: 800, Height: 800},
			viewWidth:     1600,
			viewHeight:    1200,
			wantWidth:     1200,
			wantHeight:    1200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rw, rh := tt.img.RelSize(tt.viewWidth, tt.viewHeight)
			if rw != tt.wantWidth || rh != tt.wantHeight {
				t.Errorf("RelSize(%d,%d) = (%d,%d), want (%d,%d)",
					tt.viewWidth, tt.viewHeight, rw, rh, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

func TestEPUBImage_ImgStyle(t *testing.T) {
	// Standard centered image
	img := EPUBImage{Width: 1600, Height: 1200, Position: ""}
	style := img.ImgStyle(800, 600, "")
	if !strings.Contains(style, "width:") {
		t.Errorf("ImgStyle should contain width, got %q", style)
	}
	if !strings.Contains(style, "height:") {
		t.Errorf("ImgStyle should contain height, got %q", style)
	}
	if !strings.Contains(style, "top:") {
		t.Errorf("ImgStyle should contain top, got %q", style)
	}
	if !strings.Contains(style, "left:") {
		t.Errorf("ImgStyle should contain left for centered, got %q", style)
	}

	// Left-aligned (rendition:page-spread-left)
	img = EPUBImage{Width: 1600, Height: 1200, Position: "rendition:page-spread-left"}
	style = img.ImgStyle(800, 600, "")
	if !strings.Contains(style, "right:0") {
		t.Errorf("ImgStyle left-aligned should contain right:0, got %q", style)
	}

	// Right-aligned (rendition:page-spread-right)
	img = EPUBImage{Width: 1600, Height: 1200, Position: "rendition:page-spread-right"}
	style = img.ImgStyle(800, 600, "")
	if !strings.Contains(style, "left:0") {
		t.Errorf("ImgStyle right-aligned should contain left:0, got %q", style)
	}

	// Explicit align parameter
	img = EPUBImage{Width: 1600, Height: 1200}
	style = img.ImgStyle(800, 600, "margin:auto")
	if !strings.Contains(style, "margin:auto") {
		t.Errorf("ImgStyle with explicit align should contain %q, got %q", "margin:auto", style)
	}
}

func TestEPUBImage_PartKey(t *testing.T) {
	img := EPUBImage{Id: 1, Part: 0}
	if got := img.PartKey(); got != "1_p0" {
		t.Errorf("PartKey() = %q, want %q", got, "1_p0")
	}

	// Part negative
	img = EPUBImage{Id: 1, Part: -1}
	if got := img.PartKey(); got != "1_p-1" {
		t.Errorf("PartKey() negative part = %q, want %q", got, "1_p-1")
	}
}
