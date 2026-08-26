# Architecture Deepening Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the shallow modules from the architecture review into deep ones: consolidate the filter package, one EPUB-family writer behind one seam, one output-dispatch seam, a format-neutral storage key, and delete internal dead code.

**Architecture:** Five tasks in two waves. Wave 1 (parallel, disjoint files): T1 delete dead code, T2 consolidate filter package, T3 storage-key seam. Wave 2 (sequential): T4 unify EPUB+KEPUB via a new `internal/pkg/epubwriter` (cycle-free), T5 route EPUB through the dispatch seam (depends on T4). Each task deepens one module behind one seam.

**Tech Stack:** Go 1.26, module `github.com/druzn3k/go-comic-converter/v3`. Image filters via `github.com/disintegration/gift`. ZIP via `archive/zip`. Templates via `html/template` + embedded files.

## Global Constraints

- Go 1.26. Build/test with `go test ./...` from repo root.
- `pkg/comic` and `pkg/epub` are public/importable. **Do not delete exported public types** (see T1 — deprecate, don't remove, public aliases/interfaces). Only `internal/` types may be deleted freely.
- Filter output must remain pixel-identical to today. Golden fixtures are generated from the **pre-task** codebase (never `-update` after the refactor unless the change is reviewed as intentional).
- `epuboptions.EPUBOptions`/`Image` are the input record; do not rename/delete exported fields (YAML/JSON tags are load-bearing).
- No CLI flag or `-json` output shape changes.
- New internal packages go under `internal/pkg/`. They may import `epubimage`, `epubzip`, `epubtemplates`, `epuboptions`, `epubimageprocessor`, `source` — but **never** `pkg/comic` or `pkg/comic/output` (cycle guard).

---

### Task 1: Delete internal dead code; deprecate public pass-throughs

**Files:**
- Delete: `pkg/comic/registry.go`
- Modify: `pkg/comic/comic_test.go` (delete `TestRegistry*`)
- Modify: `pkg/comic/options.go`, `pkg/comic/viewport.go`, `pkg/epub/epub.go` (add `// Deprecated:` comments only — no deletion)

**Interfaces:**
- No signature changes. `type Options = epuboptions.EPUBOptions`, the `viewport` re-exports, and `type EPUB interface` remain (deprecated) so public callers are not broken.

- [ ] **Step 1: Delete the dead registry**
  Delete `pkg/comic/registry.go` and the `TestRegistryRegisterLookup`, `TestRegistryLookupMissing`, `TestRegistryNames`, `TestRegistryConcurrentSafe` functions in `comic_test.go`. Verify no other reference: `grep -rn "newRegistry\|\.register(\|\.lookup(\|\.names()" pkg/` → only the deleted file.

- [ ] **Step 2: Deprecate, don't delete, the public pass-throughs**
  Add `// Deprecated: use epuboptions.EPUBOptions directly.` above `type Options` in `pkg/comic/options.go`; `// Deprecated: import pkg/comic/viewport directly.` above the re-exports in `pkg/comic/viewport.go`; `// Deprecated: a single-implementation interface; New returns *epub.` above `type EPUB interface` in `pkg/epub/epub.go`. Do NOT change any return type or delete any symbol.

- [ ] **Step 3: Run tests**
  `go build ./... && go test ./pkg/comic/... ./pkg/epub/...`
  Expected: PASS.

- [ ] **Step 4: Commit**
  `git commit -m "refactor: delete dead registry; deprecate public pass-through aliases"`

---

### Task 2: Consolidate the filter package (one package, fixed bridge, no behavior change)

**Files:**
- Move into `pkg/comic/filters` (package `filters`): the five files from `internal/pkg/epubimagefilters/` (auto_crop, auto_contrast, crop_split_double_page, cover_title, pixel)
- Modify: `pkg/comic/filters/{builtin_crop,builtin_contrast,builtin_orient,builtin_blank,filter,default_chain}.go`
- Modify: `internal/pkg/epubimageprocessor/processor.go`
- Modify: `main.go` (replace `optionsToFilterConfigs` with a total `defaultFilterConfigs`)
- Delete: `internal/pkg/epubimagefilters/` directory

**Interfaces:**
- Produces: the five gift-filter constructors now live in `pkg/comic/filters` (same signatures: `AutoCrop(img, bounds, l, u, r, b, limit int, skip bool) gift.Filter`, `AutoContrast() gift.Filter`, `CropSplitDoublePage(right bool) gift.Filter`, `CoverTitle(title, align string, pctWidth, pctMargin, maxFontSize, borderSize int) gift.Filter`, `Pixel() gift.Filter`).
- Produces: `FilterContext` loses the dead field `ImageOptions` → `{ Part int; Right bool; IsDoublePage bool; OriginalBounds image.Rectangle }`.
- Produces: `func defaultFilterConfigs(img epuboptions.Image) []FilterConfig` — TOTAL (not lossy) options→recipe bridge.
- **`DefaultChain` / `DefaultChainOpts` remain** (the default processing path is NOT rewritten in this task — full single-engine collapse is deferred: the recipe path and default path have different double-page-split semantics, and unifying them risks pixel drift).

- [ ] **Step 1: Relocate the five gift filters**
  Move the five files into `pkg/comic/filters` (package `filters`), preserving constructor signatures. Update `default_chain.go` to call them directly (drop the `epubimagefilters` import). `grep -rn "epubimagefilters" --include="*.go"` → empty. Delete `internal/pkg/epubimagefilters/`.

- [ ] **Step 2: Make the thin wrapper builtins the real implementations**
  Rewrite `builtin_crop.go` `AutoCropFilter.Apply`, `builtin_contrast.go` `AutoContrastFilter.Apply`, `builtin_orient.go` `CropSplitDoublePageFilter.Apply`, `builtin_blank.go` `PixelFilter.Apply` so each calls the relocated constructor directly (they already do — now in-package, no wrapper-over-wrapper). Fix `builtin_blank.go`'s `PixelFilter` to call `Pixel()` instead of re-implementing it. Do NOT change the `split_double_page` (`SplitDoublePageFilter`) semantics.

- [ ] **Step 3: Remove the dead `FilterContext.ImageOptions`**
  Delete the `ImageOptions epuboptions.Image` field from `FilterContext` (filter.go:16) and the single assignment in `processor.go` `transformImage`. `grep -rn "ImageOptions" pkg/comic/filters/ internal/pkg/epubimageprocessor/` → no readers.

- [ ] **Step 4: Fix the bridge — make it total**
  In `main.go`, replace `optionsToFilterConfigs` with `defaultFilterConfigs(img epuboptions.Image) []filters.FilterConfig` that emits the FULL set matching `DefaultChain`: `split_double_page` (when `AutoSplitDoublePage`, params keep_original/manga), `auto_crop` (crop margins/limit/skip), `auto_contrast`, `contrast`/`brightness` (same int scale as DefaultChain — do NOT divide by 100), `resize`, `grayscale` (mode), `pixel`. Update the `-recipe-save` call site (main.go:264) to `defaultFilterConfigs(cmd.Options.Image)`. Add a unit test asserting `defaultFilterConfigs` output matches `DefaultChain`'s filter set for a representative config.

- [ ] **Step 5: Golden parity test (generated pre-task)**
  BEFORE any filter change (commit order matters): generate golden output from the current codebase — convert a fixed `testdata` image via the default path and commit its bytes as a fixture. Then implement Steps 1-4. Add a processor test (`processor_test.go`) that runs the default path and asserts byte-identity with the committed fixture. Run without `-update`.

- [ ] **Step 6: Run tests**
  `go build ./... && go test ./pkg/comic/filters/... ./internal/pkg/epubimageprocessor/... ./...`
  Expected: PASS, including the golden parity test (proving Steps 1-4 changed no pixel output).

- [ ] **Step 7: Commit**
  `git commit -m "refactor: consolidate filter package; relocate epubimagefilters; total default bridge; drop dead FilterContext field"`

---

### Task 3: Format-neutral storage key (stop EPUB layout leaking into CBZ/HTML)

**Files:**
- Modify: `internal/pkg/epubimage/epub_image.go` (add `StorageKey()`; keep `EPUBImgPath()` as `"OEBPS/" + ImgPath()`)
- Modify: `internal/pkg/epubimageprocessor/processor.go` (write processed images under `ImgPath()`, not `EPUBImgPath()`)
- Modify: `pkg/comic/output/cbz.go`, `pkg/comic/output/html.go` (use `StorageKey()`)

**Interfaces:**
- Produces: `func (i EPUBImage) StorageKey() string { return i.ImgPath() }` — the format-neutral key under which the processed image is stored in the temp ZIP (no `OEBPS/` prefix). `ImgPath()` already returns `"Images/img_<id>_p<part>.<format>"`.
- Invariant: the key written by the processor (storage writer) and the key read by CBZ/HTML/KEPUB are identical and format-neutral.

- [ ] **Step 1: Verify the current storage write key**
  Read `internal/pkg/epubimageprocessor/processor.go` and `internal/pkg/epubzip/storage_image_writer.go`; confirm the processor writes images under `EPUBImgPath()` (the `OEBPS/`-prefixed key). Record the exact call site.

- [ ] **Step 2: Write under the neutral key; read under it**
  Change the processor's storage-write to use `img.ImgPath()` (drop `OEBPS/`). Add `StorageKey()` to `epubimage.EPUBImage` returning `i.ImgPath()`. Update `cbz.go:86,96` and `html.go:84,92` to use `.StorageKey()`. Ensure `EPUBImgPath()` still returns `"OEBPS/" + i.ImgPath()` (EPUB rendering unchanged).

- [ ] **Step 3: Run tests**
  `go test ./pkg/comic/output/... ./internal/pkg/epubimageprocessor/... ./internal/pkg/epubimage/...`
  Expected: PASS (CBZ/HTML/KEPUB/EPUB still read the images they wrote).

- [ ] **Step 4: Commit**
  `git commit -m "refactor: format-neutral StorageKey; decouple CBZ/HTML from EPUB ZIP layout"`

---

### Task 4: Unify EPUB + KEPUB writers behind `internal/pkg/epubwriter`

**Files:**
- Create: `internal/pkg/epubwriter/epubwriter.go` (the deep, cycle-free shared writer)
- Create: `internal/pkg/epubtemplates/kepub_text.go` (export `var KepubText string` = the kobolink template)
- Modify: `pkg/epub/epub.go` (thin adapter over `epubwriter`)
- Modify: `pkg/comic/output/kepub.go` (thin adapter over `epubwriter`; delete ~450 duplicated lines; use passed `parts`)
- Modify: `pkg/comic/output/output.go` (no change)

**Interfaces:**
- Produces: `internal/pkg/epubwriter.Part { Cover epubimage.EPUBImage; Images []epubimage.EPUBImage }`
- Produces: `internal/pkg/epubwriter.Variant { Extension, TextTemplate string; KoboStyle, AppleBooks, EscapeTitle bool }`
- Produces: `func epubwriter.Write(ctx context.Context, parts []Part, variant Variant, opts epuboptions.EPUBOptions) error`
- `epubwriter` imports only `epubimage`, `epubzip`, `epubtemplates`, `epuboptions`, `epubimageprocessor` — **never** `pkg/comic` or `pkg/comic/output` (cycle guard). `comic.GetParts` stays in `pkg/comic`; callers (pkg/epub) convert `comic.Part` → `epubwriter.Part`; `kepub.go` converts its received `[]OutputPart` → `[]epubwriter.Part` (no re-load, no re-split).

- [ ] **Step 1: Export the KEPUB text template**
  Create `internal/pkg/epubtemplates/kepub_text.go`: `var KepubText string` holding the exact `kepubTextTemplate` string from `kepub.go:43-55` (kobolink wrapper, `{{.ImagePath}}` without `../`). Delete the inline `kepubTextTemplate` in kepub.go.

- [ ] **Step 2: Build the shared writer**
  Port into `epubwriter.go` the EPUB-side implementations of `xmlEscape`, `render`, `getTree` (with `StripFirstDirectoryFromToc`), `computeViewPort`, `writePart`, `writeCoverImage`, `writeTitleImage`, `writePageImage`/`writeImage`, `writeBlank`, and the zip-writing `Write` loop. Replace the EPUB/KEPUB differences with `Variant` fields: `TextTemplate` (page template), `KoboStyle` (inject `<meta name="kobo-style" content="kobostyle"/>`), `AppleBooks` (write `META-INF/com.apple.ibooks.display-options.xml`), `EscapeTitle` (wrap `html.EscapeString(title)`), `Extension`. The writer opens `epubzip.NewStorageImageReader(opts.ImgStorage())` itself.

- [ ] **Step 3: EPUB as a thin adapter**
  `pkg/epub/epub.go`: `New(opts)` returns `*epub` (per T1 deprecation it may keep the `EPUB` interface). `Write(ctx)` calls `comic.GetParts`, converts to `[]epubwriter.Part`, and calls `epubwriter.Write(ctx, parts, epubVariant, opts)` where `epubVariant = Variant{Extension:".epub", TextTemplate:epubtemplates.Text, KoboStyle:false, AppleBooks:true, EscapeTitle:true}`.

- [ ] **Step 4: KEPUB as a thin adapter**
  `pkg/comic/output/kepub.go`: keep `KEPUBWriter` + `Format/Extension/SupportsPartSplit`. `Write(ctx, parts, opts)` converts the passed `[]OutputPart` → `[]epubwriter.Part` and calls `epubwriter.Write(ctx, parts, kepubVariant, opts)` where `kepubVariant = Variant{Extension:".kepub.epub", TextTemplate:epubtemplates.KepubText, KoboStyle:true, AppleBooks:false, EscapeTitle:false}`. Delete the duplicated `xmlEscape`, `getTree`, `computeViewPort`, `writePart`, `writeCoverImage`, `writeTitleImage`, `writePageImage`, `writeBlank`, `processImages`, `kepubContentData`, `generateContentOPF`, `kepubTextTemplate` (~450 lines).

- [ ] **Step 5: Golden parity tests (generated pre-task)**
  BEFORE Step 2: generate and commit golden `.epub` and `.kepub.epub` ZIP fixtures from the pre-task codebase (one fixed input). After Steps 1-4, add tests asserting the post-task EPUB and KEPUB outputs are byte-identical to those fixtures. Also assert `kepub.go` no longer re-splits parts (its output matches when fed the same `[]OutputPart` that `comic.GetParts` produced).

- [ ] **Step 6: Run tests**
  `go test ./pkg/epub/... ./pkg/comic/output/... ./...`
  Expected: PASS, including golden parity.

- [ ] **Step 7: Commit**
  `git commit -m "refactor: unify EPUB and KEPUB writers behind internal/pkg/epubwriter"`

---

### Task 5: Route EPUB through the shared output-dispatch seam

**Files:**
- Create: `pkg/comic/output/epub.go` (register EPUB as an `OutputWriter`; thin adapter over `pkg/epub`)
- Modify: `pkg/epub/epub.go` (add `WriteParts(ctx, parts []epubwriter.Part, opts) error` so the adapter passes parts through)
- Modify: `main.go` (`generate`: remove `format=="epub"` special case and the duplicated `all` loop)
- Modify: `pkg/comic/converter.go` (`Convert`: remove the "EPUB requires pkg/epub" error; add a `writeAll` fan-out that includes epub; thread the recipe chain)
- Modify: `pkg/comic/output/output.go` (add a way to carry the recipe chain — see Step 2)
- Modify: `pkg/comic/server/server.go` (`runWorker`: route through `comic.New(...).Convert`, default `OutputFormat` to `epub`)
- Modify: `cmd/wasm/main.go` (route through `comic.NewWithRecipe`)

**Interfaces:**
- Produces: `output.Get("epub")` returns an `OutputWriter`. `output.Available()` returns `["cbz","epub","html","kepub"]`.
- Produces: the recipe chain flows into every writer. Add `type ChainCarrier interface { SetRecipe(*filters.Chain) }`; `Convert` type-asserts the returned writer to it and calls `SetRecipe`. `EPUBWriter` and `KEPUBWriter` implement it and pass the chain to `pkg/epub`.

- [ ] **Step 1: Register EPUB as an OutputWriter**
  Create `pkg/comic/output/epub.go`: `EPUBWriter` with `Format() "epub"`, `Extension() ".epub"`, `SupportsPartSplit() true`, and `Write(ctx, parts, opts)` → convert `[]OutputPart` → `[]epubwriter.Part`, call `epub.New(opts).WriteParts(ctx, epubwriterParts, opts)`. Add `init(){ Register(...) }`.

- [ ] **Step 2: Thread the recipe chain**
  In `pkg/comic/output/output.go`, add `type ChainCarrier interface { SetRecipe(*filters.Chain) }`. In `pkg/comic/converter.go` `Convert`/`writeAll`, after `output.Get(format)`, if the writer implements `ChainCarrier`, call `SetRecipe(c.chain)`. Implement `SetRecipe` on `EPUBWriter` and `KEPUBWriter` (store the chain; pass to `pkg/epub` which applies it in its image processor).

- [ ] **Step 3: Simplify `main.go` dispatch**
  Delete the `format=="epub"` special case (main.go:342-355) and the duplicated `all` loop; route every format (including `epub`) through `runSingleFormat`/`comic.Converter`. `-output-format all` fans out via `converter.go`'s `writeAll` (which now includes epub).

- [ ] **Step 4: Simplify `comic.Converter.Convert`**
  Delete the `format==""||format=="epub"` error (converter.go:86-89). Implement `writeAll` to iterate `output.Available()` (now includes epub).

- [ ] **Step 5: Fix the server worker**
  `server.go` `runWorker`: replace `epub.New(opts).Write(ctx)` with `comic.New(opts).Convert(ctx)`; default `opts.OutputFormat` to `"epub"` when empty so the HTTP contract is preserved. Add a test that a submitted job still yields an `.epub`.

- [ ] **Step 6: Fix wasm recipe path**
  `cmd/wasm/main.go`: when `Recipe` is set, use `comic.NewWithRecipe(opts, chain).Convert(ctx)`; else `comic.New(opts).Convert(ctx)`. This makes `-recipe` reach EPUB.

- [ ] **Step 7: Run tests**
  `go test ./...`; assert `output.Available()` contains all four formats and `-output-format all` writes all four.

- [ ] **Step 8: Commit**
  `git commit -m "refactor: route EPUB through the output-dispatch seam; recipe reaches EPUB"`

---

## Self-Review (plan vs report)

- **C1 (three representations → one engine):** Task 2 consolidates into one package + a total (non-lossy) bridge and removes the dead field, but **defers** the risky "single engine" collapse (DefaultChain → recipe) because the two paths have different double-page-split semantics; full collapse would risk pixel drift. Flagged to user.
- **C2 (unify EPUB+KEPUB):** Task 4. ✅
- **C3 (narrow options at processing seam):** Task 2 Step 3 removes the dead `FilterContext.ImageOptions`; the broader "thread a resolved spec everywhere" is scoped out as speculative. Flagged.
- **C4 (route EPUB through dispatch seam):** Task 5. ✅
- **C5 (EPUBImage leak):** Task 3. ✅
- **C6 (delete shallow pass-throughs):** Task 1 deletes internal dead code (registry.go) and **deprecates** (does not delete) the public shims to avoid breaking the importable `pkg/comic`/`pkg/epub` API. Flagged.
