package converter

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRejectsPdfWithCopyFormat(t *testing.T) {
	// Create a dummy PDF file for validation
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "test.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 fake"), 0644); err != nil {
		t.Fatal(err)
	}

	c := New()
	c.Options.Input = pdfPath
	c.Options.Image.Format = "copy"
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"

	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for PDF input with copy format, got nil")
	}
	if !contains(err.Error(), "copy") || !contains(err.Error(), "PDF") {
		t.Errorf("error should mention 'copy' and 'PDF', got: %v", err)
	}
}

func TestValidateAcceptsPdfWithJpegFormat(t *testing.T) {
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "test.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 fake"), 0644); err != nil {
		t.Fatal(err)
	}

	c := New()
	c.Options.Input = pdfPath
	c.Options.Image.Format = "jpeg"
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"

	err := c.Validate()
	// May fail on output dir, but should NOT fail on format+pdf combo
	if err != nil && contains(err.Error(), "copy") {
		t.Errorf("should not fail on format+pdf when format is jpeg: %v", err)
	}
}

func TestValidateAcceptsDirectoryWithCopy(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.Options.Input = dir
	c.Options.Image.Format = "copy"
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"

	err := c.Validate()
	// May fail on output dir, but should NOT fail on format+pdf combo
	if err != nil && contains(err.Error(), "copy") {
		t.Errorf("should not fail on directory with copy format: %v", err)
	}
}

func TestValidateOutputFormatDefault(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.Options.Input = dir
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"
	// OutputFormat empty means default
	c.Options.OutputFormat = ""

	err := c.Validate()
	// Should not fail on output format (may fail on output dir)
	if err != nil && (contains(err.Error(), "output-format") || contains(err.Error(), "format should be")) {
		t.Errorf("should not fail on empty output-format: %v", err)
	}
}

func TestValidateOutputFormatEpub(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.Options.Input = dir
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"
	c.Options.OutputFormat = "epub"

	err := c.Validate()
	if err != nil && contains(err.Error(), "output-format") {
		t.Errorf("epub should be a valid output format: %v", err)
	}
}

func TestValidateOutputFormatKepub(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.Options.Input = dir
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"
	c.Options.OutputFormat = "kepub"

	err := c.Validate()
	if err != nil && contains(err.Error(), "output-format") {
		t.Errorf("kepub should be a valid output format: %v", err)
	}
}

func TestValidateOutputFormatCbz(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.Options.Input = dir
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"
	c.Options.OutputFormat = "cbz"

	err := c.Validate()
	if err != nil && contains(err.Error(), "output-format") {
		t.Errorf("cbz should be a valid output format: %v", err)
	}
}

func TestValidateOutputFormatHtml(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.Options.Input = dir
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"
	c.Options.OutputFormat = "html"

	err := c.Validate()
	if err != nil && contains(err.Error(), "output-format") {
		t.Errorf("html should be a valid output format: %v", err)
	}
}

func TestValidateOutputFormatAll(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.Options.Input = dir
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"
	c.Options.OutputFormat = "all"

	err := c.Validate()
	if err != nil && contains(err.Error(), "output-format") {
		t.Errorf("'all' should be a valid output format: %v", err)
	}
}

func TestValidateOutputFormatInvalid(t *testing.T) {
	dir := t.TempDir()
	c := New()
	c.Options.Input = dir
	c.Options.Profile = "SR"
	c.Options.Image.View.Color.Foreground = "000"
	c.Options.Image.View.Color.Background = "FFF"
	c.Options.OutputFormat = "invalid"

	err := c.Validate()
	if err == nil {
		t.Fatal("expected error for invalid output format, got nil")
	}
	if !contains(err.Error(), "output-format") {
		t.Errorf("error should mention 'output-format', got: %v", err)
	}
}
