# Three Future Directions for go-comic-converter

> Analysis date: 2026-07-13  
> Module: `github.com/celogeek/go-comic-converter/v3` (Go 1.23)  
> Current state: 31 findings resolved, 22.5% coverage, 7 fuzz targets, 26 device profiles, interface-based image pipeline

---

## Codebase Reality (grounding context)

The three paths below are grounded in these architectural facts observed in the code:

1. **Two pluggable interfaces already exist**: `EPUBImageProcessor` (full-process vs passthrough) and `EPUBProgress` (progressbar vs JSON). The design pattern is established.
2. **Source loaders are NOT pluggable**: `loadDir`/`loadCbz`/`loadCbr`/`loadPdf` are methods on `ePUBImageProcessor` (loader.go:54-76), dispatched by file extension in a hardcoded switch. The passthrough package duplicates this dispatch logic entirely (passthrough.go).
3. **Filter chain is hardcoded**: `transformImage` in processor.go:120-200 applies filters in a fixed order (crop → auto-rotate → auto-contrast → brightness/contrast → resize → grayscale). Filters live in `epubimagefilters/` as `gift.Filter` implementations but cannot be reordered or extended without modifying `transformImage`.
4. **EPUB output is monolithic but well-separated**: `pkg/epub/epub.go` (566 lines) owns all EPUB-specific logic. The image pipeline produces format-agnostic processed images stored in a temp ZIP (`StorageImageWriter`/`StorageImageReader`). The output format coupling is isolated in `writePart`, `epubtemplates`, and `epubzip`.
5. **Device profiles are hardcoded Go structs**: 26 profiles in `profiles.go` (77 lines), compiled into the binary. No data-driven profile loading.
6. **One public API entry point**: `pkg/epub.New(options epuboptions.EPUBOptions) EPUB` with `Write(ctx) error`. Options types are public in `pkg/epuboptions/` but the processor, loader, filters, and progress interfaces are all `internal/`.
7. **JSON output mode exists**: `-json` flag produces structured progress output, showing the tool already thinks about programmatic consumption.

---

## Path 1: Embeddable Go Library + Local Service Mode

### Vision

In 6-12 months, go-comic-converter becomes a **library-first Go toolkit** where the CLI is one consumer among many. Developers import `pkg/comic` to embed comic-to-EPUB conversion in their Go applications — a Calibre plugin, a NAS auto-converter, a Docker sidecar that watches a drop folder. An optional built-in HTTP server mode (`-serve :8080`) lets non-Go applications (Home Assistant, a web frontend, a Slack bot) submit conversions via REST API. The binary stays a single static binary; the library is just `go get`-able.

### Concrete Changes

**New package: `pkg/comic` — the stable public API**
```
pkg/comic/
├── comic.go          # Converter type, the new primary public entry point
├── source.go         # Source interface + registry (pluggable input readers)
├── output.go         # Output interface + registry (pluggable output writers)
├── filter.go         # Filter interface + chain builder
├── profile.go        # Profile struct + loader (data-driven profiles)
└── options.go        # Public options, re-exported from epuboptions
```

**Key new APIs:**
```go
// pkg/comic/comic.go
type Converter struct { ... }

func New(opts comic.Options) *Converter

// Register custom source readers (replaces hardcoded loadDir/loadCbz/loadCbr/loadPdf)
func RegisterSource(scheme string, loader SourceLoader)

// Register custom output writers (replaces EPUB-only output)
func RegisterOutput(format string, writer OutputWriter)

// Register custom image filters
func RegisterFilter(name string, factory FilterFactory)

// Convert is the one-call API
func (c *Converter) Convert(ctx context.Context) error

// ProgressCallback lets callers receive progress events without channels
type ProgressCallback func(event ProgressEvent)
```

```go
// pkg/comic/source.go
type SourceLoader interface {
    // Load returns a channel of Tasks (decoded images) for parallel processing
    Load(ctx context.Context, input string) (<-chan Task, int, error)
}

// Built-in registrations in init():
//   "dir"  → directory loader
//   "cbz"  → CBZ/ZIP loader  
//   "cbr"  → CBR/RAR loader
//   "pdf"  → PDF loader
```

