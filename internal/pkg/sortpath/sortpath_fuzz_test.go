package sortpath

import (
	"sort"
	"strings"
	"testing"
)

// FuzzParsePart tests that the sortpath parser never panics on
// arbitrary filename strings.
//
// The parser splits filenames into string+number parts for natural
// sorting. It uses a regex under the hood — this fuzz ensures no
// pathological inputs cause a panic or infinite loop.
func FuzzParsePart(f *testing.F) {
	f.Add("page001.jpg")
	f.Add("Chapter 10 - Page 1.jpg")
	f.Add("Tome1/Chap1/Image1.jpg")
	f.Add("image_001.jpg")
	f.Add("img-2-3.jpg")
	f.Add("page.5.jpg")
	f.Add("image.jpg")
	f.Add("12345.jpg")
	f.Add("a.jpg")

	f.Fuzz(func(t *testing.T, filename string) {
		result := parsePart(filename)

		// Empty input produces an empty fullname — this is correct.
		if filename == "" {
			if result.fullname != "" {
				t.Errorf("expected empty fullname for empty input, got %q", result.fullname)
			}
			return
		}

		if result.fullname == "" {
			t.Error("parsePart returned empty fullname for non-empty input")
		}
		if result.number < 0 {
			t.Errorf("parsePart returned negative number %f for %q", result.number, filename)
		}
	})
}

// FuzzBy tests that sort.Interface returned by By() never panics.
//
// The input is a raw byte slice split by newlines into filenames.
// Go fuzzing only supports limited top-level types, so we encode
// the filename list as newline-separated bytes.
func FuzzBy(f *testing.F) {
	f.Add([]byte("a.jpg\nb.jpg\nc.jpg"))
	f.Add([]byte("img1.jpg\nimg2.jpg\nimg10.jpg"))
	f.Add([]byte("page1.jpg\npage1.jpg"))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, data []byte) {
		content := strings.TrimSpace(string(data))
		if content == "" {
			t.Skip()
		}

		names := strings.Split(content, "\n")
		if len(names) < 2 {
			t.Skip()
		}

		for _, mode := range []int{0, 1, 2} {
			s := By(names, mode)
			sort.Sort(s)

			if s.Len() != len(names) {
				t.Errorf("By(%d) changed slice length: %d → %d", mode, len(names), s.Len())
			}
		}
	})
}
