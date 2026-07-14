package output

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"text/template"
	"time"

	"github.com/gofrs/uuid/v5"

	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimagepassthrough"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubprogress"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubtemplates"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubtree"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/druzn3k/go-comic-converter/v3/internal/pkg/utils"
	"github.com/druzn3k/go-comic-converter/v3/pkg/comic/viewport"
	"github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func init() {
	Register(func() OutputWriter { return KEPUBWriter{} })
}

// KEPUBWriter produces KEPUB files (Kobo-enhanced EPUB) from processed comic images.
type KEPUBWriter struct{}

func (w KEPUBWriter) Format() string          { return "kepub" }
func (w KEPUBWriter) Extension() string       { return ".kepub.epub" }
func (w KEPUBWriter) SupportsPartSplit() bool { return true }

// kepubTextTemplate is the XHTML page template for KEPUB.
// Key difference from EPUB: images are wrapped in <div class="kobolink">
// to enable Kobo's panel zoom feature.
var kepubTextTemplate = "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
	"<!DOCTYPE html>\n" +
	"<html xmlns=\"http://www.w3.org/1999/xhtml\" xmlns:epub=\"http://www.idpf.org/2007/ops\">\n" +
	"  <head>\n" +
	"    <meta charset=\"utf-8\" />\n" +
	"    <title>{{ .Title | xmlEscape }}</title>\n" +
	"    <link href=\"style.css\" type=\"text/css\" rel=\"stylesheet\"/>\n" +
	"    <meta name=\"viewport\" content=\"{{ .ViewPort }}\"/>\n" +
	"  </head>\n" +
	"  <body>\n" +
	"    <div class=\"kobolink\"><img src=\"{{ .ImagePath }}\" alt=\"{{ .Title | xmlEscape }}\" style=\"{{ .ImageStyle }}\"/></div>\n" +
	"  </body>\n" +
	"</html>"

var kepubNewlineRegex = regexp.MustCompile("\n+")

// xmlEscape escapes special XML characters.
func xmlEscape(s string) string {
	var escaped strings.Builder
	escaped.Grow(len(s))
	for _, r := range s {
		switch r {
		case '&':
			escaped.WriteString("&amp;")
		case '<':
			escaped.WriteString("&lt;")
		case '>':
			escaped.WriteString("&gt;")
		case '"':
			escaped.WriteString("&quot;")
		case '\'':
			escaped.WriteString("&apos;")
		default:
			escaped.WriteRune(r)
		}
	}
	return escaped.String()
}

func (w KEPUBWriter) Write(ctx context.Context, parts []OutputPart, opts epuboptions.EPUBOptions) ([]string, error) {
	uid := uuid.Must(uuid.NewV4()).String()
	publisher := "GO Comic Converter"
	updatedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	funcMap := template.FuncMap{
		"mod":       func(i, j int) bool { return i%j == 0 },
		"zoom":      func(s int, z float32) int { return int(float32(s) * z) },
		"xmlEscape": xmlEscape,
	}

	templates := make(map[string]*template.Template, 3)
	for name, src := range map[string]string{
		"text":  kepubTextTemplate,
		"blank": epubtemplates.Blank,
		"style": epubtemplates.Style,
	} {
		t := template.Must(template.New(name).Funcs(funcMap).Parse(src))
		templates[name] = t
	}

	render := func(name string, data map[string]any) string {
		tmpl := templates[name]
		var result strings.Builder
		if err := tmpl.Execute(&result, data); err != nil {
			panic(err)
		}
		return kepubNewlineRegex.ReplaceAllString(result.String(), "\n")
	}

	var imageProcessor epubimageprocessor.EPUBImageProcessor
	if opts.Image.Format == "copy" {
		imageProcessor = epubimagepassthrough.New(opts)
	} else {
		imageProcessor = epubimageprocessor.New(opts)
	}

	kepubParts, imgStorage, err := w.processImages(ctx, imageProcessor, opts)
	if err != nil {
		return nil, err
	}

	if opts.Dry {
		if len(kepubParts) > 0 {
			tree := w.getTree(kepubParts[0].Images, true)
			utils.Printf("TOC:\n  - %s\n%s\n", opts.Title, tree)
			if opts.DryVerbose {
				if opts.Image.HasCover {
					utils.Printf("Cover:\n%s\n", w.getTree([]epubimage.EPUBImage{kepubParts[0].Cover}, false))
				}
				utils.Printf("Files:\n%s\n", w.getTree(kepubParts[0].Images, false))
			}
		}
		return nil, nil
	}

	defer func() {
		_ = imgStorage.Close()
		_ = imgStorage.Remove()
	}()

	totalParts := len(kepubParts)

	bar := epubprogress.New(epubprogress.Options{
		Max:         totalParts,
		Description: "Writing Part",
		CurrentJob:  2,
		TotalJob:    2,
		Quiet:       opts.Quiet,
		Json:        opts.Json,
	})

	opts.Image.View.Width, opts.Image.View.Height = w.computeViewPort(kepubParts, opts)

	outputPaths := make([]string, 0, totalParts)
	for i, part := range kepubParts {
		ext := filepath.Ext(opts.Output)
		suffix := ""
		if totalParts > 1 {
			fmtLen := utils.FormatNumberOfDigits(totalParts)
			fmtPart := "Part " + fmtLen + " of " + fmtLen
			suffix = fmt.Sprintf(fmtPart, i+1, totalParts)
		}
		path := opts.Output[:len(opts.Output)-len(ext)] + suffix + ext

		if err := w.writePart(
			path,
			i+1,
			totalParts,
			part,
			imgStorage,
			opts,
			uid,
			publisher,
			updatedAt,
			render,
		); err != nil {
			return outputPaths, err
		}

		outputPaths = append(outputPaths, path)
		_ = bar.Add(1)
	}
	_ = bar.Close()

	hasError := false
	for pId, part := range kepubParts {
		if pId == 0 && opts.Image.HasCover && part.Cover.Error != nil {
			hasError = true
			utils.Printf("Error on image %s: %v\n", filepath.Join(part.Cover.Path, part.Cover.Name), part.Cover.Error)
		}
		for _, img := range part.Images {
			if img.Part == 0 && img.Error != nil {
				hasError = true
				utils.Printf("Error on image %s: %v\n", filepath.Join(img.Path, img.Name), img.Error)
			}
		}
	}

	if !opts.Json {
		utils.Println()
	}

	if hasError {
		return outputPaths, errors.New("one or more images are corrupted")
	}
	return outputPaths, nil
}

