package epubimageprocessor

import (
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimageloader"
	"context"
	"bytes"
	"archive/zip"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"golang.org/x/image/font/gofont/gomonobold"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/nwaples/rardecode/v2"
	pdfimage "github.com/raff/pdfreader/image"
	"github.com/raff/pdfreader/pdfread"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/sortpath"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/utils"
)


// decodeBounded decodes an image but rejects dimensions exceeding maxDim
// to prevent decompression bomb attacks.



// only accept jpg, png and webp as source file
func (e ePUBImageProcessor) isSupportedImage(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg", ".png", ".webp", ".tiff":
		{
			return !strings.HasPrefix(filepath.Base(path), ".")
		}
	}
	return false
}

// load images from input
func (e ePUBImageProcessor) load(ctx context.Context) (totalImages int, output chan epubimageloader.Task, err error) {
	fi, err := os.Stat(e.Input)
	if err != nil {
		return
	}

	// get all images though a channel of bytes
	if fi.IsDir() {
		return e.loadDir(ctx)
	} else {
		switch ext := strings.ToLower(filepath.Ext(e.Input)); ext {
		case ".cbz", ".zip":
			return e.loadCbz(ctx)
		case ".cbr", ".rar":
			return e.loadCbr(ctx)
		case ".pdf":
			return e.loadPdf(ctx)
		default:
			err = fmt.Errorf("unknown file format (%s): support .cbz, .zip, .cbr, .rar, .pdf", ext)
			return
		}
	}
}

func (e ePUBImageProcessor) corruptedImage(path, name string) image.Image {
	var w, h float64 = 1200, 1920
	f, _ := truetype.Parse(gomonobold.TTF)
	face := truetype.NewFace(f, &truetype.Options{Size: 64, DPI: 72})
	txt := name
	if path != "" {
		txt += "\nin " + filepath.Clean(path)
	}
	txt += "\nis corrupted!"

	g := gg.NewContext(int(w), int(h))
	g.SetColor(color.White)
	g.Clear()
	g.SetColor(color.Black)
	g.DrawRoundedRectangle(0, 0, w, h, 0.5)
	g.SetLineWidth(6)
	g.Stroke()
	g.DrawRoundedRectangle(0, 0, 480, 640, 0.5)
	g.SetFontFace(face)
	g.DrawStringWrapped(txt, w/2, h/2, 0.5, 0.5, 640, 1.5, gg.AlignCenter)
	return g.Image()
}

// load a directory of images
func (e ePUBImageProcessor) loadDir(ctx context.Context) (totalImages int, output chan epubimageloader.Task, err error) {
	images := make([]string, 0)

	input := filepath.Clean(e.Input)
	err = filepath.WalkDir(input, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip symlinks to prevent directory traversal outside input
		if d.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if !d.IsDir() && epubimageloader.IsSupportedImage(path, true) {
			images = append(images, path)
		}
		return nil
	})

	if err != nil {
		return
	}

	totalImages = len(images)

	if totalImages == 0 {
		err = epubimageloader.ErrNoImagesFound
		return
	}

	sort.Sort(sortpath.By(images, e.SortPathMode))

	// Queue all file with id
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

	// read in parallel and get an image
	output = make(chan epubimageloader.Task, e.Workers)
	wg := &sync.WaitGroup{}
	for range e.WorkersRatio(50) {
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
				if !e.Dry {
					var f *os.File
					f, err = os.Open(job.Path)
					if err == nil {
						img, _, err = epubimageloader.DecodeBounded(f, epubimageloader.MaxImageDim)
						_ = f.Close()
					}
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

	// wait all done and close
	go func() {
		wg.Wait()
		close(output)
	}()

	return
}

// load a zip file that include images
func (e ePUBImageProcessor) loadCbz(ctx context.Context) (totalImages int, output chan epubimageloader.Task, err error) {
	r, err := zip.OpenReader(e.Input)
	if err != nil {
		return
	}

	images := make([]*zip.File, 0)
	for _, f := range r.File {
		if !f.FileInfo().IsDir() && e.isSupportedImage(f.Name) {
			images = append(images, f)
		}
	}

	totalImages = len(images)

	if totalImages == 0 {
		_ = r.Close()
		err = epubimageloader.ErrNoImagesFound
		return
	}

	var names []string
	for _, img := range images {
		names = append(names, img.Name)
	}
	sort.Sort(sortpath.By(names, e.SortPathMode))

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

	output = make(chan epubimageloader.Task, e.Workers)
	wg := &sync.WaitGroup{}
	for range e.WorkersRatio(50) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			default:
			}
			for job := range jobs {
			select {
			case <-ctx.Done():
				return
			default:
			}
				var img image.Image
				var err error
				if !e.Dry {
					var f io.ReadCloser
					f, err = job.F.Open()
					if err == nil {
						img, _, err = epubimageloader.DecodeBounded(f, epubimageloader.MaxImageDim)
					}
					_ = f.Close()
				}

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
	return
}

// load a rar file that include images
func (e ePUBImageProcessor) loadCbr(ctx context.Context) (totalImages int, output chan epubimageloader.Task, err error) {
	var isSolid bool
	files, err := rardecode.List(e.Input)
	if err != nil {
		return
	}

	names := make([]string, 0)
	for _, f := range files {
		if !f.IsDir && e.isSupportedImage(f.Name) {
			if f.Solid {
				isSolid = true
			}
			names = append(names, f.Name)
		}
	}

	totalImages = len(names)
	if totalImages == 0 {
		err = epubimageloader.ErrNoImagesFound
		return
	}

	sort.Sort(sortpath.By(names, e.SortPathMode))

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
		if isSolid && !e.Dry {
			r, rerr := rardecode.OpenReader(e.Input)
			if rerr != nil {
				feederErr <- fmt.Errorf("error processing %s: %w", e.Input, rerr)
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
				// before decode. Memory usage is entry_size + decoded_image.
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

	// send file to the queue
	output = make(chan epubimageloader.Task, e.Workers)
	wg := &sync.WaitGroup{}

	// Check if feeder had an immediate error
	select {
	case fErr := <-feederErr:
		return 0, nil, fErr
	default:
	}

	for range e.WorkersRatio(50) {
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
				if !e.Dry {
					var f io.ReadCloser
					f, err = job.Open()
					if err == nil {
						img, _, err = epubimageloader.DecodeBounded(f, epubimageloader.MaxImageDim)
					}
					_ = f.Close()
				}

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
		return 0, nil, fErr
	default:
	}

	return
}

// extract image from a pdf
func (e ePUBImageProcessor) loadPdf(ctx context.Context) (totalImages int, output chan epubimageloader.Task, err error) {
	pdf := pdfread.Load(e.Input)
	if pdf == nil {
		err = fmt.Errorf("can't read pdf")
		return
	}

	totalImages = len(pdf.Pages())
	pageFmt := "page " + utils.FormatNumberOfDigits(totalImages)
	output = make(chan epubimageloader.Task)
	go func() {
		defer close(output)
		defer pdf.Close()
		for i := range totalImages {
			var img image.Image
			var err error
			if !e.Dry {
				img, err = pdfimage.Extract(pdf, i+1)
			}

			name := fmt.Sprintf(pageFmt, i+1)
			if err != nil {
				img = epubimageloader.CorruptedImage("", name)
			}
			output <- epubimageloader.Task{
				Id:    i,
				Image: img,
				Path:  "",
				Name:  name,
				Error: err,
			}
		}
	}()

	return
}
