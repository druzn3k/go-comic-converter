# Implementation Plan: 5 Features for go-comic-converter v3

**Generated:** 2026-07-14
**Go Version:** 1.23
**Module:** `github.com/druzn3k/go-comic-converter/v3`

---

## Architecture Summary

The conversion pipeline flows:
```
main.go dispatch → pkg/comic/converter.go (Convert) → GetParts → processor.Load()
  → transformImage() → filters.DefaultChain() [HARDCODED] → gift filter → storage
  → buildOutputParts() → output.Get(format).Write() → output file
```

The new composable filter system (`pkg/comic/filters/`) is fully built (Filter interface, Chain, Recipe YAML, condition DSL, builtins, registry) but **NOT** wired — `transformImage()` still calls the old `DefaultChain()` (gift-based).

---

## Feature 1: Wire Recipe System into Pipeline (Medium)

### Summary

`filters.Recipe`/`FromYAML`/`BuiltinRecipe` are implemented and tested, but `processor.go:transformImage()` hardcodes `filters.DefaultChain()`. CLI flags `-recipe`, `-recipe-show`, `-recipe-save` are registered but never read during conversion.

### Files to Modify

| File | Change | Effort |
|------|--------|--------|
| `internal/pkg/epubimageprocessor/processor.go` | Accept optional `*filters.Chain`; use it in `transformImage()` instead of `DefaultChain` | Medium |
| `pkg/comic/converter.go` | Add `recipe *filters.Chain` field; pass into processor; expose `SetRecipe()` | Small |
| `main.go` | Wire `-recipe`/`-recipe-show`/`-recipe-save` in `generate()` and `runSingleFormat()` | Medium |
| `internal/pkg/converter/options.go` | (no change needed — flags already exist) | — |
| `internal/pkg/converter/converter.go` | (no change needed — flags already registered) | — |

### Detailed Changes

#### 1a. `processor.go` — Accept recipe chain

**Current state (line 28-34, 230-260):**
```go
type ePUBImageProcessor struct {
    epuboptions.EPUBOptions
}
func New(o epuboptions.EPUBOptions) EPUBImageProcessor {
    return ePUBImageProcessor{o}
}
func (e ePUBImageProcessor) transformImage(…) epubimage.EPUBImage {
    g, dstBounds, isDoublePage := filters.DefaultChain(src, filters.DefaultChainOpts{…})
    // …draw with gift…
}
```

**Change:** Add `chain *filters.Chain` field; use it in `transformImage()`:
- Add a `SetRecipe(chain *filters.Chain)` method (or pass via options)
- In `transformImage()`: if `e.chain != nil`, call `chain.Apply(ctx, src, fctx)` to get `[]image.Image`; fall back to `DefaultChain` if nil
- The `Filter.Apply` returns `[]image.Image` — handle multi-output (split pages) by creating `EPUBImage` entries for each result
- Need to reconcile the `isDoublePage` flag: the new filter `split_double_page` already handles this; `DefaultChain` also returns it. For the recipe path, determine `isDoublePage` from aspect ratio + multi-output count
- New signature for `transformImage` stays compatible — `chain` is field-level, set once

**Key constraint:** The old `DefaultChain` path MUST still work when no recipe is provided (backward compatibility).

#### 1b. `pkg/comic/converter.go` — Thread recipe through

- Add `chain *filters.Chain` to `Converter` struct (line 26-29)
- Add `NewWithRecipe(opts Options, chain *filters.Chain) *Converter` constructor
- In `Convert()` (line 46-80): pass chain to processor if set
- Add `SetRecipe(chain *filters.Chain)` setter

#### 1c. `main.go` — Wire CLI flags

**`-recipe` flag handling in `generate()` (line 242-317):**
1. Before calling processor, if `cmd.Options.Recipe != ""`:
   - Check if it matches a builtin name: `filters.BuiltinRecipe(name)`
   - Otherwise treat as file path: `os.ReadFile(path)` then `filters.FromYAML(string(data))`
2. Pass loaded chain to `comic.NewWithRecipe()` or `imageProcessor` via a new setter
3. If recipe loading fails: `cmd.Fatal(err)`