**Refactor existing code:**
- Extract `loadDir`/`loadCbz`/`loadCbr`/`loadPdf` from `internal/pkg/epubimageprocessor/loader.go` (494 lines) into standalone `SourceLoader` implementations in `pkg/comic/source/` subpackages. Eliminate the duplication between `epubimageprocessor/loader.go` and `epubimagepassthrough/passthrough.go` (434 lines) — both currently reimplement the same source dispatch.
- Move `EPUBImageProcessor` interface from `internal/` to `pkg/comic/` as the `Filter` chain. The `transformImage` hardcoded pipeline becomes a default filter chain that users can override.
- `pkg/epub` becomes a thin wrapper: `epub.New(options).Write()` delegates to `comic.New(options).Convert()` with the EPUB output writer registered by default.
- `main.go` and `internal/pkg/converter` remain the CLI layer but call into `pkg/comic` instead of `pkg/epub` directly.

**New: HTTP server mode (`cmd/serve/` or inline in main.go)**
```go
// POST /api/convert  (multipart: input file + JSON options)
// GET  /api/profiles
// GET  /api/progress/:jobid  (SSE stream)
// GET  /api/health
```
- Uses `net/http` stdlib only (no framework dependency — matches current zero-web-deps philosophy)
- Job queue with configurable max concurrent conversions
- Reuses existing `EPUBProgress` JSON encoder for SSE events
- Input via multipart upload or local path (with `--allow-local-paths` flag for trusted environments)

**Config: data-driven profiles**
- Move `profiles.go` (77 lines, 26 hardcoded profiles) to `pkg/comic/profiles.yaml` embedded via `go:embed`
- Add `~/.go-comic-converter/profiles/` directory for user-defined profiles
- Profile struct gains optional fields: `DefaultFormat`, `DefaultQuality`, `RecommendedFilters`

### Effort Estimate

**Medium-Large (6-8 person-weeks)**

Breakdown:
- Source interface extraction + dedup: 1.5 weeks (touching loader.go 494ln + passthrough.go 434ln, the most coupled code)
- Filter chain generalization: 1 week (transformImage is 80 lines but the ordering logic is subtle)
- `pkg/comic` public API surface + docs: 1 week
- HTTP server mode: 1.5 weeks (stdlib only, but job queue + SSE + multipart needs care)
- Data-driven profiles: 0.5 weeks
- Migration of `pkg/epub` to delegate: 0.5 weeks
- Tests + examples: 0.5 weeks

### Pros

- **Leverages existing interface pattern**: `EPUBImageProcessor` and `EPUBProgress` already prove the design works. The refactor formalizes what's already half-done — source loaders are the main missing piece.
- **Eliminates loader duplication**: `epubimageprocessor/loader.go` (494ln) and `epubimagepassthrough/passthrough.go` (434ln) share ~80% of their source-dispatch code. The SourceLoader interface eliminates this maintenance burden permanently.
- **Single binary, zero new deps for library use**: Go's static compilation means `go get github.com/celogeek/go-comic-converter/v3/pkg/comic` gives embedders a self-contained toolkit. HTTP server uses only `net/http`.
- **JSON mode already exists**: The `-json` flag and `epubprogress/json.go` show the tool already thinks about programmatic consumption. The HTTP server is a natural extension of this existing capability.
- **Enables ecosystem growth**: NAS integrations (Synology, Unraid), Calibre plugins, and Docker sidecars become possible without forking. The watch-folder pattern is the #1 feature request pattern for automated comic libraries.

### Cons

