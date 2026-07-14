package filters

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Condition evaluates whether a filter should apply given the image context.
type Condition func(fctx FilterContext) bool

// ParseCondition parses a simple comparison expression into a Condition.
//
// Supported variables: width, height, part, is_double_page
// Supported operators: >, <, >=, <=, ==, !=
// Operands may be variable names, integers, floats, true, or false.
//
// Examples:
//   - "width > height"
//   - "part == 0"
//   - "is_double_page == true"
//   - "width >= 1000"
func ParseCondition(expr string) (Condition, error) {
	re := regexp.MustCompile(`^\s*(\w+)\s*(>=|<=|==|!=|>|<)\s*([-+]?\d+(?:\.\d+)?|\w+)\s*$`)
	m := re.FindStringSubmatch(expr)
	if m == nil {
		return nil, fmt.Errorf("invalid condition: %q", expr)
	}
	varName, op, valueStr := m[1], m[2], m[3]

	return func(fctx FilterContext) bool {
		left := getConditionValue(varName, fctx)
		right := getConditionValue(valueStr, fctx)
		switch op {
		case ">":
			return left > right
		case "<":
			return left < right
		case ">=":
			return left >= right
		case "<=":
			return left <= right
		case "==":
			return left == right
		case "!=":
			return left != right
		}
		return false
	}, nil
}

func getConditionValue(name string, fctx FilterContext) float64 {
	switch strings.ToLower(name) {
	case "width":
		return float64(fctx.OriginalBounds.Dx())
	case "height":
		return float64(fctx.OriginalBounds.Dy())
	case "part":
		return float64(fctx.Part)
	case "is_double_page":
		if fctx.IsDoublePage {
			return 1
		}
		return 0
	case "true":
		return 1
	case "false":
		return 0
	default:
		// Try parsing as number
		if v, err := strconv.ParseFloat(name, 64); err == nil {
			return v
		}
		return 0
	}
}
