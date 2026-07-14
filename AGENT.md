# AGENT.md — go-comic-converter

## Project Overview
**go-comic-converter** (v3) converts CBZ/CBR/Directory/PDF sources into EPUB files optimized for e-readers (Kindle, Kobo, reMarkable). A single CLI binary written in Go 1.26.

Module: `github.com/druzn3k/go-comic-converter/v3`  
Entry: `main.go` → CLI flag parsing → `epub.New(options).Write()`  
Config: `~/.go-comic-converter.yaml` (YAML, saved/loaded via `go-flags`-like custom parser)

## Directory Tree

```
.
├── main.go              # CLI entry: parse flags → dispatch (generate/serve/watch/batch/version/save/show/reset)
├── go.mod               # Module: github.com/druzn3k/go-comic-converter/v3 (Go 1.26)
├── go.sum
├── README.md
├── PLAN.md              # Current development plan
├── LICENSE.txt
├── Makefile
├── Dockerfile
├── .github/workflows/ci.yml
│
├── pkg/                          # Public packages (importable)
│   ├── comic/                    # Library API for conversion
│   │   ├── converter.go          # Converter orchestrator, Convert(), GetParts()
│   │   ├── parts.go              # Part splitting logic
│   │   ├── batch.go              # Batch processing (glob)
│   │   ├── watch.go              # Directory watcher with debounce
│   │   ├── registry.go           # Format registration
│   │   ├── options.go            # Options alias (re-exports epuboptions)
│   │   ├── types.go              # Part type
│   │   ├── source/               # Source readers (CBZ, CBR, dir, PDF)
│   │   ├── output/               # Output writers (CBZ, KEPUB, HTML)
│   │   ├── server/               # HTTP server (handlers, jobs, SSE)
│   │   ├── filters/              # Filter recipe system (Chain, Recipe, builtins)
│   │   └── viewport/             # Viewport dimension helpers
│   ├── epub/
│   │   └── epub.go               # EPUB creation orchestrator
│   └── epuboptions/              # Options data types
│       ├── epub_options.go       # EPUBOptions struct
│       ├── image.go              # Image options struct
│       ├── crop.go               # Crop options struct
│       ├── view.go               # View options struct
│       └── color.go              # Color options struct
│
└── internal/pkg/                 # Internal implementation
    ├── converter/                # CLI flag parser & validation
    │   ├── converter.go          # Converter struct, InitParse, Parse, Validate, Stats
    │   ├── options.go            # Options struct (embeds EPUBOptions), config load/save/reset
    │   ├── profiles.go           # Device profiles (26 device presets)
    │   └── order.go              # order interface for flag ordering
    │
    ├── epubimage/
    │   └── epub_image.go         # EPUBImage struct: paths, keys, styles for each image in EPUB
    │
    ├── epubimagefilters/         # Image processing filters (gift.Filter implementations)
    │   ├── auto_contrast.go      # AutoContrast filter using histogram-based contrast
    │   ├── auto_crop.go          # AutoCrop: margin-detection cropping
    │   ├── cover_title.go        # CoverTitle: overlay title text on cover image
    │   ├── crop_split_double_page.go  # Split double-page spreads
    │   └── pixel.go              # Fallback 1x1 white pixel for empty images
    │
    ├── epubimageprocessor/       # Full image pipeline (load→process→store)
    │   ├── processor.go          # EPUBImageProcessor interface, transformImage, CoverTitleData
    │   └── loader.go             # Source dispatching
    │
    ├── epubimagepassthrough/     # No-processing mode (format=copy)
    │   └── passthrough.go        # Passthrough loader for direct copy
    │
    ├── epubprogress/             # Progress bar / JSON progress output
    │   ├── epub_progress.go      # EPUBProgress interface (progressbar or jsonprogress)
    │   └── json.go               # JSON progress encoder
    │
    ├── epubtemplates/            # EPUB XML templates (embedded Go template strings)
    │   ├── content.go            # Content.opf generation (metadata, manifest, spine, guide)
    │   ├── toc.go                # toc.ncx generation
    │   ├── applebooks.go         # Apple Books compatibility metadata
    │   ├── blank.go              # Empty page templates
    │   ├── container.go          # container.xml
    │   ├── style.go              # CSS styles
    │   └── text.go               # Page XHTML templates
    │
    ├── epubtree/                 # Directory tree structure for TOC
    │   └── epub_tree.go          # EPUBTree + Node (tree of filenames)
    │
    ├── epubzip/                  # EPUB zip writer & image storage
    │   ├── epub_zip.go           # EPUBZip: write files into a ZIP with EPUB magic
    │   ├── image.go              # CompressImage, CompressRaw (JPEG/PNG/WebP + deflate)
    │   ├── storage_image_reader.go   # StorageImageReader: read pre-processed images from temp ZIP
    │   └── storage_image_writer.go   # StorageImageWriter: write processed images to temp ZIP
    │
    ├── sortpath/                 # Natural/alphanumeric filename sorting
    │   ├── by.go                 # sort.Interface wrapper
    │   └── parser.go             # Number-aware filename part parser
    │
    └── utils/
        ├── utils.go              # Printf/Fatalf/Println helpers + NumberOfDigits/Format
        └── utils_test.go         # Tests for utils
```