- **API stability commitment**: Once `pkg/comic` is public, breaking changes require a v4 module path. The current `pkg/epub` API is minimal (one function); the new API surface is 5x larger and needs careful design to avoid future regret.
- **Source extraction is risky**: The four loaders interleave file discovery, sorting (`sortpath.By`), and channel-based fan-out. Extracting them cleanly without breaking the parallel processing semantics requires careful refactoring of the `load()` → `transformImage()` → `imageOutput` channel pipeline.
- **HTTP server adds attack surface**: Multipart upload handling, path traversal protection, and resource limits need careful work. The current tool only reads local files; a server accepts untrusted input. The `MaxImageDim = 20000` decompression-bomb guard helps but isn't sufficient for a network service.
- **Two-package confusion**: Users must understand when to use `pkg/comic` vs `pkg/epub`. Need clear documentation that `pkg/epub` is the EPUB-specific convenience layer.

### Killer Feature

**`go get` + 3 lines of code to convert comics in any Go program.** A developer writes `comic.New(opts).Convert(ctx)` and gets the full pipeline — 26 device profiles, auto-crop, auto-contrast, parallel processing — without shelling out to a CLI. This transforms the tool from a utility into infrastructure.

### Doability Check (2-week stopping point)

**Yes.** After 2 weeks, deliver: SourceLoader interface extracted, loader duplication eliminated, `pkg/comic.New().Convert()` working with EPUB output. The HTTP server and data-driven profiles are additive and can land in a follow-up. The 2-week deliverable already reduces code duplication by ~400 lines and makes the library embeddable for the core use case.

---

## Path 2: Extensible Filter Pipeline with Data-Driven Profiles

### Vision

In 6-12 months, the image processing pipeline becomes a **composable, user-configurable filter chain** where power users define custom processing recipes, share them as YAML profiles, and the community contributes new filters without modifying core code. The hardcoded `transformImage` sequence is replaced by a declarative filter chain. A "recipe" system lets users save and share processing pipelines: "Manga Optimizer for Old Scans", "Watercolor Art Preservation", "Kobo Clara Night Mode". The tool becomes the de facto standard for e-reader image optimization because its filter ecosystem is richer than any GUI tool.

### Concrete Changes

**New package: `pkg/comic/filters` — public filter registry and chain**
```
pkg/comic/filters/
├── filter.go           # Filter interface, Chain type, registration
├── chain.go            # Chain builder, YAML deserialization of filter chains
├── builtin_crop.go     # AutoCrop, ManualCrop (moved from epubimagefilters)
├── builtin_contrast.go # AutoContrast, Brightness, Contrast
├── builtin_orient.go   # AutoRotate, SplitDoublePage
├── builtin_color.go    # Grayscale, Duotone, Threshold
├── builtin_resize.go   # Resize, Sharpen
└── builtin_blank.go    # BlankDetection, PixelFallback
```

**Key new APIs:**
```go
// pkg/comic/filters/filter.go
type Filter interface {
    // Apply processes a single image, returning 1 or more output images
    // (split double-page produces 3)
    Apply(ctx context.Context, img image.Image, opts FilterContext) []image.Output
    // Name returns the filter identifier for YAML/CLI
    Name() string
}

// Chain is an ordered list of filters
type Chain struct {
    filters []Filter
}

// FromYAML builds a chain from a declarative recipe
func FromYAML(yaml string) (*Chain, error)

// Recipe is a serializable filter chain definition
type Recipe struct {
    Name        string         `yaml:"name"`
    Description string         `yaml:"description"`
    Filters     []FilterConfig `yaml:"filters"`
}
type FilterConfig struct {
    Name   string            `yaml:"name"`
    Params map[string]any    `yaml:"params"`
    Condition string         `yaml:"condition"` // e.g. "width > height"
}
```

**Refactor `transformImage` (processor.go:120-200):**
- Current hardcoded sequence becomes the `DefaultChain` — a built-in Recipe that reproduces exact current behavior
- The `ePUBImageProcessor.transformImage` method becomes `chain.Apply(ctx, input)` 
- The double-page split logic (which produces 3 images) becomes a `SplitDoublePage` filter that returns multiple outputs, handled by the chain executor
- Condition support: filters can be conditionally applied (e.g., "only if width > height" for auto-rotate)

