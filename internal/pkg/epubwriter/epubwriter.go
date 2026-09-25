// Package epubwriter writes EPUB-family outputs (EPUB and KEPUB) from
// already-split image parts. It is cycle-free: it does not import pkg/comic
// or pkg/comic/output.
package epubwriter

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"html"
	"path/filepath"
	"regexp"
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

// ErrImageCorrupted is returned by Write when some images had errors
// and could not be fully processed. The output is still written with placeholders.
var ErrImageCorrupted = errors.New("one or more images are corrupted")

// Part is a single output part: a cover plus its page images.
type Part struct {
	Cover  epubimage.EPUBImage
	Images []epubimage.EPUBImage
}

// Variant selects the EPUB-family flavor to write.
type Variant struct {
	Extension, TextTemplate string
	KoboStyle, AppleBooks, EscapeTitle bool
}

// Write produces the output file(s) described by opts.Output, using the given
// parts and variant. It opens the temporary image storage itself and removes it
// on return.
func Write(ctx context.Context, parts []Part, variant Variant, opts epuboptions.EPUBOptions) error {
	if len(parts) == 0 {
		return nil
	}

	if opts.Dry {
		return writeDry(parts, opts)
	}

	imgStorage, err := epubzip.NewStorageImageReader(opts.ImgStorage())
	if err != nil {
		return err
	}
	defer func() {
		_ = imgStorage.Close()
		_ = imgStorage.Remove()
	}()

	var imageProcessor epubimageprocessor.EPUBImageProcessor
	if opts.Image.Format == "copy" {
		imageProcessor = epubimagepassthrough.New(opts)
	} else {
		imageProcessor = epubimageprocessor.New(opts)
	}

	uid := uuid.Must(uuid.NewV4()).String()
	publisher := "GO Comic Converter"
	updatedAt := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	totalParts := len(parts)
	bar := epubprogress.New(epubprogress.Options{
		Max:         totalParts,
		Description: "Writing Part",
		CurrentJob:  2,
		TotalJob:    2,
		Quiet:       opts.Quiet,
		Json:        opts.Json,
	})

	opts.Image.View.Width, opts.Image.View.Height = computeViewPort(parts, opts.Image.View)

	render := newRenderer(variant.TextTemplate)

	for i, part := range parts {
		path := outputPath(opts.Output, variant.Extension, i+1, totalParts)
		if err := writePart(path, i+1, totalParts, part, imgStorage, imageProcessor, variant, opts, uid, publisher, updatedAt, render); err != nil {
			_ = bar.Close()
			return err
		}
		_ = bar.Add(1)
	}
	_ = bar.Close()

	hasError := false
	for pId, part := range parts {
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
		return ErrImageCorrupted
	}
	return nil
}

func writeDry(parts []Part, opts epuboptions.EPUBOptions) error {
	p := parts[0]
	utils.Printf("TOC:\n  - %s\n%s\n", opts.Title, getTree(p.Images, true, opts.StripFirstDirectoryFromToc))
	if opts.DryVerbose {
		if opts.Image.HasCover {
			utils.Printf("Cover:\n%s\n", getTree([]epubimage.EPUBImage{p.Cover}, false, opts.StripFirstDirectoryFromToc))
		}
		utils.Printf("Files:\n%s\n", getTree(p.Images, false, opts.StripFirstDirectoryFromToc))
	}
	return nil
}

func outputPath(output, extension string, currentPart, totalParts int) string {
	base := output
	if strings.HasSuffix(base, extension) {
		base = base[:len(base)-len(extension)]
	} else {
		ext := filepath.Ext(base)
		base = base[:len(base)-len(ext)]
	}

	suffix := ""
	if totalParts > 1 {
		// Match the "<base> Part 1 of 2<ext>" naming the cbz and html writers use.
		// The suffix used to be concatenated with no separator, producing
		// "inPart 1 of 2.epub" for the same conversion.
		suffix = fmt.Sprintf(" Part %d of %d", currentPart, totalParts)
	}
	return base + suffix + extension
}

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

var newlineRegex = regexp.MustCompile("\n+")

func newRenderer(textTemplate string) func(string, map[string]any) string {
	funcMap := template.FuncMap{
		"mod":       func(i, j int) bool { return i%j == 0 },
		"zoom":      func(s int, z float32) int { return int(float32(s) * z) },
		"xmlEscape": xmlEscape,
	}

	templates := make(map[string]*template.Template, 3)
	for name, src := range map[string]string{
		"text":  textTemplate,
		"blank": epubtemplates.Blank,
		"style": epubtemplates.Style,
	} {
		t := template.Must(template.New(name).Funcs(funcMap).Parse(src))
		templates[name] = t
	}

	return func(name string, data map[string]any) string {
		tmpl := templates[name]
		var result strings.Builder
		if err := tmpl.Execute(&result, data); err != nil {
			panic(err)
		}
		return newlineRegex.ReplaceAllString(result.String(), "\n")
	}
}