## API Surface

### `pkg/epub` — EPUB creation
```go
func New(options epuboptions.EPUBOptions) EPUB  // returns EPUB interface
// EPUB interface:
//   Write() error — generates the EPUB file(s)
```

### `pkg/epuboptions` — Data types (all structs, no methods except WorkersRatio/ImgStorage)
- `EPUBOptions` — top-level options (Input, Output, Author, Title, TitlePage, LimitMb, StripFirstDirectoryFromToc, SortPathMode, Image, Dry, DryVerbose, Quiet, Json, Workers)
- `Image` — image processing options (Crop, Quality, Brightness, Contrast, AutoContrast, AutoRotate, AutoSplitDoublePage, KeepDoublePageIfSplit, KeepSplitDoublePageAspect, NoBlankImage, Manga, HasCover, View, GrayScale, GrayScaleMode, Resize, Format, AppleBookCompatibility)
- `Crop` — {Enabled, Left, Up, Right, Bottom, Limit, SkipIfLimitReached}
- `View` — {Width, Height, AspectRatio, PortraitOnly, Color}
- `Color` — {Foreground, Background}

### `internal/pkg/converter` — CLI parser
```go
func New() *Converter
// Converter:
//   Options *Options
//   Cmd     *flag.FlagSet
//   LoadConfig() error
//   InitParse()
//   Parse()
//   Validate() error
//   Fatal(error)
//   Stats()
// Options:
//   embeds epuboptions.EPUBOptions
//   Profile string
//   Show/Save/Reset bool  — config management
//   Auto/NoFilter/MaxQuality/BestQuality/GreatQuality/GoodQuality bool — shortcuts
```

### `internal/pkg/epubimage` — Image metadata in EPUB
```go
type EPUBImage struct { Id, Part int; Format string; IsBlank, IsCover bool; Img *image.Image; Name string }
// Methods: SpaceKey/Path, PartKey, PageKey/Path, ImgKey/Path, MediaType, ImgStyle, RelSize
```

### `internal/pkg/epubimageprocessor` — Image pipeline
```go
type EPUBImageProcessor interface {
    Load() ([]epubimage.EPUBImage, error)
    CoverTitleData(CoverTitleDataOptions) (epubzip.Image, error)
}
type CoverTitleDataOptions struct { Title, Align string; FontSize,PctWidth,PctMargin,Border int }
func New(o epuboptions.EPUBOptions) EPUBImageProcessor
```

### `internal/pkg/epubimagepassthrough` — Copy-mode image pipeline
```go
func New(o epuboptions.EPUBOptions) EPUBImageProcessor
```