**`-recipe-show` flag handling:**
- Print the effective chain and exit BEFORE conversion
- If `-recipe` given: show loaded recipe YAML
- If no `-recipe`: show `DefaultChain` description (list of filter names and params reconstructed from `epuboptions.Image`)
- Format as YAML using the same `Recipe` struct serialization

**`-recipe-save` flag handling:**
- Serialize current `Image` options into a `Recipe` struct:
  - Map: `crop.Enabled` → `auto_crop` with params from `Crop` fields
  - Map: `AutoContrast` → `auto_contrast`
  - Map: `Contrast != 0` → `contrast` with value
  - Map: `Brightness != 0` → `brightness` with value
  - Map: `GrayScale` → `grayscale` with `GrayScaleMode`
  - Map: `Resize` → `resize` with `View.Width/Height`
- Marshal to YAML, write to stdout or a file specified by `-recipe-save` value (if it's a non-empty string, use as path)
- Exit after writing

**`-recipe-show` and `-recipe-save` are exit-early flags** — they run and then `os.Exit(0)`.

### Dependencies

- **Independent** — can be done in parallel with Features 2, 3, 4, 5
- Blocked by: nothing (recipe system is already built)
- Blocks: nothing

### Testing Strategy

1. **Unit test** in `processor_test.go`: verify `transformImage` uses recipe chain when set, falls back to `DefaultChain` when nil
2. **Unit test** in `converter_test.go`: verify `NewWithRecipe` stores and passes chain
3. **Integration test** in `main_test.go` or a new `recipe_integration_test.go`:
   - `-recipe manga-standard` → verify conversion runs with recipe
   - `-recipe-show` → verify YAML output matches expected filter list
   - `-recipe-save` → verify generated YAML parses back via `FromYAML`
   - No recipe → verify backward-compatible behavior

### Verification Gates

1. `go test ./pkg/comic/filters/...` — existing recipe tests still pass
2. `go test ./internal/pkg/epubimageprocessor/...` — processor tests pass with recipe integration
3. `go run . -input testdata/sample.cbz -recipe manga-standard -output /tmp/test.epub` — completes successfully
4. `go run . -input testdata/sample.cbz -recipe-show` — prints filter chain YAML and exits
5. `go run . -input testdata/sample.cbz -recipe-save` — prints recipe YAML to stdout
6. `go run . -recipe-save=/tmp/my-recipe.yaml` — writes recipe to file
7. `go run . -input testdata/sample.cbz -output /tmp/test.epub` (no recipe) — unchanged behavior

---

## Feature 2: Wire Worker Goroutines into HTTP Server (Medium)

### Summary

`POST /api/convert` submits jobs to `JobQueue`, `GET /api/progress/{id}` streams SSE, but **no goroutine ever executes conversions**. Jobs sit in "queued" status forever. The `handleProgress` SSE handler blocks on an empty `Progress` channel until client disconnect.

### Files to Modify

| File | Change | Effort |
|------|--------|--------|
| `pkg/comic/server/server.go` | Add `ctx` field; launch worker goroutines in `New()`; add `runWorker()` method | Medium |
| `pkg/comic/server/jobs.go` | Import `comic` package; add `StartProcessing()` to `Job`; add `Pop()` or iterator to `JobQueue` | Medium |
| `pkg/comic/server/handlers.go` | `handleConvertMultipart` must NOT `defer os.RemoveAll(tmpDir)` before job completes — pass ownership to worker | Small |
| `pkg/comic/converter.go` | Possibly expose a progress callback hook | Small |

### Detailed Changes

#### 2a. `server.go` — Worker goroutine loop

**Current state (line 23-28, 49-55):**
```go
type Server struct {
    cfg    Config
    queue  *JobQueue
    server *http.Server
    mux    *http.ServeMux
}
func (s *Server) Start(ctx context.Context) error { … ListenAndServe() }
```

**Change:**
1. Add `ctx context.Context` and `cancel context.CancelFunc` to `Server`
2. In `New()`: store the background context; launch `cfg.MaxConcurrent` worker goroutines
3. Add `runWorker()` method:
   - Loop: select on `ctx.Done()` (return) and `s.queue.sem` (acquire slot)
   - Call `s.queue.NextPending()` to get next queued job
   - If no jobs: release sem, sleep briefly, continue
   - Set job.Status = "processing"
   - Call `comic.New(opts).Convert(ctx)` — need to reconstruct options from job.Opts
   - Send progress events to `job.Progress` channel (non-blocking send with select/default)
   - On success: `job.Done(nil)`
   - On error: `job.Done(err)`
   - Always: release sem, clean up temp files
4. In `Shutdown()`: cancel context, wait for workers to finish

#### 2b. `jobs.go` — Job queue enhancements

**Add to `JobQueue`:**
```go
// NextPending returns the next job with status "queued", or nil.
func (q *JobQueue) NextPending() *Job {
    var found *Job
    q.jobs.Range(func(key, value interface{}) bool {
        j := value.(*Job)
        j.mu.Lock()
        if j.Status == "queued" {
            j.Status = "processing"
            found = j
            j.mu.Unlock()
            return false // stop iteration
        }
        j.mu.Unlock()
        return true
    })
    return found
}
```

**Add to `Job`:**
- `Progress` already exists as `chan string, 100` — good
- Add `Cleanup func()` field for temp file cleanup
- Add `SendProgress(msg string)` helper that does non-blocking send

#### 2c. `handlers.go` — Fix temp file lifecycle

**Problem:** `handleConvertMultipart` (line 53-98) does `defer os.RemoveAll(tmpDir)` — temp files are deleted before any worker can read them.

**Fix:**
- Remove the `defer os.RemoveAll(tmpDir)` from handler
- Instead, pass a cleanup function to the job: `job.Cleanup = func() { os.RemoveAll(tmpDir) }`
- Worker calls `job.Cleanup()` after conversion completes (success or failure)
- Also attach the conversion opts to the job (currently only the path string is stored): change `Opts string` to include serialized options or store `epuboptions.EPUBOptions` directly

#### 2d. `converter.go` — Progress callback (if needed)

The worker needs to emit progress events. Options:
- **Option A (simpler):** Worker emits coarse-grained events: "processing", "writing output", "done"
- **Option B (finer):** Add a progress callback to `Converter` or reuse the existing `epubprogress` package

**Recommendation:** Start with Option A. The SSE client gets meaningful status updates without plumbing a callback through the entire pipeline. Add a `ProgressCallback func(string)` field to `Converter` that gets called at key milestones (after load, after process, after write).

### Dependencies

- **Independent** — can be done in parallel with Features 1, 3, 4, 5
- Blocked by: nothing (job queue already exists)
- Blocks: nothing directly, but Feature 3 (ComicInfo) would benefit from server testing

### Testing Strategy

1. **Unit test** in `jobs_test.go`: `NextPending()` returns jobs in insertion order, skips processing/completed
2. **Unit test** in `server_test.go`: worker picks up job, calls converter, updates status
3. **Integration test**: Start server, POST `/api/convert` with a small CBZ, poll `/api/progress/{id}` until done, verify output file exists
4. **Concurrency test**: Submit N jobs with `MaxConcurrent=2`, verify exactly 2 run at a time

### Verification Gates

1. `go test ./pkg/comic/server/...` — all tests pass
2. Start server: `go run . -serve :8080`
3. `curl -X POST http://localhost:8080/api/convert -F "file=@testdata/sample.cbz"` returns `{"job_id":"…"}`
4. `curl http://localhost:8080/api/progress/{id}` streams progress events and terminates with `{"type":"done","status":"completed"}`
5. Output file exists at generated path
6. Ctrl+C server — workers drain gracefully, no goroutine leaks

---

## Feature 3: Add ComicInfo.xml Metadata to CBZ Output (Small)

### Summary

CBZ output contains only numbered image files. Comic server apps (Komga, Kavita, etc.) read `ComicInfo.xml` as the first entry in the ZIP. The `PartMetadata` struct already carries the needed fields.

### ComicInfo.xml Schema

Standard ComicRack `ComicInfo.xml` format — a minimal subset:
```xml
<?xml version="1.0"?>
<ComicInfo xmlns:xsd="http://www.w3.org/2001/XMLSchema"
           xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">
  <Title>…</Title>
  <Series>…</Series>
  <Number>…</Number>
  <Summary>…</Summary>
  <Publisher>…</Publisher>
  <Genre>…</Genre>
  <PageCount>…</PageCount>
  <Writer>…</Writer>
  <Year>…</Year>
  <Month>…</Month>
  <Day>…</Day>
  <LanguageISO>…</LanguageISO>
  <Format>…</Format>
  <Manga>…</Manga>
  <Web>…</Web>
  <Notes>…</Notes>
</ComicInfo>
```

For MVP, support: `Title`, `Series`, `Number`, `Summary`, `Publisher`, `Genre`, `PageCount`, `Writer`, `Manga`, `Notes`. Fill from `PartMetadata` and `ImageConfig`.

### Files to Create/Modify

| File | Change | Effort |
|------|--------|--------|
| `pkg/comic/output/comicinfo.go` (NEW) | `ComicInfo` struct with XML tags; `Marshal()` method; `WriteComicInfo(zw *zip.Writer, meta PartMetadata, pageCount int)` | Small |
| `pkg/comic/output/cbz.go` | Call `WriteComicInfo()` as first ZIP entry in `writeCBZ()` | Small |
| `pkg/comic/output/output.go` | Add optional fields to `PartMetadata`: `Series`, `Number`, `Summary`, `Genre`, `Writer`, `Manga` | Small |
| `pkg/comic/converter.go` | Populate new `PartMetadata` fields in `buildOutputParts()` (from opts) | Small |
| `pkg/epuboptions/epub_options.go` | Add `Series`, `Number`, `Summary`, `Genre`, `Writer`, `Manga` fields to `EPUBOptions` (with `yaml:"-"` for CLI-only) | Small |
| `internal/pkg/converter/converter.go` | Add CLI flags: `-series`, `-number`, `-summary`, `-genre`, `-writer`, `-manga` | Small |

### Detailed Changes

#### 3a. `pkg/comic/output/comicinfo.go` (NEW)

```go
package output

import "encoding/xml"

type ComicInfo struct {
    XMLName   xml.Name `xml:"ComicInfo"`
    XmlnsXsd  string   `xml:"xmlns:xsd,attr"`
    XmlnsXsi  string   `xml:"xmlns:xsi,attr"`
    Title     string   `xml:"Title,omitempty"`
    Series    string   `xml:"Series,omitempty"`
    Number    string   `xml:"Number,omitempty"`
    Summary   string   `xml:"Summary,omitempty"`
    Publisher string   `xml:"Publisher,omitempty"`
    Genre     string   `xml:"Genre,omitempty"`
    PageCount int      `xml:"PageCount"`
    Writer    string   `xml:"Writer,omitempty"`
    Manga     string   `xml:"Manga,omitempty"`
    Notes     string   `xml:"Notes,omitempty"`
}
```

Provide `func MarshalComicInfo(meta PartMetadata, pageCount int) ([]byte, error)` that populates and marshals the struct.

#### 3b. `cbz.go` — Write ComicInfo.xml first

In `writeCBZ()` (line 62-134), BEFORE the image loop (line 109):
```go
// Write ComicInfo.xml as first entry
comicInfoData, err := MarshalComicInfo(part.Metadata, len(entries))
if err != nil { return err }
w, err := zw.Create("ComicInfo.xml")
if err != nil { return err }
_, err = w.Write(comicInfoData)
if err != nil { return err }
```

This goes between `zip.NewWriter(f)` (line 69) and the entry loop (line 109).

#### 3c. `PartMetadata` extensions

Add optional fields:
```go
type PartMetadata struct {
    Title       string
    Author      string
    Publisher   string
    Series      string  // NEW
    Number      string  // NEW (as string, since comic numbering varies)
    Summary     string  // NEW
    Genre       string  // NEW
    Manga       string  // NEW ("Yes"/"No"/"YesAndRightToLeft")
    UID         string
    UpdatedAt   string
    ImageConfig epuboptions.Image
}
```

#### 3d. CLI flags

Add to `converter.InitParse()` under a "Metadata" section:
- `-series` → `EPUBOptions.Series`
- `-number` → `EPUBOptions.Number`
- `-summary` → `EPUBOptions.Summary`
- `-genre` → `EPUBOptions.Genre`
- `-writer` → `EPUBOptions.Author` (reuse existing)
- `-manga` → `EPUBOptions.Manga` (bool flag, maps to "Yes"/"No")

### Dependencies

- **Independent** — can be done in parallel with Features 1, 2, 4, 5
- Blocked by: nothing (PartMetadata already flows through)
- Blocks: nothing

### Testing Strategy

1. **Unit test** in `comicinfo_test.go`: `MarshalComicInfo` produces valid XML with correct fields
2. **Unit test** in `cbz_test.go`: verify `ComicInfo.xml` is first entry in output CBZ
3. **Integration test**: Convert a CBZ with `-series`/`-number` flags, unzip output, verify `ComicInfo.xml` content
4. **Round-trip**: generated XML parses back with `xml.Unmarshal`

### Verification Gates

1. `go test ./pkg/comic/output/...` — ComicInfo marshals correctly, CBZ includes it
2. `go run . -input testdata/sample.cbz -series "My Series" -number "1" -manga -output /tmp/test.cbz`
3. `unzip -p /tmp/test.cbz ComicInfo.xml` shows valid XML with `Series=My Series`, `Number=1`, `Manga=Yes`
4. Verify ComicInfo.xml is the very first entry: `unzip -l /tmp/test.cbz | head -5`

---

## Feature 4: Add WebP Output Format (Small)

### Summary

`golang.org/x/image/webp` (already in `go.mod`) supports **decoding only** — no encoding. Need an external encoder to add "webp" as an output format option.

### WebP Encoding Library Analysis

| Library | Approach | Maturity | Tradeoff |
|---------|----------|----------|----------|
| `github.com/chai2010/webp` | CGo binding to libwebp | Most established, 1k+ stars | Requires CGo + libwebp dev headers |
| `github.com/deepteams/webp` | Pure Go encoder/decoder | Newer, benchmarked well | Fewer stars, less battle-tested |
| `github.com/HugoSmits86/nativewebp` | Pure Go encoder | Moderate | Wraps x/image for decode |
| `github.com/gen2brain/webp` | WASM-based | Niche | WASM runtime dependency |

**Recommendation:** Use `github.com/chai2010/webp` — it's the most mature, has both lossy and lossless encoding, and is the de facto standard in the Go ecosystem for WebP encoding. The CGo dependency is acceptable given the project already has no cross-compilation constraints documented.

**Alternative if CGo is unacceptable:** `github.com/deepteams/webp` (pure Go).

### Files to Modify

| File | Change | Effort |
|------|--------|--------|
| `go.mod` / `go.sum` | Add `github.com/chai2010/webp` dependency | Small |
| `internal/pkg/epubzip/image.go` | Add `"webp"` case to `CompressImage` switch | Small |
| `internal/pkg/converter/converter.go` | Add `"webp"` to format validation in `Validate()` | Small |
| `internal/pkg/converter/converter.go` | Add `"webp"` to output format validation | Small |
| `pkg/epubimage/epub_image.go` | Add `"image/webp"` MIME type to `MediaType()` | Small |

### Detailed Changes

#### 4a. `go.mod` — Add dependency

```bash
go get github.com/chai2010/webp
```

#### 4b. `epubzip/image.go` — WebP encoding

Add import: `"github.com/chai2010/webp"`

Add case to switch (line 27-34):
```go
case "webp":
    err = webp.Encode(&data, img, &webp.Options{Quality: float32(quality)})
```

The `webp.Encode` function takes `image.Image`, `io.Writer`, and `*webp.Options` (Quality 0-100). Quality maps directly from the existing `quality int` parameter.

#### 4c. `converter.go` — Format validation

**Image format validation (line 400):**
```go
if !slices.Contains([]string{"jpeg", "png", "copy", "webp"}, c.Options.Image.Format) {
    return errors.New("format should be jpeg, png, webp or copy")
}
```

**Output format validation (line 430-433):**
WebP is an image format, not a container format. The `-format webp` flag controls the internal image encoding (used inside EPUB/KEPUB/CBZ), not the output container. So output format validation does NOT need "webp" added — it stays `epub, kepub, cbz, html, all`.

#### 4d. `epub_image.go` — MIME type

In `MediaType()` (line 77-79), the format-to-MIME mapping already returns `"image/" + i.Format`, so `"image/webp"` is generated automatically. No change needed.

### Dependencies

- **Independent** — can be done in parallel with Features 1, 2, 3, 5
- Blocked by: nothing
- Blocks: nothing

### Testing Strategy

1. **Unit test** in `epubzip/image_test.go`: `CompressImage` with format="webp" produces valid WebP bytes
2. **Integration test**: Convert a sample image to EPUB with `-format webp`, verify images inside EPUB are valid WebP
3. **Round-trip**: Decode the generated WebP back to `image.Image` using `golang.org/x/image/webp` (decode-only, already in go.mod)

### Verification Gates

1. `go test ./internal/pkg/epubzip/...` — WebP compression produces valid output
2. `go run . -input testdata/sample.cbz -format webp -output /tmp/test.epub`
3. Unzip EPUB, verify image files have `.webp` extension and are valid WebP (decodable)
4. `go run . -format webp` validates without error

---

## Feature 5: Add Debouncing to Watch Mode (Small)

### Summary

`pkg/comic/watch.go` uses fsnotify to monitor a directory. Each `Create` event spawns a conversion goroutine immediately. File save workflows (editors writing temp → rename) fire multiple `Create` events, launching redundant conversions. No debouncing, no temp-file filtering, sparse logging.

### Files to Modify

| File | Change | Effort |
|------|--------|--------|
| `pkg/comic/watch.go` | Add debounce timer per filename, temp-file filter, enhanced logging | Small |

### Detailed Changes

#### 5a. `watch.go` — Debounce timer

**Current state (line 21-61):** Simple event loop with immediate dispatch.

**New structure:**
```go
type pendingJob struct {
    timer   *time.Timer
    opts    Options
}

func Watch(ctx context.Context, dir string, opts Options) error {
    watcher, err := fsnotify.NewWatcher()
    // …
    pending := make(map[string]*pendingJob)
    var mu sync.Mutex
    const debounceDelay = 500 * time.Millisecond

    for {
        select {
        case <-ctx.Done():
            // Drain and cancel all pending timers
            mu.Lock()
            for _, p := range pending {
                p.timer.Stop()
            }
            mu.Unlock()
            return ctx.Err()
        case event, ok := <-watcher.Events:
            if !ok { return nil }
            ext := strings.ToLower(filepath.Ext(event.Name))
            if !supportedExts[ext] { continue }

            // Skip temp files
            base := filepath.Base(event.Name)
            if isTempFile(base) {
                log.Printf("Watch: skipping temp file %s", event.Name)
                continue
            }

            mu.Lock()
            if p, exists := pending[event.Name]; exists {
                p.timer.Stop() // reset debounce
            }
            jobOpts := opts
            jobOpts.Input = event.Name
            jobOpts.Output = outputPath(event.Name, opts)
            timer := time.AfterFunc(debounceDelay, func() {
                mu.Lock()
                delete(pending, event.Name)
                mu.Unlock()
                log.Printf("Watch: converting %s", event.Name)
                if err := New(jobOpts).Convert(ctx); err != nil {
                    log.Printf("Watch: %s: %v", event.Name, err)
                } else {
                    log.Printf("Watch: %s complete", event.Name)
                }
            })
            pending[event.Name] = &pendingJob{timer: timer, opts: jobOpts}
            mu.Unlock()
        // …
        }
    }
}
```

#### 5b. `watch.go` — Temp file filter

```go
func isTempFile(name string) bool {
    return strings.HasPrefix(name, ".") ||
           strings.HasSuffix(name, "~") ||
           strings.HasSuffix(name, ".tmp") ||
           strings.HasSuffix(name, ".swp") ||
           strings.HasSuffix(name, ".swx") ||
           strings.Contains(name, ".goutputstream-")
}
```

#### 5c. `watch.go` — Enhanced logging

- Log when conversion starts: `Watch: converting <filename>`
- Log when conversion ends: `Watch: <filename> complete` or `Watch: <filename>: <error>`
- Log when skipping temp file: `Watch: skipping temp file <filename>`

### Dependencies

- **Independent** — can be done in parallel with Features 1, 2, 3, 4
- Blocked by: nothing
- Blocks: nothing

### Testing Strategy

1. **Unit test** for `isTempFile()`: verify `.swp`, `file~`, `.hidden`, `file.tmp`, `.#file` are all filtered
2. **Unit test** for debounce: create file, wait 200ms, create again — only one conversion triggered after 500ms
3. **Manual test**: `go run . -watch /tmp/watch-test`, then `cp test.cbz /tmp/watch-test/` — one conversion logged

### Verification Gates

1. `go test ./pkg/comic/...` — `isTempFile` tests pass
2. Start watcher: `go run . -watch /tmp/watch-test`
3. Copy file: `cp testdata/sample.cbz /tmp/watch-test/` → single conversion log line
4. Simulate editor save: `cp testdata/sample.cbz /tmp/watch-test/.#sample.cbz` → "skipping temp file" logged
5. Rapidly touch file twice: conversion starts exactly once after 500ms quiet period

---

## Feature Dependency Graph

```
┌─────────────────────────────────────────────────────────┐
│                    ALL INDEPENDENT                       │
│                                                         │
│  Feature 1 (Recipe)     Feature 2 (HTTP Workers)        │
│  Feature 3 (ComicInfo)  Feature 4 (WebP)                │
│  Feature 5 (Watch Debounce)                             │
│                                                         │
│  Zero cross-feature dependencies. All can be developed  │
│  and tested in parallel.                                │
└─────────────────────────────────────────────────────────┘
```

**No feature blocks another.** They touch different subsystems:
- Feature 1: processor pipeline + main.go dispatch
- Feature 2: server package only
- Feature 3: CBZ output writer only
- Feature 4: image encoding + validation
- Feature 5: watch.go only

**Recommended execution order:** All 5 in parallel. The only shared touchpoint is `main.go` (Features 1 and 2 both touch it), but the changes are in different sections (`generate()` vs `serve()`).

---

## Overall Testing Strategy

After all features are implemented:

1. `go test ./...` — full test suite passes (14 test packages currently)
2. `go vet ./...` — no warnings
3. Manual E2E:
   - Convert CBZ → EPUB with recipe: `go run . -input test.cbz -recipe manga-standard`
   - Convert CBZ → CBZ with ComicInfo: `go run . -input test.cbz -output test.cbz -series "Test"`
   - Convert with WebP: `go run . -input test.cbz -format webp`
   - HTTP server + convert: `go run . -serve :8080` + curl test
   - Watch mode: `go run . -watch /tmp/w` + file copy
4. Race detection: `go test -race ./...`

---

## Risk Assessment

| Feature | Risk | Mitigation |
|---------|------|------------|
| Recipe wiring | Medium — `DefaultChain` and recipe `Chain` have different APIs (gift vs image.Image), need bridging | Keep both paths; recipe path calls `chain.Apply()` per image, maps results to `EPUBImage` |
| HTTP workers | Medium — temp file lifecycle and concurrency | Pass cleanup function to job; use existing semaphore |
| ComicInfo | Low — additive change to CBZ writer | Standard XML marshal; first ZIP entry |
| WebP | Low — straightforward format addition | Use well-tested `chai2010/webp` library |
| Watch debounce | Low — isolated change to one file | Map-based timer management with mutex |