**Data-driven device profiles:**
```yaml
# ~/.go-comic-converter/profiles/kobo-clara-night.yaml
code: KoCN
description: Kobo Clara HD - Night Mode
view:
  width: 1072
  height: 1448
  foreground: "FFF"
  background: "000"
recipe:
  name: night-mode
  filters:
    - name: auto_crop
      params: { left: 1, up: 1, right: 1, bottom: 3 }
    - name: auto_contrast
    - name: grayscale
      params: { mode: luminance }
    - name: threshold
      params: { level: 128 }
      condition: "width < height"
    - name: resize
      params: { mode: fit, resampling: lanczos }
```

**New built-in filters (high-value additions):**
- `threshold` — binarization for line art/manga (common request for e-ink)
- `duotone` — map to a 2-color palette (sepia, night-mode green-on-black)
- `sharpen` — unsharp mask for low-quality scans
- `denoise` — light noise reduction for old scans
- `border` — add configurable margins/borders

**Recipe sharing:**
- `-recipe ~/.go-comic-converter/recipes/manga-old-scans.yaml` CLI flag
- `-recipe-show` prints the effective filter chain
- `-recipe-save` saves current filter configuration as a recipe
- Built-in recipes embedded in binary: `manga-standard`, `manga-old-scan`, `color-comic`, `night-mode`, `max-fidelity`

**Batch mode:**
- `-batch ~/Comics/**/*.cbz` processes multiple inputs with the same recipe
- `-watch ~/Dropbox/Comics/` monitors a directory and auto-converts new files (using `fsnotify`)

### Effort Estimate

**Medium (4-5 person-weeks)**

Breakdown:
- Filter interface + Chain executor (handling multi-output filters): 1 week
- Migrate existing filters to new interface: 0.5 weeks (filters already exist as `gift.Filter` impls, mostly wrapping)
- YAML recipe deserialization + condition evaluation: 1 week
- Data-driven profiles (embed YAML + user dir loading): 0.5 weeks
- 3-5 new high-value filters (threshold, duotone, sharpen, denoise): 1 week
- Batch/watch mode: 0.5 weeks
- Tests + recipe examples: 0.5 weeks

### Pros

- **Transforms `transformImage` from maintenance burden to extensibility surface**: The current 80-line hardcoded pipeline in processor.go:120-200 is the single most complex method. Making it a chain eliminates the "add a flag, add an if-block, add a filter call" pattern that currently dominates changes.
- **Filters already exist as discrete units**: `epubimagefilters/` has 5 files with clean `gift.Filter` implementations. The migration to a `Filter` interface is a wrapping exercise, not a rewrite.
- **YAML profiles solve the 26-profile maintenance problem**: `profiles.go` is a compiled Go file that requires a rebuild + release for every new device. Data-driven profiles let users add devices (e.g., the new Kindle Colorsoft) without waiting for a release.
- **Recipe sharing creates community flywheel**: Users sharing YAML recipes on GitHub/Discord creates an ecosystem that GUI tools can't match. "Here's my manga-old-scan recipe" is a shareable artifact.
- **Batch + watch mode serves the #1 automation use case**: Users with large comic libraries currently write shell scripts. Built-in batch/watch with recipe reuse is a significant UX improvement for power users.

### Cons

- **Condition evaluation needs a mini-expression engine**: `"width > height"` conditions require parsing and evaluating expressions safely. A limited DSL (comparisons on image dimensions, format, part number) avoids embedding a full scripting language, but it's still new surface area to test and document.
- **Multi-output filters break the simple chain model**: `SplitDoublePage` produces 3 images from 1 input. The chain executor must handle fan-out (1→N→M), which complicates the parallel processing pipeline. The current code handles this with special-case logic in `Load()` (processor.go:80-115); generalizing it is non-trivial.
- **Recipe compatibility across versions**: If a filter's params change, old recipes break. Need versioning in recipe YAML (`apiVersion: 1`) and a migration story. The current flag-based config doesn't have this problem because changes are compiled in.
- **Watch mode adds `fsnotify` dependency**: Current deps are all image/XML processing. `fsnotify` is a new category (filesystem watching) with platform-specific behavior (inotify on Linux, FSEvents on macOS, ReadDirectoryChangesW on Windows).
- **Potential for user-created bad recipes**: A user chains threshold → denoise → sharpen → threshold and gets garbage. Need sensible defaults and maybe a "recipe validation" pass that warns about likely-bad combinations.

