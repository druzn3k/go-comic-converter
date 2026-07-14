package source

import (
	"context"
	"image"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/sortpath"
)

// dirSource implements Source for a directory of image files.
type dirSource struct {
	includeWebpTiff bool
	input    string
	sortMode int
}

func (d *dirSource) Name() string {
	return d.input
}

func (d *dirSource) Load(ctx context.Context) (<-chan epubimageloader.Task, int, error) {
	images := make([]string, 0)

	input := filepath.Clean(d.input)
	err := filepath.WalkDir(input, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip symlinks to prevent directory traversal outside input
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !entry.IsDir() && epubimageloader.IsSupportedImage(path, d.includeWebpTiff) {
			images = append(images, path)
		}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}

	totalImages := len(images)
	if totalImages == 0 {
		return nil, 0, epubimageloader.ErrNoImagesFound
	}

	sort.Sort(sortpath.By(images, d.sortMode))

	// Queue all files with id
	type job struct {
		Id   int
		Path string
	}
	jobs := make(chan job)
	go func() {
		defer close(jobs)
		for i, path := range images {
			jobs <- job{i, path}
		}
	}()

	// Read in parallel and get an image
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
				var f *os.File
				f, err = os.Open(job.Path)
				if err == nil {
					img, _, err = epubimageloader.DecodeBounded(f, epubimageloader.MaxImageDim)
					_ = f.Close()
				}

				p, fn := filepath.Split(job.Path)
				if p == input {
					p = ""
				} else {
					p = p[len(input)+1:]
				}
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

	// Wait all done and close
	go func() {
		wg.Wait()
		close(output)
	}()

	return output, totalImages, nil
}
