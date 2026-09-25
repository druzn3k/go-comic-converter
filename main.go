/*
Convert CBZ/CBR/Dir into EPUB for e-reader devices (Kindle Devices, ...)

My goal is to make a simple, cross-platform, and fast tool to convert comics into EPUB.

EPUB is now support by Amazon through [SendToKindle](https://www.amazon.com/gp/sendtokindle/), by Email or by using the App. So I've made it simple to support the size limit constraint of those services.
*/
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"
	"time"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/converter"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/utils"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/output"
	comicServer "github.com/druzn3k/go-comic-converter/v3/pkg/comic/server"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
	"gopkg.in/yaml.v3"

	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
)

// version is set at release time via
// -ldflags "-X main.version=<tag>" (see .goreleaser.yml) or the Dockerfile
// VERSION build arg; "(devel)" for local/dev builds.
var version = "(devel)"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cmd := converter.New()
	if err := cmd.LoadConfig(); err != nil {
		cmd.Fatal(err)
	}
	cmd.InitParse()
	cmd.Parse()

	switch {
	case cmd.Options.Version:
		printVersion()
	case cmd.Options.Serve != "":
		serve(ctx, cmd)
	case cmd.Options.Batch != "":
		batch(ctx, cmd)
	case cmd.Options.Watch != "":
		watch(ctx, cmd)
	case cmd.Options.Save:
		save(cmd)
	case cmd.Options.Show:
		show(cmd)
	case cmd.Options.Reset:
		reset(cmd)
	default:
		generate(ctx, cmd)
	}
}

