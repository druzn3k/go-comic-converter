package filters

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed recipes/*.yaml
var builtinRecipeFS embed.FS

// Recipe is a serializable filter chain definition.
type Recipe struct {
	APIVersion  int            `yaml:"apiVersion"`
	Name        string         `yaml:"name"`
	Description string         `yaml:"description"`
	Filters     []FilterConfig `yaml:"filters"`
}

// FilterConfig is a single filter step in a recipe.
type FilterConfig struct {
	Name      string         `yaml:"name"`
	Params    map[string]any `yaml:"params"`
	Condition string         `yaml:"condition,omitempty"`
}

// FromYAML builds a Chain from a YAML recipe string.
func FromYAML(yamlStr string) (*Chain, error) {
	var recipe Recipe
	if err := yaml.Unmarshal([]byte(yamlStr), &recipe); err != nil {
		return nil, fmt.Errorf("recipe YAML: %w", err)
	}
	if recipe.APIVersion == 0 {
		recipe.APIVersion = 1
	}
	if recipe.APIVersion != 1 {
		return nil, fmt.Errorf("unsupported recipe apiVersion %d (supported: 1)", recipe.APIVersion)
	}
	return FromRecipe(&recipe)
}

// FromRecipe builds a Chain from a Recipe struct.
func FromRecipe(r *Recipe) (*Chain, error) {
	chain := NewChain()
	for _, fc := range r.Filters {
		factory, ok := Lookup(fc.Name)
		if !ok {
			return nil, fmt.Errorf("unknown filter: %s", fc.Name)
		}
		filter, err := factory(fc.Params)
		if err != nil {
			return nil, fmt.Errorf("filter %s: %w", fc.Name, err)
		}
		if fc.Condition != "" {
			cond, err := ParseCondition(fc.Condition)
			if err != nil {
				return nil, fmt.Errorf("condition %q: %w", fc.Condition, err)
			}
			filter = &ConditionalFilter{Filter: filter, Condition: cond}
		}
		chain.Add(filter)
	}
	return chain, nil
}

// BuiltinRecipe loads a built-in recipe by name (without .yaml extension).
func BuiltinRecipe(name string) (*Chain, error) {
	data, err := builtinRecipeFS.ReadFile("recipes/" + name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("builtin recipe %q not found", name)
	}
	return FromYAML(string(data))
}

// BuiltinRecipeNames returns the list of available built-in recipe names.
func BuiltinRecipeNames() []string {
	entries, err := fs.ReadDir(builtinRecipeFS, "recipes")
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, strings.TrimSuffix(e.Name(), ".yaml"))
		}
	}
	return names
}
