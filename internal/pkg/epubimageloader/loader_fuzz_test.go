package epubimageloader

import (
	"bytes"
	"testing"
)

// FuzzDecodeBounded tests that DecodeBounded never panics on arbitrary input.
// It accepts raw bytes, wraps them in a reader, and attempts to decode.
//
// Most random byte sequences are not valid images, so the function is
// expected to return an error for those. The goal is to ensure:
//   - No panic or crash on crafted/malicious input
//   - No OOM from dimension-bomb images (handled by DecodeConfig pre-check)
//   - Graceful error handling for truncated, empty, or corrupted data
//
// Seed corpus values exercise common image formats, edge sizes, and
// known problematic patterns (empty, truncated, oversized headers).
func FuzzDecodeBounded(f *testing.F) {
	// Seed corpus: valid-looking headers for common formats
	f.Add([]byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46, 0x00, 0x01}) // JPEG SOI+APP0
	f.Add([]byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a})                         // PNG header
	f.Add([]byte{0x49, 0x49, 0x2a, 0x00})                                                     // TIFF little-endian
	f.Add([]byte{0x52, 0x49, 0x46, 0x46})                                                     // WEBP ("RIFF")

	// Seed corpus: edge cases
	f.Add([]byte{})                         // empty
	f.Add([]byte{0xff, 0xd8, 0xff})         // truncated JPEG
	f.Add([]byte{0x89, 0x50, 0x4e})         // truncated PNG
	f.Add(bytes.Repeat([]byte{0xff}, 1024)) // solid fill (not a valid image)

	f.Fuzz(func(t *testing.T, data []byte) {
		// DecodeBounded should never panic, regardless of input
		img, format, err := DecodeBounded(bytes.NewReader(data), MaxImageDim)

		if err != nil {
			// Most random byte sequences are invalid — skip.
			// We only care about crashes, not decode successes.
			t.Skip()
		}

		// If decoding succeeded, basic invariants must hold:
		// - format must be non-empty
		// - image dimensions must be within the declared bounds
		if format == "" {
			t.Error("DecodeBounded returned empty format for valid image")
		}
		if img.Bounds().Dx() > MaxImageDim || img.Bounds().Dy() > MaxImageDim {
			t.Errorf("DecodeBounded returned image exceeding maxDim: %dx%d",
				img.Bounds().Dx(), img.Bounds().Dy())
		}
	})
}

// FuzzIsSupportedImage tests that IsSupportedImage never panics on
// arbitrary path strings.
func FuzzIsSupportedImage(f *testing.F) {
	// Seed corpus
	f.Add("image.jpg")
	f.Add("image.jpeg")
	f.Add("image.png")
	f.Add("image.webp")
	f.Add("image.tiff")
	f.Add("image.txt")
	f.Add("")
	f.Add(".jpg")
	f.Add("noext")
	f.Add("../path/../traversal.jpg")
	f.Add(".hidden.jpg")

	f.Fuzz(func(t *testing.T, path string) {
		// Must never panic
		_ = IsSupportedImage(path, true)
		_ = IsSupportedImage(path, false)
	})
}

// FuzzCorruptedImageNeverPanics tests that CorruptedImage handles
// unusual path and name values without crashing.
//
// Note: CorruptedImage generates a 1200x1920 image with rendered text,
// so it's always expected to succeed for any string input.
func FuzzCorruptedImageNeverPanics(f *testing.F) {
	f.Add("normal.jpg", "normal")
	f.Add("", "root")
	f.Add("very/long/path/with/many/components/image.jpg", "name")
	f.Add("\x00null\x00.jpg", "\x00name\x00")
	f.Add(string(make([]byte, 1000)), string(make([]byte, 1000)))

	f.Fuzz(func(t *testing.T, path, name string) {
		img := CorruptedImage(path, name)
		if img == nil {
			t.Error("CorruptedImage returned nil")
		}
	})
}
