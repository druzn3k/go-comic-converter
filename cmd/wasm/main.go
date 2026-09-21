//go:build js

package main

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"syscall/js"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/converter"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

type wasmOptions struct {
	InputName         string  `json:"inputName"`
	OutputFormat      string  `json:"outputFormat"`
	ImageFormat       string  `json:"imageFormat"`
	Quality           int     `json:"quality"`
	Grayscale         bool    `json:"grayscale"`
	GrayscaleMode     int     `json:"grayscaleMode"`
	Crop              bool    `json:"crop"`
	CropLeft          int     `json:"cropLeft"`
	CropUp            int     `json:"cropUp"`
	CropRight         int     `json:"cropRight"`
	CropBottom        int     `json:"cropBottom"`
	CropLimit         int     `json:"cropLimit"`
	Brightness        int     `json:"brightness"`
	Contrast          int     `json:"contrast"`
	AutoContrast      bool    `json:"autoContrast"`
	AutoRotate        bool    `json:"autoRotate"`
	AutoSplitDouble   bool    `json:"autoSplitDouble"`
	KeepDoubleIfSplit bool    `json:"keepDoubleIfSplit"`
	KeepSplitAspect   bool    `json:"keepSplitAspect"`
	NoBlankImage      bool    `json:"noBlankImage"`
	Manga             bool    `json:"manga"`
	HasCover          bool    `json:"hasCover"`
	Resize            bool    `json:"resize"`
	Profile           string  `json:"profile"`
	AspectRatio       float64 `json:"aspectRatio"`
	PortraitOnly      bool    `json:"portraitOnly"`
	TitlePage         int     `json:"titlePage"`
	LimitMb           int     `json:"limitMb"`
	Title             string  `json:"title"`
	Author            string  `json:"author"`
	Series            string  `json:"series"`
	Number            string  `json:"number"`
	Genre             string  `json:"genre"`
	MangaTag          bool    `json:"mangaTag"`
	Recipe            string  `json:"recipe"`
}

func main() {
	c := make(chan struct{}, 0)

	js.Global().Set("convert", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) < 2 {
			return "error: expected 2 arguments (inputBytes, options)"
		}

		// Read input bytes from JS
		inputBytes := make([]byte, args[0].Get("byteLength").Int())
		js.CopyBytesToGo(inputBytes, args[0])

		// Parse options from JS
		optsJSON := args[1].String()
		var wopts wasmOptions
		if err := json.Unmarshal([]byte(optsJSON), &wopts); err != nil {
			return "error: invalid options: " + err.Error()
		}

		// Setup virtual filesystem paths
		// input bytes flow directly via InputBytes
		baseName := wopts.InputName
		for _, ext := range []string{".cbz", ".zip", ".cbr", ".rar", ".pdf"} {
			baseName = strings.TrimSuffix(baseName, ext)
		}

		// Determine output format and set correct extension
		outputFormat := wopts.OutputFormat
		if outputFormat == "" {
			outputFormat = "epub"
		}
		var outputExt string
		switch outputFormat {
		case "kepub":
			outputExt = ".kepub.epub"
		case "cbz":
			outputExt = ".cbz"
		case "html":
			outputExt = ".html"
		default:
			outputExt = ".epub"
		}
		outputPath := "/output/" + baseName + outputExt

		// Ensure output directory exists
		if err := os.MkdirAll("/output", 0755); err != nil {
			return "error: " + err.Error()
		}
		// Input bytes passed directly via InputBytes — no memfs write

		// Map profiles to dimensions (same presets as the CLI)
		profileWidth := 1200
		profileHeight := 1920
		if p, ok := converter.NewProfiles()[wopts.Profile]; ok {
			profileWidth = p.Width
			profileHeight = p.Height
		}

		// Build EPUBOptions
		opts := epuboptions.EPUBOptions{
			Input:        wopts.InputName,
			InputBytes:   inputBytes,
			Output:       outputPath,
			Title:        wopts.Title,
			Author:       wopts.Author,
			Series:       wopts.Series,
			Number:       wopts.Number,
			Genre:        wopts.Genre,
			Manga:        wopts.MangaTag,
			TitlePage:    wopts.TitlePage,
			LimitMb:      wopts.LimitMb,
			OutputFormat: outputFormat,
			Quiet:        true,
			Image: epuboptions.Image{
				Quality:                   wopts.Quality,
				GrayScale:                 wopts.Grayscale,
				GrayScaleMode:             wopts.GrayscaleMode,
				Brightness:                wopts.Brightness,
				Contrast:                  wopts.Contrast,
				AutoContrast:              wopts.AutoContrast,
				AutoRotate:                wopts.AutoRotate,
				AutoSplitDoublePage:       wopts.AutoSplitDouble,
				KeepDoublePageIfSplit:     wopts.KeepDoubleIfSplit,
				KeepSplitDoublePageAspect: wopts.KeepSplitAspect,
				NoBlankImage:              wopts.NoBlankImage,
				Manga:                     wopts.Manga,
				HasCover:                  wopts.HasCover,
				Resize:                    wopts.Resize,
				Format:                    wopts.ImageFormat,
				View: epuboptions.View{
					Width:        profileWidth,
					Height:       profileHeight,
					AspectRatio:  wopts.AspectRatio,
					PortraitOnly: wopts.PortraitOnly,
				},
				Crop: epuboptions.Crop{
					Enabled: wopts.Crop,
					Left:    wopts.CropLeft,
					Up:      wopts.CropUp,
					Right:   wopts.CropRight,
					Bottom:  wopts.CropBottom,
					Limit:   wopts.CropLimit,
				},
			},
		}

		if opts.Image.Format == "" {
			opts.Image.Format = "jpeg"
		}
		if opts.Image.Quality == 0 {
			opts.Image.Quality = 85
		}
		if !opts.Image.GrayScale && opts.Image.GrayScaleMode == 0 {
			opts.Image.GrayScaleMode = 1
		}
		if opts.Title == "" {
			opts.Title = baseName
		}

		ctx := context.Background()

		// Report progress
		reportProgress := func(msg string) {
			js.Global().Call("onWasmProgress", msg)
		}

		reportProgress("Processing images...")

		// Run conversion through the shared converter so recipes reach EPUB too.
		var convertErr error
		var chain *filters.Chain
		if wopts.Recipe != "" {
			chain, convertErr = filters.BuiltinRecipe(wopts.Recipe)
			if convertErr != nil {
				return "error: " + convertErr.Error()
			}
		}
		if chain != nil {
			convertErr = comic.NewWithRecipe(opts, chain).Convert(ctx)
		} else {
			convertErr = comic.New(opts).Convert(ctx)
		}

		if convertErr != nil {
			return "error: " + convertErr.Error()
		}

		reportProgress("Reading output...")

		// Read output from virtual filesystem
		// outputPath already has the correct extension per format
		readPath := outputPath
		// For "all" format, return the EPUB (primary format)
		if outputFormat == "all" {
			readPath = "/output/" + baseName + ".epub"
		}

		outputBytes, err := os.ReadFile(readPath)
		if err != nil {
			return "error: reading output: " + err.Error()
		}

		reportProgress("Done")

		// Return output bytes to JS
		dst := js.Global().Get("Uint8Array").New(len(outputBytes))
		js.CopyBytesToJS(dst, outputBytes)

		// Also return the filename for the download
		result := js.Global().Get("Object").New()
		result.Set("data", dst)
		result.Set("filename", readPath)
		return result
	}))

	<-c
}
