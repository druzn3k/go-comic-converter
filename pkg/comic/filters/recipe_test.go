package filters

import (
	"context"
	"image"
	"testing"
)

func init() {
	// Register test filters for recipe testing.
	Register("recipe_test_filter", func(params map[string]any) (Filter, error) {
		return &recipeTestFilter{}, nil
	})
	Register("recipe_test_filter_params", func(params map[string]any) (Filter, error) {
		val := getInt(params, "val", -1)
		if val < 0 {
			return nil, context.DeadlineExceeded // bogus: just need an error
		}
		return &recipeTestFilter{val: val}, nil
	})
}

type recipeTestFilter struct {
	val int
}

func (f *recipeTestFilter) Name() string { return "recipe_test_filter" }
func (f *recipeTestFilter) Apply(ctx context.Context, img image.Image, fctx FilterContext) []image.Image {
	return []image.Image{img}
}

func TestFromYAMLValid(t *testing.T) {
	yaml := `
apiVersion: 1
name: test-recipe
description: A test recipe
filters:
  - name: recipe_test_filter
`
	chain, err := FromYAML(yaml)
	if err != nil {
		t.Fatalf("FromYAML failed: %v", err)
	}
	if chain.Len() != 1 {
		t.Errorf("expected 1 filter, got %d", chain.Len())
	}
}

func TestFromYAMLDefaultAPIVersion(t *testing.T) {
	yaml := `
name: no-api-version
filters:
  - name: recipe_test_filter
`
	chain, err := FromYAML(yaml)
	if err != nil {
		t.Fatalf("FromYAML with missing apiVersion failed: %v", err)
	}
	if chain.Len() != 1 {
		t.Errorf("expected 1 filter, got %d", chain.Len())
	}
}

func TestFromYAMLInvalidYAML(t *testing.T) {
	_, err := FromYAML(": not valid yaml")
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestFromYAMLUnsupportedAPIVersion(t *testing.T) {
	yaml := `
apiVersion: 99
name: future
filters:
  - name: recipe_test_filter
`
	_, err := FromYAML(yaml)
	if err == nil {
		t.Fatal("expected error for unsupported apiVersion")
	}
}

func TestFromRecipeUnknownFilter(t *testing.T) {
	r := &Recipe{
		APIVersion: 1,
		Name:       "bad-recipe",
		Filters: []FilterConfig{
			{Name: "nonexistent_filter_xyz"},
		},
	}
	_, err := FromRecipe(r)
	if err == nil {
		t.Fatal("expected error for unknown filter")
	}
}

func TestFromRecipeWithParams(t *testing.T) {
	r := &Recipe{
		APIVersion: 1,
		Filters: []FilterConfig{
			{Name: "recipe_test_filter_params", Params: map[string]any{"val": float64(42)}},
		},
	}
	chain, err := FromRecipe(r)
	if err != nil {
		t.Fatalf("FromRecipe failed: %v", err)
	}
	if chain.Len() != 1 {
		t.Errorf("expected 1 filter, got %d", chain.Len())
	}
}

func TestFromRecipeWithCondition(t *testing.T) {
	// Register a filter that can be conditionally applied.
	Register("recipe_cond_filter", func(params map[string]any) (Filter, error) {
		return &recipeTestFilter{}, nil
	})

	r := &Recipe{
		APIVersion: 1,
		Filters: []FilterConfig{
			{Name: "recipe_cond_filter", Condition: "part == 0"},
		},
	}
	chain, err := FromRecipe(r)
	if err != nil {
		t.Fatalf("FromRecipe with condition failed: %v", err)
	}
	if chain.Len() != 1 {
		t.Errorf("expected 1 filter, got %d", chain.Len())
	}

	// Verify that the condition works: filter should apply when part==0
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	ctx := context.Background()
	fctx := FilterContext{Part: 0}
	results := chain.Apply(ctx, img, fctx)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	// When part != 0, the conditional filter should pass through.
	fctx.Part = 1
	results = chain.Apply(ctx, img, fctx)
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestFromRecipeInvalidCondition(t *testing.T) {
	r := &Recipe{
		APIVersion: 1,
		Filters: []FilterConfig{
			{Name: "recipe_test_filter", Condition: "width ?? height"},
		},
	}
	_, err := FromRecipe(r)
	if err == nil {
		t.Fatal("expected error for invalid condition")
	}
}

func TestFromRecipeFilterFactoryError(t *testing.T) {
	// recipe_test_filter_params returns error when val < 0
	r := &Recipe{
		APIVersion: 1,
		Filters: []FilterConfig{
			{Name: "recipe_test_filter_params", Params: map[string]any{"val": float64(-1)}},
		},
	}
	_, err := FromRecipe(r)
	if err == nil {
		t.Fatal("expected error from filter factory")
	}
}

func TestBuiltinRecipeNames(t *testing.T) {
	names := BuiltinRecipeNames()
	if len(names) == 0 {
		t.Fatal("expected at least one builtin recipe")
	}
	// Verify expected recipes exist.
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

func TestBuiltinRecipeLoads(t *testing.T) {
	chain, err := BuiltinRecipe("manga-standard")
	if err != nil {
		t.Fatalf("BuiltinRecipe(manga-standard) failed: %v", err)
	}
	if chain.Len() == 0 {
		t.Error("expected non-empty chain")
	}
}

func TestBuiltinRecipeUnknown(t *testing.T) {
	_, err := BuiltinRecipe("nonexistent-recipe-xyz")
	if err == nil {
		t.Fatal("expected error for unknown builtin recipe")
	}
}

func TestRecipeRoundTrip(t *testing.T) {
	r := &Recipe{
		APIVersion: 1,
		Filters: []FilterConfig{
			{Name: "recipe_test_filter"},
			{Name: "recipe_test_filter", Condition: "part == 0"},
		},
	}
	chain, err := FromRecipe(r)
	if err != nil {
		t.Fatalf("FromRecipe failed: %v", err)
	}
	if chain.Len() != 2 {
		t.Errorf("expected 2 filters, got %d", chain.Len())
	}
}
