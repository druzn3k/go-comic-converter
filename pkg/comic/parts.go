package comic

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubzip"
)

// GetParts loads images via the processor, sorts them, extracts the cover,
// opens the temp ZIP reader, and splits into size-bounded parts.
// This is format-agnostic — output writers consume the resulting []Part.
func GetParts(ctx context.Context, proc epubimageprocessor.EPUBImageProcessor, opts Options) (parts []Part, imgStorage epubzip.StorageImageReader, err error) {
	images, err := proc.Load(ctx)
	if err != nil {
		return parts, epubzip.StorageImageReader{}, err
	}

	// sort result by id and part
	sort.Slice(images, func(i, j int) bool {
		if images[i].Id == images[j].Id {
			return images[i].Part < images[j].Part
		}
		return images[i].Id < images[j].Id
	})

	// Check for corrupted images in strict mode
	if opts.Strict {
		for _, img := range images {
			if img.Error != nil {
				return parts, epubzip.StorageImageReader{}, fmt.Errorf("strict mode: %s: %w",
					filepath.Join(img.Path, img.Name), img.Error)
			}
		}
	}

	if len(images) == 0 {
		return parts, epubzip.StorageImageReader{}, fmt.Errorf("no images found")
	}

	parts = make([]Part, 0)
	cover := images[0]
	if opts.Image.HasCover || (cover.DoublePage && !opts.Image.KeepDoublePageIfSplit) {
		images = images[1:]
	}

	if opts.Dry {
		parts = append(parts, Part{
			Cover:  cover,
			Images: images,
		})
		return parts, epubzip.StorageImageReader{}, nil
	}

	imgStorage, err = epubzip.NewStorageImageReader(opts.ImgStorage())
	if err != nil {
		return parts, epubzip.StorageImageReader{}, err
	}

	// compute size of the EPUB part and try to be as close as possible of the target
	maxSize := uint64(opts.LimitMb * 1024 * 1024)
	xhtmlSize := uint64(1024)
	// descriptor files + title + cover
	baseSize := uint64(128*1024) + imgStorage.Size(cover.EPUBImgPath())*2

	currentSize := baseSize
	currentImages := make([]epubimage.EPUBImage, 0)
	part := 1

	for _, img := range images {
		imgSize := imgStorage.Size(img.EPUBImgPath()) + xhtmlSize
		if maxSize > 0 && len(currentImages) > 0 && currentSize+imgSize > maxSize {
			parts = append(parts, Part{
				Cover:  cover,
				Images: currentImages,
			})
			part++
			currentSize = baseSize
			currentImages = make([]epubimage.EPUBImage, 0)
		}
		currentSize += imgSize
		currentImages = append(currentImages, img)
	}
	if len(currentImages) > 0 {
		parts = append(parts, Part{
			Cover:  cover,
			Images: currentImages,
		})
	}

	return parts, imgStorage, nil
}
