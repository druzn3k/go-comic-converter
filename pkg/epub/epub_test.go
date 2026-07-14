package epub

import (
	"context"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubtemplates"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func createTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 5, 5))
	for x := range 5 {
		for y := range 5 {
			img.Set(x, y, color.RGBA{uint8(x * 50), uint8(y * 50), 128, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestNew(t *testing.T) {
	e := New(epuboptions.EPUBOptions{})
	if e == nil {
		t.Fatal("New() returned nil")
	}
	var _ EPUB = e
}

func TestTextTemplateEscaping(t *testing.T) {
	funcMap := template.FuncMap{
		"xmlEscape": func(s string) string {
			var escaped strings.Builder
			for _, r := range s {
				switch r {
				case '&':
					escaped.WriteString("&amp;")
				case '<':
					escaped.WriteString("&lt;")
				case '>':
					escaped.WriteString("&gt;")
				case '"':
					escaped.WriteString("&quot;")
				case '\'':
					escaped.WriteString("&apos;")
				default:
					escaped.WriteRune(r)
				}
			}
			return escaped.String()
		},
	}

	tmpl := template.Must(template.New("test").Funcs(funcMap).Parse(epubtemplates.Text))
	var buf strings.Builder
	err := tmpl.Execute(&buf, map[string]any{
		"Title":      `<script>alert("xss")</script>`,
		"ViewPort":   "width=1200,height=1920",
		"ImagePath":  "Images/img_1.jpeg",
		"ImageStyle": "max-width:1200px;max-height:1920px",
	})
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if strings.Contains(output, "<script>") {
		t.Errorf("Title should be XML-escaped, got: %s", output)
	}
	if !strings.Contains(output, "&lt;script&gt;") {
		t.Errorf("Expected &lt;script&gt; entities, got: %s", output)
	}
	if strings.Contains(output, `alt="<script>`) {
		t.Errorf("alt attribute should be XML-escaped, got: %s", output)
	}
}

func TestBlankTemplateEscaping(t *testing.T) {
	funcMap := template.FuncMap{
		"xmlEscape": func(s string) string {
			var escaped strings.Builder
			for _, r := range s {
				switch r {
				case '&':
					escaped.WriteString("&amp;")
				case '<':
					escaped.WriteString("&lt;")
				case '>':
					escaped.WriteString("&gt;")
				case '"':
					escaped.WriteString("&quot;")
				case '\'':
					escaped.WriteString("&apos;")
				default:
					escaped.WriteRune(r)
				}
			}
			return escaped.String()
		},
	}

	tmpl := template.Must(template.New("test").Funcs(funcMap).Parse(epubtemplates.Blank))
	var buf strings.Builder
	err := tmpl.Execute(&buf, map[string]any{
		"Title":    `Inject & escape "test"`,
		"ViewPort": "width=1200,height=1920",
	})
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()
	if strings.Contains(output, `"test"`) && strings.Contains(output, "&") {
		t.Errorf("Title should be XML-escaped, got: %s", output)
	}
}

func TestStyleTemplateRendering(t *testing.T) {
	funcMap := template.FuncMap{
		"xmlEscape": func(s string) string { return s },
	}
	tmpl := template.Must(template.New("test").Funcs(funcMap).Parse(epubtemplates.Style))
	var buf strings.Builder
	err := tmpl.Execute(&buf, map[string]any{
		"View": epuboptions.View{
			Width:  1200,
			Height: 1920,
			Color: epuboptions.Color{
				Foreground: "000",
				Background: "FFF",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if !strings.Contains(output, "color: #000") {
		t.Errorf("Expected foreground color #000, got: %s", output)
	}
	if !strings.Contains(output, "background: #FFF") {
		t.Errorf("Expected background color #FFF, got: %s", output)
	}
	if !strings.Contains(output, "width: 1200px") {
		t.Errorf("Expected width 1200px, got: %s", output)
	}
	if !strings.Contains(output, "height: 1920px") {
		t.Errorf("Expected height 1920px, got: %s", output)
	}
}

func TestContentTemplateXMLValidity(t *testing.T) {
	content := epubtemplates.Content{
		Title:        `Safe Title`,
		HasTitlePage: true,
		UID:          "test-uid",
		Author:       "Test Author",
		Publisher:    "Test Publisher",
		UpdatedAt:    "2026-01-01T00:00:00Z",
		ImageOptions: epuboptions.Image{
			View: epuboptions.View{
				Color: epuboptions.Color{Foreground: "000", Background: "FFF"},
			},
		},
		Cover: epubimage.EPUBImage{
			Id:     1,
			Part:   0,
			Format: "jpeg",
		},
		Images: []epubimage.EPUBImage{
			{Id: 2, Part: 0, Format: "jpeg"},
		},
		Current: 1,
		Total:   1,
	}

	output := content.String()
	if output == "" {
		t.Fatal("Content.String() returned empty string")
	}
	if !strings.Contains(output, "<?xml") {
		t.Error("Expected XML declaration")
	}
	if !strings.Contains(output, "<package") {
		t.Error("Expected package element")
	}
}

func TestErrImageCorruptedSentinel(t *testing.T) {
	if ErrImageCorrupted == nil {
		t.Fatal("ErrImageCorrupted should be a non-nil sentinel error")
	}
	if !errors.Is(ErrImageCorrupted, ErrImageCorrupted) {
		t.Error("errors.Is(ErrImageCorrupted, ErrImageCorrupted) should be true")
	}
	wrapped := fmt.Errorf("wrapped: %w", ErrImageCorrupted)
	if !errors.Is(wrapped, ErrImageCorrupted) {
		t.Error("errors.Is should work with wrapped ErrImageCorrupted")
	}
}

func TestWriteReturnsErrImageCorrupted(t *testing.T) {
	dir := t.TempDir()
	valid := createTestJPEG(t)
	if err := os.WriteFile(filepath.Join(dir, "good.jpg"), valid, 0644); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte{0xff, 0xd8, 0xff} // truncated JPEG header
	if err := os.WriteFile(filepath.Join(dir, "bad.jpg"), corrupt, 0644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "out.epub")
	opts := epuboptions.EPUBOptions{
		Input:  dir,
		Output: output,
		Image: epuboptions.Image{
			Format: "jpeg",
			View: epuboptions.View{
				Width:  1200,
				Height: 1920,
				Color: epuboptions.Color{
					Foreground: "000",
					Background: "FFF",
				},
			},
		},
	}

	e := New(opts)
	err := e.Write(context.Background())
	if err == nil {
		t.Fatal("expected ErrImageCorrupted with corrupt images, got nil")
	}
	if !errors.Is(err, ErrImageCorrupted) {
		t.Errorf("expected ErrImageCorrupted, got: %v", err)
	}
	if _, statErr := os.Stat(output); statErr != nil {
		t.Logf("output file may not exist: %v", statErr)
	}
}

func TestWriteStrictReturnsEarly(t *testing.T) {
	dir := t.TempDir()
	valid := createTestJPEG(t)
	if err := os.WriteFile(filepath.Join(dir, "good.jpg"), valid, 0644); err != nil {
		t.Fatal(err)
	}
	corrupt := []byte{0xff, 0xd8} // truncated
	if err := os.WriteFile(filepath.Join(dir, "bad.jpg"), corrupt, 0644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "out.epub")
	opts := epuboptions.EPUBOptions{
		Input:  dir,
		Output: output,
		Image: epuboptions.Image{
			Format: "jpeg",
			View: epuboptions.View{
				Width:  1200,
				Height: 1920,
				Color: epuboptions.Color{
					Foreground: "000",
					Background: "FFF",
				},
			},
		},
		Strict: true,
	}

	e := New(opts)
	err := e.Write(context.Background())
	if err == nil {
		t.Fatal("expected error in strict mode, got nil")
	}
	if _, statErr := os.Stat(output); statErr == nil {
		t.Error("expected no output file when strict aborts early")
	}
}
