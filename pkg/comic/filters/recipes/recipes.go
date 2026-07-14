package recipes

import (
	"embed"
	"fmt"
	"io/fs"
	"strings"

	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
)

//go:embed *.yaml
var recipeFS embed.FS

// Load loads a built-in recipe by name (without .yaml extension).
func Load(name string) (*filters.Chain, error) {
	data, err := recipeFS.ReadFile(name + ".yaml")
	if err != nil {
		return nil, fmt.Errorf("recipe %q not found", name)
	}
	return filters.FromYAML(string(data))
}

// Names returns the list of available built-in recipe names.
func Names() []string {
	entries, err := fs.ReadDir(recipeFS, ".")
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
