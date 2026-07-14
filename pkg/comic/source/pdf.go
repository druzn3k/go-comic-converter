package source

import (
	"context"
	"fmt"
	"image"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageloader"
	pdfimage "github.com/raff/pdfreader/image"
	"github.com/raff/pdfreader/pdfread"
)

// pdfSource implements Source for PDF files.
type pdfSource struct {
	input    string
	sortMode int
}

func (p *pdfSource) Name() string {
	return p.input
}

func (p *pdfSource) Load(ctx context.Context) (<-chan epubimageloader.Task, int, error) {
	pdf := pdfread.Load(p.input)
	if pdf == nil {
		return nil, 0, fmt.Errorf("can't read pdf")
	}

	totalImages := len(pdf.Pages())
	pageFmt := "page " + numberOfDigitsFmt(totalImages)
	output := make(chan epubimageloader.Task)
	go func() {
		defer close(output)
		defer pdf.Close()
		for i := range totalImages {
			var img image.Image
			var err error
			img, err = pdfimage.Extract(pdf, i+1)

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

	return output, totalImages, nil
}

// numberOfDigitsFmt returns a fmt.Sprintf format string that zero-pads numbers to
// the width needed for the given count, e.g. for 100 it returns "%03d".
func numberOfDigitsFmt(n int) string {
	x, count := 10, 1
	if n < 0 {
		n = -n
		count++
	}
	for ; x <= n; count++ {
		x *= 10
	}
	return "%0" + fmt.Sprintf("%d", count) + "d"
}
