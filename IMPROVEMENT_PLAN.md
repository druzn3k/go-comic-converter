# Improvement Plan for go-comic-converter

> **Grounded in**: direct source reading of all key files + three `explore`-agent subsystem reports + cross-verification of `audit-findings.md` (923 lines, 196 sections). Every file:line reference below has been verified against actual source.

## Priority Matrix

| # | Finding | Priority | Category |
|---|---------|----------|----------|
| 1 | Race on `err` named return in `Load()` — multiple goroutines write shared variable | CRITICAL | Bug |
| 2 | Nil pointer deref in `loadCbr` solid mode — `f.Name` when `f` is nil | CRITICAL | Bug |
| 3 | XML injection in XHTML templates (Title/Author via `text/template`) | CRITICAL | Security |
| 4 | Decompression bomb / unbounded image decode (no dimension limits) | CRITICAL | Security / Performance |
| 5 | TOC bug for PDF sources — empty `img.Path` causes images missing from TOC | HIGH | Bug |
| 6 | `utils.Fatalf` (os.Exit) in goroutines leaks resources, skips cleanup | HIGH | Bug |
| 7 | No panic recovery in worker goroutines | HIGH | Bug |
| 8 | `bar.Close()` called from worker goroutines (thread-safety + double-close risk) | HIGH | Bug |
| 9 | FD leaks in `NewStorageImageReader` error paths | HIGH | Bug |
| 10 | Temp file TOCTOU race / no crash cleanup (`Output + ".tmp"`) | HIGH | Security / Bug |
| 11 | `flag.ExitOnError` makes `Parse()` error unreachable + dead code | HIGH | Maintainability |
| 12 | `version()` crashes when offline — prints nothing before network fetch | HIGH | UX |
| 13 | ~80% loader code duplication between processor and passthrough | HIGH | Maintainability |
| 14 | `getSpineAuto` mutates input slice via `String()` method (side effect) | MEDIUM | Bug |
| 15 | Template re-parsed + regex compiled per page in hot loop | MEDIUM | Performance |
| 16 | `EPUBImage.Error` set but never checked by any reader | MEDIUM | Bug |
| 17 | `loadPdf` not supported in passthrough mode (silent gap) | MEDIUM | Bug / UX |
| 18 | `filepath.WalkDir` follows symlinks (input escape) | MEDIUM | Security |
| 19 | `regexp.MustCompile` on every `Validate()` call | MEDIUM | Performance |
| 20 | `imageOutput` channel unbuffered — storage write blocks all workers | MEDIUM | Performance |
| 21 | AutoContrast iterates all 65,536 histogram buckets via map | MEDIUM | Performance |
| 22 | AutoCrop `img.At(x,y)` per-pixel is extremely slow | MEDIUM | Performance |
| 23 | CoverTitle font-size linear search, font re-parsed per call, ignored parse error | MEDIUM | Bug / Performance |
| 24 | `flate.BestCompression` on pre-compressed JPEG wastes CPU for ~0% gain | MEDIUM | Performance |
| 25 | Near-zero test coverage (1 test file, Example-only) | MEDIUM | Test Coverage |
| 26 | jsonprogress `current` field race (latent) + no throttling | MEDIUM | Bug / Performance |
| 27 | Solid CBR reads entire entry into memory before decode | MEDIUM | Performance |
| 28 | `loadCbr` Fatalf inside feeder goroutine leaks all workers | MEDIUM | Bug |
| 29 | Profiles `String()` non-deterministic map iteration (random `-help` order) | MEDIUM | Bug / UX |
| 30 | Config file permissions `0666` instead of `0600` | MEDIUM | Security |
| 31 | `io.EOF` detected via string comparison instead of `errors.Is` | LOW | Maintainability |
| 32 | `os.UserHomeDir()` error ignored in `FileName()` | LOW | Bug |
| 33 | Color regex case-sensitive — rejects lowercase hex | LOW | Bug / UX |
| 34 | sortpath edge cases (range captured but unused, decimal-only, negative, case) | LOW | Bug |
| 35 | `GrayScaleMode` JSON tag mismatch (`gray_scale_mode` vs `grayscale_mode`) | LOW | Bug |
| 36 | "Convertor" misspelling in `content.go:108` | LOW | Maintainability |
| 37 | Discarded errors (`_ = f.Close()`, `_ = bar.Close()`) | LOW | Maintainability |
| 38 | `isZeroValue` copied from stdlib | LOW | Maintainability |
| 39 | `CompressImage`/`CompressRaw` ~90% code duplication | LOW | Maintainability |
| 40 | `getSpineAuto` stateful `isOnTheRight` toggle — order-dependent behavior | LOW | Maintainability |
| 41 | No context/cancellation support throughout | LOW | UX |
| 42 | `go-latest` / `pdfreader` / `freetype` pinned to old/unmaintained commits | LOW | Dependency |
| 43 | `fogleman/gg` and `golang/freetype` font rendering overlap | LOW | Dependency |
| 44 | No CI, no Makefile, no linting pipeline | NICE-TO-HAVE | Maintainability |
| 45 | `gofrs/uuid` v4 incompatible import path (should be v5) | NICE-TO-HAVE | Dependency |
| 46 | Redundant IDE suppression annotations (`HttpUrlsUsage` ×4) | NICE-TO-HAVE | Maintainability |
| 47 | Deprecated ZIP time fields (`ModifiedTime`/`ModifiedDate`) | NICE-TO-HAVE | Maintainability |

---

## Findings & Recommendations

