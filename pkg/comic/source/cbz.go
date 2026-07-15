package source

import (
	"archive/zip"
	"bytes"
	"context"
	"image"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/sortpath"
)

// cbzSource implements Source for CBZ/ZIP comic archives.
type cbzSource struct {
	includeWebpTiff bool
	input    string
	sortMode int
}

func (c *cbzSource) Name() string {
	return c.input
}

func (c *cbzSource) Load(ctx context.Context) (<-chan epubimageloader.Task, int, error) {
	r, err := zip.OpenReader(c.input)
	if err != nil {
		return nil, 0, err
	}

	images := make([]*zip.File, 0)
	for _, f := range r.File {
		if !f.FileInfo().IsDir() && isSupportedImage(f.Name) {
			images = append(images, f)
		}
	}

	totalImages := len(images)
	if totalImages == 0 {
		_ = r.Close()
		return nil, 0, epubimageloader.ErrNoImagesFound
	}

	var names []string
	for _, img := range images {
		names = append(names, img.Name)
	}
	sort.Sort(sortpath.By(names, c.sortMode))

	indexedNames := make(map[string]int)
	for i, name := range names {
		indexedNames[name] = i
	}

	type job struct {
		Id int
		F  *zip.File
	}
	jobs := make(chan job)
	go func() {
		defer close(jobs)
		for _, img := range images {
			jobs <- job{indexedNames[img.Name], img}
		}
	}()

	output := make(chan epubimageloader.Task, maxProcs())
	wg := &sync.WaitGroup{}
	for range decodeWorkers() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				var img image.Image
				var err error
				var f io.ReadCloser
				f, err = job.F.Open()
				if err == nil {
					img, _, err = epubimageloader.DecodeBounded(f, epubimageloader.MaxImageDim)
				}
				_ = f.Close()

				p, fn := filepath.Split(filepath.Clean(job.F.Name))
				if err != nil {
					img = epubimageloader.CorruptedImage(p, fn)
				}
				output <- epubimageloader.Task{
					Id:    job.Id,
					Image: img,
					Path:  p,
					Name:  fn,
					Error: err,
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(output)
		_ = r.Close()
	}()

	return output, totalImages, nil
}

// isSupportedImage checks if the file name has a supported image extension.
func isSupportedImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".tiff":
		return !strings.HasPrefix(filepath.Base(path), ".")
	}
	return false
}

// cbzBytesSource implements Source for CBZ/ZIP archives loaded from a byte slice.
type cbzBytesSource struct {
	includeWebpTiff bool
	data     []byte
	name     string
	sortMode int
}

func (c *cbzBytesSource) Name() string {
	return c.name
}

func (c *cbzBytesSource) Load(ctx context.Context) (<-chan epubimageloader.Task, int, error) {
	r, err := zip.NewReader(bytes.NewReader(c.data), int64(len(c.data)))
	if err != nil {
		return nil, 0, err
	}

	images := make([]*zip.File, 0)
	for _, f := range r.File {
		if !f.FileInfo().IsDir() && isSupportedImage(f.Name) {
			images = append(images, f)
		}
	}

	totalImages := len(images)
	if totalImages == 0 {
		return nil, 0, epubimageloader.ErrNoImagesFound
	}

	var names []string
	for _, img := range images {
		names = append(names, img.Name)
	}
	sort.Sort(sortpath.By(names, c.sortMode))

	indexedNames := make(map[string]int)
	for i, name := range names {
		indexedNames[name] = i
	}

	type job struct {
		Id int
		F  *zip.File
	}
	jobs := make(chan job)
	go func() {
		defer close(jobs)
		for _, img := range images {
			jobs <- job{indexedNames[img.Name], img}
		}
	}()

	output := make(chan epubimageloader.Task, maxProcs())
	wg := &sync.WaitGroup{}
	for range decodeWorkers() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				var img image.Image
				var err error
				var f io.ReadCloser
				f, err = job.F.Open()
				if err == nil {
					img, _, err = epubimageloader.DecodeBounded(f, epubimageloader.MaxImageDim)
				}
				_ = f.Close()

				p, fn := filepath.Split(filepath.Clean(job.F.Name))
				if err != nil {
					img = epubimageloader.CorruptedImage(p, fn)
				}
				output <- epubimageloader.Task{
					Id:    job.Id,
					Image: img,
					Path:  p,
					Name:  fn,
					Error: err,
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(output)
	}()

	return output, totalImages, nil
}
