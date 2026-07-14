package output

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func init() {
	Register(func() OutputWriter { return CBZWriter{} })
}

// CBZWriter produces a CBZ file (ZIP of sorted images) from processed parts.
// Since the image pipeline already produces compressed JPEGs/PNGs in the
// temp ZIP, the CBZ writer simply copies them into a new ZIP with proper
// sort ordering.
type CBZWriter struct{}

func (w CBZWriter) Format() string           { return "cbz" }
func (w CBZWriter) Extension() string        { return ".cbz" }
func (w CBZWriter) SupportsPartSplit() bool  { return false }

func (w CBZWriter) Write(ctx context.Context, parts []OutputPart, opts epuboptions.EPUBOptions) ([]string, error) {
	if len(parts) == 0 {
		return nil, fmt.Errorf("no parts to write")
	}

	// Open the temp ZIP storage containing already-processed images
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
		if err := w.writeCBZ(outputPath, part, imgStorage); err != nil {
			return outputPaths, fmt.Errorf("write cbz: %w", err)
		}
		outputPaths = append(outputPaths, outputPath)
	}

	return outputPaths, nil
}

func (w CBZWriter) writeCBZ(outputPath string, part OutputPart, imgStorage epubzip.StorageImageReader) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Collect all image paths sorted by Id then Part
	type imgEntry struct {
		ID       int
		Part     int
		EPUBPath string // path inside the temp storage ZIP
		CBZName  string // desired name inside the CBZ
	}

	var entries []imgEntry

	// Cover image
	coverEntry := imgEntry{
		ID:       part.Cover.Id,
		Part:     part.Cover.Part,
		EPUBPath: part.Cover.EPUBImgPath(),
		CBZName:  fmt.Sprintf("%s.%s", part.Cover.ImgKey(), part.Cover.Format),
	}
	entries = append(entries, coverEntry)

	// Page images
	for _, img := range part.Images {
		entries = append(entries, imgEntry{
			ID:       img.Id,
			Part:     img.Part,
			EPUBPath: img.EPUBImgPath(),
			CBZName:  fmt.Sprintf("%s.%s", img.ImgKey(), img.Format),
		})
	}

	// Sort by Id then Part (same ordering as the image pipeline)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].ID == entries[j].ID {
			return entries[i].Part < entries[j].Part
		}
		return entries[i].ID < entries[j].ID
	})

	for _, entry := range entries {
		zf := imgStorage.Get(entry.EPUBPath)
		if zf == nil {
			continue
		}

		rc, err := zf.Open()
		if err != nil {
			return err
		}

		w, err := zw.Create(entry.CBZName)
		if err != nil {
			rc.Close()
			return err
		}

		_, err = io.Copy(w, rc)
		rc.Close()
		if err != nil {
			return err
		}
	}

	return nil
}

// partOutputPath computes the output file path for a given part.
// For single-part conversions, uses opts.Output directly.
// For multi-part conversions, appends a part suffix.
func partOutputPath(opts epuboptions.EPUBOptions, part OutputPart) string {
	if part.TotalParts <= 1 {
		return opts.Output
	}
	ext := filepath.Ext(opts.Output)
	base := opts.Output[:len(opts.Output)-len(ext)]
	return fmt.Sprintf("%s Part %d of %d%s", base, part.PartNumber, part.TotalParts, ext)
}