### 1. Race on `err` named return in `Load()`
- **Category**: Bug
- **Severity**: CRITICAL
- **Description**: `Load()` declares `err` as a named return (`func (e ePUBImageProcessor) Load() (images []epubimage.EPUBImage, err error)`). Multiple worker goroutines write to this same `err` variable at lines 73 and 83: `if err = imgStorage.Add(...)`. Since N goroutines run concurrently, these writes race. The Go race detector (`go test -race`) would flag this. While `utils.Fatalf` calls `os.Exit(1)` immediately after, which masks the race in practice, the behavior is **undefined** per the Go memory model. If `os.Exit` were replaced with error propagation (finding #6), the race becomes a real correctness bug — the first goroutine's error could be overwritten by a second goroutine's nil or different error.
- **Files**: `internal/pkg/epubimageprocessor/processor.go:36` (named return), `:73` (first write), `:83` (second write).
- **Recommended fix**: Use a local error variable in each goroutine: `if e := imgStorage.Add(...); e != nil { ... }`. Combined with finding #6 (error channel), send the local error to `errc` instead of writing to the named return.
- **Estimated effort**: Small

### 2. Nil pointer dereference in `loadCbr` solid mode
- **Category**: Bug
- **Severity**: CRITICAL
- **Description**: In the solid-CBR feeder goroutine, `r.Next()` returns `(f *rar.FileHeader, err error)`. On error (non-EOF), `f` is nil. The error handler accesses `f.Name`: `utils.Fatalf("\nerror processing image %s: %s\n", f.Name, rerr)` — this panics with a nil pointer dereference before the fatal message is even printed. The user sees a raw Go panic instead of a useful error message.
- **Files**: `internal/pkg/epubimageprocessor/loader.go:327` (`f.Name` when `f` is nil).
- **Recommended fix**: Use `e.Input` or a static string instead of `f.Name` in the error path:
  ```go
  f, rerr := r.Next()
  if rerr != nil {
      if rerr == io.EOF { break }
      utils.Fatalf("\nerror processing archive %s: %s\n", e.Input, rerr)
  }
  ```
  Or better: replace `utils.Fatalf` with error propagation (finding #6).
- **Estimated effort**: Small

### 3. XML injection in XHTML page templates
- **Category**: Security
- **Severity**: CRITICAL
- **Description**: `text.xhtml.tmpl` and `blank.xhtml.tmpl` are rendered via Go's `text/template`, which does **not** auto-escape. The `{{ .Title }}` placeholder appears in `<title>` (both templates) and in `alt="{{ .Title }}"` (`text.xhtml.tmpl:11`). `Title` derives from user input (`-title` flag) or from the input filename (`converter.go:~370`). A title containing `</title><script>alert(1)</script>` or `"><img src=x onerror=...>` produces malformed / XSS-capable XHTML. While EPUB readers are not browsers, malformed XHTML can break rendering in strict parsers (Kobo, Apple Books) and is an injection vector for any web-based reader. Note: `content.opf` and `toc.ncx` use `etree` (`CreateText`/`CreateAttr`) which **does** properly escape — the inconsistency is the bug.
- **Files**: `internal/pkg/epubtemplates/text.xhtml.tmpl:6,11`; `internal/pkg/epubtemplates/blank.xhtml.tmpl:6`; rendered at `pkg/epub/epub.go:75` (`writeImage`), `:109` (`writeBlank`), `:128` (`writeCoverImage`), `:163` (`writeTitleImage`).
- **Recommended fix**: Switch the page template rendering from `text/template` to `html/template` (which auto-escapes in HTML/XML context), or manually XML-escape `.Title` before injecting. The simplest correct fix: replace `template.New("parser")` at `epub.go:49` with `html/template.New("parser")` and verify the `.ImageStyle` and `.ViewPort` values are static/numeric (they are — generated from `ImgStyle()` and `Port()`). Alternatively, wrap `.Title` with a `xmlEscape` funcmap function.
- **Estimated effort**: Small

### 4. Decompression bomb / unbounded image decode
- **Category**: Security / Performance
- **Severity**: CRITICAL
- **Description**: The processor pipeline calls `image.Decode` (`loader.go:159,251,369`) without first checking dimensions via `image.DecodeConfig`. Go's stdlib decoders have no built-in dimension cap — a crafted 50KB JPEG advertising 50000×50000 pixels allocates ~10GB of `image.RGBA`. With `WorkersRatio(50)` (e.g. 16 concurrent decodes), a single malicious CBZ containing many such images can OOM-kill the process. Passthrough uses `DecodeConfig` first but only for metadata, never rejecting oversized dimensions. Additionally, `copyRawDataToStorage` in passthrough divides `float64(config.Height) / float64(config.Width)` — if `config.Width` is 0 (crafted header), this panics with division by zero.
- **Files**: `internal/pkg/epubimageprocessor/loader.go:159` (loadDir), `:251` (loadCbz), `:369` (loadCbr); `internal/pkg/epubimagepassthrough/passthrough.go:404` (div-by-zero), `:403` (DecodeConfig without rejection).
- **Recommended fix**: Add a `decodeConfig` pre-check before every `image.Decode` call. Define a max dimension constant (e.g. `const maxImageDim = 20000`). Call `image.DecodeConfig` first; if `cfg.Width * cfg.Height > maxPixels` or either dimension is 0, return a `corruptedImage` placeholder with an error. Centralize in a `decodeBounded` helper. For passthrough, check `config.Width == 0 || config.Height == 0` before division.
- **Estimated effort**: Medium

### 5. TOC bug for PDF sources — images missing from TOC
- **Category**: Bug
- **Severity**: HIGH
- **Description**: `Toc()` builds the TOC by iterating `strings.Split(img.Path, string(filepath.Separator))` for each image. For PDF sources, all images have `Path: ""`. `strings.Split("", separator)` returns `[""]`. `filepath.Join(".", "")` = `"."`. `paths["."]` already exists (the root `ol`), so `continue` is hit — the image's page link is **never added to the TOC**. PDF-sourced EPUBs have an empty TOC (only the "beginning" link is present). This is a functional bug that affects usability.
- **Files**: `internal/pkg/epubtemplates/toc.go:34-42`.
- **Recommended fix**: Special-case empty `img.Path`: if `img.Path == ""`, add the image link directly to the root `ol` without the path-splitting loop. Or set `img.Path` to a default like `"Pages"` for PDF sources in `loadPdf`.
- **Estimated effort**: Small

### 6. `utils.Fatalf` in goroutines causes resource leaks
- **Category**: Bug
- **Severity**: HIGH
- **Description**: `utils.Fatalf` calls `os.Exit(1)`. When called inside a worker goroutine (`processor.go:74,84` on `imgStorage.Add` error; `loader.go:315,327,333` in CBR feeder), `os.Exit` terminates the process **without** running deferred functions: `defer wg.Done()` never runs, `close(imageOutput)` never runs, `defer imgStorage.Close()` never runs, `defer r.Close()` never runs. The `.tmp` storage file is left in an indeterminate state. Other worker goroutines are killed mid-flight. This is systemic — `utils.Fatalf` is called from library code (`internal/pkg`) throughout, making the code untestable (can't recover from `os.Exit` in tests).
- **Files**: `internal/pkg/epubimageprocessor/processor.go:74,84`; `internal/pkg/epubimageprocessor/loader.go:315,327,333`; `internal/pkg/utils/utils.go` (Fatalf definition).
- **Recommended fix**: Replace in-goroutine `utils.Fatalf` with error propagation via channel. Workers send errors to an `errc chan error` (buffered size 1). The main `Load()` goroutine collects errors and returns the first one. Only the top-level `main()` should call `os.Exit`. Pattern:
  ```go
  errc := make(chan error, 1)
  // in worker:
  if e := imgStorage.Add(...); e != nil {
      select { case errc <- e: default: }
      return
  }
  // in Load(), after collecting:
  select { case err := <-errc: return nil, err; default: }
  ```
- **Estimated effort**: Medium

### 7. No panic recovery in worker goroutines
- **Category**: Bug
- **Severity**: HIGH
- **Description**: Zero `recover()` calls exist in `epubimageprocessor` or `epubimagepassthrough`. A panic in any worker goroutine (nil pointer in `image.Decode`, divide-by-zero in aspect ratio, nil font from `truetype.Parse` error in `cover_title.go`) crashes the entire process with no cleanup. The temp ZIP, progress bar, and all file handles are abandoned.
- **Files**: `internal/pkg/epubimageprocessor/processor.go:64-97` (worker goroutine); `internal/pkg/epubimageprocessor/loader.go` (decoder goroutines); `internal/pkg/epubimagefilters/cover_title.go:~25` (nil font risk).
- **Recommended fix**: Add a `defer` recovery wrapper at the top of every worker goroutine:
  ```go
  go func() {
      defer wg.Done()
      defer func() {
          if r := recover(); r != nil {
              errc <- fmt.Errorf("worker panic: %v\n%s", r, debug.Stack())
          }
      }()
      // ... worker body
  }()
  ```
- **Estimated effort**: Small

### 8. `bar.Close()` called from worker goroutines
- **Category**: Bug
- **Severity**: HIGH
- **Description**: `bar.Close()` is called from inside worker goroutines (`processor.go:74,84`) when `imgStorage.Add` fails. `EPUBProgress` implementations may not be thread-safe — `progressbar.ProgressBar.Close()` from multiple goroutines could cause issues. Additionally, the main goroutine calls `bar.Close()` at the end of `Load()` (after the collection loop), which would be a double-close if a worker already closed it (though `os.Exit` prevents this in practice). This is fragile and coupled with the `os.Exit` pattern.
- **Files**: `internal/pkg/epubimageprocessor/processor.go:74,84` (worker close), `:99` (main close).
- **Recommended fix**: Never close shared resources from worker goroutines. Workers should signal failure via `errc`; the main goroutine handles `bar.Close()` in a `defer` after the collection loop. This is subsumed by finding #6.
- **Estimated effort**: Small (subsumed by #6)

### 9. File descriptor leaks in `NewStorageImageReader`
- **Category**: Bug
- **Severity**: HIGH
- **Description**: `NewStorageImageReader` opens a file (`os.Open`), then calls `fh.Stat()` and `zip.NewReader()`. If `Stat` fails (line 21-23) or `zip.NewReader` fails (line 25-27), the function returns a zero-value `StorageImageReader` **without closing `fh`**. The file descriptor leaks until GC. On long runs or repeated invocations, this can exhaust FDs.
- **Files**: `internal/pkg/epubzip/storage_image_reader.go:19-28`.
- **Recommended fix**: Add `fh.Close()` on every error path before returning:
  ```go
  s, err := fh.Stat()
  if err != nil {
      fh.Close()
      return StorageImageReader{}, err
  }
  fz, err := zip.NewReader(fh, s.Size())
  if err != nil {
      fh.Close()
      return StorageImageReader{}, err
  }
  ```
- **Estimated effort**: Small

### 10. Temp file TOCTOU race and no crash cleanup
- **Category**: Security / Bug
- **Severity**: HIGH
- **Description**: `ImgStorage()` returns `Output + ".tmp"` (`epub_options.go:34`). `StorageImageWriter` creates this with `os.Create` (truncates). The path is deterministic and predictable — two concurrent conversions of the same output would collide and corrupt each other's temp ZIP. There's no `O_EXCL` to detect a pre-existing temp file from a crashed prior run (it's silently truncated). On crash/panic, the `.tmp` file is never cleaned up (findings #6, #7 compound this). The cleanup `defer imgStorage.Remove()` in `epub.go:Write()` only runs if no error occurs before the defer is registered.
- **Files**: `pkg/epuboptions/epub_options.go:34`; `internal/pkg/epubzip/storage_image_writer.go` (os.Create); `pkg/epub/epub.go:296` (defer cleanup).
- **Recommended fix**: Use `os.CreateTemp` in the output directory with a random suffix, or use `os.OpenFile` with `O_CREATE|O_EXCL|O_WRONLY` and retry with a new name on `EEXIST`. Ensure cleanup runs on **all** paths — move `imgStorage.Remove()` to a `defer` that always executes. Add a check at startup: if `Output + ".tmp"` exists, warn the user.
- **Estimated effort**: Medium

### 11. `flag.ExitOnError` makes `Parse()` error unreachable
- **Category**: Maintainability
- **Severity**: HIGH
- **Description**: `converter.go:23` creates the FlagSet with `flag.ExitOnError`. This means `cmd.Parse()` will **call `os.Exit(2)`** on parse errors — the `Parse()` method's error return is dead code. The custom `Usage` function and `Fatal` handler are bypassed. `isZeroValue` is copied verbatim from the stdlib `flag` package to support the custom Usage.
- **Files**: `internal/pkg/converter/converter.go:23` (ExitOnError), `:229-290` (Parse), `:241-267` (isZeroValue copy).
- **Recommended fix**: Change to `flag.ContinueOnError`. Handle the returned error in `Parse()` by calling `c.Fatal(err)`. Remove the `isZeroValue` copy if possible, or annotate it.
- **Estimated effort**: Small

### 12. `version()` crashes when offline
- **Category**: UX
- **Severity**: HIGH
- **Description**: `version()` calls `githubTag.Fetch()` before printing the local version. If the network is unavailable, `utils.Fatalln("failed to fetch the latest version")` exits with no output — the user asked for `-version` and gets nothing. The local version (available from `debug.ReadBuildInfo()`) is never printed.
- **Files**: `main.go:42-44`.
- **Recommended fix**: Print local version info first, then attempt the remote fetch. If the fetch fails, print the local version and a note that the latest version check failed:
  ```go
  bi, ok := debug.ReadBuildInfo()
  // print local version
  utils.Printf("go-comic-converter\n  Version: %s\n", bi.Main.Version)
  // attempt remote (optional)
  v, err := githubTag.Fetch()
  if err != nil {
      utils.Printf("  (could not check for updates: %v)\n", err)
      return
  }
  // print latest
  ```
- **Estimated effort**: Small

### 13. ~80% loader code duplication between processor and passthrough
- **Category**: Maintainability
- **Severity**: HIGH
- **Description**: The file discovery, sorting, zip/rar iteration, and `isSupportedImage` logic is duplicated almost verbatim between `epubimageprocessor/loader.go` and `epubimagepassthrough/passthrough.go`. The only differences: processor adds goroutine fan-out; passthrough is sequential. `isSupportedImage` even differs — processor accepts `.webp`/`.tiff`, passthrough does not (a silent feature gap). Any bug fix or new format must be applied in two places.
- **Files**: `internal/pkg/epubimageprocessor/loader.go:46,52-60,105-140,188-224,267-308`; `internal/pkg/epubimagepassthrough/passthrough.go:48,393-399,62-82,115-148,165-202`.
- **Recommended fix**: Extract a shared `internal/pkg/epubimageloader` package with `Discover`, `ListCbz`, `ListCbr`, `IsSupportedImage`, `CorruptedImage`. Both processor and passthrough import this and add their own decode/store strategy. This also fixes the `.webp`/`.tiff` gap.
- **Estimated effort**: Large

### 14. `getSpineAuto` mutates input slice via `String()` method
- **Category**: Bug
- **Severity**: MEDIUM
- **Description**: `Content.String()` calls `getSpineAuto`, which writes `img.Position` back to `o.Images[i]` at line 270. Since `Content` is passed by value but `Images` is a slice (shared backing array), this modifies the caller's slice — a hidden side effect from a `String()` method. The `Position` field is later read by `epubimage.ImgStyle` to determine CSS alignment. This works because `Content.String()` is called before the images are rendered to XHTML, but it's a surprising mutation that violates the principle that `String()` should be read-only. Additionally, `getSpineAuto` uses a stateful `isOnTheRight` toggle that makes its behavior depend on call order.
- **Files**: `internal/pkg/epubtemplates/content.go:270` (`o.Images[i] = img`), `:237-275` (getSpineAuto with `isOnTheRight` toggle).
- **Recommended fix**: Compute `Position` as a separate explicit pass before calling `String()`, or return a new slice from `getSpineAuto` instead of mutating in place. Document that `Position` must be set before rendering.
- **Estimated effort**: Medium

### 15. Template re-parsed + regex compiled per page in hot loop
- **Category**: Performance
- **Severity**: MEDIUM
- **Description**: `epub.render()` (`epub.go:87`) calls `template.Must(e.templateProcessor.Parse(templateString))` on **every** call — for a 500-page comic, `text.xhtml.tmpl` is parsed 500+ times. Additionally, `regexp.MustCompile("\n+")` is compiled on every `render` call (`epub.go:87`). Both are expensive operations repeated in a hot loop.
- **Files**: `pkg/epub/epub.go:66` (render), `:87` (regex per call).
- **Recommended fix**: Pre-parse all templates once in `New()` and cache them in a `map[string]*template.Template`. Move the regex to a package-level `var`. `render()` does `tmpl.Execute()` + `newlineRe.ReplaceAllString()` only.
- **Estimated effort**: Small

### 16. `EPUBImage.Error` set but never read
- **Category**: Bug
- **Severity**: MEDIUM
- **Description**: `EPUBImage.Error` is populated by `corruptedImage()` when decode fails. The **only** reader is in `epub.go:Write()` at the very end (`:448-461`) where it's printed as a warning — **after** the EPUB is fully written. The error is never used to skip the image, abort early, or mark the output as degraded. A user converting a CBZ with 50 corrupt images gets a finished EPUB with 50 placeholder pages and a post-hoc warning that may scroll off-screen.
- **Files**: `internal/pkg/epubimage/epub_image.go` (Error field); `internal/pkg/epubimageprocessor/loader.go:79`; `pkg/epub/epub.go:448-461`.
- **Recommended fix**: Print corrupt-image warnings **during** processing (not after). Return non-zero exit code if any errors occurred. Add a `--strict` flag to abort on first corrupt image. Move the error summary before the success stats.
- **Estimated effort**: Small

### 17. PDF not supported in passthrough mode
- **Category**: Bug / UX
- **Severity**: MEDIUM
- **Description**: `epubimageprocessor.Load()` supports `loadPdf`; `epubimagepassthrough.Load()` does not. If a user runs with `--format copy` and a PDF input, the error `"unknown file format (.pdf): support .cbz, .zip, .cbr, .rar"` gives no hint that passthrough doesn't support PDF.
- **Files**: `internal/pkg/epubimageprocessor/loader.go:62-76` (has pdf); `internal/pkg/epubimagepassthrough/passthrough.go:20-42` (no pdf).
- **Recommended fix**: Detect the combination early in `Validate()` and return a clear error: `"format 'copy' does not support PDF input"`. Or add PDF support to passthrough (copy extracted page images without re-encoding).
- **Estimated effort**: Small

### 18. `filepath.WalkDir` follows symlinks
- **Category**: Security
- **Severity**: MEDIUM
- **Description**: `loadDir` uses `filepath.WalkDir(input, ...)`. WalkDir follows symlinks by default. If the input directory contains a symlink to `/etc/` or `../../`, WalkDir will walk into it. A symlink loop causes infinite recursion.
- **Files**: `internal/pkg/epubimageprocessor/loader.go:108`; `internal/pkg/epubimagepassthrough/passthrough.go:~65`.
- **Recommended fix**: Check `d.Type()&os.ModeSymlink != 0` in the WalkDir callback and skip symlinks, or resolve and verify the target is within the input directory.
- **Estimated effort**: Small

### 19. `regexp.MustCompile` on every `Validate()` call
- **Category**: Performance
- **Severity**: MEDIUM
- **Description**: `Validate()` calls `regexp.MustCompile("^[0-9A-F]{3}$")` on every invocation (`converter.go:~380,~384`). `MustCompile` parses and compiles the regex each time.
- **Files**: `internal/pkg/converter/converter.go:~380,~384`.
- **Recommended fix**: Move to package-level: `var hexColorRe = regexp.MustCompile("^[0-9A-F]{3}$")`.
- **Estimated effort**: Small

### 20. Unbuffered `imageOutput` channel blocks all workers
- **Category**: Performance
- **Severity**: MEDIUM
- **Description**: `imageOutput := make(chan epubimage.EPUBImage)` is unbuffered. Each worker blocks on `imageOutput <- img` until the single collecting goroutine receives. If the collector is slow (appending to slice, progress bar update), all N workers stall. This serializes the tail of processing and reduces effective parallelism.
- **Files**: `internal/pkg/epubimageprocessor/processor.go:52`.
- **Recommended fix**: Buffer the channel: `make(chan epubimage.EPUBImage, e.WorkersRatio(wr)*2)`.
- **Estimated effort**: Small

### 21. AutoContrast iterates all 65,536 histogram buckets via map
- **Category**: Performance
- **Severity**: MEDIUM
- **Description**: `AutoContrast.mean()` builds a `map[int]int` histogram, then iterates `for colorIdx := range 1 << 16` — all 65,536 possible 16-bit values — even though the histogram is sparse (typically ~256 distinct values for 8-bit images). The map adds hash overhead and GC pressure. This is 256× more work than necessary.
- **Files**: `internal/pkg/epubimagefilters/auto_contrast.go` (mean function, line ~24, ~31-36).
- **Recommended fix**: Use a fixed `[65536]int` array instead of a map (eliminates hash overhead), and iterate only non-zero entries. Or iterate the map keys directly.
- **Estimated effort**: Small

### 22. AutoCrop `img.At(x,y)` per-pixel is extremely slow
- **Category**: Performance
- **Severity**: MEDIUM
- **Description**: `AutoCrop.findMargin()` calls `img.At(x, y)` for each pixel — each call involves color model conversion, interface dispatch, and potential allocation. For a 4000×6000 image, the four-edge scan touches tens of millions of pixels through this slow path. `colorIsBlank` additionally calls `color.GrayModel.Convert(c)` per pixel. The blank threshold (0xe0/224) is also hardcoded with no way to configure.
- **Files**: `internal/pkg/epubimagefilters/auto_crop.go` (findMargin, colorIsBlank).
- **Recommended fix**: Type-assert to the concrete image type (`*image.RGBA`, `*image.NRGBA`, `*image.Gray`) and access the pixel slice directly. Fall back to `img.At` only for unknown types. Consider sampling (check every Nth pixel) for large images.
- **Estimated effort**: Medium

### 23. CoverTitle font-size linear search, font re-parsed, ignored error
- **Category**: Bug / Performance
- **Severity**: MEDIUM
- **Description**: `cover_title.go` parses the font on every `Draw` call: `f, _ := truetype.Parse(gomonobold.TTF)` — error is silently ignored; if parsing fails, `f` is nil and `truetype.NewFace(f, ...)` panics. Font size search is linear: `for fontSize = p.maxFontSize; fontSize >= 12; fontSize -= 1` — can iterate 1000+ times, each creating a new `truetype.NewFace` (allocation). If text doesn't fit at size 12, it overflows with no truncation.
- **Files**: `internal/pkg/epubimagefilters/cover_title.go:~25,~26-32`.
- **Recommended fix**: Cache the parsed font at package level (parse once). Use binary search for font size. Check the parse error: `f, err := truetype.Parse(...); if err != nil { return err }`.
- **Estimated effort**: Small

### 24. `flate.BestCompression` on pre-compressed JPEG wastes CPU
- **Category**: Performance
- **Severity**: MEDIUM
- **Description**: `CompressImage` and `CompressRaw` use `flate.BestCompression` on JPEG/PNG data that is already compressed. For JPEG, deflate provides minimal gain (5–10% at best) but `BestCompression` is the slowest level. For PNG (already deflate-compressed), double compression is nearly useless. This wastes significant CPU on every image.
- **Files**: `internal/pkg/epubzip/image.go:21,51`.
- **Recommended fix**: Use `flate.BestSpeed` for JPEG (already compressed, minimal gain) and `flate.DefaultCompression` for PNG. Or skip deflate entirely for JPEG and store raw (ZIP Store method).
- **Estimated effort**: Small

### 25. Near-zero test coverage
- **Category**: Test Coverage
- **Severity**: MEDIUM
- **Description**: The entire project has exactly one test file: `internal/pkg/utils/utils_test.go` with Example-style tests for 4 functions. No unit tests for: converter (flag parsing, validation, config), epubimageprocessor (image pipeline, loaders), epubimagefilters (crop, contrast, split), epubzip (compression, storage), epubtemplates (XML generation), sortpath (natural sort), epubtree, epubprogress, or the top-level `epub.Write()`. Every finding in this plan has no regression test.
- **Files**: `internal/pkg/utils/utils_test.go` (the only test file).
- **Recommended fix**: See **Test Strategy** section below.
- **Estimated effort**: Large

### 26. jsonprogress race (latent) + no throttling
- **Category**: Bug / Performance
- **Severity**: MEDIUM
- **Description**: `jsonprogress.Add()` increments `p.current` without a mutex or atomic — latent data race if called from multiple goroutines. Additionally, every `Add(1)` writes a JSON line to stdout — for a 1000-image comic, that's 1000 JSON lines with no throttling (progressbar has `OptionThrottle(65ms)` but jsonprogress does not). `Close()` is a no-op that doesn't write a final "complete" state.
- **Files**: `internal/pkg/epubprogress/json.go:14,19,26`.
- **Recommended fix**: Use `atomic.Int64` for `current`. Add throttling (buffer writes, flush every ~65ms). Write a final "complete" JSON object in `Close()`.
- **Estimated effort**: Small

### 27. Solid CBR reads entire entry into memory before decode
- **Category**: Performance
- **Severity**: MEDIUM
- **Description**: `loadCbr` solid-archive branch reads the entire uncompressed file into a `bytes.Buffer` before passing to workers (`loader.go:~320`: `io.Copy(&b, r)`). For large images in solid RARs, this doubles peak memory. Passthrough's `copyRawDataToStorage` uses `io.ReadAll` — same full-buffer pattern.
- **Files**: `internal/pkg/epubimageprocessor/loader.go:~320`; `internal/pkg/epubimagepassthrough/passthrough.go:~410`.
- **Recommended fix**: Use `io.Pipe` with a reader goroutine to stream the solid RAR entry to `image.Decode`. If not feasible with the `rardecode` API, document the memory cost and add a dimension pre-check (finding #4) to limit exposure.
- **Estimated effort**: Medium

### 28. `loadCbr` Fatalf inside feeder goroutine leaks all workers
- **Category**: Bug
- **Severity**: MEDIUM
- **Description**: In the solid-CBR path, the feeder goroutine calls `utils.Fatalf` on errors (`loader.go:315,327,333`). This `os.Exit` kills the process while N decoder goroutines are still running, `output` channel is never closed, and `defer r.Close()` never runs. Same root cause as finding #6.
- **Files**: `internal/pkg/epubimageprocessor/loader.go:315,327,333`.
- **Recommended fix**: Same as finding #6 — replace `utils.Fatalf` with error propagation via channel.
- **Estimated effort**: Medium (subsumed by #6)

### 29. Profiles `String()` non-deterministic map iteration
- **Category**: Bug / UX
- **Severity**: MEDIUM
- **Description**: `Profiles.String()` iterates a map with `for _, v := range p` — map iteration order is non-deterministic in Go. Every `-help` call shows device profiles in a different order. This is confusing and makes documentation screenshots non-reproducible.
- **Files**: `internal/pkg/converter/profiles.go:62-69`.
- **Recommended fix**: Sort by profile code before formatting:
  ```go
  codes := make([]string, 0, len(p))
  for k := range p { codes = append(codes, k) }
  sort.Strings(codes)
  for _, code := range codes {
      v := p[code]
      s = append(s, fmt.Sprintf(...))
  }
  ```
- **Estimated effort**: Small

### 30. Config file permissions `0666` instead of `0600`
- **Category**: Security
- **Severity**: MEDIUM
- **Description**: `SaveConfig` uses `os.Create` which creates the file with `0666 & ~umask`. The config file may contain user preferences. On a multi-user system, other users can read the config. Should use `0600`.
- **Files**: `internal/pkg/converter/options.go:~202` (SaveConfig).
- **Recommended fix**: Use `os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)` instead of `os.Create`.
- **Estimated effort**: Small

### 31. `io.EOF` detected via string comparison
- **Category**: Maintainability
- **Severity**: LOW
- **Description**: `LoadConfig()` detects "file not found" via `err.Error() != "EOF"` and `strings.Contains(err.Error(), "no such file")` instead of `errors.Is(err, os.ErrNotExist)` and `err != io.EOF`. This breaks on non-English locales and is fragile.
- **Files**: `internal/pkg/converter/options.go:~84,~91`.
- **Recommended fix**: Use `errors.Is(err, os.ErrNotExist)` for file-not-found and `err == io.EOF` (or `errors.Is(err, io.EOF)`) for empty file.
- **Estimated effort**: Small

### 32. `os.UserHomeDir()` error ignored in `FileName()`
- **Category**: Bug
- **Severity**: LOW
- **Description**: `FileName()` calls `os.UserHomeDir()` and ignores the error, using an empty string. The config path becomes `/.go-comic-converter.yaml` (root filesystem), which will fail on `SaveConfig` with permission denied. The error message will be confusing. Additionally, `LoadConfig` swallows all `os.Open` errors (including permission errors, not just missing file).
- **Files**: `internal/pkg/converter/options.go:76` (FileName), `:84` (LoadConfig error swallowing).
- **Recommended fix**: Return an error from `FileName()` or handle at the call site. In `LoadConfig`, distinguish permission errors from missing-file errors.
- **Estimated effort**: Small

### 33. Color regex case-sensitive
- **Category**: Bug / UX
- **Severity**: LOW
- **Description**: `Validate()` uses `^[0-9A-F]{3}$` which only accepts uppercase hex. A user passing `--foreground-color fff` gets a validation error. CSS accepts both cases, so the validation is overly strict.
- **Files**: `internal/pkg/converter/converter.go:~319,~384`.
- **Recommended fix**: Use `^[0-9A-Fa-f]{3}$` or normalize to uppercase before validation.
- **Estimated effort**: Small

### 34. sortpath edge cases
- **Category**: Bug
- **Severity**: LOW
- **Description**: The natural sort parser has multiple edge cases: (a) the regex captures a range end `r[3]` (e.g. `s2-3`) but **never uses it** — `s2-3` and `s2-4` compare as equal; (b) decimal-only filenames like `.5` are not matched as numeric; (c) negative numbers not handled; (d) `float64` precision loss for numbers > 2^53; (e) case-insensitive `strings.ToLower` may cause unstable ordering for names differing only in case.
- **Files**: `internal/pkg/sortpath/parser.go:28-31` (r[3] unused), `:16` (number==0), `:37` (ToLower).
- **Recommended fix**: (a) Use `r[3]` for range comparison or remove the capture group; (b) allow leading-dot numbers; (c) handle leading `-`; (d) use `int64` for large numbers; (e) document case-insensitive behavior.
- **Estimated effort**: Small

### 35. `GrayScaleMode` JSON tag mismatch
- **Category**: Bug
- **Severity**: LOW
- **Description**: `Image.GrayScaleMode` has `json:"gray_scale_mode"` but `yaml:"grayscale_mode"` — inconsistent naming between serialization formats. Could confuse API consumers and cause field mapping issues.
- **Files**: `pkg/epuboptions/image.go:16`.
- **Recommended fix**: Use the same field name in both tags: `json:"grayscale_mode"`.
- **Estimated effort**: Small

### 36. "Convertor" misspelling
- **Category**: Maintainability
- **Severity**: LOW
- **Description**: `content.go:108` has `"Go Comic Convertor"` — should be `"Go Comic Converter"`. This appears in the EPUB metadata (publisher field).
- **Files**: `internal/pkg/epubtemplates/content.go:108`.
- **Recommended fix**: Fix the spelling.
- **Estimated effort**: Small

### 37. Discarded errors throughout
- **Category**: Maintainability
- **Severity**: LOW
- **Description**: Multiple `_ = f.Close()`, `_ = bar.Close()`, `_ = r.Close()` calls discard errors. Pattern appears at: `loader.go:160,216,253,371`; `processor.go:81,92,112`; `epub.go:285,298`; `main.go:89` (JSON encode error swallowed).
- **Recommended fix**: Where non-actionable, add a comment. For `f.Close()` after reads, consider `defer` with error capture. Low priority.
- **Estimated effort**: Small

### 38. `isZeroValue` copied from stdlib
- **Category**: Maintainability
- **Severity**: LOW
- **Description**: `converter.go:241-267` contains a verbatim copy of `isZeroValue` from Go's `flag` package (including the panic recovery). This is a DRY concern and will drift from stdlib fixes.
- **Files**: `internal/pkg/converter/converter.go:241-267`.
- **Recommended fix**: Add a comment `// Adapted from go/flag (Go 1.23)` to document the source. Low priority.
- **Estimated effort**: Small

### 39. `CompressImage`/`CompressRaw` ~90% code duplication
- **Category**: Maintainability
- **Severity**: LOW
- **Description**: Both functions create a deflate writer, write data, close writer, and create an `Image` struct with `FileHeader`. ~90% identical code.
- **Files**: `internal/pkg/epubzip/image.go:21-48,51-78`.
- **Recommended fix**: Extract a shared `deflateAndWrap(filename string, data []byte) (Image, error)` helper.
- **Estimated effort**: Small

### 40. `getSpineAuto` stateful toggle
- **Category**: Maintainability
- **Severity**: LOW
- **Description**: `getSpineAuto` uses `isOnTheRight` as a stateful toggle inside `getSpread`, making its behavior depend on call order. This is fragile and hard to reason about.
- **Files**: `internal/pkg/epubtemplates/content.go:237-275`.
- **Recommended fix**: Document the algorithm or refactor to make the state explicit.
- **Estimated effort**: Small

### 41. No context/cancellation support
- **Category**: UX
- **Severity**: LOW
- **Description**: No `context.Context` is threaded through the pipeline. Ctrl+C during a 500-image conversion kills the process (via signal), skipping cleanup. The temp ZIP is left behind. Workers don't check for cancellation. The `version()` GitHub fetch has no timeout.
- **Files**: Entire pipeline — no `context.Context` parameter anywhere.
- **Recommended fix**: Thread a `context.Context` from `main()` through `Write()` → `Load()` → workers. Use `signal.NotifyContext` for SIGINT. Clean up temp files on cancellation. Large refactor — Phase 3.
- **Estimated effort**: Large

### 42. Pinned old/unmaintained dependency commits
- **Category**: Dependency
- **Severity**: LOW
- **Description**: `go-latest` pinned to 2017, `pdfreader` to 2022, `golang/freetype` to 2017 (archived). `go-latest` pulls in `google/go-github v17` (current v60+), `hashicorp/go-version`, `go-querystring`.
- **Files**: `go.mod`.
- **Recommended fix**: Replace `go-latest` with direct `net/http` call to GitHub tags API (~20 lines). Check if `pdfreader` has a maintained fork. Replace `freetype` with `golang.org/x/image/font/sfnt`.
- **Estimated effort**: Medium

### 43. Font rendering dependency overlap
- **Category**: Dependency
- **Severity**: LOW
- **Description**: Both `fogleman/gg` (2D rendering) and `golang/freetype` (font rendering) are direct dependencies. `gg` uses `freetype` internally. `cover_title.go` uses `freetype` directly for `truetype.Parse`/`NewFace`.
- **Files**: `go.mod`; `internal/pkg/epubimagefilters/cover_title.go`.
- **Recommended fix**: Consolidate on `golang.org/x/image/font/sfnt` (already a transitive dep) and drop the direct `freetype` dependency.
- **Estimated effort**: Medium

### 44. No CI, no Makefile, no linting
- **Category**: Maintainability
- **Severity**: NICE-TO-HAVE
- **Description**: No `.github/workflows/`, no `Makefile`, no `.golangci.yml`. No automated testing, linting, or build verification.
- **Files**: Project root (absent files).
- **Recommended fix**: Add a minimal GitHub Actions workflow: `go build`, `go test ./...`, `golangci-lint run`. Add a `Makefile` with `build`, `test`, `lint`, `vet` targets.
- **Estimated effort**: Small

### 45. `gofrs/uuid` v4 incompatible import path
- **Category**: Dependency
- **Severity**: NICE-TO-HAVE
- **Description**: `go.mod` uses `github.com/gofrs/uuid v4.4.0+incompatible`. The `+incompatible` suffix means v4 without a module path. v5 exists with proper path.
- **Files**: `go.mod`; `pkg/epub/epub.go:48`.
- **Recommended fix**: Upgrade to `github.com/gofrs/uuid/v5` and update the import.
- **Estimated effort**: Small

### 46. Redundant IDE suppression annotations
- **Category**: Maintainability
- **Severity**: NICE-TO-HAVE
- **Description**: `content.go:32` has `//goland:noinspection HttpUrlsUsage,HttpUrlsUsage,HttpUrlsUsage,HttpUrlsUsage` — `HttpUrlsUsage` appears 4 times. Copy-paste artifact.
- **Files**: `internal/pkg/epubtemplates/content.go:32`.
- **Recommended fix**: Reduce to a single `HttpUrlsUsage`.
- **Estimated effort**: Small

### 47. Deprecated ZIP time fields
- **Category**: Maintainability
- **Severity**: NICE-TO-HAVE
- **Description**: `epub_zip.go:31-33` and `image.go:49,75` set deprecated `ModifiedTime`/`ModifiedDate` alongside `Modified`. The `//goland:noinspection GoDeprecation` annotation suppresses warnings.
- **Files**: `internal/pkg/epubzip/epub_zip.go:31-33`; `internal/pkg/epubzip/image.go:49,75`.
- **Recommended fix**: Eventually drop the deprecated fields and rely on `Modified` only. Keep for now if older EPUB reader compatibility is needed.
- **Estimated effort**: Small

---

## Phased Roadmap

### Phase 1: Critical Fixes (immediate)

| # | Finding | Effort |
|---|---------|--------|
| 1 | Race on `err` named return — use local error variables in goroutines | Small |
| 2 | Nil pointer deref in `loadCbr` — use `e.Input` instead of `f.Name` in error path | Small |
| 3 | XML injection — switch `text/template` → `html/template` for XHTML templates | Small |
| 4 | Decompression bomb — add `DecodeConfig` pre-check with max dimension | Medium |
| 5 | TOC bug for PDF — special-case empty `img.Path` in `Toc()` | Small |
| 6 | `utils.Fatalf` in goroutines — introduce `errc` channel, propagate errors | Medium |
| 7 | Panic recovery — add `defer recover()` in every worker goroutine | Small |
| 8 | `bar.Close()` from workers — subsumed by #6 | Small |
| 9 | FD leaks in `NewStorageImageReader` — add `fh.Close()` on error paths | Small |

**Phase 1 goal**: Make the tool safe against untrusted input, crash cleanly, and not race. These 9 items are the minimum for a correct, safe release.

### Phase 2: Quality Improvements (short-term)

| # | Finding | Effort |
|---|---------|--------|
| 10 | Temp file cleanup — `os.CreateTemp` or `O_EXCL`, always-run cleanup | Medium |
| 11 | `flag.ExitOnError` → `ContinueOnError`, wire to `Fatal()` | Small |
| 12 | `version()` offline — print local version before network fetch | Small |
| 14 | `getSpineAuto` side effect — compute `Position` in separate pass | Medium |
| 15 | Template caching + regex to package-level | Small |
| 16 | Error surfacing — warnings during processing, non-zero exit, `--strict` | Small |
| 17 | PDF + passthrough — detect early, clear error | Small |
| 18 | Symlink following — skip symlinks in WalkDir | Small |
| 19 | Regex to package-level var | Small |
| 20 | Channel buffering — buffer `imageOutput` | Small |
| 23 | CoverTitle binary search + error check + font caching | Small |
| 24 | `flate.BestSpeed` for JPEG | Small |
| 26 | jsonprogress `atomic.Int64` + throttling | Small |
| 29 | Profiles sort in `String()` | Small |
| 30 | Config file permissions `0600` | Small |
| 31 | EOF via `errors.Is` | Small |
| 32 | `UserHomeDir` error handling | Small |
| 33 | Color regex case-insensitive | Small |
| 35 | `GrayScaleMode` JSON tag fix | Small |
| 36 | "Convertor" → "Converter" | Small |

### Phase 3: Architectural Changes (medium-term)

| # | Finding | Effort |
|---|---------|--------|
| 13 | Extract shared `epubimageloader` package | Large |
| 21 | AutoContrast — fixed array, iterate non-zero entries | Small |
| 22 | AutoCrop — type-assert to concrete image types | Medium |
| 27 | Solid CBR streaming via `io.Pipe` | Medium |
| 41 | Context/cancellation throughout pipeline | Large |
| 25 | Build out test suites (see Test Strategy) | Large |

### Phase 4: Nice-to-Have (long-term)

| # | Finding | Effort |
|---|---------|--------|
| 34 | sortpath edge cases (range, decimal, negative, case) | Small |
| 37 | Discarded errors audit | Small |
| 38 | `isZeroValue` annotation | Small |
| 39 | `CompressImage`/`CompressRaw` dedup | Small |
| 40 | `getSpineAuto` stateful toggle refactor | Small |
| 42 | Dependency audit (replace `go-latest`, `freetype`) | Medium |
| 43 | Font dependency consolidation | Medium |
| 44 | CI/Makefile/linting | Small |
| 45 | uuid v5 upgrade | Small |
| 46 | Redundant IDE suppression cleanup | Small |
| 47 | Deprecated ZIP time fields cleanup | Small |

---

## Recurring Patterns

### 1. `utils.Fatalf` (os.Exit) used as error handling from library code
The most systemic issue. `utils.Fatalf` / `utils.Fatalln` call `os.Exit(1)`, which skips all deferred cleanup. This pattern appears in goroutines (`processor.go:74,84`, `loader.go:315,327,333`), in `main.go`, and in `converter.Fatal`. Every usage inside a goroutine is a resource leak. Every usage after opening a file/ZIP is an FD leak. **Root cause**: no error-propagation path from workers back to the caller; `os.Exit` in library code (`internal/pkg`) makes the code untestable. **Fix**: thread errors via channels; only `main()` should exit.

### 2. `text/template` used where `html/template` or `etree` is needed
The codebase is inconsistent: `content.opf` and `toc.ncx` use `etree` (proper XML escaping), but XHTML page templates use `text/template` (no escaping). This inconsistency is the source of the XML injection finding. **Fix**: standardize on `html/template` for all XML/HTML output.

### 3. Duplicated logic across processor and passthrough
~80% of the loader code is duplicated. `isSupportedImage`, `loadDir`, `loadCbz`, `loadCbr` are near-identical with subtle divergence (`.webp`/`.tiff` support). Bugs must be fixed twice and feature gaps emerge silently. The worker goroutine pattern is also repeated across `loadDir`, `loadCbz`, `loadCbr`. **Fix**: shared loader package (Phase 3).

### 4. No decode limits or input validation at boundaries
The pipeline trusts input images completely — no dimension check, no file size check, no page count limit for PDFs, no zero-dimension guard in passthrough. The code doesn't defensively validate external input at trust boundaries. **Fix**: dimension pre-check, optional file-size limit, PDF page-count limit, zero-dimension guard.

### 5. Deferred cleanup that doesn't run on error/panic
`defer imgStorage.Close()`, `defer imgStorage.Remove()`, `defer r.Close()` — these are registered but never execute when `os.Exit` or an unrecovered panic fires. The `defer` in `epub.go:Write()` (`imgStorage.Remove()`) only runs on the success path. **Fix**: register defers before any error-producing code; use error channels instead of os.Exit; add panic recovery.

### 6. Package-level state and re-computation
`regexp.MustCompile` in `Validate()` and `render()`, template parsing per-page, `truetype.Parse` per CoverTitle call, `cover16LevelOfGray` recreated per call — repeated work that should be done once at package init or struct construction. **Fix**: package-level vars or struct fields initialized once.

### 7. `String()` methods with side effects
`Content.String()` mutates `o.Images[i]` via `getSpineAuto` — a `String()` method that modifies its input is surprising and violates the principle that `String()` should be read-only. **Fix**: compute `Position` in an explicit pass before rendering.

---

## Test Strategy

### Priority 1: Pure functions (high ROI, easy to test)

**`internal/pkg/sortpath`**
- Test `By()` with all 3 modes against known input sequences
- Edge cases: mixed alpha/numeric, leading zeros, multi-digit, Unicode, decimal-only, negative, range filenames (`s2-3` vs `s2-4`)
- Property test: sorted output is stable for equal keys
- **Bug to cover**: range portion (`r[3]`) captured but unused — `s2-3` and `s2-4` should sort differently

**`internal/pkg/utils`**
- Expand beyond Example tests: `NumberOfDigits(0)`, `FormatNumberOfDigits` with multi-digit totals, `IntToString`/`FloatToString` edge cases
- `BoolToString` has no test — add one

**`internal/pkg/converter.Validate`**
- Table-driven test: valid inputs, out-of-range brightness/contrast, invalid profile, invalid color format (uppercase + lowercase), invalid format enum, invalid sort mode, invalid title page, missing input, output path derivation
- Boundary: `LimitMb = 0` (unlimited), `LimitMb = 19` (reject), `LimitMb = 20` (accept)
- `Workers = 0` behavior

**`internal/pkg/converter/profiles`**
- Test that all 26 profiles are present with valid dimensions
- Test `Profiles.String()` output is **sorted** (after fix #29)

### Priority 2: XML generation correctness

**`internal/pkg/epubtemplates`**
- Test `Content.String()` produces valid XML (parse with `etree.ReadFromString`, assert structure)
- **Injection test**: pass Title/Author containing `</title>`, `<script>`, `&`, `<`, `"` — assert output is escaped and parseable
- Test `Toc()` with nested directories, strip-first-directory, single image
- **Bug to cover**: empty `img.Path` (PDF sources) — assert images appear in TOC (after fix #5)
- Test `getSpineAuto` does not mutate input slice (after fix #14)
- Test XHTML templates with `html/template` (after fix #3): assert `.Title` is escaped in `<title>` and `alt=`

### Priority 3: Image filter correctness

**`internal/pkg/epubimagefilters`**
- `AutoCrop`: test with known margins (solid border image → expected crop bounds), test `skipIfLimitReached` behavior, test all-blank image, test partially-blank
- `AutoContrast`: test with bimodal histogram → expected contrast stretch, test all-same-value image, test empty image
- `CropSplitDoublePage`: test even-width, odd-width, narrow image (not double page)
- `Pixel`: test 0×0 image → 1×1 output, test normal image → unchanged
- `CoverTitle`: test text fitting, alignment, empty text, text overflow at min font size

### Priority 4: EPUB image and sizing

**`internal/pkg/epubimage`**
- Test `RelSize` with zero dimensions, square images, portrait, landscape
- Test `ImgStyle` CSS generation for all `Position` values
- Test path/key generation for correctness

**`pkg/epub`**
- Test `computeAspectRatio` with mixed aspect ratios (most common wins)
- Test `computeViewPort` with `AspectRatio = -1` (keep device), `= 0` (auto), `> 0` (explicit)
- Test `getParts` size splitting: single part (under limit), multi-part (over limit), edge case (image larger than limit)

### Priority 5: Integration / E2E

**`pkg/epub.Write()`**
- Create a small test CBZ (3-4 small JPEGs in `t.TempDir()`), convert, verify output EPUB:
  - `unzip -l` shows expected file list (mimetype first, uncompressed)
  - `mimetype` content is `application/epub+zip`
  - `content.opf` is valid XML with correct metadata
  - `toc.ncx` is valid XML
  - Page XHTML files are valid XML
  - EPUB passes `epubcheck` (if available)
- Test multi-part splitting: create images exceeding `LimitMb`, verify multiple output files
- Test dry-run mode: no files written, TOC printed
- Test passthrough mode: raw images copied, no re-encoding
- Test PDF input: verify TOC is not empty (after fix #5)

### Priority 6: Error paths

- Corrupt image in CBZ → placeholder generated, warning printed, non-zero exit (after fix #16)
- Oversized image (decompression bomb) → rejected with clear error (after fix #4)
- Missing input directory → clear error
- Unwritable output → clear error
- Temp file from crashed run → detected/warned (after fix #10)
- Offline `version()` → local version printed (after fix #12)

### Test infrastructure
- Use `testdata/` directories with small fixture images (e.g. 10×10 JPEG/PNG)
- Use `t.TempDir()` for all file operations
- No external dependencies in tests (mock `go-latest` if testing `version()`)
- Add `go test ./...` to CI
- Add `go test -race` to CI (would catch finding #1 immediately)

---

## Dependencies

### Concerns

| Dependency | Version | Concern | Recommendation |
|---|---|---|---|
| `github.com/tcnksm/go-latest` | `v0.0.0-20170313` | Unmaintained since 2017. Pulls `google/go-github v17` (current v60+), `hashicorp/go-version`, `go-querystring`. Heavy transitive deps for a simple "check latest GitHub tag" feature. No timeout/context support. | Replace with direct `net/http` call to `https://api.github.com/repos/celogeek/go-comic-converter/tags`. ~20 lines of code, zero dependencies. Also fixes the offline `version()` crash. |
| `github.com/raff/pdfreader` | `v0.0.0-20220308` | Unmaintained, no module support beyond pseudo-version. No context/cancellation. Niche library. | Monitor for forks. If PDF support is rarely used, consider documenting as experimental. |
| `github.com/golang/freetype` | `v0.0.0-20170609` | **Archived** (last commit 2017). Only used for `truetype.Parse`/`NewFace` in `cover_title.go`. | Replace with `golang.org/x/image/font/sfnt` (already a transitive dep via `x/image`). Drop `freetype` direct dependency. |
| `github.com/fogleman/gg` | `v1.3.0` | Used for 2D drawing in `CoverTitleData` and `corruptedImage`. Reasonably maintained but pulls in `freetype` transitively. | Keep for now; evaluate if `gg` usage can be replaced with `image/draw` + `sfnt` to shed both `gg` and `freetype`. |
| `github.com/gofrs/uuid` | `v4.4.0+incompatible` | `+incompatible` suffix — v4 without module path. v5 exists with proper path. | Upgrade to `github.com/gofrs/uuid/v5`. Trivial import change. |
| `github.com/disintegration/gift` | `v1.2.1` | Last release 2022. Stable but not actively developed. Core to image pipeline. | No action needed; well-designed and stable. Monitor for security advisories. |
| `github.com/beevik/etree` | `v1.5.0` | Stable, maintained. Used for XML generation (safe path). | No action needed. |
| `github.com/schollz/progressbar/v3` | `v3.18.0` | Actively maintained. | No action needed. |
| `github.com/nwaples/rardecode/v2` | `v2.1.0` | Actively maintained. | No action needed. |
| `gopkg.in/yaml.v3` | `v3.0.1` | Maintained. Known CVE in `yaml.v2` (not v3). | No action needed. |
| `golang.org/x/image` | `v0.24.0` | Maintained. Provides TIFF/WebP decoders, Go fonts. | No action needed. |

### Summary
The dependency tree is small and mostly stable. The main concerns are:
1. **`go-latest`** — 2017, pulls ancient `go-github v17` for a feature that could be 20 lines of `net/http`. Replacing it also fixes the offline `version()` crash (finding #12).
2. **`golang/freetype`** — archived, replaceable with `x/image/font/sfnt` (already a transitive dep).
3. **`gofrs/uuid` +incompatible** — cosmetic, trivial upgrade.
4. No critical CVEs in current dependency versions.
