package epub

import (
	"strings"
	"testing"
	"text/template"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubtemplates"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

func TestNew(t *testing.T) {
	e := New(epuboptions.EPUBOptions{})
	if e == nil {
		t.Fatal("New() returned nil")
	}
	// Verify it implements EPUB interface
	var _ EPUB = e
}

func TestTextTemplateEscaping(t *testing.T) {
	// Verify that text/template with xmlEscape funcmap escapes Title
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

	// Title appears in <title> and alt="" — must be escaped
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
	// Content.String() uses etree which should properly escape
	content := epubtemplates.Content{
		Title:        `Safe Title & <test>`,
		HasTitlePage: true,
		UID:          "test-uid",
		Author:       `Test & Author <test@example.com>`,
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

	// Verify basic XML structure
	if !strings.Contains(output, "<?xml") {
		t.Error("Expected XML declaration")
	}
	if !strings.Contains(output, "<package") {
		t.Error("Expected package element")
	}

	// Verify Title and Author are XML-escaped by etree
	if strings.Contains(output, "& <test>") {
		t.Error("Title should be XML-escaped, got raw: & <test>")
	}
}
