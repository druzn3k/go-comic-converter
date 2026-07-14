# go-comic-converter

**Go version:** 1.26  
**Module:** `github.com/druzn3k/go-comic-converter/v3`  
**Test coverage:** 55.7%

Convert CBZ/CBR/Dir/PDF into EPUB, KEPUB, CBZ, or HTML for e-reader devices (Kindle, Kobo, reMarkable, ...)

My goal is to make a simple, cross-platform, and fast tool to convert comics into EPUB, KEPUB, CBZ, or HTML.

EPUB is supported by Amazon through [SendToKindle](https://www.amazon.com/gp/sendtokindle/), by Email or by using the App. I've made it simple to support the size limit constraint of those services. KEPUB output adds Kobo panel zoom support, CBZ output targets comic server apps, and HTML produces a self-contained browser viewer.


## WASM Browser App

A browser-based version of go-comic-converter runs entirely in your browser
via WebAssembly. No server needed — all processing is local.

```
make wasm        # Build wasm/main.wasm
make wasm-serve  # Open http://localhost:8080
```

Open `wasm/index.html` in a browser, drop a CBZ file, adjust options,
and download the converted EPUB. Supports all output formats (EPUB, KEPUB,
CBZ, HTML), all filter options, recipe system, and ComicInfo.xml metadata.

See [PLAN.md](PLAN.md) for architecture details and future plans.

## Features
- Support input from zip, cbz, rar, cbr, pdf, directory
- Support all Kindle devices and Kobo
- Support multiple output formats: EPUB, KEPUB (Kobo enhanced), CBZ, HTML
- Automatic KEPUB output for Kobo device profiles
- **Output image formats**: JPEG, PNG, **WebP**
- **Filter recipe system**: YAML-defined processing pipelines via `-recipe`
- **HTTP server mode**: REST API with job queue, SSE progress, multipart upload
- **Watch mode**: auto-convert files added to a directory
- **Batch mode**: glob-pattern bulk conversion
- **ComicInfo.xml**: metadata embedded in CBZ output for Komga/Kavita
- Support Landscape and Portrait mode
- Customize output image quality
- Intelligent cropping (support removing even page numbers)
- Customize brightness and contrast
- Auto contrast
- Auto rotate (if reader mainly read on portrait)
- Auto split double page (for easy read on portrait)
- Keep double page if split
- Keep split double page aspect ratio (best for landscape rendering)
- Remove blank image (empty image is removed)
- Manga or Normal mode
- Support cover page or not (first page will be taken in that case)
- Support title page (cover with embedded title and part)
- Split EPUB size for easy upload
- 3 sorting methods (depending on your source, you can ensure the page go in the right order)
- Save and reuse your own perfect settings
- Multi tasks for fast conversion
- Apple Book Compatibility Mode
- Strict mode (abort on first corrupted image)
- Graceful cancellation (Ctrl+C)
- JSON output for programmatic usage

When you read the comic on a Kindle, you can customize how you read it with the `Aa` button:
- Landscape / Portrait
- Activate panel view for small device

# Output Formats

The tool supports four output formats, selected via the `-output-format` flag:

### `epub` (default)
Standard EPUB format, compatible with Kindle (via SendToKindle), Kobo, reMarkable, and most e-readers.
When using a Kobo device profile, KEPUB is auto-selected unless explicitly overridden.

### `kepub`
Kobo Enhanced EPUB with support for Kobo's panel zoom feature. Images are wrapped in Kobo-specific
markup for page-flip and panel navigation. Output filename uses the `.kepub.epub` extension.
Auto-selected when a Kobo profile is used (e.g., `KoC`, `KoL`, `KoG`).

### `cbz`
Re-packaged processed images as a CBZ (ZIP of sorted images) archive. Images go through the
full processing pipeline (crop, auto-contrast, resize, etc.) before being stored. Ideal for
comic reader apps like Komga, Kavita, or Panelity.

### `html`
Self-contained HTML viewer with all images embedded as base64 data URIs and a vanilla JS
page-flip navigation. No server required — open the HTML file directly in a browser.
Perfect for quick previews or sharing.
The tool supports four output formats, selected via the `-output-format` flag.
Output image format (inside EPUB/KEPUB/CBZ) can be JPEG, PNG, or **WebP** via `-format`.
To select an output format:
```
$ go-comic-converter -profile SR -input ~/Download/MyComic -output-format html
```

# Installation

## From source

First ensure to have a working version of GO: [Installation](https://go.dev/doc/install)

Then install the last version of the tool:
```
$ go install github.com/druzn3k/go-comic-converter/v3
```

To force install a specific version:
```
# specific version
$ go install github.com/druzn3k/go-comic-converter/v3@v3.0.0

# main branch
$ go install github.com/druzn3k/go-comic-converter/v3@main

# specific commit
$ go install github.com/druzn3k/go-comic-converter/v3@COMMIT_HASH
```

## Docker

A multi-stage Docker image is provided for containerized usage:

```
$ docker build -t go-comic-converter .
$ docker run --rm go-comic-converter --help
```

Mount your comics and convert:
```
$ docker run --rm -v ~/Download:/data:ro go-comic-converter \
    -profile SR -input /data/MyComic.cbz
```

To start the HTTP server mode:
```
$ docker run --rm -p 8080:8080 -v ~/Download:/data go-comic-converter \
    -serve :8080
```

## From source (traditional Go install)

Add GOPATH to your PATH:
```
$ export PATH=$(go env GOPATH)/bin:$PATH
```

# Upgrade from V2

The configuration file structure changes in the v3 compare to v2.

You need to recreate your config and save it again.

Use the `show`, `reset` and `save` option.

# Check last version

You can check if a new version is available with:
```
$ go-comic-converter -version
go-comic-converter
  Path             : github.com/druzn3k/go-comic-converter/v3
  Sum              : h1:tUFF2m/fGlOJOwC0/PlTopMfcBMprKvgr6TiQHQxEeo=
  Version          : v3.0.0
  Available Version: v3.0.0

To install the latest version:
$ go install github.com/druzn3k/go-comic-converter/v3@v3.0.0
```

# Supported image files

The supported image files are jpeg and png from the sources.

The extensions can be: `jpg`, `jpeg`, `png`, `webp`, `tiff`.

The case for extensions doesn't matter.

For the passthrough mode (format=copy), the supported extensions are: `jpg`, `jpeg`, `png`

# Usage

## Convert directory

Convert every supported image files found in the input directory:

```
$ go-comic-converter -profile SR -input ~/Download/MyComic
```

By default, it will output: ~/Download/MyComic.epub

## Convert CBZ, ZIP, CBR, RAR, PDF

Convert every supported image files found in the input directory:

```
$ go-comic-converter -profile SR -input ~/Download/MyComic.[CBZ,ZIP,CBR,RAR,PDF]
```

By default, it will output: ~/Download/MyComic.epub

## Convert with size limit

If you send your ePub through Amazon service, you have some size limitation:
  - Email  : 50Mb (including encoding, so 40Mb for RAW file)
  - App    : 50Mb
  - Website: 200Mb

You can split your file using the "-limitmb MB" option:

```
go-comic-converter -profile SR -input ~/Download/MyComic.[CBZ,ZIP,CBR,RAR,PDF] -limitmb 200
```

If you have more than 1 file the output will be:
  - ~/Download/MyComic Part 01 of 03.epub
  - ~/Download/MyComic Part 02 of 03.epub
  - ...

The ePub include as a first page:
  - Title
  - Part NUM / TOTAL

If the total is above 1, then the title of the EPUB include:
  - Title [part/total]

## Convert to different output formats

You can convert to EPUB, KEPUB (Kobo enhanced), CBZ, or HTML using the
`-output-format` flag. The output file extension changes automatically.

### KEPUB (Kobo)

KEPUB enables panel zoom on Kobo devices. When using a Kobo profile, the
tool automatically selects KEPUB output:

```
$ go-comic-converter -profile KoC -input ~/Download/MyComic.cbz
# Output: ~/Download/MyComic.kepub.epub
```

Override with explicit format:

```
$ go-comic-converter -profile KoC -input ~/Download/MyComic.cbz -output-format epub
# Output: ~/Download/MyComic.epub
```

### CBZ

Process and repackage images as a CBZ archive for comic reader apps:

```
$ go-comic-converter -profile SR -input ~/Download/MyComic -output-format cbz
# Output: ~/Download/MyComic.cbz
```

### HTML

Generate a self-contained HTML viewer with base64-embedded images and
page-flip navigation. Open directly in any browser:

```
$ go-comic-converter -profile SR -input ~/Download/MyComic.cbr -output-format html
# Output: ~/Download/MyComic.html
```

### All formats

Produce all output formats in a single conversion run. The image pipeline
runs once and each format writer produces its output:

```
$ go-comic-converter -profile SR -input ~/Download/MyComic -output-format all
# Output: ~/Download/MyComic.epub, ~/Download/MyComic.cbz, ~/Download/MyComic.html
# (KEPUB also included when a Kobo profile is used)
```

## Strict mode

By default, the tool replaces corrupted images with a 1x1 white placeholder
and continues. Use `-strict` to abort on the first corrupted image:

```
$ go-comic-converter -profile SR -input ~/Download/MyComic -strict
```

## Dry run

If you want to preview what will be set during the conversion without running the conversion, then you can use the `-dry` option.

```
$ go-comic-converter -input ~/Downloads/mymanga.cbr -profile SR -auto -manga -limitmb 200 -dry
Go Comic Converter

Options:
    Input                           : ~/Downloads/mymanga.cbr
    Output                          : ~/Downloads/mymanga.epub
    Author                          : GO Comic Converter
    Title                           : mymanga
    Workers                         : 20
    Profile                         : SR - Standard Resolution - 1200x1920
    Format                          : jpeg
    Quality                         : 85
    Grayscale                       : true
    Grayscale mode                  : normal
    Crop                            : true
    Crop ratio                      : 1 Left - 1 Up - 1 Right - 3 Bottom - Limit 0% - Skip false
    Auto contrast                   : true
    Auto rotate                     : true
    Auto split double page          : true
    Keep double page if split       : true
    Keep split double page aspect   : true
    No blank image                  : true
    Manga                           : true
    Has cover                       : true
    Limit                           : 200 Mb
    Strip first directory from toc  : false
    Sort path mode                  : path=alphanumeric, file=alpha
    Foreground color                : #000
    Background color                : #FFF
    Resize                          : true
    Aspect ratio                    : auto
    Portrait only                   : false
    Title page                      : always
    Apple book compatibility        : false

TOC:
  - mymanga
  - Chapter 1
  - Chapter 2
  - Chapter 3
```

## Dry verbose

You can choose different way to sort path and files, depending on your source. You can preview the sorted result with the option `dry-verbose` associated with `dry`.

The option `sort` allow you to change the sorting order.

```
$ go-comic-converter -input ~/Downloads/mymanga.cbr -profile SR -auto -manga -limitmb 200 -dry -dry-verbose -sort 2
Go Comic Converter

Options:
    Input                           : ~/Downloads/mymanga.cbr
    Output                          : ~/Downloads/mymanga.epub
    Author                          : GO Comic Converter
    Title                           : mymanga
    Workers                         : 20
    Profile                         : SR - Standard Resolution - 1200x1920
    Format                          : jpeg
    Quality                         : 85
    Grayscale                       : true
    Grayscale mode                  : normal
    Crop                            : true
    Crop ratio                      : 1 Left - 1 Up - 1 Right - 3 Bottom - Limit 0% - Skip false
    Auto contrast                   : true
    Auto rotate                     : true
    Auto split double page          : true
    Keep double page if split       : true
    Keep split double page aspect   : true
    No blank image                  : true
    Manga                           : true
    Has cover                       : true
    Limit                           : 200 Mb
    Strip first directory from toc  : false
    Sort path mode                  : path=alphanumeric, file=alpha
    Foreground color                : #000
    Background color                : #FFF
    Resize                          : true
    Aspect ratio                    : auto
    Portrait only                   : false
    Title page                      : always
    Apple book compatibility        : false

TOC:
  - mymanga
  - Chapter 1
  - Chapter 2
  - Chapter 3

Cover:
  - Chapter 1
    - img1.jpg

Files:
  - Chapter 1
    - img2.jpg
    - img10.jpg
  - Chapter 2
    - img01.jpg
    - img02.jpg
    - img03.jpg
  - Chapter 3
    - img1.jpg
    - img2-3.jpg
    - img4.jpg
```

## Change default settings

### Show current default option
```
$ go-comic-converter -show

Go Comic Converter

Options:
    Profile                         : SR - Standard Resolution - 1200x1920
    Format                          : jpeg
    Quality                         : 85
    Grayscale                       : true
    Grayscale mode                  : normal
    Crop                            : true
    Crop ratio                      : 1 Left - 1 Up - 1 Right - 3 Bottom - Limit 0% - Skip false
    Auto contrast                   : false
    Auto rotate                     : false
    Auto split double page          : false
    No blank image                  : true
    Manga                           : false
    Has cover                       : true
    Strip first directory from toc  : false
    Sort path mode                  : path=alphanumeric, file=alpha
    Foreground color                : #000
    Background color                : #FFF
    Resize                          : true
    Aspect ratio                    : auto
    Portrait only                   : false
    Title page                      : always
    Apple book compatibility        : false
```

### Change default settings
```
$ go-comic-converter -manga -auto -profile SR -limitmb 200 -save

Go Comic Converter

Options:
    Profile                         : SR - Standard Resolution - 1200x1920
    Format                          : jpeg
    Quality                         : 85
    Grayscale                       : true
    Grayscale mode                  : normal
    Crop                            : true
    Crop ratio                      : 1 Left - 1 Up - 1 Right - 3 Bottom - Limit 0% - Skip false
    Auto contrast                   : true
    Auto rotate                     : true
    Auto split double page          : true
    Keep double page if split       : true
    Keep split double page aspect   : true
    No blank image                  : true
    Manga                           : true
    Has cover                       : true
    Limit                           : 200 Mb
    Strip first directory from toc  : false
    Sort path mode                  : path=alphanumeric, file=alpha
    Foreground color                : #000
    Background color                : #FFF
    Resize                          : true
    Aspect ratio                    : auto
    Portrait only                   : false
    Title page                      : always
    Apple book compatibility        : false

Saving to ~/.go-comic-converter.yaml
```

If you want to change a setting, you can change only one of them
```
$ go-comic-converter -manga=0 -save

Options:
    Profile                         : SR - Standard Resolution - 1200x1920
    Format                          : jpeg
    Quality                         : 85
    Grayscale                       : true
    Grayscale mode                  : normal
    Crop                            : true
    Crop ratio                      : 1 Left - 1 Up - 1 Right - 3 Bottom - Limit 0% - Skip false
    Auto contrast                   : false
    Auto rotate                     : false
    Auto split double page          : false
    No blank image                  : true
    Manga                           : false
    Has cover                       : true
    Strip first directory from toc  : false
    Sort path mode                  : path=alphanumeric, file=alpha
    Foreground color                : #000
    Background color                : #FFF
    Resize                          : true
    Aspect ratio                    : auto
    Portrait only                   : false
    Title page                      : always
    Apple book compatibility        : false

Saving to ~/.go-comic-converter.yaml
```

### Reset default
To reset all value to default:

```
$ go-comic-converter -reset
Go Comic Converter

Options:
    Profile                         : SR - Standard Resolution - 1200x1920
    Format                          : jpeg
    Quality                         : 85
    Grayscale                       : true
    Grayscale mode                  : normal
    Crop                            : true
    Crop ratio                      : 1 Left - 1 Up - 1 Right - 3 Bottom - Limit 0% - Skip false
    Auto contrast                   : false
    Auto rotate                     : false
    Auto split double page          : false
    No blank image                  : true
    Manga                           : false
    Has cover                       : true
    Strip first directory from toc  : false
    Sort path mode                  : path=alphanumeric, file=alpha
    Foreground color                : #000
    Background color                : #FFF
    Resize                          : true
    Aspect ratio                    : auto
    Portrait only                   : false
    Title page                      : always
    Apple book compatibility        : false

Reset default to ~/.go-comic-converter.yaml
```

# My own settings

After playing around with the options, I have my perfect settings for all my devices.

```
$ go-comic-converter -reset
$ go-comic-converter -profile SR -quality 90 -manga -aspect-ratio 1.6 -limitmb 200 -save

Options:
    Profile                         : SR - Standard Resolution - 1200x1920
    Format                          : jpeg
    Quality                         : 90
    Grayscale                       : true
    Grayscale mode                  : normal
    Crop                            : true
    Crop ratio                      : 1 Left - 1 Up - 1 Right - 3 Bottom - Limit 0% - Skip false
    Auto contrast                   : false
    Auto rotate                     : false
    Auto split double page          : false
    No blank image                  : true
    Manga                           : true
    Has cover                       : true
    Limit                           : 200 Mb
    Strip first directory from toc  : false
    Sort path mode                  : path=alphanumeric, file=alpha
    Foreground color                : #000
    Background color                : #FFF
    Resize                          : true
    Aspect ratio                    : 1:1.60
    Portrait only                   : false
    Title page                      : always
    Apple book compatibility        : false

Saving to ~/.go-comic-converter.yaml
```

Explanation:
- `-profile SR`: standard resolution (fast conversion from Amazon as images do not need to be resized)
- `-quality 90`: JPEG output quality of images
- `-manga`: manga mode, read right to left
- `-limitmb 200`: size limit to 200MB allowing upload from SendToKindle website
- `-aspect-ratio`: ensure aspect ratio is 1:1.6, best for kindle devices.
# Help

```
$ go-comic-converter -h

Usage of go-comic-converter:

Output:
  -input string
    	Source of comic to convert: directory, cbz, zip, cbr, rar, pdf
  -output string
    	Output of the EPUB (directory or EPUB): (default [INPUT].epub)
  -author string (default "GO Comic Converter")
    	Author of the EPUB
  -title string
    	Title of the EPUB

Config:
  -profile string (default "SR")
    	Profile to use: 
    	    - KoAO    - 1404 x 1872 - Kobo Aura ONE (kepub)
    	    - KoF     - 1440 x 1920 - Kobo Forma (kepub)
    	    - KoE     - 1404 x 1872 - Kobo Elipsa (kepub)
    	    - KV      - 1072 x 1448 - Kindle Paperwhite 3/4/Voyage/Oasis
    	    - KoG     -  768 x 1024 - Kobo Glo (kepub)
    	    - KoA     -  758 x 1024 - Kobo Aura (kepub)
    	    - RM1     - 1404 x 1872 - reMarkable 1
    	    - RM2     - 1404 x 1872 - reMarkable 2
    	    - K1      -  600 x 670  - Kindle 1
    	    - K11     - 1072 x 1448 - Kindle 11
    	    - K2      -  600 x 670  - Kindle 2
    	    - K34     -  600 x 800  - Kindle Keyboard/Touch
    	    - KPW5    - 1236 x 1648 - Kindle Paperwhite 5/Signature Edition
    	    - KoAH2O  - 1080 x 1430 - Kobo Aura H2O (kepub)
    	    - KoN     -  758 x 1024 - Kobo Nia (kepub)
    	    - KoL     - 1264 x 1680 - Kobo Libra H2O/Kobo Libra 2 (kepub)
    	    - HR      - 2400 x 3840 - High Resolution
    	    - KO      - 1264 x 1680 - Kindle Oasis 2/3
    	    - KS      - 1860 x 2480 - Kindle Scribe
    	    - KoMT    -  600 x 800  - Kobo Mini/Touch (kepub)
    	    - KoAHD   - 1080 x 1440 - Kobo Aura HD (kepub)
    	    - KoC     - 1072 x 1448 - Kobo Clara HD/Kobo Clara 2E (kepub)
    	    - KoS     - 1440 x 1920 - Kobo Sage (kepub)
    	    - SR      - 1200 x 1920 - Standard Resolution
    	    - K578    -  600 x 800  - Kindle
    	    - KDX     -  824 x 1000 - Kindle DX/DXG
    	    - KPW     -  758 x 1024 - Kindle Paperwhite 1/2
    	    - KoGHD   - 1072 x 1448 - Kobo Glo HD (kepub)
  -quality int (default 85)
    	Quality of the image
  -grayscale (default true)
    	Grayscale image. Ideal for eInk devices.
  -grayscale-mode int
    	Grayscale Mode
    	0 = normal
    	1 = average
    	2 = luminance
  -crop (default true)
    	Crop images
  -crop-ratio-left int (default 1)
    	Crop ratio left: ratio of pixels allow to be non blank while cutting on the left.
  -crop-ratio-up int (default 1)
    	Crop ratio up: ratio of pixels allow to be non blank while cutting on the top.
  -crop-ratio-right int (default 1)
    	Crop ratio right: ratio of pixels allow to be non blank while cutting on the right.
  -crop-ratio-bottom int (default 3)
    	Crop ratio bottom: ratio of pixels allow to be non blank while cutting on the bottom.
  -crop-limit int
    	Crop limit: maximum number of cropping in percentage allowed. 0 mean unlimited.
  -crop-skip-if-limit-reached
    	Crop skip if limit reached.
  -brightness int
    	Brightness readjustment: between -100 and 100, > 0 lighter, < 0 darker
  -contrast int
    	Contrast readjustment: between -100 and 100, > 0 more contrast, < 0 less contrast
  -autocontrast
    	Improve contrast automatically
  -autorotate
    	Auto Rotate page when width > height
  -autosplitdoublepage
    	Auto Split double page when width > height
  -keepdoublepageifsplit (default true)
    	Keep the double page if split
  -keepsplitdoublepageaspect (default true)
    	Keep aspect of split part of a double page (best for landscape rendering)
  -noblankimage (default true)
    	Remove blank image
  -manga
    	Manga mode (right to left)
  -hascover (default true)
    	Has cover. Indicate if your comic have a cover. The first page will be used as a cover and include after the title.
  -limitmb int
    	Limit size of the EPUB: Default nolimit (0), Minimum 20
  -strip
    	Strip first directory from the TOC if only 1
  -sort int (default 1)
    	Sort path mode
    	0 = alpha for path and file
    	1 = alphanumeric for path and alpha for file
    	2 = alphanumeric for path and file
  -foreground-color string (default "000")
    	Foreground color in hexadecimal format RGB. Black=000, White=FFF
  -background-color string (default "FFF")
    	Background color in hexadecimal format RGB. Black=000, White=FFF, Light Gray=DDD, Dark Gray=777
  -resize (default true)
    	Reduce image size if exceed device size
  -format string (default "jpeg")
    	Format of output images: jpeg (lossy), png (lossless), webp (lossy), copy (no processing)
    	Aspect ratio (height/width) of the output
    	 -1 = same as device
    	  0 = same as source
    	1.6 = amazon advice for kindle
  -portrait-only
    	Portrait only: force orientation to portrait only.
  -titlepage int (default 1)
    	Title page
    	0 = never
    	1 = always
    	2 = only if epub is split

Default config:
  -show
    	Show your default parameters
  -save
    	Save your parameters as default
  -reset
    	Reset your parameters to default

Shortcut:
  -auto
    	Activate all automatic options
  -nofilter
    	Deactivate all filters
  -maxquality
    	Max quality: color png + noresize
  -bestquality
    	Max quality: color jpg q100 + noresize
  -greatquality
    	Max quality: grayscale jpg q90 + noresize
  -goodquality
    	Max quality: grayscale jpg q90

Compatibility:
  -applebookcompatibility
    	Apple book compatibility

Other:
  -workers int (default number of CPUs)
    	Number of workers
  -output-format string (default "epub")
    	Output format: epub, cbz, kepub, html, or all (default epub)
    	Kobo profiles automatically select "kepub" unless overridden.
    	"all" produces every format from a single conversion run.
  -strict
    	Abort on first corrupted image instead of continuing with a placeholder
  -dry
    	Dry run to show all options
  -dry-verbose
    	Display also sorted files after the TOC
  -quiet
    	Disable progress bar
  -json
    	Output progression and information in Json format
  -version
    	Show current and available version
  -help
    	Show this help message

# Credit

This project is largely inspired from KCC (Kindle Comic Converter). Thanks:
 - [ciromattia](https://github.com/ciromattia/kcc)
 - [darodi fork](https://github.com/darodi/kcc)

# UI

Thanks for UI contribution:
 - [manueldidonna / Comic2Books](https://github.com/manueldidonna/comic2books)

## Next Features

See [PLAN.md](PLAN.md) for the latest development plan.

Previous milestones (all completed):
- WebP output format
- HTTP server mode with REST API + SSE progress
- Watch mode with debouncing and temp-file filtering
- Filter recipe system (YAML-defined processing pipelines)
- ComicInfo.xml metadata for CBZ output
- Test coverage raised from 26.5% to 55.7%
- Go toolchain updated to 1.26
