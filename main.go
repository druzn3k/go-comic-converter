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
	"runtime/debug"
	"syscall"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/converter"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/utils"
	"github.com/celogeek/go-comic-converter/v3/pkg/comic/output"
	"github.com/celogeek/go-comic-converter/v3/pkg/epub"
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

	// Fetch latest version from GitHub API
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

	// Determine output format:
	// 1. Explicit -output-format flag
	// 2. Profile PreferredFormat
	// 3. Default to "epub"
	format := cmd.Options.OutputFormat
	if format == "" || format == "epub" {
		if profile := cmd.Options.GetProfile(); profile != nil && profile.PreferredFormat != "" {
			format = profile.PreferredFormat
		}
	}
	if format == "" {
		format = "epub"
	}

	if format == "epub" {
		// Use existing EPUB generation path
		if err := epub.New(cmd.Options.EPUBOptions).Write(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				utils.Println("\nCancelled")
				os.Exit(1)
			}
			if errors.Is(err, epub.ErrImageCorrupted) {
				if !cmd.Options.Dry {
					cmd.Stats()
				}
				utils.Fatalf("Error: %v\n", err)
			} else {
				utils.Fatalf("Error: %v\n", err)
			}
		}
	} else {
		// Use OutputWriter registry for other formats
		writer := output.Get(format)
		if writer == nil {
			cmd.Fatal(fmt.Errorf("unsupported output format: %s", format))
		}

		paths, err := writer.Write(ctx, nil, cmd.Options.EPUBOptions)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				utils.Println("\nCancelled")
				os.Exit(1)
			}
			utils.Fatalf("Error: %v\n", err)
		}
		_ = paths
	}

	if !cmd.Options.Dry {
		cmd.Stats()
	}
}