### `internal/pkg/epubprogress` — Progress UI
```go
type Options struct { Quiet, Json bool; Max, Description string; CurrentJob, TotalJob int }
type EPUBProgress interface { Add(int) error; Close() error }
func New(o Options) EPUBProgress
```

### `internal/pkg/epubtemplates` — EPUB XML generation
```go
type Content struct { Title, HasTitlePage, UID, Author, Publisher, UpdatedAt string; ImageOptions epuboptions.Image; Cover, Images []epubimage.EPUBImage; Current, Total int }
func (o Content) String() string
func (o Content) getMeta() []tag
func (o Content) getManifest() []tag
func (o Content) getSpineAuto() []tag
func (o Content) getSpinePortrait() []tag
func (o Content) getGuide() []tag
```

### `internal/pkg/epubtree` — Directory tree for TOC
```go
func New() *EPUBTree
func (n *EPUBTree) Root() *Node
func (n *EPUBTree) Add(filename string)
type Node struct { ... }
func (n *Node) ChildCount() int
func (n *Node) FirstChild() *Node
func (n *Node) WriteString(indent string) string
```

### `internal/pkg/epubzip` — ZIP writer
```go
type EPUBZip struct { ... }
func New(path string) (EPUBZip, error)
func (e EPUBZip) Close() error
func (e EPUBZip) WriteMagic() error        // EPUB magic mimetype
func (e EPUBZip) Copy(fz *zip.File) error  // raw copy
func (e EPUBZip) WriteRaw(raw Image) error  // write pre-compressed
func (e EPUBZip) WriteContent(file string, content []byte) error
type Image struct { Header *zip.FileHeader; Data []byte }
func CompressImage(filename, format string, img image.Image, quality int) (Image, error)
func CompressRaw(filename string, uncompressedData []byte) (Image, error)
type StorageImageWriter struct { ... }
func NewStorageImageWriter(filename, format string) (StorageImageWriter, error)
func (e StorageImageWriter) Add(filename string, img image.Image, quality int) error
func (e StorageImageWriter) AddRaw(filename string, uncompressedData []byte) error
type StorageImageReader struct { ... }
func NewStorageImageReader(filename string) (StorageImageReader, error)
func (e StorageImageReader) Get(filename string) *zip.File
func (e StorageImageReader) Size(filename string) uint64
```

### `internal/pkg/sortpath` — Natural sorting
```go
func By(filenames []string, mode int) sort.Interface
// mode: 0=alpha path+file, 1=alphanumeric path+alpha file, 2=alphanumeric path+file
```

### `internal/pkg/utils` — I/O helpers
```go
func Printf(format string, a ...interface{})
func Fatalf(format string, args ...interface{})
func Println(a ...interface{})
func Fatalln(a ...interface{})
func IntToString(i int) string
func FloatToString(f float64, precision int) string
func BoolToString(b bool) string
func NumberOfDigits(i int) int
func FormatNumberOfDigits(i int) string
```

## Dependencies

### Direct
| Package | Usage |
|---|---|
| `github.com/beevik/etree` v1.5.0 | XML DOM for EPUB content.opf generation |
| `github.com/deepteams/webp` v1.2.7 | Pure-Go WebP encoding |
| `github.com/disintegration/gift` v1.2.1 | Image filtering pipeline (crop, resize, color) |
| `github.com/fogleman/gg` v1.3.0 | 2D rendering for title images |
| `github.com/fsnotify/fsnotify` v1.7.0 | File system notifications (watch mode) |
| `github.com/gofrs/uuid/v5` v5.0.0 | EPUB unique identifier |
| `github.com/golang/freetype` | TrueType font rendering for title text |
| `github.com/nwaples/rardecode/v2` v2.1.0 | RAR/CBR archive reader |
| `github.com/raff/pdfreader` v0.0.0 | PDF page extraction |
| `github.com/schollz/progressbar/v3` v3.18.0 | Terminal progress bar |
| `golang.org/x/image` v0.24.0 | TIFF/WebP image decoders, Go fonts |
| `gopkg.in/yaml.v3` v3.0.1 | Config file YAML parsing |


