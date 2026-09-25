package source

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func writeTestPNG(t *testing.T, path string) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	img.Set(0, 0, color.RGBA{255, 0, 0, 255})
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// A relative walk root such as "." made the no longer existing
// p[len(input)+1:] slice panic when WalkDir cleaned the root out of the path
// ("go-comic-converter -input ."), and produced a wrong Path for nested files.
func TestDirSourceRelativeRootPaths(t *testing.T) {
	tmp := t.TempDir()
	writeTestPNG(t, filepath.Join(tmp, "flat.png"))
	if err := os.Mkdir(filepath.Join(tmp, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestPNG(t, filepath.Join(tmp, "sub", "nested.png"))
	t.Chdir(tmp)

	for _, input := range []string{".", "./", tmp} {
		t.Run(input, func(t *testing.T) {
			ch, n, err := New(input, 0).Load(context.Background())
			if err != nil {
				t.Fatalf("Load(%q): %v", input, err)
			}
			paths := map[string]string{}
			for task := range ch {
				paths[task.Name] = task.Path
			}
			if len(paths) != n {
				t.Fatalf("got %d tasks, want %d", len(paths), n)
			}
			if p := paths["flat.png"]; p != "" {
				t.Errorf("flat.png Path = %q, want %q", p, "")
			}
			if p := paths["nested.png"]; p != "sub" {
				t.Errorf("nested.png Path = %q, want %q", p, "sub")
			}
		})
	}
}