// processImages loads and processes images, then splits them into parts.
func (w KEPUBWriter) processImages(
	ctx context.Context,
	imageProcessor epubimageprocessor.EPUBImageProcessor,
	opts epuboptions.EPUBOptions,
) (parts []kepubPart, imgStorage epubzip.StorageImageReader, err error) {
	images, err := imageProcessor.Load(ctx)
	if err != nil {
		return
	}

	sort.Slice(images, func(i, j int) bool {
		if images[i].Id == images[j].Id {
			return images[i].Part < images[j].Part
		}
		return images[i].Id < images[j].Id
	})

	if opts.Strict {
		for _, img := range images {
			if img.Error != nil {
				err = fmt.Errorf("strict mode: %s: %w", filepath.Join(img.Path, img.Name), img.Error)
				return
			}
		}
	}

	parts = make([]kepubPart, 0)
	cover := images[0]
	if opts.Image.HasCover || (cover.DoublePage && !opts.Image.KeepDoublePageIfSplit) {
		images = images[1:]
	}

	if opts.Dry {
		parts = append(parts, kepubPart{Cover: cover, Images: images})
		return
	}

	imgStorage, err = epubzip.NewStorageImageReader(opts.ImgStorage())
	if err != nil {
		return
	}

	maxSize := uint64(opts.LimitMb * 1024 * 1024)
	xhtmlSize := uint64(1024)
	baseSize := uint64(128*1024) + imgStorage.Size(cover.EPUBImgPath())*2

	currentSize := baseSize
	currentImages := make([]epubimage.EPUBImage, 0)

	for _, img := range images {
		imgSize := imgStorage.Size(img.EPUBImgPath()) + xhtmlSize
		if maxSize > 0 && len(currentImages) > 0 && currentSize+imgSize > maxSize {
			parts = append(parts, kepubPart{Cover: cover, Images: currentImages})
			currentSize = baseSize
			currentImages = make([]epubimage.EPUBImage, 0)
		}
		currentSize += imgSize
		currentImages = append(currentImages, img)
	}
	if len(currentImages) > 0 {
		parts = append(parts, kepubPart{Cover: cover, Images: currentImages})
	}

	return
}

type kepubPart struct {
	Cover  epubimage.EPUBImage
	Images []epubimage.EPUBImage
}

// computeViewPort calculates optimal viewport dimensions.
func (w KEPUBWriter) computeViewPort(parts []kepubPart, opts epuboptions.EPUBOptions) (int, int) {
	aspects := make([]viewport.PartAspect, len(parts))
	for i, p := range parts {
		images := make([]viewport.ImageAspect, len(p.Images))
		for j, img := range p.Images {
			images[j] = viewport.ImageAspect{OriginalAspectRatio: img.OriginalAspectRatio}
		}
		aspects[i] = viewport.PartAspect{
			Cover:  viewport.ImageAspect{OriginalAspectRatio: p.Cover.OriginalAspectRatio},
			Images: images,
		}
	}
	return viewport.ComputeViewPort(aspects, opts.Image.View)
}