### Indirect
`google/go-github`, `google/go-querystring`, `hashicorp/go-version`, `mitchellh/colorstring`, `rivo/uniseg`, `golang.org/x/net`, `golang.org/x/sys`, `golang.org/x/term`

## CLI Entry Point (`main.go`)

The CLI is structured as a command dispatch pattern:

1. `converter.New()` creates Converter with `flag.FlagSet`
2. `cmd.LoadConfig()` loads `~/.go-comic-converter.yaml`
5. Dispatch by mode flag:
   - `-version` → `version()` (prints build info + latest GitHub release)
   - `-serve` → `serve()` (starts HTTP server on given address)
   - `-watch` → `watch()` (monitors directory for new files)
   - `-batch` → `batch()` (glob-pattern bulk conversion)
   - `-save` → `save(cmd)` (writes current options to config file)
   - `-show` → `show(cmd)` (prints options)
   - `-reset` → `reset(cmd)` (resets config to defaults)
   - `-recipe-show` → loads/prints recipe chain and exits
   - `-recipe-save` → serializes current options as recipe YAML
   - default → `generate(cmd)`:
     - `cmd.Validate()` checks inputs
     - Applies profile dimensions if set
     - Loads recipe if `-recipe` specified
     - Calls `comic.NewWithRecipe(opts, chain).Convert()` for non-EPUB formats
     - Or `epub.New(opts).Write()` for EPUB output
     - Prints stats on completion

## Image Processing Pipeline

The core pipeline (in `internal/pkg/epubimageprocessor`):

1. **Load** source: directory, CBZ/ZIP, CBR/RAR, or PDF via `source.Source` dispatcher
2. **Sort** files using `sortpath.By` with configurable mode (0=alpha, 1=alphanumeric path, 2=alphanumeric both)
3. **For each image**, run `transformImage`:
   - Decode image (JPEG, PNG, WebP, TIFF)
   - If corrupted → show warning, use 1x1 white pixel
   - **If recipe chain is set**: apply user-defined filter pipeline from `-recipe`
   - **If no recipe**: apply hardcoded default chain:
     - Auto-rotate (if width > height and enabled)
     - Auto-split double page (if width >> height)
     - Crop (margin detection with configurable ratios/limits)
     - Auto-contrast (histogram-based)
     - Brightness/contrast adjustment
     - Grayscale conversion (3 modes: normal/average/luminance)
     - Resize to device dimensions (if enabled)
   - Convert to output format (JPEG/PNG/WebP)
   - Compress and store in temp ZIP via `StorageImageWriter`
4. **Parallel**: uses `runtime.NumCPU()` workers with channel-based fan-out

Passthrough mode (`format=copy`) skips all processing, copies raw JPG/PNG data directly.

## EPUB Generation (`pkg/epub/epub.go`)

1. Groups images into parts based on size limit (`-limitmb`)
2. For each part:
   - Creates temp ZIP storage if needed
   - Writes `mimetype` (uncompressed, `application/epub+zip`)
   - Writes `container.xml`
   - Writes `style.css`
   - Writes blank/spacer pages
   - Writes cover image (with optional title overlay)
   - Writes title page (if enabled)
   - Writes page XHTML files for each image
   - Writes image files
   - Generates `content.opf` with metadata, manifest, spine, guide
   - Generates `toc.ncx`
   - Optionally writes Apple Books compatibility XML
   - Closes EPUB zip
3. Cleans up temp storage

## Profiles (Device Presets)

