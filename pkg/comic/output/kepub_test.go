package output

import (
	"archive/zip"
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func testImage() epubimage.EPUBImage {
	return epubimage.EPUBImage{
		Id:     1,
		Part:   0,
		Width:  600,
		Height: 800,
		Format: "jpeg",
		Path:   "ch1",
		Name:   "page01.jpg",
	}
}

func testImageOptions() epuboptions.Image {
	return epuboptions.Image{
		Format: "jpeg",
		View:   epuboptions.View{Width: 600, Height: 800},
	}
}

func TestKEPUBWriterFormat(t *testing.T) {
	w := KEPUBWriter{}
	if w.Format() != "kepub" {
		t.Errorf("Format() = %q, want %q", w.Format(), "kepub")
	}
}

func TestKEPUBWriterExtension(t *testing.T) {
	w := KEPUBWriter{}
	if w.Extension() != ".kepub.epub" {
		t.Errorf("Extension() = %q, want %q", w.Extension(), ".kepub.epub")
	}
}

func TestKEPUBWriterSupportsPartSplit(t *testing.T) {
	w := KEPUBWriter{}
	if !w.SupportsPartSplit() {
		t.Error("KEPUB should support part splitting")
	}
}

func TestKEPUBTextTemplateHasKobolinkDiv(t *testing.T) {
	if !strings.Contains(kepubTextTemplate, `<div class="kobolink">`) {
		t.Error("KEPUB text template missing kobolink div wrapper")
	}
	if !strings.Contains(kepubTextTemplate, `</div>`) {
		t.Error("KEPUB text template missing closing div")
	}
}

func TestKEPUBContentOPFHasKoboStyleMeta(t *testing.T) {
	w := KEPUBWriter{}
	opf := w.generateContentOPF(kepubContentData{
		Title:        "Test Comic",
		UID:          "test-uid",
		Author:       "Test Author",
		Current:      1,
		Total:        1,
		ImageOptions: testImageOptions(),
		Images:       []epubimage.EPUBImage{testImage()},
	})
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

func TestKEPUBContentOPFNamespace(t *testing.T) {
	w := KEPUBWriter{}
	opf := w.generateContentOPF(kepubContentData{
		Title:        "Test",
		UID:          "uid",
		Author:       "Author",
		Current:      1,
		Total:        1,
		ImageOptions: testImageOptions(),
		Images:       []epubimage.EPUBImage{testImage()},
	})
	if !strings.Contains(opf, `http://www.idpf.org/2007/opf`) {
		t.Error("content.opf missing IDPF namespace")
	}
}

func TestKEPUBWriterRegistration(t *testing.T) {
	w := Get("kepub")
	if w == nil {
		t.Fatal("KEPUB writer not registered in registry")
	}
	if w.Format() != "kepub" {
		t.Errorf("registered KEPUB writer Format() = %q, want %q", w.Format(), "kepub")
	}
}

func TestKEPUBWriterAvailability(t *testing.T) {
	avail := Available()
	found := false
	for _, f := range avail {
		if f == "kepub" {
			found = true
			break
		}
	}
	if !found {
		t.Error("kepub not found in Available() formats")
	}
}

func TestKEPUBZIPMimetype(t *testing.T) {
	dir := t.TempDir()
	outputPath := dir + "/test.kepub.epub"

	wz, err := createTestKEPUBZip(outputPath)
	if err != nil {
		t.Fatalf("failed to create test KEPUB zip: %v", err)
	}

	found := false
	for _, f := range wz.File {
		if f.Name == "mimetype" {
			found = true
			if f.Method != zip.Store {
				t.Error("mimetype must be stored (uncompressed)")
			}
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			_, err = buf.ReadFrom(rc)
			rc.Close()
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != "application/epub+zip" {
				t.Errorf("mimetype content = %q, want %q", buf.String(), "application/epub+zip")
			}
		}
	}
	if !found {
		t.Error("mimetype entry not found in KEPUB zip")
	}
}

func createTestKEPUBZip(path string) (*zip.ReadCloser, error) {
	f, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	wz := zip.NewWriter(f)

	t, err := wz.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		wz.Close()
		f.Close()
		return nil, err
	}
	t.Write([]byte("application/epub+zip"))

	c, err := wz.CreateHeader(&zip.FileHeader{
		Name:   "META-INF/container.xml",
		Method: zip.Deflate,
	})
	if err != nil {
		wz.Close()
		f.Close()
		return nil, err
	}
	c.Write([]byte(`<?xml version="1.0"?><container xmlns="urn:oasis:names:tc:opendocument:xmlns:container" version="1.0"><rootfiles><rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/></rootfiles></container>`))

	wz.Close()
	f.Close()

	return zip.OpenReader(path)
}