func printVersion() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		utils.Fatalln("failed to fetch current version")
	}

	// `go install ...@vX.Y.Z` binaries carry the version in build info instead of
	// ldflags. Only accept a clean release tag there — pseudo-versions from VCS
	// stamping (v3.0.0-<ts>-<sha>+dirty) and prereleases stay "(devel)".
	v := version
	if v == "(devel)" && strings.HasPrefix(bi.Main.Version, "v") &&
		!strings.ContainsAny(bi.Main.Version, "-+") {
		v = bi.Main.Version
	}

	utils.Printf("go-comic-converter\n")
	utils.Printf("  Path             : %s\n", bi.Main.Path)
	utils.Printf("  Sum              : %s\n", bi.Main.Sum)
	utils.Printf("  Version          : %s\n\n", v)

	latestVersion := "unknown"
	resp, err := http.Get("https://api.github.com/repos/druzn3k/go-comic-converter/tags")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var tags []struct {
				Name string `json:"name"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&tags); err == nil && len(tags) > 0 {
				latestVersion = tags[0].Name
			}
		}
	}

	utils.Printf("  Available Version: %s\n", latestVersion)
	if latestVersion != "unknown" {
		utils.Printf("\nTo install the latest version:\n")
		utils.Printf("$ go install %s@%s\n", bi.Main.Path, latestVersion)
	}
}

func save(cmd *converter.Converter) {
	if err := cmd.Options.SaveConfig(); err != nil {
		cmd.Fatal(err)
	}
	utils.Printf(
		"%s%s\n\nSaving to %s\n",
		cmd.Options.Header(),
		cmd.Options.ShowConfig(),
		cmd.Options.FileName(),
	)
}

func show(cmd *converter.Converter) {
	utils.Println(cmd.Options.Header(), cmd.Options.ShowConfig())
}

func reset(cmd *converter.Converter) {
	if err := cmd.Options.ResetConfig(); err != nil {
		cmd.Fatal(err)
	}
	utils.Printf(
		"%s%s\n\nReset default to %s\n",
		cmd.Options.Header(),
		cmd.Options.ShowConfig(),
		cmd.Options.FileName(),
	)
}

// serve starts the HTTP server mode.
func serve(ctx context.Context, cmd *converter.Converter) {
	s := comicServer.New(ctx, comicServer.Config{
		Addr:            cmd.Options.Serve,
		MaxConcurrent:   cmd.Options.MaxConcurrent,
		AllowLocalPaths: cmd.Options.AllowLocalPaths,
		ShutdownTimeout: 30 * time.Second,
	})

	utils.Printf("Starting server on %s\n", cmd.Options.Serve)
	if err := s.Start(ctx); err != nil && err != http.ErrServerClosed {
		utils.Fatalf("Server error: %v\n", err)
	}
}

// batch processes multiple inputs via glob pattern.
func batch(ctx context.Context, cmd *converter.Converter) {
	utils.Printf("Batch processing: %s\n", cmd.Options.Batch)
	if err := comic.ConvertBatch(ctx, cmd.Options.Batch, cmd.Options.EPUBOptions); err != nil {
		utils.Fatalf("Batch error: %v\n", err)
	}
}

// watch monitors a directory for new files and auto-converts.
func watch(ctx context.Context, cmd *converter.Converter) {
	utils.Printf("Watching: %s\n", cmd.Options.Watch)
	if err := comic.Watch(ctx, cmd.Options.Watch, cmd.Options.EPUBOptions); err != nil {
		utils.Fatalf("Watch error: %v\n", err)
	}
}

// runSingleFormat dispatches a single format through the shared comic.Converter.
func runSingleFormat(ctx context.Context, format string, opts epuboptions.EPUBOptions, cmd *converter.Converter, chain *filters.Chain) error {
	writer := output.Get(format)
	if writer == nil {
		return fmt.Errorf("unsupported output format: %s", format)
	}

	// Use the format's correct extension instead of .epub
	ext := filepath.Ext(opts.Output)
	if ext != "" {
		opts.Output = opts.Output[:len(opts.Output)-len(ext)] + writer.Extension()
	} else {
		opts.Output = opts.Output + writer.Extension()
	}
	opts.OutputFormat = format

	var c *comic.Converter
	if chain != nil {
		c = comic.NewWithRecipe(opts, chain)
	} else {
		c = comic.New(opts)
	}

	err := c.Convert(ctx)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("\nCancelled")
			os.Exit(1)
		}
		if errors.Is(err, comic.ErrImageCorrupted) {
			if !opts.Dry {
				cmd.Stats()
			}
			utils.Fatalf("Error: %v\n", err)
		}
		return err
	}
	return nil
}

func generate(ctx context.Context, cmd *converter.Converter) {
	// --- Recipe handling (exit-early, before validation) ---
	if cmd.Options.RecipeShow && cmd.Options.Recipe == "" {
		names := filters.BuiltinRecipeNames()
		info := map[string]any{
			"message":       "No recipe specified. Available builtin recipes:",
			"builtin":       names,
			"default_chain": "Uses standard processing (crop, contrast, resize, grayscale, etc.)",
		}
		out, _ := yaml.Marshal(info)
		fmt.Print(string(out))
		os.Exit(0)
	}
	if cmd.Options.RecipeSave && cmd.Options.Recipe == "" {
		recipe := filters.Recipe{
			APIVersion:  1,
			Name:        "custom",
			Description: "Recipe from current options",
			Filters:     filters.DefaultFilterConfigs(cmd.Options.Image),
		}
		out, err := yaml.Marshal(recipe)
		if err != nil {
			cmd.Fatal(fmt.Errorf("failed to marshal recipe: %w", err))
		}
		fmt.Print(string(out))
		os.Exit(0)
	}
	var chain *filters.Chain
	if cmd.Options.Recipe != "" {
		var err error
		chain, err = loadRecipe(cmd.Options.Recipe)
		if err != nil {
			cmd.Fatal(err)
		}
		if cmd.Options.RecipeShow {
			recipeData, _ := yaml.Marshal(map[string]string{
				"recipe":  cmd.Options.Recipe,
				"filters": fmt.Sprintf("%d filter(s)", chain.Len()),
			})
			fmt.Print(string(recipeData))
			os.Exit(0)
		}
	}

	if err := cmd.Validate(); err != nil {
		cmd.Fatal(err)
	}

	if profile := cmd.Options.GetProfile(); profile != nil {
		cmd.Options.Image.View.Width = profile.Width
		cmd.Options.Image.View.Height = profile.Height
	}

	if cmd.Options.Json {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"type": "options", "data": cmd.Options,
		})
	} else {
		utils.Println(cmd.Options)
	}

	// Determine output format
	format := cmd.Options.OutputFormat
	if format == "" || format == "epub" {
		if profile := cmd.Options.GetProfile(); profile != nil && profile.PreferredFormat != "" {
			format = profile.PreferredFormat
		}
	}
	if format == "" {
		format = "epub"
	}

	if format == "all" {
		// "all" path: run each registered format once
		for _, runFormat := range output.Available() {
			if err := runSingleFormat(ctx, runFormat, cmd.Options.EPUBOptions, cmd, chain); err != nil {
				cmd.Fatal(err)
			}
		}
	} else {
		// OutputWriter path: load images, dispatch to format writer
		runSingleFormat(ctx, format, cmd.Options.EPUBOptions, cmd, chain)
	}

	if !cmd.Options.Dry {
		cmd.Stats()
	}
}

// loadRecipe loads a filter chain by name (builtin) or from a YAML file path.
func loadRecipe(nameOrPath string) (*filters.Chain, error) {
	chain, err := filters.BuiltinRecipe(nameOrPath)
	if err == nil {
		return chain, nil
	}
	// Not a builtin; treat as file path
	data, err := os.ReadFile(nameOrPath)
	if err != nil {
		return nil, fmt.Errorf("recipe %q not found as builtin or file: %w", nameOrPath, err)
	}
	return filters.FromYAML(string(data))
}