26 profiles defined in `internal/pkg/converter/profiles.go`:
- **Tablet**: HR (2400×3840), SR (1200×1920)
- **Kindle**: K1, K11, K2, K34, K578, KDX, KPW, KV, KPW5, KO, KS
- **Kobo**: KoMT, KoG, KoGHD, KoA, KoAHD, KoAH2O, KoAO, KoN, KoC, KoL, KoF, KoS, KoE
- **reMarkable**: RM1, RM2

## Config File

Location: `~/.go-comic-converter.yaml`  
Format: YAML, mirrors `epuboptions.EPUBOptions` + `Profile` field  
Managed by: `-show` (print), `-save` (write current as defaults), `-reset` (restore defaults)

## Test Coverage

**Overall: 55.7%** (19 packages with tests, 3 without).

Tests exist for all core packages. Key coverage:
- `pkg/epuboptions`: 100%
- `internal/pkg/epubimage`: 100%
- `internal/pkg/epubtree`: 100%
- `pkg/comic/viewport`: 100%
- `internal/pkg/epubimagefilters`: 86.2%
- `internal/pkg/epubimageloader`: 80.6%
- `internal/pkg/converter`: 71.0%
- `internal/pkg/epubimageprocessor`: 66.7%
- `pkg/comic/server`: 64.9%
- `pkg/epub`: 60.5%
- `pkg/comic/filters`: 60.4%
- `internal/pkg/epubzip`: 50.0%

Packages without tests (low-priority): `epubprogress` (terminal I/O), `epubtemplates` (covered by output), `main.go` (thin dispatch).

### Adding tests
- Use hand-written test stubs, no mock frameworks
- Use `image.NewRGBA()` for test images
- Use `t.TempDir()` for temp files
- Run `go test -race ./<package>/...` before committing

## Build

```
go build -o go-comic-converter .
go test ./...
go vet ./...
go test -race ./...
```

### Docker
```
docker build -t go-comic-converter .
docker run --rm go-comic-converter --help
```

### CI
GitHub Actions workflow in `.github/workflows/ci.yml` — runs build + test + vet on push.

## Key Design Patterns

1. **Options composition**: `Options` embeds `epuboptions.EPUBOptions`, which embeds `Image`, which embeds `Crop`/`View`/`Color`. YAML tags for config persistence, JSON tags for `-json` output.
2. **Interface-based progress**: `EPUBProgress` interface abstracts terminal bar vs. JSON output.
3. **Interface-based image processor**: `EPUBImageProcessor` interface with two implementations: full processing vs. passthrough copy.
4. **Temp ZIP storage**: Processed images go to a temporary ZIP (`Output + ".tmp"`), then are copied into the final EPUB. This enables parallel processing without holding all images in memory.
5. **Custom flag ordering**: `Converter.order []order` tracks flag registration order and groups them into sections for `-help` display, since `flag.FlagSet` doesn't preserve insert order.
6. **Stateless templates**: EPUB XML content is generated via `String()` methods on template structs, using `strings.Builder` and `etree` for XML construction.

## Common Operations

### Adding a new device profile
Add a `Profile{Code, Description, Width, Height}` entry to `NewProfiles()` in `internal/pkg/converter/profiles.go`.

### Adding a new CLI flag
1. Add field to the appropriate options struct (with `yaml` and `json` tags if persistable)
2. Set default in `NewOptions()`
3. Register in `InitParse()` via `AddStringParam`/`AddIntParam`/`AddFloatParam`/`AddBoolParam`
4. Apply the value in `Validate()` if needed
5. Document in `README.md`

### Adding a new image filter
1. Create a `gift.Filter` implementation in `internal/pkg/epubimagefilters/`
2. Optionally add an enable field to `epuboptions.Image`
3. Apply the filter in `transformImage` in `internal/pkg/epubimageprocessor/processor.go`

### Adding a new input format
1. Add a loader method in `internal/pkg/epubimageprocessor/loader.go` (or `passthrough.go`)
2. Wire it into the `load()` method
3. Add dependency to `go.mod`
