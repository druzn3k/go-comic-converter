package comic

import "testing"

func TestIsTempFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{".swp suffix", "notes.swp", true},
		{"~ suffix", "file~", true},
		{".hidden", ".config", true},
		{".tmp suffix", "data.tmp", true},
		{".swx suffix", "vim.swx", true},
		{".goutputstream-", "abc.goutputstream-xyz", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTempFile(tc.filename); got != tc.want {
				t.Errorf("isTempFile(%q) = %v, want %v", tc.filename, got, tc.want)
			}
		})
	}
}

func TestIsTempFileClean(t *testing.T) {
	cleanNames := []string{"comic.cbz", "image.jpg", "normal.zip"}
	for _, name := range cleanNames {
		t.Run(name, func(t *testing.T) {
			if isTempFile(name) {
				t.Errorf("isTempFile(%q) = true, want false", name)
			}
		})
	}
}

func TestIsTempFileDotPrefix(t *testing.T) {
	if !isTempFile(".comic.cbz") {
		t.Error("isTempFile('.comic.cbz') = false, want true (hidden files are temp)")
	}
}