func (w KEPUBWriter) writePart(
	path string,
	currentPart, totalParts int,
	part kepubPart,
	imgStorage epubzip.StorageImageReader,
	opts epuboptions.EPUBOptions,
	uid, publisher, updatedAt string,
	render func(string, map[string]any) string,
) error {
	hasTitlePage := opts.TitlePage == 1 || (opts.TitlePage == 2 && totalParts > 1)

	wz, err := epubzip.New(path)
	if err != nil {
		return err
	}
	defer func(wz epubzip.EPUBZip) {
		_ = wz.Close()
	}(wz)

	title := opts.Title
	if totalParts > 1 {
		title = title + " [" + utils.IntToString(currentPart) + "/" + utils.IntToString(totalParts) + "]"
	}

	type zipContent struct {
		Name    string
		Content string
	}
	content := []zipContent{
		{"META-INF/container.xml", epubtemplates.Container},
		{"OEBPS/content.opf", w.generateContentOPF(kepubContentData{
			Title:        title,
			HasTitlePage: hasTitlePage,
			UID:          uid,
			Author:       opts.Author,
			Publisher:    publisher,
			UpdatedAt:    updatedAt,
			ImageOptions: opts.Image,
			Cover:        part.Cover,
			Images:       part.Images,
			Current:      currentPart,
			Total:        totalParts,
		})},
		{"OEBPS/toc.xhtml", epubtemplates.Toc(title, hasTitlePage, opts.StripFirstDirectoryFromToc, part.Images)},
		{"OEBPS/Text/style.css", render("style", map[string]any{
			"View": opts.Image.View,
		})},
	}

	if err = wz.WriteMagic(); err != nil {
		return err
	}
	for _, c := range content {
		if err := wz.WriteContent(c.Name, []byte(c.Content)); err != nil {
			return err
		}
	}

	if err = w.writeCoverImage(wz, part.Cover, currentPart, totalParts, opts, render); err != nil {
		return err
	}

	if hasTitlePage {
		if err = w.writeTitleImage(wz, part.Cover, title, opts, render); err != nil {
			return err
		}
	}

	lastImage := part.Images[len(part.Images)-1]
	for _, img := range part.Images {
		if err := w.writePageImage(wz, img, imgStorage.Get(img.EPUBImgPath()), opts, render); err != nil {
			return err
		}

		if !opts.Image.View.PortraitOnly &&
			(img.DoublePage ||
				(!opts.Image.KeepDoublePageIfSplit && img.Part == 1) ||
				(img.Part == 0 && img == lastImage)) {
			if err := w.writeBlank(wz, img, opts, render); err != nil {
				return err
			}
		}
	}

	return nil
}

func (w KEPUBWriter) writeCoverImage(
	wz epubzip.EPUBZip,
	img epubimage.EPUBImage,
	part, totalParts int,
	opts epuboptions.EPUBOptions,
	render func(string, map[string]any) string,
) error {
	title := "Cover"
	text := ""
	if totalParts > 1 {
		text = utils.IntToString(part) + " / " + utils.IntToString(totalParts)
		title = title + " " + text
	}

	if err := wz.WriteContent(
		"OEBPS/Text/cover.xhtml",
		[]byte(render("text", map[string]any{
			"Title":      title,
			"ViewPort":   opts.Image.View.Port(),
			"ImagePath":  "Images/cover.jpeg",
			"ImageStyle": img.ImgStyle(opts.Image.View.Width, opts.Image.View.Height, ""),
		})),
	); err != nil {
		return err
	}

	var imageProcessor epubimageprocessor.EPUBImageProcessor
	if opts.Image.Format == "copy" {
		imageProcessor = epubimagepassthrough.New(opts)
	} else {
		imageProcessor = epubimageprocessor.New(opts)
	}

	coverTitle, err := imageProcessor.CoverTitleData(epubimageprocessor.CoverTitleDataOptions{
		Src:         img.Raw,
		Name:        "cover",
		Text:        text,
		Align:       "bottom",
		PctWidth:    50,
		PctMargin:   50,
		MaxFontSize: 96,
		BorderSize:  8,
	})
	if err != nil {
		return err
	}

	return wz.WriteRaw(coverTitle)
}

