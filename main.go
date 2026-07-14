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
	"sort"
	"syscall"
	"time"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/converter"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimagepassthrough"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/utils"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/output"
	comicServer "github.com/druzn3k/go-comic-converter/v3/pkg/comic/server"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epub"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
	"gopkg.in/yaml.v3"

	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
)

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
		version()
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

func version() {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		utils.Fatalln("failed to fetch current version")
	}

	utils.Printf("go-comic-converter\n")
	utils.Printf("  Path             : %s\n", bi.Main.Path)
	utils.Printf("  Sum              : %s\n", bi.Main.Sum)
	utils.Printf("  Version          : %s\n\n", bi.Main.Version)

	latestVersion := "unknown"
	resp, err := http.Get("https://api.github.com/repos/druzn3k/go-comic-converter/tags")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var tags []struct{ Name string `json:"name"` }
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

// runSingleFormat dispatches a non-EPUB format through the OutputWriter path.
// It loads images via the processor, creates OutputParts, and calls the
// registered OutputWriter for the given format.
func runSingleFormat(ctx context.Context, format string, opts epuboptions.EPUBOptions, cmd *converter.Converter, chain *filters.Chain) error {
	var imageProcessor epubimageprocessor.EPUBImageProcessor
	if opts.Image.Format == "copy" {
		imageProcessor = epubimagepassthrough.New(opts)
	} else {
		p := epubimageprocessor.New(opts)
		if chain != nil {
			if sp, ok := p.(interface{ SetRecipe(*filters.Chain) }); ok {
				sp.SetRecipe(chain)
			}
		}
		imageProcessor = p
	}

	images, err := imageProcessor.Load(ctx)
	if err != nil {
		return err
	}

	// Sort and validate
	sort.Slice(images, func(i, j int) bool {
		if images[i].Id == images[j].Id {
			return images[i].Part < images[j].Part
		}
		return images[i].Id < images[j].Id
	})

	if opts.Strict {
		for _, img := range images {
			if img.Error != nil {
				return fmt.Errorf("strict mode: %s: %v",
					filepath.Join(img.Path, img.Name), img.Error)
			}
		}
	}

	if len(images) == 0 {
		return fmt.Errorf("no images found")
	}

	// Separate cover and build parts
	cover := images[0]
	pageImages := images
	if opts.Image.HasCover {
		pageImages = images[1:]
	}

	parts := []output.OutputPart{{
		Cover:      cover,
		Images:     pageImages,
		PartNumber: 1,
		TotalParts: 1,
		Metadata: output.PartMetadata{
			Title:       opts.Title,
			Author:      opts.Author,
			Publisher:   "GO Comic Converter",
			ImageConfig: opts.Image,
		},
	}}

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

	paths, err := writer.Write(ctx, parts, opts)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Println("\nCancelled")
			os.Exit(1)
		}
		return err
	}
	_ = paths
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
		utils.Println(string(out))
		os.Exit(0)
	}
	if cmd.Options.RecipeSave && cmd.Options.Recipe == "" {
		recipe := filters.Recipe{
			APIVersion:  1,
			Name:        "custom",
			Description: "Recipe from current options",
			Filters:     optionsToFilterConfigs(&cmd.Options.EPUBOptions),
		}
		out, err := yaml.Marshal(recipe)
		if err != nil {
			cmd.Fatal(fmt.Errorf("failed to marshal recipe: %w", err))
		}
		utils.Println(string(out))
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
			utils.Println(string(recipeData))
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
		for _, f := range output.Available() {
			runFormat := f
			runOpts := cmd.Options.EPUBOptions
			runOpts.Output = cmd.Options.Output

			writer := output.Get(runFormat)
			if writer == nil {
				continue
			}

			// Adjust extension
			ext := filepath.Ext(runOpts.Output)
			if ext != "" {
				runOpts.Output = runOpts.Output[:len(runOpts.Output)-len(ext)] + writer.Extension()
			} else {
				runOpts.Output = runOpts.Output + writer.Extension()
			}

			if err := runSingleFormat(ctx, runFormat, runOpts, cmd, chain); err != nil {
				cmd.Fatal(err)
			}
		}
	} else if format == "epub" {
		// Legacy EPUB path: handles everything internally
		if err := epub.New(cmd.Options.EPUBOptions).Write(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				utils.Println("\nCancelled")
				os.Exit(1)
			}
			if errors.Is(err, epub.ErrImageCorrupted) {
				if !cmd.Options.Dry {
					cmd.Stats()
				}
			}
			utils.Fatalf("Error: %v\n", err)
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

// optionsToFilterConfigs converts the current image options into a list of
// filter configurations that approximates the default processing chain.
func optionsToFilterConfigs(opts *epuboptions.EPUBOptions) []filters.FilterConfig {
	var cfgs []filters.FilterConfig
	img := opts.Image
	if img.Crop.Enabled {
		cfgs = append(cfgs, filters.FilterConfig{
			Name: "auto_crop",
			Params: map[string]any{
				"left":   img.Crop.Left,
				"up":     img.Crop.Up,
				"right":  img.Crop.Right,
				"bottom": img.Crop.Bottom,
			},
		})
	}
	if img.AutoContrast {
		cfgs = append(cfgs, filters.FilterConfig{Name: "auto_contrast"})
	}
	if img.Contrast != 0 {
		cfgs = append(cfgs, filters.FilterConfig{
			Name:   "contrast",
			Params: map[string]any{"amount": float64(img.Contrast) / 100.0},
		})
	}
	if img.Brightness != 0 {
		cfgs = append(cfgs, filters.FilterConfig{
			Name:   "brightness",
			Params: map[string]any{"amount": float64(img.Brightness) / 100.0},
		})
	}
	if img.Resize && img.View.Width > 0 && img.View.Height > 0 {
		cfgs = append(cfgs, filters.FilterConfig{
			Name: "resize",
			Params: map[string]any{
				"width":  img.View.Width,
				"height": img.View.Height,
			},
		})
	}
	if img.GrayScale {
		cfgs = append(cfgs, filters.FilterConfig{Name: "grayscale"})
	}
	return cfgs
}
