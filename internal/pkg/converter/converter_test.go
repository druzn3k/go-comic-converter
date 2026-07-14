package converter

import (
	"flag"
	"testing"

	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func TestNewOptions(t *testing.T) {
	o := NewOptions()
	if o == nil {
		t.Fatal("NewOptions() returned nil")
	}
	if o.Profile != "SR" {
		t.Errorf("expected default profile SR, got %q", o.Profile)
	}
	if o.Image.Quality != 85 {
		t.Errorf("expected default quality 85, got %d", o.Image.Quality)
	}
	if !o.Image.GrayScale {
		t.Error("expected grayscale default true")
	}
	if !o.Image.NoBlankImage {
		t.Error("expected noblankimage default true")
	}
}

func TestAvailableProfiles(t *testing.T) {
	o := NewOptions()
	s := o.AvailableProfiles()
	if s == "" {
		t.Fatal("AvailableProfiles() returned empty")
	}
	// Should contain key profiles
	for _, code := range []string{"SR", "HR", "KPW5", "KS"} {
		if !contains(s, code) {
			t.Errorf("AvailableProfiles() missing %q", code)
		}
	}
}

func TestNew(t *testing.T) {
	c := New()
	if c == nil {
		t.Fatal("New() returned nil")
	}
	if c.Options == nil {
		t.Fatal("Options is nil")
	}
	if c.Cmd == nil {
		t.Fatal("Cmd is nil")
	}
	// Verify Cmd has ContinueOnError (we changed from ExitOnError)
	if c.Cmd.ErrorHandling() != flag.ContinueOnError {
		t.Errorf("expected ContinueOnError, got %v", c.Cmd.ErrorHandling())
	}
}

func TestHeader(t *testing.T) {
	o := NewOptions()
	h := o.Header()
	if h != "Go Comic Converter\n\nOptions:" {
		t.Errorf("unexpected header: %q", h)
	}
}

func TestProfile(t *testing.T) {
	o := NewOptions()
	p := o.GetProfile()
	if p == nil {
		t.Fatal("GetProfile() returned nil")
	}
	if p.Code != "SR" {
		t.Errorf("expected SR, got %q", p.Code)
	}
	if p.Width != 1200 {
		t.Errorf("expected width 1200, got %d", p.Width)
	}
	if p.Height != 1920 {
		t.Errorf("expected height 1920, got %d", p.Height)
	}

	// Test invalid profile
	o.Profile = "NONEXISTENT"
	if p := o.GetProfile(); p != nil {
		t.Errorf("expected nil for invalid profile, got %v", p)
	}
}

func TestFileName(t *testing.T) {
	o := NewOptions()
	fn := o.FileName()
	if fn == "" {
		t.Fatal("FileName() returned empty")
	}
	// Should contain config filename
	if !contains(fn, ".go-comic-converter.yaml") {
		t.Errorf("expected .go-comic-converter.yaml in path, got %q", fn)
	}
}

func TestWorkersRatio(t *testing.T) {
	opts := NewOptions().EPUBOptions
	opts.Workers = 10

	if n := opts.WorkersRatio(50); n != 5 {
		t.Errorf("expected 5 workers (50%% of 10), got %d", n)
	}
	if n := opts.WorkersRatio(0); n != 1 {
		t.Errorf("expected minimum 1 worker, got %d", n)
	}
	if n := opts.WorkersRatio(100); n != 10 {
		t.Errorf("expected 10 workers (100%% of 10), got %d", n)
	}
}

func TestImgStorage(t *testing.T) {
	opts := NewOptions().EPUBOptions
	opts.Output = "/tmp/test.epub"
	storage := opts.ImgStorage()
	if storage != "/tmp/test.epub.tmp" {
		t.Errorf("expected /tmp/test.epub.tmp, got %q", storage)
	}
}

func TestDimension(t *testing.T) {
	v := epuboptions.View{
		Width:  1200,
		Height: 1920,
	}
	if d := v.Dimension(); d != "1200x1920" {
		t.Errorf("expected 1200x1920, got %q", d)
	}
	if p := v.Port(); p != "width=1200,height=1920" {
		t.Errorf("expected width=1200,height=1920, got %q", p)
	}
}

// contains reports whether s contains substr.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString is a simple substring search.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