func (w KEPUBWriter) writeTitleImage(
	wz epubzip.EPUBZip,
	img epubimage.EPUBImage,
	title string,
	opts epuboptions.EPUBOptions,
	render func(string, map[string]any) string,
) error {
	titleAlign := ""
	if !opts.Image.View.PortraitOnly {
		if opts.Image.Manga {
			titleAlign = "right:0"
		} else {
			titleAlign = "left:0"
		}
	}

	if !opts.Image.View.PortraitOnly {
		if err := wz.WriteContent(
			"OEBPS/Text/space_title.xhtml",
			[]byte(render("blank", map[string]any{
				"Title":    "Blank Page Title",
				"ViewPort": opts.Image.View.Port(),
			})),
		); err != nil {
			return err
		}
	}

	if err := wz.WriteContent(
		"OEBPS/Text/title.xhtml",
		[]byte(render("text", map[string]any{
			"Title":      title,
			"ViewPort":   opts.Image.View.Port(),
			"ImagePath":  "Images/title.jpeg",
			"ImageStyle": img.ImgStyle(opts.Image.View.Width, opts.Image.View.Height, titleAlign),
		})),
	); err != nil {
		return err
	}

	var imageProcessor epubimageprocessor.EPUBImageProcessor
	if opts.Image.Format == "copy" {
		imageProcessor = epubimagepassthrough.New(opts)
	} else {
		imageProcessor = epubimageprocessor.New(opts)
	}

	coverTitle, err := imageProcessor.CoverTitleData(epubimageprocessor.CoverTitleDataOptions{
		Src:         img.Raw,
		Name:        "title",
		Text:        title,
		Align:       "center",
		PctWidth:    100,
		PctMargin:   100,
		MaxFontSize: 64,
		BorderSize:  4,
	})
	if err != nil {
		return err
	}

	return wz.WriteRaw(coverTitle)
}

func (w KEPUBWriter) writePageImage(
	wz epubzip.EPUBZip,
	img epubimage.EPUBImage,
	zipImg *zip.File,
	opts epuboptions.EPUBOptions,
	render func(string, map[string]any) string,
) error {
	err := wz.WriteContent(
		img.EPUBPagePath(),
		[]byte(render("text", map[string]any{
			"Title":      "Image " + utils.IntToString(img.Id) + " Part " + utils.IntToString(img.Part),
			"ViewPort":   opts.Image.View.Port(),
			"ImagePath":  img.ImgPath(),
			"ImageStyle": img.ImgStyle(opts.Image.View.Width, opts.Image.View.Height, ""),
		})),
	)
	if err == nil {
		err = wz.Copy(zipImg)
	}
	return err
}

func (w KEPUBWriter) writeBlank(
	wz epubzip.EPUBZip,
	img epubimage.EPUBImage,
	opts epuboptions.EPUBOptions,
	render func(string, map[string]any) string,
) error {
	return wz.WriteContent(
		img.EPUBSpacePath(),
		[]byte(render("blank", map[string]any{
			"Title":    "Blank Page " + utils.IntToString(img.Id),
			"ViewPort": opts.Image.View.Port(),
		})),
	)
}

// generateContentOPF generates the content.opf XML for KEPUB.
// Key difference from EPUB: includes <meta name="kobo-style" content="kobostyle"/>.
func (w KEPUBWriter) generateContentOPF(data kepubContentData) string {
	stdContent := epubtemplates.Content{
		Title:        data.Title,
		HasTitlePage: data.HasTitlePage,
		UID:          data.UID,
		Author:       data.Author,
		Publisher:    data.Publisher,
		UpdatedAt:    data.UpdatedAt,
		ImageOptions: data.ImageOptions,
		Cover:        data.Cover,
		Images:       data.Images,
		Current:      data.Current,
		Total:        data.Total,
	}
	opf := stdContent.String()

	// Inject kobo-style meta after the opening <metadata> element
	koboMeta := "  <meta name=\"kobo-style\" content=\"kobostyle\"/>\n"
	idx := strings.Index(opf, "<metadata")
	if idx >= 0 {
		end := strings.Index(opf[idx:], ">")
		if end >= 0 {
			insertAt := idx + end + 1
			opf = opf[:insertAt] + "\n" + koboMeta + opf[insertAt:]
		}
	}

	return opf
}

type kepubContentData struct {
	Title        string
	HasTitlePage bool
	UID          string
	Author       string
	Publisher    string
	UpdatedAt    string
	ImageOptions epuboptions.Image
	Cover        epubimage.EPUBImage
	Images       []epubimage.EPUBImage
	Current      int
	Total        int
}

// getTree creates a directory tree representation from images.
func (w KEPUBWriter) getTree(images []epubimage.EPUBImage, skipFiles bool) string {
	t := epubtree.New()
	for _, img := range images {
		if skipFiles {
			t.Add(img.Path)
		} else {
			t.Add(filepath.Join(img.Path, img.Name))
		}
	}
	c := t.Root()
	return c.WriteString("")
}
