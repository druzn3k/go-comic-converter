package output

import (
	"context"
	"encoding/base64"
	"fmt"
	"html/template"
	"io"
	"os"
	"sort"

	_ "embed"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

func init() {
	Register(func() OutputWriter { return HTMLWriter{} })
}

//go:embed viewer.tmpl.html
var viewerTemplate string

// HTMLWriter produces a self-contained HTML file with all images
// embedded as base64 data URIs and a vanilla JS page-flip viewer.
type HTMLWriter struct{}

func (w HTMLWriter) Format() string          { return "html" }
func (w HTMLWriter) Extension() string       { return ".html" }
func (w HTMLWriter) SupportsPartSplit() bool { return false }

type htmlPageData struct {
	Title string
	Pages []template.JS
}

// Write produces one HTML file per part by reading processed images
// from the temp ZIP, base64-encoding them, and rendering the viewer template.
func (w HTMLWriter) Write(ctx context.Context, parts []OutputPart, opts epuboptions.EPUBOptions) ([]string, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("no parts to write")
	}

	imgStorage, err := epubzip.NewStorageImageReader(opts.ImgStorage())
	if err != nil {
		return nil, fmt.Errorf("open image storage: %w", err)
	}
	defer imgStorage.Close()
	defer imgStorage.Remove()

	outputPaths := make([]string, 0, len(parts))

	for _, part := range parts {
		select {
		case <-ctx.Done():
			return outputPaths, ctx.Err()
		default:
		}

		outputPath := partOutputPath(opts, part)
		if err := w.writeHTML(outputPath, part, imgStorage); err != nil {
			return outputPaths, fmt.Errorf("write html: %w", err)
		}
		outputPaths = append(outputPaths, outputPath)
	}

	return outputPaths, nil
}

func (w HTMLWriter) writeHTML(outputPath string, part OutputPart, imgStorage epubzip.StorageImageReader) error {
	type imgEntry struct {
		ID       int
		Part     int
		EPUBPath string
		Format   string
	}

	var entries []imgEntry

	entries = append(entries, imgEntry{
		ID:       part.Cover.Id,
		Part:     part.Cover.Part,
		EPUBPath: part.Cover.EPUBImgPath(),
		Format:   part.Cover.Format,
	})

	for _, img := range part.Images {
		entries = append(entries, imgEntry{
			ID:       img.Id,
			Part:     img.Part,
			EPUBPath: img.EPUBImgPath(),
			Format:   img.Format,
		})
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ID == entries[j].ID {
			return entries[i].Part < entries[j].Part
		}
		return entries[i].ID < entries[j].ID
	})

	pages := make([]template.JS, 0, len(entries))
	for _, entry := range entries {
		zf := imgStorage.Get(entry.EPUBPath)
		if zf == nil {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			return err
		}

		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}

		mimeType := "image/" + entry.Format
		dataURI := "data:" + mimeType + ";base64," + base64.StdEncoding.EncodeToString(data)
		pages = append(pages, template.JS(dataURI))
	}

	tmpl, err := template.New("viewer").Parse(viewerTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	return tmpl.Execute(f, htmlPageData{
		Title: part.Metadata.Title,
		Pages: pages,
	})
}
