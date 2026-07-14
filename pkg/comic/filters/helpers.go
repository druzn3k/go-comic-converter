package filters

// getInt extracts an int parameter from a map, or returns the default.
func getInt(params map[string]any, key string, defaultVal int) int {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	}
	return defaultVal
}

// getBool extracts a bool parameter from a map, or returns the default.
func getBool(params map[string]any, key string, defaultVal bool) bool {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	b, ok := v.(bool)
	if ok {
		return b
	}
	return defaultVal
}

// getFloat extracts a float64 parameter from a map, or returns the default.
func getFloat(params map[string]any, key string, defaultVal float64) float64 {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	}
	return defaultVal
}

// getString extracts a string parameter from a map, or returns the default.
func getString(params map[string]any, key string, defaultVal string) string {
	if params == nil {
		return defaultVal
	}
	v, ok := params[key]
	if !ok {
		return defaultVal
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return defaultVal
}