### Killer Feature

**Shareable YAML processing recipes.** A user discovers the perfect filter chain for their manga collection and shares a 20-line YAML file. Another user downloads it, runs `-recipe manga-old-scan.yaml`, and gets identical results. This transforms the tool from a personal utility into a community platform for e-reader image optimization — something no GUI converter offers.

### Doability Check (2-week stopping point)

**Yes.** After 2 weeks, deliver: Filter interface + Chain executor, all existing filters migrated, `DefaultChain` reproducing current behavior, and 2 new filters (threshold + duotone). Data-driven profiles and batch/watch mode are additive. The 2-week deliverable already makes the pipeline extensible and delivers the threshold/duotone filters that users will immediately want.

---

## Path 3: Multi-Output Delivery Engine

### Vision

In 6-12 months, go-comic-converter becomes a **universal e-reader delivery engine** that outputs not just EPUB but KEPUB (Kobo's native enhanced format), CBZ (for comic reader apps like Panelity/Komga/Kavita), and a self-contained offline HTML viewer. The same image pipeline — auto-crop, auto-contrast, split, resize — feeds all output formats. Users convert once and deliver to any device in its native format: KEPUB for Kobo's superior panel zoom, EPUB for Kindle, CBZ for Komga/Kavita servers, HTML for browser-based reading. The tool becomes the single conversion step in a multi-device workflow.

### Concrete Changes

**New package: `pkg/comic/output` — output writer abstraction**
```
pkg/comic/output/
├── output.go           # OutputWriter interface + registry
├── epub.go             # EPUB writer (refactored from pkg/epub/epub.go)
├── kepub.go            # KEPUB writer (Kobo enhanced EPUB)
├── cbz.go              # CBZ writer (re-packaged processed images)
└── html.go             # Self-contained HTML viewer writer
```

**Key new interface:**
```go
// pkg/comic/output/output.go
type OutputWriter interface {
    // Write produces one or more output files from processed image parts
    Write(ctx context.Context, parts []OutputPart, opts Options) ([]string, error)
    // Format returns the output format identifier
    Format() string
    // SupportsPartSplit indicates if the format benefits from size-based splitting
    SupportsPartSplit() bool
}

type OutputPart struct {
    Cover       image.Image
    Images      []image.Image  // processed, ready to embed
    Metadata    PartMetadata
    PartNumber  int
    TotalParts  int
}
type PartMetadata struct {
    Title, Author, Publisher string
    UID, UpdatedAt           string
    ImageOptions             epuboptions.Image
}
```

**Refactor `pkg/epub/epub.go` (566 lines):**
- Extract the part-splitting logic (`getParts`, 30 lines) into a shared `pkg/comic/parts.go` — it's format-agnostic (just size-based grouping of processed images from the temp ZIP)
- Extract `computeAspectRatio` and `computeViewPort` (30 lines) into shared code — they operate on processed image dimensions, not EPUB structure
- The EPUB writer keeps: `writePart`, `writeImage`, `writeBlank`, `writeCoverImage`, `writeTitleImage`, and all `epubtemplates` usage
- The temp ZIP storage lifecycle (`StorageImageWriter` → `StorageImageReader`) becomes shared infrastructure managed by the `Converter`, not by the EPUB writer

**New: KEPUB writer (`output/kepub.go`)**
- KEPUB is EPUB with Kobo-specific enhancements:
  - `div.kobolink` wrappers around each image for panel-zoom
  - `kobo-style` metadata in content.opf
  - `.kepub.epub` extension
- ~80% of the code is shared with the EPUB writer (same OPF/NCX structure)
- Key difference: page XHTML uses `<div class="kobolink"><img .../></div>` instead of bare `<img>`, and the style.css includes kobo-specific spans
- Estimated ~150 lines of new code (templates + writer), heavily reusing `epubtemplates`

**New: CBZ writer (`output/cbz.go`)**
- CBZ is a ZIP of images — the simplest possible output
- The image pipeline already produces processed, compressed images in the temp ZIP
- The CBZ writer just renames/repackages the temp ZIP with proper sort order
- Estimated ~50 lines of new code
- Enables use with Komga, Kavita, Panelity, CDisplayEX, and other comic readers
- `SupportsPartSplit() = false` (CBZ has no size limit concern for local reading)

**New: HTML viewer writer (`output/html.go`)**
- Self-contained single-file HTML with embedded base64 images
- JavaScript page-flip viewer (vanilla JS, ~200 lines, embedded via `go:embed`)
- Supports keyboard navigation (arrow keys), swipe (touch events), and double-page spread
- Ideal for: browser-based reading, sharing a single file, previewing before converting to EPUB
- Part splitting: each part becomes a separate HTML file
- Estimated ~300 lines (writer + embedded JS/CSS/HTML template)

**CLI changes:**
- `-output-format epub|kepub|cbz|html` flag (default: `epub`)
- `-output-format all` produces all four formats from one conversion run (the image pipeline runs once)
- Output extension auto-adjusted: `.kepub.epub`, `.cbz`, `.html`
- README updated with format-specific recommendations

**Profile integration:**
- Device profiles gain a `PreferredFormat` field: Kobo profiles default to `kepub`, Kindle to `epub`, tablet/generic to `cbz`
- `-profile KoC -output-format kepub` becomes the idiomatic Kobo workflow

### Effort Estimate

**Medium (5-6 person-weeks)**

Breakdown:
- OutputWriter interface + part-splitting extraction: 1 week (the extraction from epub.go is the delicate part)
- KEPUB writer: 1 week (mostly template work + testing on Kobo devices)
- CBZ writer: 0.5 weeks (nearly trivial given existing temp ZIP)
- HTML viewer: 1.5 weeks (JS viewer + base64 embedding + template)
- CLI changes + format auto-detection: 0.5 weeks
- Profile `PreferredFormat` field: 0.25 weeks
- Tests (format validation, multi-output run): 0.75 weeks

### Pros

- **The image pipeline is already format-agnostic**: The temp ZIP (`StorageImageWriter`/`StorageImageReader`) stores processed, compressed images. The EPUB writer reads from it. Other writers can read from the same temp ZIP with zero pipeline changes. This is the lowest-risk path because the hard work (image processing) is already decoupled from the output.
- **KEPUB is a natural first target**: It's 80% identical to EPUB — same OPF/NCX structure, same zip structure. The Kobo-specific additions (kobolink divs, kobo-style metadata) are template changes, not architectural changes. Kobo users get panel-zoom, which is the #1 Kobo advantage over Kindle.
- **CBZ output is nearly free**: The temp ZIP already contains sorted, processed, compressed images. A CBZ writer is essentially "rename the temp ZIP." This immediately enables the Komga/Kavita self-hosted comic server ecosystem, which is a large and growing audience.
- **HTML viewer solves the preview problem**: Users currently do a full EPUB conversion + transfer to device just to check if their crop/contrast settings look right. A 5-second HTML output lets them preview in a browser before committing to a device transfer.
- **`-output-format all` runs the pipeline once**: The expensive part (image processing) happens once. Producing 4 output formats from one pipeline run is a unique capability no other tool offers.

### Cons

- **OutputWriter interface design is tricky**: The current EPUB writer (`writePart`) interleaves zip creation, template rendering, and image writing in a single 60-line method. Extracting a clean interface that EPUB, KEPUB, CBZ, and HTML can all implement requires careful separation of concerns. The `writePart` method has 7 sequential steps with error handling between each.
- **KEPUB requires Kobo device testing**: The format has undocumented quirks (kobo span placement, metadata field expectations). Without physical Kobo devices for testing, KEPUB output may have subtle rendering issues. This is the main risk — the code is easy to write but hard to verify.
- **HTML viewer is scope creep risk**: A "simple" JS page-flip viewer can balloon. Touch swipe, keyboard nav, double-page spread, loading indicators, responsive design — each is a rabbit hole. Need strict scope: vanilla JS, no dependencies, basic features only.
- **Output path logic gets complex**: Currently `Validate()` in converter.go computes one output path. With 4 formats, the logic for default extensions, part suffixes, and `-output-format all` (4 files per part) needs careful handling to avoid path collisions.
- **CBZ/HTML lose EPUB features**: TOC, metadata, part-splitting for size limits, title pages — these EPUB-specific features don't exist in CBZ or HTML. Users may be confused when `-strip` or `-titlepage` flags have no effect on CBZ output. Need clear per-format feature documentation.

### Killer Feature

**`-output-format all`: one conversion, four formats.** A user runs `go-comic-converter -input manga.cbz -profile KoC -output-format all` and gets `manga.epub` (Kindle), `manga.kepub.epub` (Kobo with panel zoom), `manga.cbz` (Komga server), and `manga.html` (browser preview) — all from a single image processing pass. No other tool delivers multi-format e-reader output from one conversion. The image pipeline — the expensive, complex part — runs exactly once.

### Doability Check (2-week stopping point)

**Yes, strongly.** After 2 weeks, deliver: OutputWriter interface, EPUB writer refactored to implement it, CBZ writer (nearly free given temp ZIP), and HTML viewer. KEPUB can follow in week 3. The 2-week deliverable already gives users 3 output formats (EPUB, CBZ, HTML) from the refactored pipeline, with CBZ being the immediate high-value win for the Komga/Kavita community.

---

## Comparison Matrix

| Dimension | Path 1: Library + Service | Path 2: Filter Pipeline | Path 3: Multi-Output |
|---|---|---|---|
| **Target audience** | Developers / integrators | Power users / community | Multi-device readers |
| **Architectural pivot** | CLI → Library-first | Hardcoded → Declarative | EPUB-only → Multi-format |
| **Effort** | Medium-Large (6-8 wk) | Medium (4-5 wk) | Medium (5-6 wk) |
| **Lines of code touched** | ~1,500 (refactor) + ~800 (new) | ~400 (refactor) + ~600 (new) | ~300 (refactor) + ~700 (new) |
| **Risk level** | Medium (API stability) | Medium (DSL + multi-output filters) | Low-Medium (pipeline already decoupled) |
| **2-week stopping point value** | High (library usable) | High (extensible + 2 new filters) | Very High (3 new formats) |
| **Community flywheel** | Ecosystem integrations | Recipe sharing | Format-specific communities |
| **Existing code leverage** | Interface pattern exists | Filters already discrete | Pipeline already format-agnostic |
| **New dependencies** | None (stdlib http) | fsnotify (for watch mode) | None |
| **Backward compatibility** | `pkg/epub` API preserved | `DefaultChain` = current behavior | EPUB output unchanged |

## Recommended Sequencing

If pursuing multiple paths, the natural order is:

1. **Path 3 first** (lowest risk, highest 2-week value, pipeline already decoupled)
2. **Path 1 second** (SourceLoader extraction from Path 3's refactor reduces work; the OutputWriter interface from Path 3 becomes part of `pkg/comic`)
3. **Path 2 third** (Filter chain builds on the stable `pkg/comic` API from Path 1; recipes can reference output formats from Path 3)

Paths 1 and 3 share significant refactoring overlap (both extract interfaces from `pkg/epub/epub.go` and the loader code). Path 2 is more independent but benefits from the stable API surface that Path 1 establishes.

## Key Insight

All three paths converge on the same architectural truth: **the image processing pipeline is already well-factored, but its interfaces are trapped inside `internal/`**. The `EPUBImageProcessor` interface, the `EPUBProgress` interface, the temp ZIP storage, and the discrete filters in `epubimagefilters/` are all designed for pluggability — they're just not exposed. The primary work across all three paths is making existing internal abstractions public, not inventing new ones. This is why all three are doable rather than aspirational.
