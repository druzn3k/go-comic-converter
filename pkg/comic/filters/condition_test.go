package filters

import (
	"image"
	"testing"
)

func TestParseConditionWidthHeight(t *testing.T) {
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 800, 600),
	}

	tests := []struct {
		expr   string
		expect bool
	}{
		{"width > height", true},   // 800 > 600
		{"width < height", false},  // 800 < 600
		{"width >= height", true},  // 800 >= 600
		{"width <= height", false}, // 800 <= 600
		{"width == height", false}, // 800 == 600
		{"width != height", true},  // 800 != 600
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			cond, err := ParseCondition(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got := cond(fctx); got != tt.expect {
				t.Errorf("expected %v, got %v", tt.expect, got)
			}
		})
	}
}

func TestParseConditionEqualBounds(t *testing.T) {
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 500, 500),
	}

	cond, err := ParseCondition("width == height")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("expected width == height for square image")
	}

	cond, err = ParseCondition("width != height")
	if err != nil {
		t.Fatal(err)
	}
	if cond(fctx) {
		t.Error("expected width == height, so width != height should be false")
	}
}

func TestParseConditionPart(t *testing.T) {
	tests := []struct {
		part   int
		expr   string
		expect bool
	}{
		{0, "part == 0", true},
		{0, "part == 1", false},
		{3, "part > 2", true},
		{3, "part >= 3", true},
		{1, "part < 10", true},
		{0, "part != 0", false},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			cond, err := ParseCondition(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			fctx := FilterContext{Part: tt.part}
			if got := cond(fctx); got != tt.expect {
				t.Errorf("part=%d, expected %v, got %v", tt.part, tt.expect, got)
			}
		})
	}
}

func TestParseConditionIsDoublePage(t *testing.T) {
	trueFctx := FilterContext{IsDoublePage: true}
	falseFctx := FilterContext{IsDoublePage: false}

	// Using numeric comparison
	cond, err := ParseCondition("is_double_page == 1")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(trueFctx) {
		t.Error("is_double_page should be 1 when true")
	}
	if cond(falseFctx) {
		t.Error("is_double_page should be 0 when false")
	}

	// Using boolean-like comparison
	cond, err = ParseCondition("is_double_page == true")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(trueFctx) {
		t.Error("is_double_page == true should hold when IsDoublePage is true")
	}

	cond, err = ParseCondition("is_double_page == false")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(falseFctx) {
		t.Error("is_double_page == false should hold when IsDoublePage is false")
	}
}

func TestParseConditionIntegerLiteral(t *testing.T) {
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 1200, 800),
	}

	cond, err := ParseCondition("width >= 1000")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("1200 >= 1000 should be true")
	}

	cond, err = ParseCondition("width >= 1500")
	if err != nil {
		t.Fatal(err)
	}
	if cond(fctx) {
		t.Error("1200 >= 1500 should be false")
	}
}

func TestParseConditionFloatLiteral(t *testing.T) {
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 800, 600),
	}

	cond, err := ParseCondition("width > 799.5")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("800 > 799.5 should be true")
	}

	cond, err = ParseCondition("width <= 800.0")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("800 <= 800.0 should be true")
	}
}

func TestParseConditionVariableComparison(t *testing.T) {
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 1600, 900),
		Part:           0,
	}

	// width compared to a variable reference on the right
	// "part" on the right side evaluates to Part value
	cond, err := ParseCondition("width > part")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("1600 > 0 should be true")
	}

	// part compared to width
	cond, err = ParseCondition("part < width")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("0 < 1600 should be true")
	}
}

func TestParseConditionWhitespace(t *testing.T) {
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 100, 200),
	}

	tests := []string{
		"width < height",
		"  width < height",
		"width < height  ",
		"  width  <  height  ",
	}

	for _, expr := range tests {
		t.Run(expr, func(t *testing.T) {
			cond, err := ParseCondition(expr)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", expr, err)
			}
			if !cond(fctx) {
				t.Errorf("expected %q to be true (100 < 200)", expr)
			}
		})
	}
}

func TestParseConditionInvalidExpressions(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"width",
		"width >",
		"> 100",
		"width > height > 0",
		"width >> height",
		"width => height",
		"width <> height",
		"width === height",
		"width AND height",
		"width + height == 100",
		"width > 100 extra",
	}

	for _, expr := range tests {
		t.Run("invalid_"+expr, func(t *testing.T) {
			cond, err := ParseCondition(expr)
			if err == nil {
				t.Errorf("expected error for %q, got nil and cond=%v", expr, cond != nil)
			}
		})
	}
}

