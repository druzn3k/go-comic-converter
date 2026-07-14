// Package epub Tools to create EPUB from images.
package epub

import (
	"archive/zip"
	"errors"
	"fmt"
	"context"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/gofrs/uuid/v5"

	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimage"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimagepassthrough"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubimageprocessor"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubprogress"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubtemplates"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubtree"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/epubzip"
	"github.com/celogeek/go-comic-converter/v3/internal/pkg/utils"
	"github.com/celogeek/go-comic-converter/v3/pkg/comic"
	"github.com/celogeek/go-comic-converter/v3/pkg/epuboptions"
)

// ErrImageCorrupted is returned by Write() when some images had errors
// and could not be fully processed. The EPUB is still written with placeholders.
var ErrImageCorrupted = errors.New("one or more images are corrupted")

type EPUB interface {
	Write(ctx context.Context) error
}

type epub struct {
	epuboptions.EPUBOptions
	UID       string
	Publisher string
	UpdatedAt string

	templates     map[string]*template.Template
	imageProcessor epubimageprocessor.EPUBImageProcessor
}

type epubPart = comic.Part

// xmlEscape escapes special XML characters in user-provided strings.
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

// newlineRegex is used by render() to collapse multiple newlines.
var newlineRegex = regexp.MustCompile("\n+")

// New initialize EPUB
func New(options epuboptions.EPUBOptions) EPUB {
	uid := uuid.Must(uuid.NewV4())

	funcMap := template.FuncMap{
		"mod":       func(i, j int) bool { return i%j == 0 },
		"zoom":      func(s int, z float32) int { return int(float32(s) * z) },
		"xmlEscape": xmlEscape,
	}

	// Pre-parse all templates once
	templates := make(map[string]*template.Template, 3)
	for name, src := range map[string]string{
		"text":  epubtemplates.Text,
		"blank": epubtemplates.Blank,
		"style": epubtemplates.Style,
	} {
		t := template.Must(template.New(name).Funcs(funcMap).Parse(src))
		templates[name] = t
	}

	var imageProcessor epubimageprocessor.EPUBImageProcessor
	if options.Image.Format == "copy" {
		imageProcessor = epubimagepassthrough.New(options)
	} else {
		imageProcessor = epubimageprocessor.New(options)
	}

	return epub{
		EPUBOptions:    options,
		UID:            uid.String(),
		Publisher:      "GO Comic Converter",
		UpdatedAt:      time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		templates:      templates,
		imageProcessor: imageProcessor,
	}
}
// render templates by name from the pre-parsed cache
func (e epub) render(name string, data map[string]any) string {
	tmpl, ok := e.templates[name]
	if !ok {
		panic("unknown template: " + name)
	}
	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		panic(err)
	}
	return newlineRegex.ReplaceAllString(result.String(), "\n")
}

// write image to the zip
func (e epub) writeImage(wz epubzip.EPUBZip, img epubimage.EPUBImage, zipImg *zip.File) error {
	err := wz.WriteContent(
		img.EPUBPagePath(),
		[]byte(e.render("text", map[string]any{
			"Title":      "Image " + utils.IntToString(img.Id) + " Part " + utils.IntToString(img.Part),
			"ViewPort":   e.Image.View.Port(),
			"ImagePath":  img.ImgPath(),
			"ImageStyle": img.ImgStyle(e.Image.View.Width, e.Image.View.Height, ""),
		})),
	)
	if err == nil {
		err = wz.Copy(zipImg)
	}

	return err
}

// write blank page
func (e epub) writeBlank(wz epubzip.EPUBZip, img epubimage.EPUBImage) error {
	return wz.WriteContent(
		img.EPUBSpacePath(),
		[]byte(e.render("blank", map[string]any{
			"Title":    "Blank Page " + utils.IntToString(img.Id),
			"ViewPort": e.Image.View.Port(),
		})),
	)
}

