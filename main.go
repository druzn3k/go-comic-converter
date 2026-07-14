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

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/converter"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimagepassthrough"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/utils"
	"github.com/celogeek/go-comic-converter/v3/pkg/comic/output"
	"github.com/celogeek/go-comic-converter/v3/pkg/epub"
	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
	comicServer "github.com/celogeek/go-comic-converter/v3/pkg/comic/server"
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

	latestVersion := "unknown"
	resp, err := http.Get("https://api.github.com/repos/celogeek/go-comic-converter/tags")
	if err == nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			var tags []struct{ Name string `json:"name"` }
			if err := json.NewDecoder(resp.Body).Decode(&tags); err == nil && len(tags) > 0 {
				latestVersion = tags[0].Name
			}
		}
	}

	utils.Printf(`go-comic-converter
  Path             : %s
  Sum              : %s
  Version          : %s
  Available Version: %s

To install the latest version:
$ go install github.com/celogeek/go-comic-converter/v3@%s
`,
		bi.Main.Path,
		bi.Main.Sum,
		bi.Main.Version,
		latestVersion,
		latestVersion,
	)
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
	s := comicServer.New(comicServer.Config{
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

// runSingleFormat dispatches a non-EPUB format through the OutputWriter path.
// It loads images via the processor, creates OutputParts, and calls the
// registered OutputWriter for the given format.
func runSingleFormat(ctx context.Context, format string, opts epuboptions.EPUBOptions, cmd *converter.Converter) error {
	var imageProcessor epubimageprocessor.EPUBImageProcessor
	if opts.Image.Format == "copy" {
		imageProcessor = epubimagepassthrough.New(opts)
	} else {
		imageProcessor = epubimageprocessor.New(opts)
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

			if err := runSingleFormat(ctx, runFormat, runOpts, cmd); err != nil {
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
		runSingleFormat(ctx, format, cmd.Options.EPUBOptions, cmd)
	}

	if !cmd.Options.Dry {
		cmd.Stats()
	}
}
