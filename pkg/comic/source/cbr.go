package source

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"io"
	"path/filepath"
	"sort"
	"sync"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/sortpath"
	"github.com/nwaples/rardecode/v2"
)

// cbrSource implements Source for CBR/RAR comic archives.
type cbrSource struct {
	includeWebpTiff bool
	input    string
	sortMode int
}

func (c *cbrSource) Name() string {
	return c.input
}

func (c *cbrSource) Load(ctx context.Context) (<-chan epubimageloader.Task, int, error) {
	var isSolid bool
	files, err := rardecode.List(c.input)
	if err != nil {
		return nil, 0, err
	}

	names := make([]string, 0)
	for _, f := range files {
		if !f.IsDir && isSupportedImage(f.Name) {
			if f.Solid {
				isSolid = true
			}
			names = append(names, f.Name)
		}
	}

	totalImages := len(names)
	if totalImages == 0 {
		return nil, 0, epubimageloader.ErrNoImagesFound
	}

	sort.Sort(sortpath.By(names, c.sortMode))

	indexedNames := make(map[string]int)
	for i, name := range names {
		indexedNames[name] = i
	}

	type job struct {
		Id   int
		Name string
		Open func() (io.ReadCloser, error)
	}

	jobs := make(chan job)
	feederErr := make(chan error, 1)
	go func() {
		defer close(jobs)
		defer func() {
			if r := recover(); r != nil {
				select {
				case feederErr <- fmt.Errorf("feeder panic: %v", r):
				default:
				}
			}
		}()
		if isSolid {
			r, rerr := rardecode.OpenReader(c.input)
			if rerr != nil {
				feederErr <- fmt.Errorf("error processing %s: %w", c.input, rerr)
				return
			}
			defer func(r *rardecode.ReadCloser) {
				_ = r.Close()
			}(r)
			for {
				f, rrerr := r.Next()
				if rrerr != nil {
					if rrerr == io.EOF {
						break
					}
					feederErr <- fmt.Errorf("error reading archive: %w", rrerr)
					return
				}
				if i, ok := indexedNames[f.Name]; ok {
					// Solid RAR requires sequential reading; buffer the full entry
					var b bytes.Buffer
					_, rrerr = io.Copy(&b, r)
					if rrerr != nil {
						feederErr <- fmt.Errorf("error reading %s: %w", f.Name, rrerr)
						return
					}
					jobs <- job{i, f.Name, func() (io.ReadCloser, error) {
						return io.NopCloser(bytes.NewReader(b.Bytes())), nil
					}}
				}
			}
		} else {
			for _, img := range files {
				if i, ok := indexedNames[img.Name]; ok {
					jobs <- job{i, img.Name, img.Open}
				}
			}
		}
	}()

	// Send file to the queue
	output := make(chan epubimageloader.Task, maxProcs())
	wg := &sync.WaitGroup{}

	// Check if feeder had an immediate error
	select {
	case fErr := <-feederErr:
		return nil, 0, fErr
	default:
	}

	for range decodeWorkers() {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					select {
					case feederErr <- fmt.Errorf("worker panic: %v", r):
					default:
					}
				}
			}()

			for job := range jobs {
				// Check if feeder had an error
				select {
				case fErr := <-feederErr:
					feederErr <- fErr // put it back for the caller
					return
				default:
				}

				var img image.Image
				var err error
				var f io.ReadCloser
				f, err = job.Open()
				if err == nil {
					img, _, err = epubimageloader.DecodeBounded(f, epubimageloader.MaxImageDim)
				}
				_ = f.Close()

				p, fn := filepath.Split(filepath.Clean(job.Name))
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

	// Check for feeder errors after workers drain
	select {
	case fErr := <-feederErr:
		return nil, 0, fErr
	default:
	}

	return output, totalImages, nil
}