// write title image
func (e epub) writeCoverImage(wz epubzip.EPUBZip, img epubimage.EPUBImage, part, totalParts int) error {
	title := "Cover"
	text := ""
	if totalParts > 1 {
		text = utils.IntToString(part) + " / " + utils.IntToString(totalParts)
		title = title + " " + text
	}

	if err := wz.WriteContent(
		"OEBPS/Text/cover.xhtml",
		[]byte(e.render("text", map[string]any{
			"Title":      title,
			"ViewPort":   e.Image.View.Port(),
			"ImagePath":  "Images/cover.jpeg",
			"ImageStyle": img.ImgStyle(e.Image.View.Width, e.Image.View.Height, ""),
		})),
	); err != nil {
		return err
	}

	coverTitle, err := e.imageProcessor.CoverTitleData(epubimageprocessor.CoverTitleDataOptions{
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

	if err := wz.WriteRaw(coverTitle); err != nil {
		return err
	}

	return nil
}

// write title image
func (e epub) writeTitleImage(wz epubzip.EPUBZip, img epubimage.EPUBImage, title string) error {
	titleAlign := ""
	if !e.Image.View.PortraitOnly {
		if e.Image.Manga {
			titleAlign = "right:0"
		} else {
			titleAlign = "left:0"
		}
	}

	if !e.Image.View.PortraitOnly {
		if err := wz.WriteContent(
			"OEBPS/Text/space_title.xhtml",
			[]byte(e.render("blank", map[string]any{
				"Title":    "Blank Page Title",
				"ViewPort": e.Image.View.Port(),
			})),
		); err != nil {
			return err
		}
	}

	if err := wz.WriteContent(
		"OEBPS/Text/title.xhtml",
		[]byte(e.render("text", map[string]any{
			"Title":      title,
			"ViewPort":   e.Image.View.Port(),
			"ImagePath":  "Images/title.jpeg",
			"ImageStyle": img.ImgStyle(e.Image.View.Width, e.Image.View.Height, titleAlign),
		})),
	); err != nil {
		return err
	}

	coverTitle, err := e.imageProcessor.CoverTitleData(epubimageprocessor.CoverTitleDataOptions{
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

	if err := wz.WriteRaw(coverTitle); err != nil {
		return err
	}

	return nil
}

// extract image and split it into part
func (e epub) getParts(ctx context.Context) (parts []epubPart, imgStorage epubzip.StorageImageReader, err error) {
	comicParts, imgStorage, err := comic.GetParts(ctx, e.imageProcessor, e.EPUBOptions)
	if err != nil {
		return nil, imgStorage, err
	}
	parts = make([]epubPart, len(comicParts))
	for i := range comicParts {
		parts[i] = comicParts[i]
	}
	return parts, imgStorage, nil
}

// create a tree from the directories.
//
// this is used to simulate the toc.
func (e epub) getTree(images []epubimage.EPUBImage, skipFiles bool) string {
	t := epubtree.New()
	for _, img := range images {
		if skipFiles {
			t.Add(img.Path)
		} else {
			t.Add(filepath.Join(img.Path, img.Name))
		}
	}
	c := t.Root()
	if skipFiles && e.StripFirstDirectoryFromToc && c.ChildCount() == 1 {
		c = c.FirstChild()
	}

	return c.WriteString("")
}
// toPartAspects converts []epubPart to []comic.PartAspect,
// extracting only the aspect ratio data needed by viewport functions.
func toPartAspects(epubParts []epubPart) []comic.PartAspect {
	aspects := make([]comic.PartAspect, len(epubParts))
	for i, p := range epubParts {
		images := make([]comic.ImageAspect, len(p.Images))
		for j, img := range p.Images {
			images[j] = comic.ImageAspect{OriginalAspectRatio: img.OriginalAspectRatio}
		}
		aspects[i] = comic.PartAspect{
			Cover:  comic.ImageAspect{OriginalAspectRatio: p.Cover.OriginalAspectRatio},
			Images: images,
		}
	}
	return aspects
}


func (e epub) computeAspectRatio(epubParts []epubPart) float64 {
	return comic.ComputeAspectRatio(toPartAspects(epubParts))
}

func (e epub) computeViewPort(epubParts []epubPart) (int, int) {
	return comic.ComputeViewPort(toPartAspects(epubParts), e.Image.View)
}

func (e epub) writePart(path string, currentPart, totalParts int, part epubPart, imgStorage epubzip.StorageImageReader) error {
	hasTitlePage := e.TitlePage == 1 || (e.TitlePage == 2 && totalParts > 1)

	wz, err := epubzip.New(path)
	if err != nil {
		return err
	}
	defer func(wz epubzip.EPUBZip) {
		_ = wz.Close()
	}(wz)

	title := e.Title
	if totalParts > 1 {
		title = title + " [" + utils.IntToString(currentPart) + "/" + utils.IntToString(totalParts) + "]"
	}

	type zipContent struct {
		Name    string
		Content string
	}
	content := []zipContent{
		{"META-INF/container.xml", epubtemplates.Container},
		{"META-INF/com.apple.ibooks.display-options.xml", epubtemplates.AppleBooks},
		{"OEBPS/content.opf", epubtemplates.Content{
			Title:        title,
			HasTitlePage: hasTitlePage,
			UID:          e.UID,
			Author:       e.Author,
			Publisher:    e.Publisher,
			UpdatedAt:    e.UpdatedAt,
			ImageOptions: e.Image,
			Cover:        part.Cover,
			Images:       part.Images,
			Current:      currentPart,
			Total:        totalParts,
		}.String()},
		{"OEBPS/toc.xhtml", epubtemplates.Toc(title, hasTitlePage, e.StripFirstDirectoryFromToc, part.Images)},
		{"OEBPS/Text/style.css", e.render("style", map[string]any{
			"View": e.Image.View,
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

	if err = e.writeCoverImage(wz, part.Cover, currentPart, totalParts); err != nil {
		return err
	}

	if hasTitlePage {
		if err = e.writeTitleImage(wz, part.Cover, title); err != nil {
			return err
		}
	}

	lastImage := part.Images[len(part.Images)-1]
	for _, img := range part.Images {
		if err := e.writeImage(wz, img, imgStorage.Get(img.EPUBImgPath())); err != nil {
			return err
		}

		// Double Page or Last Image that is not a double page
		if !e.Image.View.PortraitOnly &&
			(img.DoublePage ||
				(!e.Image.KeepDoublePageIfSplit && img.Part == 1) ||
				(img.Part == 0 && img == lastImage)) {
			if err := e.writeBlank(wz, img); err != nil {
				return err
			}
		}
	}
	return nil
}

// create the zip
func (e epub) Write(ctx context.Context) error {
	epubParts, imgStorage, err := e.getParts(ctx)
	if err != nil {
		return err
	}

	if e.Dry {
		p := epubParts[0]
		utils.Printf("TOC:\n  - %s\n%s\n", e.Title, e.getTree(p.Images, true))
		if e.DryVerbose {
			if e.Image.HasCover {
				utils.Printf("Cover:\n%s\n", e.getTree([]epubimage.EPUBImage{p.Cover}, false))
			}
			utils.Printf("Files:\n%s\n", e.getTree(p.Images, false))
		}
		return nil
	}
	defer func() {
		_ = imgStorage.Close()
		_ = imgStorage.Remove()
	}()

	totalParts := len(epubParts)

	bar := epubprogress.New(epubprogress.Options{
		Max:         totalParts,
		Description: "Writing Part",
		CurrentJob:  2,
		TotalJob:    2,
		Quiet:       e.Quiet,
		Json:        e.Json,
	})

	e.Image.View.Width, e.Image.View.Height = e.computeViewPort(epubParts)
	for i, part := range epubParts {
		ext := filepath.Ext(e.Output)
		suffix := ""
		if totalParts > 1 {
			fmtLen := utils.FormatNumberOfDigits(totalParts)
			fmtPart := "Part " + fmtLen + " of " + fmtLen
			suffix = fmt.Sprintf(fmtPart, i+1, totalParts)
		}

		path := e.Output[0:len(e.Output)-len(ext)] + suffix + ext

		if err := e.writePart(
			path,
			i+1,
			totalParts,
			part,
			imgStorage,
		); err != nil {
			return err
		}

		_ = bar.Add(1)
	}
	_ = bar.Close()

	// display corrupted images
	hasError := false
	for pId, part := range epubParts {
		if pId == 0 && e.Image.HasCover && part.Cover.Error != nil {
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

	if !e.Json {
		utils.Println()
	}

	if hasError {
		return ErrImageCorrupted
	}
	return nil
}
