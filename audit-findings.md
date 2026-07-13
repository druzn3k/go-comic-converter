# go-comic-converter v3 — Comprehensive Code Audit

**Date:** 2026-07-13  
**Scope:** Every Go source file in the project  
**Module:** github.com/celogeek/go-comic-converter/v3 (Go 1.23)

---

## Table of Contents

1. [main.go](#1-maingo)
2. [pkg/epub/epub.go](#2-pkgepubepubgo)
3. [internal/pkg/converter/converter.go](#3-internalpkgconverterconvertergo)
4. [internal/pkg/converter/options.go](#4-internalpkgconverteroptionsgo)
5. [internal/pkg/epubimageprocessor/processor.go](#5-internalpkgepubimageprocessorprocessorgo)
6. [internal/pkg/epubimageprocessor/loader.go](#6-internalpkgepubimageprocessorloadergo)
7. [internal/pkg/epubimagepassthrough/passthrough.go](#7-internalpkgepubimagepassthroughpassthroughgo)
8. [internal/pkg/epubimagefilters/*.go](#8-internalpkgepubimagefiltersgo)
9. [internal/pkg/epubzip/*.go](#9-internalpkgepubzipgo)
10. [internal/pkg/epubtemplates/content.go](#10-internalpkgepubtemplatescontentgo)
11. [internal/pkg/epubtemplates/toc.go](#11-internalpkgepubtemplatestocgo)
12. [internal/pkg/sortpath/*.go](#12-internalpkgsortpathgo)
13. [internal/pkg/epubtree/epub_tree.go](#13-internalpkgepubtreeepub_treego)
14. [internal/pkg/epubprogress/*.go](#14-internalpkgepubprogressgo)
15. [internal/pkg/utils/utils.go](#15-internalpkgutilsgo)
16. [pkg/epuboptions/*.go](#16-pkgepuboptionsgo)
17. [internal/pkg/epubimage/epub_image.go](#17-internalpkgepubimageepub_imagego)
18. [internal/pkg/converter/profiles.go](#18-internalpkgconverterprofilesgo)
19. [internal/pkg/converter/order.go](#19-internalpkgconverterordergo)
20. [Cross-Cutting Concerns](#20-cross-cutting-concerns)

---

## 1. main.go

### Code Quality
- **Clean dispatch pattern**: `switch` on mode flags (version/save/show/reset/generate) is readable and maintainable.
- **JSON encode error swallowed** (line 89): `_ = json.NewEncoder(os.Stdout).Encode(...)` silently discards errors. If stdout is closed or a pipe breaks, the user gets no feedback.

### Bugs / Edge Cases
- **`version()` crashes when offline** (lines 42–44): If `githubTag.Fetch()` fails (no network), `utils.Fatalln("failed to fetch the latest version")` exits before printing the **local** version. The user asked for `-version` and gets nothing. Should print local version first, then attempt remote fetch.
- **`latestVersion.Segments()[0]`** (line 57): If `v.Versions[0]` is somehow malformed, `Segments()` could return an empty slice and panic. Guarded by `len(v.Versions) < 1` check at line 46, but `Segments()` itself could return empty for a zero-version tag.

### Concurrency
- N/A — single-threaded entry point.

### Security
- No direct security issues. The version check contacts GitHub API but doesn't send user data.

### Performance
- Network call on every `-version` invocation. Could cache the result or make it optional.

### Over/Under-Engineering
- **Under-engineering**: No `context.Context` support — the GitHub fetch has no timeout. If the network hangs, `-version` hangs indefinitely.

### Missing Tests
- No tests for `version()`, `save()`, `show()`, `reset()`, `generate()`.

### API Design
- `generate(cmd)` applies profile dimensions (lines 81–84) but only `Width`/`Height` — the `AspectRatio` is not set from the profile. This is by design (AspectRatio has its own logic), but it's non-obvious.

### Dependencies
- **`github.com/tcnksm/go-latest`** — last commit 2017, effectively unmaintained. Pulls in `google/go-github v17` (ancient) and `hashicorp/go-version` as indirect deps. Should be replaced with a direct `go-github` call or removed.

### TODO/FIXME/HACK
- None.

---

## 2. pkg/epub/epub.go

### Code Quality
- **Regex compiled on every `render` call** (line 87): `regexp.MustCompile("\n+")` is called for every image page, blank page, cover, and title. Should be a package-level `var`.
  ```go
  // line 87 — called on every render
  return regexp.MustCompile("\n+").ReplaceAllString(result.String(), "\n")
  ```
- **`writeImage` uses `err == nil` pattern** (lines 93–106): 
  ```go
  err := wz.WriteContent(...)
  if err == nil {
      err = wz.Copy(zipImg)
  }
  return err
  ```
  Idiomatic Go would use `if err != nil { return err }` then `return wz.Copy(zipImg)`.
- **Duplicate `// write title image` comment** on both `writeCoverImage` (line 109) and `writeTitleImage` (line 137) — copy-paste error.
- **Misspelling**: `"Go Comic Convertor"` in content.go line 108 (used by this orchestrator) — should be "Converter".

### Bugs / Edge Cases
- **`getParts` panic on empty images** (line 184): `cover := images[0]` — if `Load()` returns an empty slice (guarded by `errNoImagesFound` upstream, but not defensively checked here).
- **`writePart` panic on empty Images** (line 458): `lastImage := part.Images[len(part.Images)-1]` — panics if `Images` is empty. Guarded by the part-splitting logic (only non-empty parts are created), but not defensively coded.
- **`computeViewPort` division by zero** (line 326): `int(float64(e.Image.View.Height)/bestAspectRatio)` — if `bestAspectRatio` is 0 (from `computeAspectRatio` returning 0 when all images have 0 aspect ratio), this panics. Guarded in practice (corrupted images get 1200×1920), but not defensively.
- **`render` panics on template execute error** (line 85): `panic(err)` — crashes the program instead of returning an error. Since templates are embedded constants, parse errors won't happen, but execute errors could with malformed data.
- **Dry mode with single image + HasCover**: If input has exactly 1 image and `HasCover` is true, `images = images[1:]` makes `images` empty. In Dry mode, `epubParts[0]` has empty `Images`. The `Write()` Dry path (lines 398–412) calls `getTree(p.Images, true)` which handles empty fine. But the non-Dry path won't create empty parts (guarded by `len(currentImages) > 0` at line 231). So this is safe but fragile.

### Concurrency
- No direct concurrency in this file — delegates to `imageProcessor.Load()` which has its own goroutine fan-out.

### Security
- **XSS / XML injection in template rendering**: `render` uses `text/template` which does **NOT** auto-escape HTML entities (that's `html/template`). The `Title` field (user-controlled) is inserted into XHTML via `{{ .Title }}` in `text.xhtml.tmpl`. A title containing `<`, `>`, or `&` would produce malformed XHTML. Severity is low (local tool, EPUB readers don't execute JS), but the EPUB would be invalid XML.
  ```go
  // line 82-86 — text/template does NOT escape HTML
  tmpl := template.Must(e.templateProcessor.Parse(templateString))
  if err := tmpl.Execute(&result, data); err != nil {
      panic(err)
  }
  ```
- **Output path overwrite**: `epubzip.New(path)` calls `os.Create(path)` which silently overwrites existing files. No check for existing files or symlinks.

### Performance
- **Regex compilation per render call** (line 87) — see Code Quality above.
- **`getParts` iterates all images** to compute part sizes — O(n), unavoidable.
- **`computeAspectRatio` builds a map of all aspect ratios** (line 301–316) — O(n) with map allocation. Fine for typical comic sizes (100–500 images).

### Over/Under-Engineering
- **Under-engineering**: No `context.Context` support — cannot cancel a long-running EPUB generation.
- **Under-engineering**: `Write()` doesn't clean up partial EPUB files if an error occurs mid-generation. If `writePart` fails for part 2 of 3, parts 1 and 2 are left on disk.

### Missing Tests
- No tests for `getParts`, `computeAspectRatio`, `computeViewPort`, `writePart`, `getTree`, `Write`.

### API Design
- **`EPUB` interface has only `Write() error`** — no way to get progress, cancel, or inspect the result. Minimal but functional.
- **`epubPart` is unexported** — users can't inspect parts. Fine for current use.

### Dependencies
- `github.com/gofrs/uuid` — used for UID generation. `uuid.Must(uuid.NewV4())` panics if crypto/rand fails (extremely unlikely).

### TODO/FIXME/HACK
- None.

---

## 3. internal/pkg/converter/converter.go

### Code Quality
- **Regex compiled inside `Validate`** (line 319): `regexp.MustCompile("^[0-9A-F]{3}$")` is compiled on every `Validate()` call. Should be package-level.
- **`isZeroValue` copied from flag package** (lines 241–267): Comment says "Taken from flag package as it is private." This is fragile — if the flag package's internals change, this breaks. Acceptable but should be noted.
- **`Parse()` shortcut logic is order-dependent** (lines 195–218): `Auto` sets flags, then `NoFilter` unsets them, then `AppleBookCompatibility` overrides, then `PortraitOnly` overrides. The order is not documented and could confuse users who pass multiple shortcuts.

### Bugs / Edge Cases
- **Color regex is case-sensitive** (line 319): `^[0-9A-F]{3}$` only accepts uppercase hex. If a user passes `--foreground-color fff`, validation fails. The CSS template uses the value directly (`#{{ .View.Color.Foreground }}`), so lowercase would work in CSS. The validation is overly strict.
- **`Validate` doesn't check `Workers`** — if `--workers 0` is passed, `WorkersRatio(50)` returns 1 (clamped at line 56 of epub_options.go), but the user's intent is silently overridden.
- **`Validate` doesn't check `GrayScaleMode` when `GrayScale` is false** — a user could set `--grayscale-mode 5` and it would fail validation even if grayscale is off. Minor, but the validation is unconditional.
- **`Stats()` reads `runtime.ReadMemStats`** (line 429) — this is a STW (stop-the-world) operation. Fine for a CLI tool but would be problematic in a server context.

### Concurrency
- N/A — single-threaded parsing.

### Security
- **Input path is user-controlled** but validated with `os.Stat` (line 307). No path traversal risk since it's only used for reading.
- **Output path is user-controlled** — `os.Create` will overwrite existing files. No symlink check.

### Performance
- Regex compilation in `Validate` (line 319) — see above.
- `reflect` usage in `isZeroValue` (line 247) — only called during `-help`, so acceptable.

### Over/Under-Engineering
- **`order` interface** (order.go) — simple 2-type interface for flag ordering. Reasonable.
- **`Converter` struct tracks `startAt`** (line 37) — used only for `Stats()`. Fine.

### Missing Tests
- No tests for `Parse()`, `Validate()`, `Usage()`, `Stats()`, `isZeroValue()`.

### API Design
- **`Fatal(err)` calls `os.Exit(1)`** via `utils.Fatalf` — makes the converter untestable in-process.
- **`Cmd` field is exported** but is a `*flag.FlagSet` — exposing implementation detail.

### Dependencies
- Only stdlib (`flag`, `reflect`, `regexp`, `runtime`, `slices`).

### TODO/FIXME/HACK
- None.

---

## 4. internal/pkg/converter/options.go

### Code Quality
- **`FileName()` ignores error** (line 76): `home, _ := os.UserHomeDir()` — if `HOME` is unset, `home` is empty, and the config file path becomes `.go-comic-converter.yaml` in the current directory. This is a **silent fallback** that could confuse users.
  ```go
  func (o *Options) FileName() string {
      home, _ := os.UserHomeDir()  // error ignored
      return filepath.Join(home, ".go-comic-converter.yaml")
  }
  ```
- **`ShowConfig()` is a large method** (lines 91–187) with a complex struct literal and many conditionals. Hard to maintain. Could be refactored into a table-driven approach with helper methods.
- **`ResetConfig()` creates a new `NewOptions()`** (line 192) — if `NewOptions()` defaults change, old config files won't have the new defaults until reset. This is expected behavior but not documented.

### Bugs / Edge Cases
- **`LoadConfig` silently ignores missing file** (line 84): `if err != nil { return nil }` — if the config file doesn't exist, no error is returned. This is intentional (first run), but if the file exists but is unreadable (permissions), the error is also swallowed.
  ```go
  f, err := os.Open(o.FileName())
  if err != nil {
      return nil  // swallows permission errors too
  }
  ```
- **`LoadConfig` EOF check** (line 91): `if err != nil && err.Error() != "EOF"` — comparing error messages by string is fragile. `yaml.v3` returns `io.EOF` for empty files, so this should be `err != io.EOF`. Actually, `yaml.Decoder.Decode` returns `io.EOF` for empty input, so `err.Error() != "EOF"` works because `io.EOF.Error() == "EOF"`. But this is brittle.
- **`SaveConfig` creates file with default permissions** (line 202): `os.Create` uses `0666` (modified by umask). The config file may contain user preferences but no secrets. Still, `0600` would be more appropriate.

### Concurrency
- N/A — config is loaded once at startup.

### Security
- **Config file permissions**: `os.Create` uses `0666 & ~umask`. Should use `0600` to prevent other users from reading/writing the config.
- **YAML deserialization**: `yaml.v3` is safe against code execution (no custom types). Extra keys are ignored. No vulnerability.

### Performance
- N/A — called once at startup.

### Over/Under-Engineering
- **`ShowConfig()` conditionals** — the `Condition` field in the struct literal makes display logic declarative. Good approach but the method is still very long.

### Missing Tests
- No tests for `LoadConfig`, `SaveConfig`, `ResetConfig`, `ShowConfig`, `GetProfile`, `AvailableProfiles`.

### API Design
- **`FileName()` returns a path but doesn't validate it** — if home dir is empty, returns a relative path silently.
- **`GetProfile()` returns `*Profile`** but the map stores `Profile` by value — taking a pointer to a map value. If the map is modified, the pointer could dangle. In practice, the map is never modified after construction, so this is safe but fragile.

### Dependencies
- `gopkg.in/yaml.v3 v3.0.1` — fixed version with CVE-2022-28948 patch. OK.

### TODO/FIXME/HACK
- None.

---

## 5. internal/pkg/epubimageprocessor/processor.go

### Code Quality
- **Exported interface `EPUBImageProcessor` but unexported struct `ePUBImageProcessor`** — the struct name uses inconsistent casing (`ePUB` vs `EPUB`). Should be `epubImageProcessor` for idiomatic Go.
- **`CoverTitleData` method** (lines 318–334) — mixes image processing with cover/title generation. The `EPUBImageProcessor` interface includes `CoverTitleData` which is only used by the EPUB orchestrator, not part of the "load and process" concern.

### Bugs / Edge Cases
- **Race condition on `err` named return** (lines 72, 82): Multiple goroutines write to the named return variable `err` without synchronization:
  ```go
  func (e ePUBImageProcessor) Load() (images []epubimage.EPUBImage, err error) {
      ...
      go func() {
          for input := range imageInput {
              ...
              if err = imgStorage.Add(...); err != nil {  // line 72 — RACE
                  _ = bar.Close()
                  utils.Fatalf(...)
              }
              ...
              if err = imgStorage.Add(...); err != nil {  // line 82 — RACE
                  _ = bar.Close()
                  utils.Fatalf(...)
              }
          }
      }()
  ```
  Multiple goroutines read/write `err` concurrently. The Go race detector would flag this. While `utils.Fatalf` calls `os.Exit(1)` which prevents the race from mattering in practice, the behavior is **undefined** per the Go memory model.
- **`bar.Close()` called from worker goroutines** (lines 73, 83) — `EPUBProgress` implementations may not be thread-safe. `progressbar.ProgressBar.Close()` from multiple goroutines could cause issues. The `bar.Close()` at line 99 (main goroutine) would be a double-close if a worker already closed it.
- **`imgStorage.Add` failure handling**: `utils.Fatalf` calls `os.Exit(1)`, which kills the process without closing `imgStorage`, the input reader, or the progress bar properly. Deferred cleanup is skipped.
- **`transformImage` doesn't handle nil `input.Image`**: If `input.Image` is nil (shouldn't happen since corrupted images get a replacement), `src.Bounds()` would panic. The loader always sets a replacement image, so this is guarded upstream but not defensively.

### Concurrency
- **RACE on `err`** — see Bugs above.
- **RACE on `bar.Close()`** — see Bugs above.
- **`imgStorage` is thread-safe**: Uses `sync.Mutex` (storage_image_writer.go line 27). Correct.
- **`imageOutput` channel**: Buffered? No — `imageOutput := make(chan epubimage.EPUBImage)` (line 52) is **unbuffered**. Workers block on send until the collector receives. This is fine (provides backpressure) but means workers can't process the next image until the collector consumes the current one. The `imageInput` channel is also unbuffered in most loaders, so the pipeline is fully backpressured.

### Security
- No direct security issues. Image decoding uses stdlib which is generally safe.

### Performance
- **`WorkersRatio(wr)`** (line 66): For JPEG, `wr=50` → half the workers. For PNG, `wr=100` → all workers. This is because PNG encoding is single-threaded and CPU-intensive, so more parallelism helps. JPEG encoding is faster. Reasonable heuristic.
- **`createImage` type switch** (lines 144–167): Allocates a new image of the same type as the source. For grayscale, always `image.NewGray`. This avoids color space conversions. Good.
- **`cover16LevelOfGray`** (lines 301–317): Creates a 16-color palette for cover images. The palette is recreated on every call. Could be a package-level var.

### Over/Under-Engineering
- **`createImage` handles 10 image types** — comprehensive but could use a simpler default. The type switch is necessary for efficient processing.
- **Under-engineering**: No way to limit memory usage — all processed images are held in temp ZIP storage, but the temp ZIP grows unbounded.

### Missing Tests
- No tests for `Load`, `transformImage`, `createImage`, `CoverTitleData`.
- **Critical untested logic**: `transformImage` — the entire image pipeline (crop, rotate, contrast, brightness, grayscale, resize) is untested.

### API Design
- **`CoverTitleData` in `EPUBImageProcessor` interface** — mixes concerns. The interface should be split into `Loader` and `CoverRenderer`.
- **`CoverTitleDataOptions` has `Src image.Image`** — the caller must provide the source image, which means the caller needs access to the decoded cover. This couples the EPUB orchestrator to the image processor internals.

### Dependencies
- `github.com/disintegration/gift v1.2.1` — image filtering. Last release 2022, still maintained but low activity.
- `github.com/fogleman/gg v1.3.0` — 2D rendering for corrupted image placeholder. Depends on `github.com/golang/freetype` (abandoned).
- `github.com/golang/freetype` — font rendering. Abandoned (last commit 2017).

### TODO/FIXME/HACK
- None.

---

## 6. internal/pkg/epubimageprocessor/loader.go

### Code Quality
- **`corruptedImage` ignores parse error** (line 95): `f, _ := truetype.Parse(gomonobold.TTF)` — the font parse error is ignored. If it fails, `f` is nil and `truetype.NewFace(f, ...)` would panic. The font is embedded, so parse failure is extremely unlikely, but the error should be handled.
- **`loadDir` path extraction** (lines 128–132): `p = p[len(input)+1:]` — if `input` doesn't have a trailing separator and `p` equals `input + "/"`, this works. But if `filepath.Clean` removes the trailing separator, `p` might be shorter than expected. Actually, `filepath.Split` returns `p` with a trailing separator, and `input = filepath.Clean(e.Input)` removes it. So `p = input + "/"` → `p[len(input)+1:]` strips the prefix correctly. If `p == input` (file in root), `p = ""`. This is correct.
- **`loadCbz` and `loadCbr` share near-identical worker code** — significant duplication. The worker goroutine logic (decode image, split path, send task) is duplicated across `loadDir`, `loadCbz`, and `loadCbr`. Could be extracted into a shared function.

### Bugs / Edge Cases
- **`loadCbr` solid mode calls `utils.Fatalf` from goroutine** (lines 315, 322, 327): `os.Exit(1)` from within a goroutine skips all deferred cleanup (RAR reader, channels, wait groups). This is a resource leak on error. Should return errors instead.
  ```go
  // line 315 — inside a goroutine, calls os.Exit
  utils.Fatalf("\nerror processing image %s: %s\n", e.Input, rerr)
  ```
- **`loadCbr` solid mode error on `f.Name`** (line 322): `utils.Fatalf("... %s ...", f.Name, rerr)` — if `r.Next()` returns an error, `f` might be nil, causing `f.Name` to panic. The error from `r.Next()` should be checked before accessing `f`.
  ```go
  f, rerr := r.Next()
  if rerr != nil {
      if rerr == io.EOF { break }
      utils.Fatalf("... %s ...", f.Name, rerr)  // f might be nil!
  }
  ```
  **This is a potential nil pointer dereference bug.**
- **`loadPdf` is single-threaded** (lines 403–432): PDF extraction runs in a single goroutine with no parallelism. For large PDFs, this is a performance bottleneck.
- **`loadPdf` doesn't sort pages** — pages are extracted in order `1..N`, so sorting isn't needed. But `SortPathMode` is ignored for PDFs. This is by design (PDF pages are inherently ordered) but not documented.
- **`loadCbz` close race**: `r.Close()` is deferred in the goroutine after `wg.Wait()` + `close(output)` (line 188). Workers access `job.F` (which is a `*zip.File` from the reader). If `r.Close()` runs before all workers finish... but `wg.Wait()` ensures all workers are done first. Safe.
- **`isSupportedImage`** (line 27): Checks file extension but not file content. A file named `image.jpg` that contains a PNG would be decoded by `image.Decode` (which auto-detects format), so this is fine.

### Concurrency
- **Goroutine leak on error in `loadCbr`**: `utils.Fatalf` → `os.Exit(1)` kills the process, leaking the RAR reader and all goroutines.
- **Channel buffering**: `output = make(chan task, e.Workers)` — buffered to `e.Workers`. With `e.WorkersRatio(50)` workers (half of Workers), the buffer is large enough. Good.
- **`loadDir` jobs channel is unbuffered** (line 118): `jobs := make(chan job)` — the producer goroutine blocks until a worker consumes. This provides backpressure but means the producer can't queue jobs ahead. For large directories, this is fine (workers are fast to consume job structs).

### Security
- **Zip slip (mitigated)**: `loadCbz` uses `job.F.Name` from the ZIP archive. `filepath.Clean` and `filepath.Split` sanitize the path. The image data is decoded (not extracted to filesystem), so no file system traversal. The path ends up in the EPUB TOC but is cleaned. **However**, a malicious CBZ with paths like `../../etc/passwd.jpg` would be cleaned to `etc/passwd.jpg` — the `..` is removed but the path is still used in the EPUB structure. Not a security issue per se, but could create unexpected directory structures in the EPUB.
- **RAR path handling**: Same as CBZ — paths are cleaned and used for display only.

### Performance
- **`loadPdf` single-threaded** — see Bugs above.
- **`loadCbr` solid mode reads entire file into memory** (line 320): `io.Copy(&b, r)` reads the entire image into a `bytes.Buffer` before sending to workers. For large images in solid RARs, this could use significant memory.
- **Image decoding is parallel** in `loadDir`, `loadCbz`, `loadCbr` — good.

### Over/Under-Engineering
- **Duplicate worker code** across 3 loaders — should be extracted.
- **`loadPdf` doesn't use the worker pattern** — could use a pool for parallel extraction if `pdfimage.Extract` is thread-safe. Unknown if it is.

### Missing Tests
- No tests for any loader function.
- **Critical untested logic**: `isSupportedImage`, path extraction, corrupted image handling, solid RAR handling.

### API Design
- **`load()` returns `(totalImages int, output chan task, err error)`** — returning a channel is unusual. The caller must consume the channel until closed. This is a valid pattern but makes error handling tricky (error is returned upfront, but per-task errors come through the channel).

### Dependencies
- `github.com/nwaples/rardecode/v2 v2.1.0` — RAR decoding. Actively maintained.
- `github.com/raff/pdfreader v0.0.0` — PDF reading. Minimal maintenance, niche library.

### TODO/FIXME/HACK
- None.

---

## 7. internal/pkg/epubimagepassthrough/passthrough.go

### Code Quality
- **Massive code duplication with `epubimageprocessor`**: `loadDir`, `loadCbz`, `loadCbr` are near-identical to the processor versions but without image decoding. ~430 lines of duplicated logic. Should share a common loader framework.
- **`CoverTitleData` delegates to full processor** (line 37): `return epubimageprocessor.New(e.EPUBOptions).CoverTitleData(o)` — creates a new processor instance just for one method. Wasteful but functional.
- **`isSupportedImage` only accepts jpg/png** (lines 368–376) — doesn't accept `.tiff` or `.webp`, unlike the processor version (which accepts `.tiff` and `.webp`). This is intentional (passthrough can only copy, not convert), but the inconsistency is not documented.

### Bugs / Edge Cases
- **`copyRawDataToStorage` division by zero** (line 404): `OriginalAspectRatio: float64(config.Height) / float64(config.Width)` — if `config.Width` is 0, panics. A valid PNG/JPEG always has non-zero dimensions, and `decodeConfig` returns an error for invalid images. But a crafted file could have 0×0 dimensions in its header. Should check.
- **`loadCbr` non-solid mode**: `file.Open()` is called in the closure (line 350) — if the file is encrypted or requires a password, `Open()` returns an error, which is handled by `copyRawDataToStorage` returning the error. But the error propagates up and stops all processing. No error recovery.
- **`loadCbr` solid mode**: `io.ReadAll(r)` reads the entire image into memory. For large images, this is memory-intensive. Same issue as processor version.
- **No PDF support**: `Load()` returns an error for `.pdf` files (line 31). Passthrough mode doesn't support PDF, which is reasonable (PDF images need extraction), but the error message doesn't mention this: `"unknown file format (.pdf): support .cbz, .zip, .cbr, .rar"`. Should explicitly say "copy mode doesn't support PDF".

### Concurrency
- **No parallelism**: All passthrough loaders are single-threaded. The processor loaders use goroutine fan-out, but passthrough doesn't. For large CBZ/CBR files, this is a performance bottleneck. File I/O is the bottleneck, and parallel I/O could help with SSDs.

### Security
- Same zip/path concerns as processor (mitigated).

### Performance
- **Single-threaded** — see Concurrency above.
- **No image decoding** (except for cover) — fast for non-cover images. Only `decodeConfig` is called for dimensions. Efficient.
- **`io.ReadAll`** reads entire file into memory — for large images (e.g., 50MB TIFF), this uses 50MB per image. Since passthrough only accepts jpg/png, sizes are typically smaller, but still.

### Over/Under-Engineering
- **Under-engineering**: No parallelism where there easily could be.
- **Massive duplication** — see Code Quality.

### Missing Tests
- No tests for any passthrough function.

### API Design
- **Same `EPUBImageProcessor` interface** — good, allows swapping.
- **`CoverTitleData` delegates to processor** — couples passthrough to processor package. If processor changes, passthrough breaks.

### Dependencies
- Same as processor (shares imports for `CoverTitleData` delegation).

### TODO/FIXME/HACK
- None.

---

## 8. internal/pkg/epubimagefilters/*.go

### auto_contrast.go

#### Code Quality
- **Map for histogram** (line 24): `bucket := map[int]int{}` — for up to 65536 entries, a map is slow. Should use `[65536]int` array:
  ```go
  // line 24 — SLOW: map for 65K entries
  bucket := map[int]int{}
  ```
- **Full iteration for mean** (line 31–36): `for colorIdx := range 1 << 16` iterates 65536 times even if the image has few unique colors. Could sort the map keys (fewer entries) and iterate those.

#### Bugs / Edge Cases
- **`mean` converts every pixel to gray** (line 22): `color.GrayModel.Convert(src.At(x, y))` — for color images, this converts to gray for the histogram. The contrast adjustment is then applied to color channels. This is a design choice (luminance-based contrast), not a bug.
- **Empty image**: If the image has 0 pixels (0×0), `limit = 0` and the loop `for colorIdx := range 1 << 16` would never break (all buckets are 0, `limit - 0` never goes negative). `colorIdx` would reach 65536, and `float32(65536) / 65536 = 1.0`. This is handled (returns 1.0 for empty images), but the 65536-iteration loop is wasteful.

#### Performance
- **O(W×H + 65536) per image** — for a 2400×3840 image: 9.2M pixel reads + 65K iterations. The map allocation and GC pressure are significant. Using a fixed-size array would eliminate the map overhead.

#### Missing Tests
- No tests for `mean`, `cap`, `pow2`, `Draw`.

### auto_crop.go

#### Code Quality
- **Labeled break statements** (`LEFT:`, `UP:`, `RIGHT:`, `BOTTOM:`) — unusual in Go but valid and readable for this use case.
- **`colorIsBlank` threshold hardcoded** (line 27): `g.Y >= 0xe0` (224) — no way to configure the "blank" threshold. Should be an option.

#### Bugs / Edge Cases
- **`findMargin` mutates `imgArea`** — `imgArea.Min.X++` etc. The `allowNonBlank` counter is recalculated per column/row based on the current `imgArea.Dy()` or `imgArea.Dx()`. Since `imgArea` shrinks as margins are found, the `allowNonBlank` count for later sides is based on the already-cropped dimensions. This is intentional (progressive cropping) but could lead to different results depending on the order of LEFT/UP/RIGHT/BOTTOM.
- **`correctLine` underflow** (line 80): `min -= exceed / 2` — if `exceed` is odd, `exceed / 2` truncates, and `min` might not return to `bMin`. The `if min < bMin` check catches this. OK.
- **Blank image detection**: If the entire image is blank, `imgArea` shrinks to 0×0. `findMargin` returns this 0×0 rectangle. The caller (processor.go) checks `size.Dx() == 0 && size.Dy() == 0` for blank detection. Correct.

#### Performance
- **O(W×H) per side** worst case — for each column, iterates all rows. For a 2400×3840 image, LEFT side: 2400 × 3840 = 9.2M pixel reads. Four sides: ~37M pixel reads. Could be optimized with sampling (check every Nth pixel).

#### Missing Tests
- **Critical: No tests for `findMargin`** — this is the core cropping logic with complex edge cases (blank images, limit exceeded, correctLine re-centering, all-blank, partially-blank).

### cover_title.go

#### Code Quality
- **Font parsed on every `Draw` call** (line 25): `f, _ := truetype.Parse(gomonobold.TTF)` — the font is re-parsed on every cover/title rendering. Should be a package-level `var`.
- **Error ignored on font parse** (line 25): `f, _ := truetype.Parse(...)` — if parse fails, `f` is nil and subsequent operations panic.

#### Bugs / Edge Cases
- **Text overflow at minimum font size** (line 26–32): The loop reduces font size from `maxFontSize` to 12. If the text doesn't fit at size 12, it still uses size 12 and overflows the text area. No truncation or wrapping.
- **`freetype.Pt` coordinates** (line 73): `textTop := textArea.Min.Y + textArea.Dy()/2 + textHeight/4` — the vertical centering is approximate. `textHeight/4` is a heuristic, not exact baseline calculation.

#### Performance
- **Font parse per call** — see Code Quality.
- **Font size loop** (line 26–32): Iterates from `maxFontSize` down to 12, creating a new `truetype.NewFace` on each iteration. For `maxFontSize=96`, that's 85 iterations with face allocation each time. Could binary search or cache faces.

#### Missing Tests
- No tests for `CoverTitle.Draw`, font size calculation, text positioning.

### crop_split_double_page.go

#### Code Quality
- Clean and simple. Correct implementation.

#### Bugs / Edge Cases
- **Odd-width images**: `srcBounds.Max.X/2` truncates for odd widths. The left half gets `Max.X/2 - Min.X` pixels, the right half gets `Max.X - Max.X/2` pixels. For width 1001: left=500, right=501. This is a 1-pixel asymmetry, which is acceptable for comic pages.

#### Missing Tests
- No tests for `CropSplitDoublePage.Bounds` or `Draw`.

### pixel.go

#### Code Quality
- Clean and simple. Correct implementation.

#### Bugs / Edge Cases
- None significant.

#### Missing Tests
- No tests, but the logic is trivial.

---

## 9. internal/pkg/epubzip/*.go

### epub_zip.go

#### Code Quality
- **Deprecated `ModifiedTime`/`ModifiedDate`** (lines 31–33): The `//goland:noinspection GoDeprecation` annotation suppresses IDE warnings. These fields are deprecated in favor of `Modified`. The code sets both, which is fine for compatibility but should eventually drop the deprecated fields.
- **`WriteContent` doesn't set `ModifiedTime`/`ModifiedDate`** (line 57): Unlike `WriteMagic` which sets both, `WriteContent` only sets `Modified`. Some older EPUB readers may rely on the DOS time fields. Minor compatibility concern.

#### Bugs / Edge Cases
- **`Close` error handling** (line 23–27): If `wz.Close()` succeeds but `w.Close()` fails, the file might be partially written. The error is returned correctly.
- **No flush/sync**: The file is not `Sync()`'d before close. If the system crashes, the EPUB might be corrupted. For a CLI tool, this is usually acceptable.

#### Security
- **Output path overwrite**: `os.Create(path)` silently overwrites. No symlink check. If `path` is a symlink to a critical file, it would be overwritten.

#### Performance
- `WriteMagic` pre-computes CRC32 — efficient.
- `WriteContent` uses deflate — standard.

### image.go

#### Code Quality
- **`CompressImage` and `CompressRaw` share ~90% identical code** — both create a deflate writer, write data, close writer, create `Image` struct with `FileHeader`. Should be refactored to share the deflate+header logic.

#### Bugs / Edge Cases
- **Double compression for JPEG/PNG**: `CompressImage` encodes to JPEG/PNG (already compressed), then deflates the encoded bytes. For JPEG, deflate provides minimal gain (5–10% at best) but costs CPU. For PNG (already deflate-compressed), the double compression is nearly useless. This is standard ZIP behavior but wasteful.
- **`CompressRaw` doesn't validate input** — accepts any `[]byte` as "uncompressed data". If given already-compressed data, it deflates it again (same double-compression issue).

#### Performance
- **`flate.BestCompression`** (line 21, 51): Slowest compression level. For already-compressed JPEG data, `flate.BestSpeed` would produce nearly identical output in much less time. Should use `flate.BestSpeed` for JPEG and `flate.DefaultCompression` for PNG.

#### Missing Tests
- No tests for `CompressImage`, `CompressRaw`, `WriteMagic`, `WriteRaw`, `WriteContent`.

### storage_image_reader.go

#### Code Quality
- Clean implementation. `files` map for O(1) lookup. Good.

#### Bugs / Edge Cases
- **`Size` estimation** (line 40): `return img.CompressedSize64 + 30 + uint64(len(img.Name))` — the `30` is the ZIP local file header fixed size. This is an approximation (doesn't include extra fields). Used for part size estimation, so approximate is fine.
- **`Close` doesn't check if `fh` is nil** — if `NewStorageImageReader` failed partially, `fh` might be nil. But the constructor returns an error, so the caller shouldn't use a failed reader.

### storage_image_writer.go

#### Code Quality
- **Mutex usage is correct** (lines 36–37, 50–51): `CompressImage`/`CompressRaw` run outside the mutex (CPU-bound, no shared state), then the ZIP write is mutex-protected. Good design.

#### Bugs / Edge Cases
- **`Close` double-close**: If `fz.Close()` fails, `fh.Close()` is called and the `fz` error is returned. If `fz.Close()` succeeds and `fh.Close()` fails, the error is returned. Correct.
- **No error if `Add`/`AddRaw` called after `Close`** — would panic on nil `e.fz`. Not a real concern (caller should manage lifecycle).

#### Concurrency
- **Thread-safe**: Mutex protects ZIP writer access. Compression runs in parallel. Correct.

#### Missing Tests
- No tests for `Add`, `AddRaw`, `Close`, concurrent access.

---

## 10. internal/pkg/epubtemplates/content.go

### Code Quality
- **Redundant IDE suppression** (line 32): `//goland:noinspection HttpUrlsUsage,HttpUrlsUsage,HttpUrlsUsage,HttpUrlsUsage` — `HttpUrlsUsage` appears 4 times. Once would suffice. Copy-paste artifact.
- **Misspelling** (line 108): `"Go Comic Convertor"` — should be `"Go Comic Converter"`.
- **`getSpineAuto` is stateful and complex** (lines 237–275): `isOnTheRight` is toggled inside `getSpread`, making the function's behavior depend on call order. This is fragile and hard to reason about.
- **`getSpineAuto` mutates input slice** (line 270–275): `o.Images[i] = img` — `Content` is passed by value but `Images` is a slice (shared backing array). Modifying `o.Images[i]` changes the caller's slice. This is a **hidden side effect** from a `String()` method.

#### Bugs / Edge Cases
- **`getManifest` panic on empty Images** (line 226): `lastImage := o.Images[len(o.Images)-1]` — panics if empty. Guarded upstream but not defensively.
- **`getGuide` panic on empty Images** (line 279): `o.Images[0].PagePath()` — same issue.
- **`getSpineAuto` side effect** (line 270–275): Writing `img.Position` back to `o.Images[i]` modifies the original slice. The `Position` field is later read by `epubimage.ImgStyle` to determine CSS alignment. This works because `Content.String()` is called before the images are rendered to XHTML, but it's a surprising side effect.

#### Security
- **XML injection**: `etree.CreateText(o.Title)` — etree **does** escape XML entities (`<`, `>`, `&`, `"`, `'`). So the title is safely escaped in `content.opf`. **However**, the title is also used in `text/template` rendering (epub.go `render`) which does **NOT** escape. So the title is safe in `content.opf` but potentially unsafe in XHTML pages.

#### Performance
- **`etree.Document.Indent(2)`** (line 76) — pretty-prints the XML. For large manifests (hundreds of images), this is O(n) and fine.
- **`SortAttrs()`** (line 68) — sorts attributes on every tag. Minor overhead for deterministic output.

#### Missing Tests
- **Critical: No tests for `Content.String()`** — the XML output must be valid EPUB. No validation testing.
- No tests for `getMeta`, `getManifest`, `getSpineAuto`, `getSpinePortrait`, `getGuide`.

### API Design
- **`Content` struct has 10 fields** — many are required. No constructor function. Callers must set all fields correctly.
- **`String()` method has side effects** (mutates `Images`) — violates the principle that `String()` should be read-only.

---

## 11. internal/pkg/epubtemplates/toc.go

### Code Quality
- **TOC building logic is complex** (lines 28–47): Uses a `paths` map to track `<ol>` elements by path. The algorithm is correct but hard to follow.
- **Empty `<ol>` removal** (lines 43–47): Iterates `FindElements("//ol")` and removes empty ones. Modifying the tree during iteration is safe because `FindElements` returns a snapshot slice.

### Bugs / Edge Cases
- **Empty `img.Path` — images missing from TOC** (line 37): For PDF sources, all images have `Path: ""`. `strings.Split("", separator)` returns `[""]`. `filepath.Join(".", "")` = `"."`. `paths["."]` already exists (the root `ol`), so `continue` is hit. The image's page link is **never added to the TOC**. This is a **bug** — PDF-sourced EPUBs have an empty TOC (only the "beginning" link is present).
  ```go
  // line 37-38 — empty Path causes skip
  for _, path := range strings.Split(img.Path, string(filepath.Separator)) {
      // If path is "", this iterates once with "" which joins to "."
      // which already exists in paths map → continue
  }
  ```
- **`images[0].PagePath()` when `hasTitle` is false** (line 53): Panics if `images` is empty.

### Security
- **Path in TOC links**: `img.PagePath()` returns `Text/page_X_pY.xhtml` — no user-controlled path components. Safe.

#### Missing Tests
- **Critical: No tests for `Toc()`** — especially the empty-path bug for PDF sources.
- No tests for tree pruning, strip-first-directory logic.

---

## 12. internal/pkg/sortpath/*.go

### parser.go

#### Code Quality
- **Regex compiled at package level** (line 10): `var splitPathRegex = regexp.MustCompile(...)` — good, not recompiled.
- **`part.compare` returns float64** (line 15): Using float64 for comparison results is unusual. `a.number - b.number` could have floating-point precision issues for very large or very close numbers. For sorting purposes, this is usually fine.

#### Bugs / Edge Cases
- **Range portion of regex is parsed but never used** (line 28–31): The regex captures `-(\d+(?:\.\d+)?)` (the end of a range like `s2-3`), but `parsePart` only uses `r[1]` (name) and `r[2]` (number). The range end `r[3]` is **discarded**. This means `s2-3` and `s2-4` compare as equal (both have `number=2`). This is a **bug** for filenames with ranges.
  ```go
  // line 28-31 — r[3] (range end) is captured but never used
  r := splitPathRegex.FindStringSubmatch(p)
  n, err := strconv.ParseFloat(r[2], 64)  // only r[2], not r[3]
  return part{p, r[1], n}
  ```
- **`number == 0` treated as "no number"** (line 16): `if a.number == 0 || b.number == 0` — a legitimate `chapter0` or `volume0` would have `number=0` and be compared as a string, not numerically. `volume0` vs `volume1`: string comparison gives `volume0 < volume1` (correct). `volume0` vs `volume00`: string comparison gives `volume0 > volume00` (likely incorrect — `00` should equal `0`). Edge case.
- **Multi-segment numbers**: `v1c2` → regex matches `v`, `1`, no range. The `c2` part is not captured. So `v1c2` and `v1c3` would compare as equal (both have `name="v"`, `number=1`). Only the **first** number in the string is parsed. This is a limitation for complex filenames.
- **Lowercase conversion** (line 37): `strings.ToLower(filename)` — sorting is case-insensitive. `IMG001.jpg` and `img001.jpg` compare as equal. Reasonable.

#### Missing Tests
- **Critical: No tests for `parsePart`, `compareParts`, `parse`** — the sorting logic is complex and untested.
- Edge cases: empty strings, multi-segment numbers, ranges, decimal numbers, negative numbers (not supported by regex).

### by.go

#### Code Quality
- Clean implementation of `sort.Interface`.

#### Bugs / Edge Cases
- None beyond the parser issues.

---

## 13. internal/pkg/epubtree/epub_tree.go

### Code Quality
- Clean, simple tree implementation.
- **`Node` fields are unexported** (`value`, `children`) — good encapsulation.

#### Bugs / Edge Cases
- **`FirstChild()` panics on empty children** (line 38): `return n.children[0]` — called from epub.go line 169 where `c.ChildCount() == 1` is checked first. Safe but not defensively coded.
- **`Add` with empty filename**: `filepath.Clean("")` returns `"."`, `strings.Split(".", separator)` returns `["."]`. Adds a single node with value `"."` as child of root. Not a real concern (empty filenames don't occur in practice).

#### Missing Tests
- No tests for `Add`, `WriteString`, tree structure building.

---

## 14. internal/pkg/epubprogress/*.go

### epub_progress.go

#### Code Quality
- **Clean factory function** — returns different implementations based on options.
- **`progressbar.DefaultSilent` returns `*progressbar.ProgressBar`** (line 33) — implements `EPUBProgress` structurally. Works because `progressbar.ProgressBar` has `Add(int) error` and `Close() error` methods.

#### Bugs / Edge Cases
- **Quiet mode still creates a progress bar**: `progressbar.DefaultSilent` creates a bar with `io.Discard` writer. The bar still tracks state and calls `Add`. Minimal overhead but not truly "no progress."

### json.go

#### Code Quality
- Clean JSON encoder.

#### Bugs / Edge Cases
- **No throttling**: Every `Add(1)` call writes a JSON line to stdout. For a 1000-image comic, that's 1000 JSON lines. Could flood stdout. The progressbar has `OptionThrottle(65ms)` but jsonprogress has no throttling.
- **`Close()` is a no-op** (line 26) — doesn't write a final "complete" state. The caller can't tell if the progress was finished or aborted.

#### Concurrency
- **`current` field is not thread-safe** (line 14) — but `Add` is only called from the main goroutine (collector loop in processor.go). Safe in current usage but fragile.

#### Missing Tests
- No tests for `jsonprogress.Add`, `jsonprogress.Close`.

---

## 15. internal/pkg/utils/utils.go

### Code Quality
- **`Fatalf`/`Fatalln` call `os.Exit(1)`** (lines 12, 22) — used throughout the codebase including from **library code** (internal/pkg packages). This is an anti-pattern for libraries — makes code untestable (can't recover from `os.Exit`) and prevents graceful shutdown.
- **`Printf`/`Println` write to stderr** (lines 9, 17) — all output goes to stderr, including informational messages. This is unusual (most CLI tools print info to stdout). The `-json` mode writes to stdout, so stderr for non-JSON is intentional to avoid mixing.

#### Bugs / Edge Cases
- **`NumberOfDigits(0)` returns 1** — correct (0 is 1 digit).
- **`NumberOfDigits` for negative numbers**: `-5` → `i=5, count=2` → returns 2 (1 digit + sign). `FormatNumberOfDigits(-5)` → `"%02d"` → used for formatting `-5` as `-5` (2 chars). Correct for format width purposes.

#### Missing Tests
- **Has tests** (utils_test.go) — Example tests for `FloatToString`, `IntToString`, `NumberOfDigits`, `FormatNumberOfDigits`. Good coverage for this simple package.
- Missing: tests for `Printf`, `Fatalf`, `Println`, `Fatalln` (hard to test due to `os.Exit`).

### API Design
- **`BoolToString` could use `strconv.FormatBool`** — but the current implementation is clear and equivalent.
- **`Fatalf`/`Fatalln` should not be in a utils package** — they should be in `main` or a CLI-specific package, not importable by library code.

---

## 16. pkg/epuboptions/*.go

### epub_options.go

#### Code Quality
- Clean data types with proper `yaml`/`json` tags.
- **`WorkersRatio` API is non-obvious** (line 34): `WorkersRatio(pct int)` takes a percentage. `WorkersRatio(50)` means "50% of workers". A clearer API would be to document this or use a different pattern.
- **`ImgStorage()` is a computed path** (line 42): `return o.Output + ".tmp"` — if `Output` is empty (shouldn't happen after Validate), returns `.tmp`. No validation.

#### Bugs / Edge Cases
- **`Workers` field has `yaml:"-"`** (line 28) — not saved to config. This means the workers setting is lost on restart. Intentional (workers is a runtime parameter) but could be confusing.

### image.go, crop.go, view.go, color.go

#### Code Quality
- All clean data types with proper tags.
- **`GrayScaleMode` JSON tag mismatch** (image.go line 16): `json:"gray_scale_mode"` but `yaml:"grayscale_mode"` — inconsistent naming between YAML and JSON. The JSON field has an underscore between "gray" and "scale", the YAML doesn't.

#### Missing Tests
- No tests for `WorkersRatio`, `ImgStorage`, `View.Dimension`, `View.Port`.

---

## 17. internal/pkg/epubimage/epub_image.go

### Code Quality
- Clean struct with well-documented methods.
- **`ImgStyle` builds CSS inline** (lines 86–113) — uses string concatenation with `append`. Could use `strings.Join` from the start (which it does at line 112). The intermediate `append` to a slice is fine.

#### Bugs / Edge Cases
- **`RelSize` with zero dimensions** (line 115–116): `if w <= 0 || h <= 0 || srcw <= 0 || srch <= 0 { return }` — returns `(0, 0)`. This would make `marginW` and `marginH` equal to `viewWidth/2` and `viewHeight/2`, which is correct (image has 0 size, centered).
- **`Position` field** (line 24): Set by `getSpineAuto` in content.go as a side effect. Read by `ImgStyle` to determine alignment. This cross-package mutation is fragile.

#### Missing Tests
- **No tests for `RelSize`** — the aspect-ratio fitting logic is critical and untested.
- No tests for `ImgStyle` CSS generation.

---

## 18. internal/pkg/converter/profiles.go

### Code Quality
- Clean, data-driven profile definitions.
- **`Profiles.String()` iterates a map** (line 62) — map iteration order is non-deterministic in Go. The output order of `AvailableProfiles()` is random. This affects `-help` output. Should sort by code.
  ```go
  // line 60-69 — map iteration is non-deterministic
  func (p Profiles) String() string {
      s := make([]string, 0)
      for _, v := range p {  // random order!
          s = append(s, ...)
      }
      return strings.Join(s, "\n")
  }
  ```

#### Bugs / Edge Cases
- **Non-deterministic help output** — see above. Every `-help` call shows profiles in a different order.

#### Missing Tests
- No tests for `NewProfiles`, `Profile.String`, `Profiles.String`.
- Could test that all 26 profiles are present and have valid dimensions.

---

## 19. internal/pkg/converter/order.go

### Code Quality
- Clean, minimal interface for flag ordering.

#### Bugs / Edge Cases
- None.

#### Missing Tests
- N/A (trivial).

---

## 20. Cross-Cutting Concerns

### CRITICAL: `os.Exit()` from Library Code

**Severity: High**  
**Files affected**: `utils.go` (Fatalf/Fatalln), `processor.go` (Load), `loader.go` (loadCbr)

Multiple `internal/pkg` packages call `utils.Fatalf` which calls `os.Exit(1)`. This:
1. **Makes the code untestable** — `os.Exit` cannot be recovered in tests.
2. **Skips deferred cleanup** — temp files, open readers, progress bars are not cleaned up.
3. **Kills from goroutines** — `os.Exit` from a goroutine is safe (it terminates the process) but skips all deferred functions in other goroutines.

**Recommendation**: Return errors to the caller. Only `main.go` should call `os.Exit`.

### CRITICAL: Race Conditions in Image Processing

**Severity: High**  
**File**: `processor.go` `Load()` method

1. **Race on `err` named return** (lines 72, 82): Multiple goroutines write to the shared `err` variable.
2. **Race on `bar.Close()`** (lines 73, 83, 99): `bar.Close()` called from both worker goroutines and the main goroutine. `progressbar.ProgressBar` is not guaranteed thread-safe for `Close()`.

**Recommendation**: Use local error variables in goroutines. Don't close shared resources from goroutines — send a signal to the main goroutine and let it handle cleanup.

### HIGH: Missing Tests for Critical Logic

**Severity: High**

The project has **one test file** (`utils_test.go`) with Example tests for trivial utilities. None of the following critical logic is tested:

| Component | Risk |
|-----------|------|
| `sortpath` number parsing & comparison | Incorrect sort order for comics |
| `epubimagefilters.findMargin` | Incorrect cropping, blank detection failures |
| `epubimagefilters.autocontrast.mean` | Incorrect contrast adjustment |
| `epubimage.EPUBImage.RelSize` | Incorrect image sizing in EPUB |
| `epub.computeViewPort` / `computeAspectRatio` | Incorrect viewport, broken rendering |
| `epub.getParts` size splitting | Incorrect EPUB part sizes, SendToKindle failures |
| `epubtemplates.Content.String()` | Invalid EPUB XML |
| `epubtemplates.Toc` | Empty TOC for PDFs, broken navigation |
| `converter.Validate` | Missing/incorrect validation |
| `epubzip` compression & storage | Corrupt EPUB files |

### HIGH: Non-Deterministic Help Output

**Severity: Medium**  
**File**: `profiles.go` `Profiles.String()`

Map iteration order is non-deterministic. `-help` output shows device profiles in random order on every invocation. Should sort by profile code.

### MEDIUM: Template XSS / XML Injection

**Severity: Medium** (local tool, low exploitability)  
**File**: `epub.go` `render()`

`text/template` does not auto-escape HTML entities. User-controlled `Title` is inserted into XHTML without escaping. A title with `<`, `>`, or `&` produces malformed XHTML. Should use `html/template` or manual escaping.

### MEDIUM: Abandoned Dependencies

| Dependency | Last Active | Risk |
|------------|-------------|------|
| `github.com/tcnksm/go-latest` | 2017 | No security patches, pulls in ancient `google/go-github v17` |
| `github.com/golang/freetype` | 2017 | No security patches, used for font rendering |
| `github.com/raff/pdfreader` | 2022 | Minimal maintenance, niche |
| `github.com/fogleman/gg` | 2021 | Depends on abandoned `golang/freetype` |

**Recommendation**: Replace `go-latest` with direct `go-github` call or remove version check. Replace `golang/freetype` with `golang.org/x/image/font` packages. Evaluate `pdfreader` alternatives.

### MEDIUM: Config File Issues

**Severity: Medium**  
**File**: `options.go`

1. **Silent home directory fallback** (line 76): `home, _ := os.UserHomeDir()` — if `HOME` is unset, config is read/written from current directory.
2. **Insecure file permissions** (line 202): `os.Create` uses `0666 & ~umask`. Should use `0600`.
3. **Error comparison by string** (line 91): `err.Error() != "EOF"` — should be `err != io.EOF`.

### MEDIUM: Performance Bottlenecks

| Issue | File | Impact |
|-------|------|--------|
| Regex compiled per call | epub.go:87 | Slows every page render |
| Font parsed per call | cover_title.go:25 | Slows cover/title generation |
| Map for 65K histogram | auto_contrast.go:24 | Slow contrast adjustment, GC pressure |
| `flate.BestCompression` on JPEG | image.go:21,51 | Wastes CPU for ~0% gain on pre-compressed data |
| Single-threaded PDF extraction | loader.go:403 | Slow for large PDFs |
| Single-threaded passthrough | passthrough.go | Slow for large archives |
| Full pixel iteration for crop | auto_crop.go | Slow for large images |

### LOW: Code Duplication

**Severity: Low** (maintainability concern)

1. **Processor vs Passthrough loaders**: ~400 lines of duplicated loader logic. Should share a common framework.
2. **`CompressImage` vs `CompressRaw`**: ~90% identical code in epubzip/image.go.
3. **Worker goroutine pattern**: Repeated in `loadDir`, `loadCbz`, `loadCbr` with minor variations.

### LOW: Missing `context.Context` Support

**Severity: Low** (CLI tool, not a server)

No package supports `context.Context`. Long-running operations (image processing, EPUB generation) cannot be cancelled. For a CLI tool, `Ctrl+C` (SIGINT) kills the process, but deferred cleanup is skipped. Adding context support would enable graceful shutdown.

### LOW: Deprecated ZIP Time Fields

**File**: `epub_zip.go:31-33`, `image.go:49,75`

`ModifiedTime` and `ModifiedDate` are deprecated. The code sets both `Modified` and the deprecated fields for compatibility. Should eventually drop the deprecated fields.

### Summary of TODO/FIXME/HACK Comments

**No explicit TODO/FIXME/HACK comments found in any Go source file.**

IDE suppression annotations found:
- `//goland:noinspection GoDeprecation` in `epub_zip.go:30`, `image.go:49,75`
- `//goland:noinspection HttpUrlsUsage` (×4, redundant) in `content.go:32`

---

## Prioritized Action Items

### P0 — Critical (fix before any release)
1. Fix race conditions in `processor.go` `Load()` — use local error variables, don't close shared resources from goroutines
2. Fix nil pointer dereference in `loadCbr` solid mode (line 322: `f.Name` when `f` is nil)
3. Stop calling `os.Exit` from library code — return errors instead
4. Fix TOC bug for PDF sources (empty `img.Path` causes images to be missing from TOC)

### P1 — High (fix soon)
5. Add tests for `sortpath`, `findMargin`, `RelSize`, `computeViewPort`, `getParts`, `Content.String()`, `Toc()`
6. Sort profiles in help output (`Profiles.String()`)
7. Fix template XSS — use `html/template` or escape user input
8. Move regex/font parsing to package-level vars
9. Use `flate.BestSpeed` for JPEG compression
10. Fix config file permissions to `0600`

### P2 — Medium (plan for next minor)
11. Replace `go-latest` with maintained alternative or remove
12. Replace `golang/freetype` with `golang.org/x/image/font`
13. Extract shared loader framework for processor/passthrough
14. Add `context.Context` support for cancellation
15. Fix `GrayScaleMode` JSON tag inconsistency (`gray_scale_mode` vs `grayscale_mode`)
16. Make color regex case-insensitive
17. Add throttling to `jsonprogress`
18. Fix "Convertor" misspelling in `content.go:108`

### P3 — Low (technical debt)
19. Use fixed-size array for auto_contrast histogram
20. Add parallelism to passthrough loaders
21. Refactor `CompressImage`/`CompressRaw` to share code
22. Add `Sync()` to EPUB zip writer
23. Remove redundant IDE suppression in `content.go:32`
24. Clean up deprecated ZIP time fields
