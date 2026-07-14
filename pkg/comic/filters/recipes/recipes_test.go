package recipes

import (
	"testing"
)

func TestNames(t *testing.T) {
	names := Names()
	if len(names) == 0 {
		t.Fatal("expected at least one builtin recipe")
	}
	expected := map[string]bool{
		"manga-standard": false,
		"manga-old-scan": false,
		"color-comic":     false,
		"night-mode":      false,
		"max-fidelity":    false,
	}
	for _, n := range names {
		if _, ok := expected[n]; ok {
			expected[n] = true
		}
	}
	for name, found := range expected {
		if !found {
			t.Errorf("expected builtin recipe %q not found in %v", name, names)
		}
	}
}

func TestLoad(t *testing.T) {
	chain, err := Load("manga-standard")
	if err != nil {
		t.Fatalf("Load(manga-standard) failed: %v", err)
	}
	if chain.Len() == 0 {
		t.Error("expected non-empty chain")
	}
}

func TestLoadUnknown(t *testing.T) {
	_, err := Load("nonexistent-recipe-xyz")
	if err == nil {
		t.Fatal("expected error for unknown recipe")
	}
}

func TestLoadAllRecipes(t *testing.T) {
	for _, name := range Names() {
		chain, err := Load(name)
		if err != nil {
			t.Errorf("Load(%q) failed: %v", name, err)
			continue
		}
		if chain.Len() == 0 {
			t.Errorf("Load(%q) returned empty chain", name)
		}
	}
}
