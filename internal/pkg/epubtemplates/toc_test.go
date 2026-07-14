package epubtemplates

import (
	"strings"
	"testing"

	"github.com/beevik/etree"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
)

func TestToc_EmptyPath(t *testing.T) {
	// PDF sources have empty Path — entries should still appear in TOC
	images := []epubimage.EPUBImage{
		{Id: 1, Part: 0, Path: "", Name: "Page 1", Format: "jpeg"},
		{Id: 2, Part: 0, Path: "", Name: "Page 2", Format: "jpeg"},
	}
	result := Toc("Test TOC", false, false, images)

	doc := etree.NewDocument()
	if err := doc.ReadFromString(result); err != nil {
		t.Fatalf("failed to parse TOC output: %v", err)
	}

	// Count <li> elements under root <ol> (excluding the "beginning" entry)
	rootOl := doc.FindElement("//ol")
	if rootOl == nil {
		t.Fatal("root <ol> not found in TOC output")
	}

	lis := rootOl.FindElements("./li")
	// First <li> is the "beginning" entry, so we should have 1 + 2 = 3 total
	if len(lis) != 3 {
		t.Errorf("expected 3 <li> elements (1 beginning + 2 images), got %d", len(lis))
	}

	// Check the second <li> (index 1) points to the first image
	if len(lis) >= 2 {
		link := lis[1].FindElement("./a")
		if link == nil {
			t.Error("<a> not found in image entry")
		} else {
			href := link.SelectAttrValue("href", "")
			if href != "Text/page_1_p0.xhtml" {
				t.Errorf("unexpected href: %q", href)
			}
			if link.Text() != "Page 1" {
				t.Errorf("unexpected link text: %q", link.Text())
			}
		}
	}
}

func TestToc_NonEmptyPath(t *testing.T) {
	// Normal (non-PDF) sources with directory-structured paths
	images := []epubimage.EPUBImage{
		{Id: 1, Part: 0, Path: "Chapter 1", Name: "page_001", Format: "jpeg"},
		{Id: 2, Part: 0, Path: "Chapter 1", Name: "page_002", Format: "jpeg"},
		{Id: 3, Part: 0, Path: "Chapter 2", Name: "page_003", Format: "jpeg"},
	}
	result := Toc("Test TOC", false, false, images)

	doc := etree.NewDocument()
	if err := doc.ReadFromString(result); err != nil {
		t.Fatalf("failed to parse TOC output: %v", err)
	}

	// Should have directory entries
	if !strings.Contains(result, "Chapter 1") {
		t.Error("TOC missing 'Chapter 1' entry")
	}
	if !strings.Contains(result, "Chapter 2") {
		t.Error("TOC missing 'Chapter 2' entry")
	}
}

func TestToc_MixedPaths(t *testing.T) {
	// Mix of empty (PDF) and non-empty paths
	images := []epubimage.EPUBImage{
		{Id: 1, Part: 0, Path: "", Name: "Cover", Format: "jpeg"},
		{Id: 2, Part: 0, Path: "Chapter 1", Name: "page_001", Format: "jpeg"},
	}
	result := Toc("Mixed TOC", false, false, images)

	doc := etree.NewDocument()
	if err := doc.ReadFromString(result); err != nil {
		t.Fatalf("failed to parse TOC output: %v", err)
	}

	rootOl := doc.FindElement("//ol")
	if rootOl == nil {
		t.Fatal("root <ol> not found")
	}

	lis := rootOl.FindElements("./li")
	// Beginning (1) + Cover (1) + Chapter 1 (1) = 3 direct children of root <ol>
	if len(lis) < 3 {
		t.Errorf("expected at least 3 direct <li> children under root <ol>, got %d", len(lis))
	}

	// Verify the Cover entry (empty path) is a direct child
	foundCover := false
	for _, li := range lis {
		link := li.FindElement("./a")
		if link != nil && link.Text() == "Cover" {
			foundCover = true
			href := link.SelectAttrValue("href", "")
			if href != "Text/page_1_p0.xhtml" {
				t.Errorf("Cover href: %q, want %q", href, "Text/page_1_p0.xhtml")
			}
			break
		}
	}
	if !foundCover {
		t.Error("Cover entry not found in TOC")
	}
}