// getTree creates a directory tree representation from images.
//
// this is used to simulate the toc.
func getTree(images []epubimage.EPUBImage, skipFiles bool, stripFirst bool) string {
	t := epubtree.New()
	for _, img := range images {
		if skipFiles {
			t.Add(img.Path)
		} else {
			t.Add(filepath.Join(img.Path, img.Name))
		}
	}
	c := t.Root()
	if skipFiles && stripFirst && c.ChildCount() == 1 {
		c = c.FirstChild()
	}
	return c.WriteString("")
}

func computeViewPort(parts []Part, view epuboptions.View) (int, int) {
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
	return viewport.ComputeViewPort(aspects, view)
}

func writePart(
	path string,
	currentPart, totalParts int,
	part Part,
	imgStorage epubzip.StorageImageReader,
	imageProcessor epubimageprocessor.EPUBImageProcessor,
	variant Variant,
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
	}
	if variant.AppleBooks {
		content = append(content, zipContent{"META-INF/com.apple.ibooks.display-options.xml", epubtemplates.AppleBooks})
	}
	content = append(content,
		zipContent{"OEBPS/content.opf", generateContentOPF(title, hasTitlePage, opts, uid, publisher, updatedAt, part, currentPart, totalParts, variant)},
		zipContent{"OEBPS/toc.xhtml", epubtemplates.Toc(title, hasTitlePage, opts.StripFirstDirectoryFromToc, part.Images)},
		zipContent{"OEBPS/Text/style.css", render("style", map[string]any{
			"View": opts.Image.View,
		})},
	)

	if err = wz.WriteMagic(); err != nil {
		return err
	}
	for _, c := range content {
		if err := wz.WriteContent(c.Name, []byte(c.Content)); err != nil {
			return err
		}
	}

	if err = writeCoverImage(wz, part.Cover, currentPart, totalParts, imageProcessor, opts, render); err != nil {
		return err
	}

	if hasTitlePage {
		if err = writeTitleImage(wz, part.Cover, title, variant.EscapeTitle, imageProcessor, opts, render); err != nil {
			return err
		}
	}

	lastImage := part.Images[len(part.Images)-1]
	for _, img := range part.Images {
		if err := writeImage(wz, img, imgStorage.Get(img.StorageKey()), opts, render); err != nil {
			return err
		}

		if !opts.Image.View.PortraitOnly &&
			(img.DoublePage ||
				(!opts.Image.KeepDoublePageIfSplit && img.Part == 1) ||
				(img.Part == 0 && img == lastImage)) {
			if err := writeBlank(wz, img, opts, render); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeCoverImage(
	wz epubzip.EPUBZip,
	img epubimage.EPUBImage,
	part, totalParts int,
	imageProcessor epubimageprocessor.EPUBImageProcessor,
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

func writeTitleImage(
	wz epubzip.EPUBZip,
	img epubimage.EPUBImage,
	title string,
	escapeTitle bool,
	imageProcessor epubimageprocessor.EPUBImageProcessor,
	opts epuboptions.EPUBOptions,
	render func(string, map[string]any) string,
) error {
	displayTitle := title
	if escapeTitle {
		displayTitle = html.EscapeString(title)
	}

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
			"Title":      displayTitle,
			"ViewPort":   opts.Image.View.Port(),
			"ImagePath":  "Images/title.jpeg",
			"ImageStyle": img.ImgStyle(opts.Image.View.Width, opts.Image.View.Height, titleAlign),
		})),
	); err != nil {
		return err
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

func writeImage(
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
		err = wz.CopyWithName(zipImg, img.EPUBImgPath())
	}
	return err
}

func writeBlank(
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

func generateContentOPF(
	title string,
	hasTitlePage bool,
	opts epuboptions.EPUBOptions,
	uid, publisher, updatedAt string,
	part Part,
	current, total int,
	variant Variant,
) string {
	opf := epubtemplates.Content{
		Title:        title,
		HasTitlePage: hasTitlePage,
		UID:          uid,
		Author:       opts.Author,
		Publisher:    publisher,
		UpdatedAt:    updatedAt,
		ImageOptions: opts.Image,
		Cover:        part.Cover,
		Images:       part.Images,
		Current:      current,
		Total:        total,
	}.String()

	if variant.KoboStyle {
		koboMeta := "  <meta name=\"kobo-style\" content=\"kobostyle\"/>\n"
		idx := strings.Index(opf, "<metadata")
		if idx >= 0 {
			end := strings.Index(opf[idx:], ">")
			if end >= 0 {
				insertAt := idx + end + 1
				opf = opf[:insertAt] + "\n" + koboMeta + opf[insertAt:]
			}
		}
	}

	return opf
}
