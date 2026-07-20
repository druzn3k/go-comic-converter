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
			img, err = extractSafe(pdf, i+1)

			name := fmt.Sprintf(pageFmt, i+1)
			if err != nil || img == nil {
				img = epubimageloader.CorruptedImage("", name)
				if err == nil {
					err = fmt.Errorf("can't extract image")
				}
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

// extractSafe extracts the first image XObject from a PDF page, returning
// (nil, error) instead of crashing via log.Fatal on unsupported encodings.
// Performs deterministic pre-validation for known log.Fatal branches, then
// wraps pdfimage.Extract in a recover() shim for any other panic.
func extractSafe(pdf *pdfread.PdfReaderT, page int) (image.Image, error) {
	// Pre-validation: check for known log.Fatal branches before calling Extract.
	if err := prevalidateExtract(pdf, page); err != nil {
		return nil, err
	}

	// Run inside a goroutine with recover() for non-log.Fatal panics.
	type result struct {
		img image.Image
		err error
	}
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{nil, fmt.Errorf("pdfimage.Extract panicked: %v", r)}
			}
		}()
		img, err := pdfimage.Extract(pdf, page)
		ch <- result{img, err}
	}()
	r := <-ch
	return r.img, r.err
}

// prevalidateExtract checks whether the first Image XObject on a PDF page
// uses an encoding that the upstream pdfimage.Extract would handle without
// calling log.Fatal. Returns nil on safe inputs, or an error describing the
// unsupported combination.
func prevalidateExtract(pd *pdfread.PdfReaderT, page int) error {
	pg := pd.Pages()[page-1]
	resources := pd.Dic(pd.Att("/Resources", pg))
	xo := pd.Dic(resources["/XObject"])
	if xo == nil {
		return nil
	}
	for _, ref := range xo {
		dic, _ := pd.Stream(ref)
		if string(dic["/Subtype"]) != "/Image" {
			continue
		}
		filter := string(dic["/Filter"])
		switch filter {
		case "/DCTDecode":
			return nil
		case "/CCITTFaxDecode":
			cs := string(dic["/ColorSpace"])
			if cs != "/DeviceGray" {
				return fmt.Errorf("CCITTFaxDecode requires DeviceGray, got %s", cs)
			}
			dparms := pd.Dic(dic["/DecodeParms"])
			k := pd.Num(dparms["/K"])
			if k > 0 {
				return fmt.Errorf("CCITTFaxDecode with K=%d > 0 is unsupported", k)
			}
			return nil
		case "/FlateDecode":
			cs := string(dic["/ColorSpace"])
			bpc := pd.Num(dic["/BitsPerComponent"])
			switch cs {
			case "/DeviceRGB":
				if bpc != 8 {
					return fmt.Errorf("FlateDecode DeviceRGB requires bpc=8, got %d", bpc)
				}
			case "/DeviceGray":
				if bpc != 1 && bpc != 2 && bpc != 4 && bpc != 8 {
					return fmt.Errorf("FlateDecode DeviceGray bpc=%d not in {1,2,4,8}", bpc)
				}
			default:
				return fmt.Errorf("FlateDecode unsupported ColorSpace %s", cs)
			}
			return nil
		default:
			return fmt.Errorf("unsupported image filter %s", filter)
		}
	}
	return nil
}

// pdfBytesSource implements Source for PDF files loaded from a byte slice.
type pdfBytesSource struct {
	data     []byte
	name     string
	sortMode int
}

func (p *pdfBytesSource) Name() string {
	return p.name
}

func (p *pdfBytesSource) Load(ctx context.Context) (<-chan epubimageloader.Task, int, error) {
	pdf := pdfread.LoadBytes(p.data)
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
			img, err = extractSafe(pdf, i+1)

			name := fmt.Sprintf(pageFmt, i+1)
			if err != nil || img == nil {
				img = epubimageloader.CorruptedImage("", name)
				if err == nil {
					err = fmt.Errorf("can't extract image")
				}
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
