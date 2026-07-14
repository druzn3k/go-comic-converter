// Package epubimageprocessor extract and transform image into a compressed jpeg.
package epubimageprocessor

import (
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"sync"

	"github.com/disintegration/gift"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimagefilters"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubprogress"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/filters"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

type EPUBImageProcessor interface {
	Load(ctx context.Context) (images []epubimage.EPUBImage, err error)
	CoverTitleData(o CoverTitleDataOptions) (epubzip.Image, error)
}

type ePUBImageProcessor struct {
	epuboptions.EPUBOptions
}

func New(o epuboptions.EPUBOptions) EPUBImageProcessor {
	return ePUBImageProcessor{o}
}

// Load extract and convert images
func (e ePUBImageProcessor) Load(ctx context.Context) (images []epubimage.EPUBImage, err error) {
	imageCount, imageInput, err := e.load(ctx)
	if err != nil {
		return nil, err
	}

	// dry run, skip conversion
	if e.Dry {
		for img := range imageInput {
			images = append(images, epubimage.EPUBImage{
				Id:     img.Id,
				Path:   img.Path,
				Name:   img.Name,
				Format: e.Image.Format,
			})
		}

		return images, nil
	}

	imageOutput := make(chan epubimage.EPUBImage, e.WorkersRatio(50))

	// processing
	bar := epubprogress.New(epubprogress.Options{
		Quiet:       e.Quiet,
		Json:        e.Json,
		Max:         imageCount,
		Description: "Processing",
		CurrentJob:  1,
		TotalJob:    2,
	})
	wg := &sync.WaitGroup{}
	errc := make(chan error, 1)

	imgStorage, err := epubzip.NewStorageImageWriter(e.ImgStorage(), e.Image.Format)
	if err != nil {
		_ = bar.Close()
		return nil, err
	}

	wr := 50
	if e.Image.Format == "png" {
		wr = 100
	}
	for range e.WorkersRatio(wr) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					select {
					case errc <- fmt.Errorf("worker panic: %v", r):
					default:
					}
				}
			}()

			for input := range imageInput {
				select {
				case <-ctx.Done():
					return
				default:
				}

				img := e.transformImage(input, 0, e.Image.Manga)

				// do not keep double page if requested
				if !(img.DoublePage && input.Id > 0 &&
					e.EPUBOptions.Image.AutoSplitDoublePage && !e.EPUBOptions.Image.KeepDoublePageIfSplit) {
					if stErr := imgStorage.Add(img.EPUBImgPath(), img.Raw, e.Image.Quality); stErr != nil {
						select {
						case errc <- fmt.Errorf("error with %s: %w", input.Name, stErr):
						default:
						}
						return
					}
					// do not keep raw image except for cover
					if img.Id > 0 {
						img.Raw = nil
					}
					select {
					case imageOutput <- img:
					case <-ctx.Done():
						return
					}
				}

				// DOUBLE PAGE
				if !e.Image.AutoSplitDoublePage || // No split required
					!img.DoublePage || // Not a double page
					(e.Image.HasCover && img.Id == 0) { // Cover
					continue
				}

				for i, b := range []bool{e.Image.Manga, !e.Image.Manga} {
					img = e.transformImage(input, i+1, b)
					if stErr := imgStorage.Add(img.EPUBImgPath(), img.Raw, e.Image.Quality); stErr != nil {
						select {
						case errc <- fmt.Errorf("error with %s: %w", input.Name, stErr):
						default:
						}
						return
					}
					img.Raw = nil
					select {
					case imageOutput <- img:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		_ = imgStorage.Close()
		close(imageOutput)
	}()

	// Check for early errors
	select {
	case loadErr := <-errc:
		// Drain remaining input to unblock producers
		go func() {
			for range imageInput {
			}
		}()
		_ = bar.Close()
		return nil, loadErr
	default:
	}

	for img := range imageOutput {
		if img.Part == 0 {
			_ = bar.Add(1)
		}
		if e.Image.NoBlankImage && img.IsBlank {
			continue
		}
		images = append(images, img)
	}

	// Check for errors after processing
	select {
	case loadErr := <-errc:
		_ = bar.Close()
		return nil, loadErr
	default:
	}

	_ = bar.Close()

	if len(images) == 0 {
		return nil, epubimageloader.ErrNoImagesFound
	}

	return images, nil
}

func (e ePUBImageProcessor) createImage(src image.Image, r image.Rectangle) draw.Image {
	if e.EPUBOptions.Image.GrayScale {
		return image.NewGray(r)
	}

	switch t := src.(type) {
	case *image.Gray:
		return image.NewGray(r)
	case *image.Gray16:
		return image.NewGray16(r)
	case *image.RGBA:
		return image.NewRGBA(r)
	case *image.RGBA64:
		return image.NewRGBA64(r)
	case *image.NRGBA:
		return image.NewNRGBA(r)
	case *image.NRGBA64:
		return image.NewNRGBA64(r)
	case *image.Alpha:
		return image.NewAlpha(r)
	case *image.Alpha16:
		return image.NewAlpha16(r)
	case *image.CMYK:
		return image.NewCMYK(r)
	case *image.Paletted:
		return image.NewPaletted(r, t.Palette)
	default:
		return image.NewNRGBA64(r)
	}
}

// transform image into 1 or 3 images
// only doublepage with autosplit has 3 versions
func (e ePUBImageProcessor) transformImage(input epubimageloader.Task, part int, right bool) epubimage.EPUBImage {
	src := input.Image
	srcBounds := src.Bounds()

	g, dstBounds, isDoublePage := filters.DefaultChain(src, filters.DefaultChainOpts{
		Image:                     e.Image,
		Part:                      part,
		Right:                     right,
		SrcBounds:                 srcBounds,
		CurrentBounds:             srcBounds,
		KeepSplitDoublePageAspect: e.Image.KeepSplitDoublePageAspect,
	})

	dst := e.createImage(src, dstBounds)
	g.Draw(dst, src)

	return epubimage.EPUBImage{
		Id:                  input.Id,
		Part:                part,
		Raw:                 dst,
		Width:               dst.Bounds().Dx(),
		Height:              dst.Bounds().Dy(),
		IsBlank:             dst.Bounds().Dx() == 1 && dst.Bounds().Dy() == 1,
		DoublePage:          isDoublePage,
		Path:                input.Path,
		Name:                input.Name,
		Format:              e.Image.Format,
		OriginalAspectRatio: float64(src.Bounds().Dy()) / float64(src.Bounds().Dx()),
		Error:               input.Error,
	}
}

type CoverTitleDataOptions struct {
	Src         image.Image
	Name        string
	Text        string
	Align       string
	PctWidth    int
	PctMargin   int
	MaxFontSize int
	BorderSize  int
}

func (e ePUBImageProcessor) cover16LevelOfGray(bounds image.Rectangle) draw.Image {
	return image.NewPaletted(bounds, color.Palette{
		color.Gray{},
		color.Gray{Y: 0x11},
		color.Gray{Y: 0x22},
		color.Gray{Y: 0x33},
		color.Gray{Y: 0x44},
		color.Gray{Y: 0x55},
		color.Gray{Y: 0x66},
		color.Gray{Y: 0x77},
		color.Gray{Y: 0x88},
		color.Gray{Y: 0x99},
		color.Gray{Y: 0xAA},
		color.Gray{Y: 0xBB},
		color.Gray{Y: 0xCC},
		color.Gray{Y: 0xDD},
		color.Gray{Y: 0xEE},
		color.Gray{Y: 0xFF},
	})
}

// CoverTitleData create a title page with the cover
func (e ePUBImageProcessor) CoverTitleData(o CoverTitleDataOptions) (epubzip.Image, error) {
	// Create a blur version of the cover
	g := gift.New(epubimagefilters.CoverTitle(o.Text, o.Align, o.PctWidth, o.PctMargin, o.MaxFontSize, o.BorderSize))
	var dst draw.Image
	if o.Name == "cover" && e.Image.GrayScale {
		dst = e.cover16LevelOfGray(o.Src.Bounds())
	} else {
		dst = e.createImage(o.Src, g.Bounds(o.Src.Bounds()))
	}
	g.Draw(dst, o.Src)

	return epubzip.CompressImage(
		"OEBPS/Images/"+o.Name+".jpeg",
		"jpeg",
		dst,
		e.Image.Quality,
	)
}
