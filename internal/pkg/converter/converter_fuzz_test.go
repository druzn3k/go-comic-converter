package converter

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// FuzzValidate tests that Validate never panics on arbitrary option combinations.
// Validate returns errors for most garbage inputs (missing input, invalid profiles,
// color codes, etc.), but it must never crash or panic.
func FuzzValidate(f *testing.F) {
	// Seed corpus: realistic combinations of input types, profiles, and formats
	f.Add("/tmp/test", "SR", "jpeg", "000", "FFF", 0, 0, 0, 1)
	f.Add("input.cbz", "KPW5", "png", "FFF", "000", 50, 0, 0, 1)
	f.Add("", "SR", "copy", "000", "FFF", 0, 0, 0, 1)
	f.Add("input.pdf", "HR", "jpeg", "AAA", "BBB", 10, 50, 50, 2)
	f.Add("input.cbr", "K1", "jpeg", "000", "FFF", 0, 0, 0, 0)

	f.Fuzz(func(t *testing.T,
		input, profile, format, fgColor, bgColor string,
		cropLimit, brightness, contrast, titlePage int) {

		dir := t.TempDir()
		if input == "" {
			input = dir
		}

		c := New()
		c.Options.Input = input
		c.Options.Profile = profile
		c.Options.Image.Format = format
		c.Options.Image.View.Color.Foreground = fgColor
		c.Options.Image.View.Color.Background = bgColor
		c.Options.Image.Crop.Limit = cropLimit
		c.Options.Image.Brightness = brightness
		c.Options.Image.Contrast = contrast
		c.Options.TitlePage = titlePage

		// Validate should never panic on any input combination
		_ = c.Validate()
	})
}

// FuzzOptionsDeserialize tests that the YAML config deserialization
// never panics on arbitrary input. It writes the fuzzed string to a
// temp file, decodes it into Options, and runs Validate.
func FuzzOptionsDeserialize(f *testing.F) {
	// Seed corpus: valid, near-valid, and garbage YAML configs
	f.Add("profile: SR\nimage:\n  format: jpeg\n  quality: 85\n")
	f.Add("profile: INVALID\nimage:\n  format: invalid\n")
	f.Add("profile: \nimage:\n  format: \n")
	f.Add("{profile: SR, image: {format: jpeg}}")
	f.Add("garbage: \x00\xff\xfe\xfd")

	f.Fuzz(func(t *testing.T, yamlContent string) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.yaml")
		if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
			t.Skip()
		}

		o := NewOptions()
		fh, err := os.Open(configPath)
		if err != nil {
			t.Skip()
		}
		defer fh.Close()

		// Decode the YAML into Options — most garbage will fail
		if err := yaml.NewDecoder(fh).Decode(o); err != nil {
			t.Skip()
		}

		// If decode "succeeded", Validate must still handle it gracefully
		c := New()
		c.Options = o
		_ = c.Validate()
	})
}
