package epubwriter

import (
	"strings"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func TestXMLEscape(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"plain text", "plain text"},
		{"a & b", "a &amp; b"},
		{"a < b", "a &lt; b"},
		{"a > b", "a &gt; b"},
		{`he said "hello"`, "he said &quot;hello&quot;"},
		{"it's", "it&apos;s"},
		{"<tag attr=\"val\">&amp;</tag>", "&lt;tag attr=&quot;val&quot;&gt;&amp;amp;&lt;/tag&gt;"},
	}
	for _, tt := range tests {
		got := xmlEscape(tt.input)
		if got != tt.want {
			t.Errorf("xmlEscape(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestComputeViewPort(t *testing.T) {
	view := epuboptions.View{
		Width:  1000,
		Height: 800,
	}

	parts := []Part{
		{
			Cover: epubimage.EPUBImage{OriginalAspectRatio: 0.75},
			Images: []epubimage.EPUBImage{
				{OriginalAspectRatio: 0.75},
				{OriginalAspectRatio: 0.75},
			},
		},
	}

	width, height := computeViewPort(parts, view)
	if width <= 0 || height <= 0 {
		t.Errorf("expected positive dimensions, got %dx%d", width, height)
	}
	// With default AspectRatio=0, common aspect ratio 0.75 used.
	// viewWidth = height / 0.75 = 800/0.75 = 1066; since 1066 > 1000, use Width=1000
	// viewHeight = 1000 * 0.75 = 750
	if width != 1000 || height != 750 {
		t.Errorf("expected 1000x750, got %dx%d", width, height)
	}
}

func TestComputeViewPortAspectRatioMinusOne(t *testing.T) {
	view := epuboptions.View{
		Width:       1024,
		Height:      768,
		AspectRatio: -1,
	}

	parts := []Part{
		{
			Cover:  epubimage.EPUBImage{OriginalAspectRatio: 2.0},
			Images: []epubimage.EPUBImage{},
		},
	}

	width, height := computeViewPort(parts, view)
	// AspectRatio = -1 means keep device dimensions unchanged.
	if width != 1024 || height != 768 {
		t.Errorf("expected 1024x768, got %dx%d", width, height)
	}
}

func TestGetTree(t *testing.T) {
	images := []epubimage.EPUBImage{
		{Path: "ch1", Name: "page01.jpg"},
		{Path: "ch1", Name: "page02.jpg"},
		{Path: "ch2", Name: "page03.jpg"},
	}

	tree := getTree(images, false, false)
	if tree == "" {
		t.Error("getTree returned empty string")
	}
	if !strings.Contains(tree, "ch1") {
		t.Error("getTree missing ch1")
	}
	if !strings.Contains(tree, "ch2") {
		t.Error("getTree missing ch2")
	}
	if !strings.Contains(tree, "page01.jpg") {
		t.Error("getTree missing page01.jpg")
	}
}

func TestGetTreeSkipFiles(t *testing.T) {
	images := []epubimage.EPUBImage{
		{Path: "ch1", Name: "page01.jpg"},
		{Path: "ch1", Name: "page02.jpg"},
	}

	tree := getTree(images, true, false)
	if tree == "" {
		t.Error("getTree returned empty string")
	}
	if !strings.Contains(tree, "ch1") {
		t.Error("getTree missing ch1")
	}
	if strings.Contains(tree, "page01.jpg") {
		t.Error("getTree should not contain filenames when skipFiles=true")
	}
}

func TestGetTreeStripFirstDirectory(t *testing.T) {
	images := []epubimage.EPUBImage{
		{Path: "ch1", Name: "page01.jpg"},
		{Path: "ch1", Name: "page02.jpg"},
	}

	tree := getTree(images, true, true)
	if strings.Contains(tree, "ch1") {
		t.Error("getTree should strip single root directory")
	}
}

func TestGenerateContentOPFHasKoboStyleMeta(t *testing.T) {
	opf := generateContentOPF(
		"Test Comic",
		true,
		epuboptions.EPUBOptions{Image: epuboptions.Image{Format: "jpeg", View: epuboptions.View{Width: 600, Height: 800}}},
		"test-uid",
		"Test Publisher",
		"2026-01-01T00:00:00Z",
		Part{
			Cover:  epubimage.EPUBImage{Id: 1, Part: 0, Format: "jpeg"},
			Images: []epubimage.EPUBImage{{Id: 2, Part: 0, Format: "jpeg"}},
		},
		1,
		1,
		Variant{KoboStyle: true},
	)

	if !strings.Contains(opf, `name="kobo-style"`) {
		t.Error("content.opf missing kobo-style metadata")
	}
	if !strings.Contains(opf, `<package`) {
		t.Error("content.opf missing <package> root element")
	}
	if !strings.Contains(opf, `<metadata`) {
		t.Error("content.opf missing <metadata> element")
	}
	if !strings.Contains(opf, `dc:title`) {
		t.Error("content.opf missing dc:title")
	}
}

func TestGenerateContentOPFNamespace(t *testing.T) {
	opf := generateContentOPF(
		"Test",
		false,
		epuboptions.EPUBOptions{Image: epuboptions.Image{Format: "jpeg", View: epuboptions.View{Width: 600, Height: 800}}},
		"uid",
		"Publisher",
		"2026-01-01T00:00:00Z",
		Part{
			Cover:  epubimage.EPUBImage{Id: 1, Part: 0, Format: "jpeg"},
			Images: []epubimage.EPUBImage{{Id: 2, Part: 0, Format: "jpeg"}},
		},
		1,
		1,
		Variant{},
	)

	if !strings.Contains(opf, `http://www.idpf.org/2007/opf`) {
		t.Error("content.opf missing IDPF namespace")
	}
}