func TestParseConditionUnknownVariable(t *testing.T) {
	// Unknown variable names evaluate to 0
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 500, 500),
	}

	// "foo" resolves to 0, so "foo == 0" is true
	cond, err := ParseCondition("foo == 0")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("unknown variable 'foo' should evaluate to 0")
	}

	cond, err = ParseCondition("500 > unknown_var")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("500 > 0 (unknown_var) should be true")
	}
}

func TestParseConditionZeroBounds(t *testing.T) {
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 0, 0),
	}

	cond, err := ParseCondition("width == 0")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("width of zero bounds should be 0")
	}

	cond, err = ParseCondition("height == 0")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("height of zero bounds should be 0")
	}
}

func TestParseConditionNegativePart(t *testing.T) {
	fctx := FilterContext{Part: -1}

	cond, err := ParseCondition("part == -1")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("part == -1 should be true")
	}

	cond, err = ParseCondition("part < 0")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("part < 0 should be true for part=-1")
	}
}

func TestParseConditionIsDoublePageNumericEdge(t *testing.T) {
	trueFctx := FilterContext{IsDoublePage: true}
	falseFctx := FilterContext{IsDoublePage: false}

	// is_double_page as a number on the left
	cond, err := ParseCondition("is_double_page > 0")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(trueFctx) {
		t.Error("is_double_page > 0 should be true when IsDoublePage is true")
	}
	if cond(falseFctx) {
		t.Error("is_double_page > 0 should be false when IsDoublePage is false")
	}

	// != 0
	cond, err = ParseCondition("is_double_page != 0")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(trueFctx) {
		t.Error("is_double_page != 0 should be true")
	}
	if cond(falseFctx) {
		t.Error("is_double_page != 0 should be false")
	}
}

func TestParseConditionAllOperatorsCoverage(t *testing.T) {
	fctx := FilterContext{
		OriginalBounds: image.Rect(0, 0, 100, 100),
	}

	tests := []struct {
		expr   string
		expect bool
	}{
		// 100 > 50
		{"width > 50", true},
		{"width > 100", false},
		{"width > 150", false},
		// 100 < 50
		{"width < 50", false},
		{"width < 100", false},
		{"width < 150", true},
		// 100 >= 50
		{"width >= 50", true},
		{"width >= 100", true},
		{"width >= 150", false},
		// 100 <= 50
		{"width <= 50", false},
		{"width <= 100", true},
		{"width <= 150", true},
		// 100 == 50
		{"width == 50", false},
		{"width == 100", true},
		{"width == 150", false},
		// 100 != 50
		{"width != 50", true},
		{"width != 100", false},
		{"width != 150", true},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			cond, err := ParseCondition(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.expr, err)
			}
			if got := cond(fctx); got != tt.expect {
				t.Errorf("%q: expected %v, got %v", tt.expr, tt.expect, got)
			}
		})
	}
}

func TestParseConditionCaseInsensitiveVariables(t *testing.T) {
	fctx := FilterContext{
		IsDoublePage:   true,
		OriginalBounds: image.Rect(0, 0, 100, 200),
	}

	tests := []struct {
		expr   string
		expect bool
	}{
		{"Width > 50", true},
		{"HEIGHT > 100", true},
		{"IS_DOUBLE_PAGE == 1", true},
		{"Is_Double_Page == true", true},
	}

	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			cond, err := ParseCondition(tt.expr)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.expr, err)
			}
			if got := cond(fctx); got != tt.expect {
				t.Errorf("%q: expected %v, got %v", tt.expr, tt.expect, got)
			}
		})
	}
}

func TestParseConditionTrueFalseAsLiterals(t *testing.T) {
	// "true" evaluates to 1, "false" evaluates to 0
	fctx := FilterContext{
		Part:           1,
		OriginalBounds: image.Rect(0, 0, 1, 1),
	}

	cond, err := ParseCondition("part == true")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx) {
		t.Error("part=1, true=1, so part==true should be true")
	}

	fctx2 := FilterContext{Part: 0}
	cond, err = ParseCondition("part == false")
	if err != nil {
		t.Fatal(err)
	}
	if !cond(fctx2) {
		t.Error("part=0, false=0, so part==false should be true")
	}
}

func TestParseConditionConditionType(t *testing.T) {
	// Verify that ParseCondition returns the correct type (Condition)
	cond, err := ParseCondition("width > 0")
	if err != nil {
		t.Fatal(err)
	}

	// Verify it satisfies the Condition type
	var c Condition = cond
	_ = c

	// Verify it matches the ConditionalFilter.Condition signature
	cf := &ConditionalFilter{
		Condition: cond,
	}
	if cf.Condition == nil {
		t.Error("condition should not be nil")
	}
}
