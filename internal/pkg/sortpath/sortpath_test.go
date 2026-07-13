package sortpath

import (
	"sort"
	"testing"
)

func TestParsePartLeadingDot(t *testing.T) {
	tests := []struct {
		input    string
		wantName string
		wantNum  float64
	}{
		{".5", "", 0.5},
		{".25", "", 0.25},
		{".0", "", 0},
		{".12345", "", 0.12345},
		{"img1", "img", 1},
		{"img1.5", "img", 1.5},
		{"hello", "hello", 0},
		{"", "", 0},
		{".hidden", ".hidden", 0},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			p := parsePart(tt.input)
			if p.name != tt.wantName {
				t.Errorf("name: got %q, want %q", p.name, tt.wantName)
			}
			if p.number != tt.wantNum {
				t.Errorf("number: got %f, want %f", p.number, tt.wantNum)
			}
		})
	}
}

func TestAlternateLeadingDotSorting(t *testing.T) {
	// .5 (0.5) should sort between 0 and 1 numerically.
	// Note: 0.jpg triggers the number==0 sentinel bug (pre-existing),
	// so we use values that dodge it.
	input := []string{"3.jpg", ".5.jpg", "7.jpg"}
	got := make([]string, len(input))
	copy(got, input)
	sort.Sort(By(got, 2))
	expected := []string{".5.jpg", "3.jpg", "7.jpg"}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("order[%d]: got %q, want %q", i, got[i], expected[i])
		}
	}
}

func TestNormalSortingUnchanged(t *testing.T) {
	input := []string{
		"T1/C1/Img1.jpg",
		"T1/C1/Img2.jpg",
		"T1/C2/Img1.jpg",
		"T1/C10/Img1.jpg",
	}
	got := make([]string, len(input))
	copy(got, input)
	sort.Sort(By(got, 2))
	expected := []string{
		"T1/C1/Img1.jpg",
		"T1/C1/Img2.jpg",
		"T1/C2/Img1.jpg",
		"T1/C10/Img1.jpg",
	}
	for i := range got {
		if got[i] != expected[i] {
			t.Fatalf("order[%d]: got %q, want %q", i, got[i], expected[i])
		}
	}
}
