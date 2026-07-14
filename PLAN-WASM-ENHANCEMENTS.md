# WASM Browser App — Enhancement Roadmap

> Generated: 2026-07-14  
> Sources: Plan agent + Architect agent review  
> Based on: PLAN.md "Future enhancements"

---

## Key Discovery: Most Features Are Already Compiled

The architect review confirmed that CBR/RAR, PDF, KEPUB, CBZ, and HTML output are **already compiled into the WASM binary**. All libraries are pure Go with no CGo. The source dispatch (`pkg/comic/source/dispatch.go`) already routes all formats. The output writers are registered via `init()` and compiled in.

**Immediate actions (0 effort, just test + document):**
- CBR/RAR input — test and remove "untested" caveat
- PDF input — test and remove "untested" caveat
- KEPUB, CBZ, HTML output — already selectable in the form dropdown, test and confirm

---

## Priority Ranking (ROI = Value / Effort)

| # | Enhancement | Effort | Dependencies | Value |
|---|-------------|--------|-------------|-------|
| 1 | **CBR/PDF/KEPUB/CBZ/HTML** — test + document | 1 day | None | High (immediate feature parity) |
| 2 | **Web Workers** — non-blocking conversion | 4 days | None | High (UI responsiveness) |
| 3 | **Batch conversion** — multi-file drag-drop | 2 days | Strongly benefits from #2 | Medium |
| 4 | **Service Worker** — offline PWA | 2 days | None | Medium |
| 5 | **Streaming I/O** — reduce memory copies | 3 days | None | Medium (performance) |
| 6 | **HTML inline preview** — view before download | 1 day | None | Low (nice-to-have) |

**Total effort:** 8–13 days for all 6 enhancements.

---

## Enhancement 1: CBR, PDF, KEPUB, CBZ, HTML — Test and Document

**Effort:** 1 day | **Dependencies:** None

The architect confirmed all format libraries are pure Go and compiled into the WASM binary:
- `nwaples/rardecode/v2` — RAR/CBR reading (pure Go)
- `raff/pdfreader` — PDF reading (pure Go)
- `pkg/comic/output/kepub.go`, `cbz.go`, `html.go` — registered via `init()`

### Actions
1. Create small test fixtures (CBZ, CBR, PDF with 2–3 images each)
2. Test each format through the WASM bridge
3. Fix any issues (likely in `cmd/wasm/main.go` output path logic — ensure `.cbr`/`.pdf` → correct output path)
4. Update PLAN.md to remove "untested" caveats

**Risk:** PDF may need virtual FS workaround if `raff/pdfreader`'s `Load()` function only accepts file paths. Mitigation: use Go's virtual FS (write bytes to `/input/file.pdf`, read via `os.Open`) — the virtual FS already supports this.

---

## Enhancement 2: Web Workers — Non-Blocking Conversion

**Effort:** 4 days | **Dependencies:** None

### Goal
Move Go WASM off the main thread so the UI stays responsive during conversion.

### Architecture

```
Main Thread                  Worker Thread
────────────                 ─────────────
app.js                       worker.js
  │                            │
  │── postMessage({file})───▶  │
  │                            ├── self.convert(inputBytes, opts)
  │                            │   (Go WASM runs, blocks worker)
  │                            ├── postMessage({result})
  │◀───────────────────────────│
  │ Blob download              │
```

### Approach
- **Persistent worker pattern**: One Web Worker instantiates WASM once, keeps Go runtime alive via `select {}`
- Go `main()` registers `convert()` via `js.Global().Set()`, then blocks forever
- Worker receives `postMessage` with input data, calls `convert()`, returns result
- UI remains fully responsive — only the worker thread blocks

### Files changed
- Create `wasm/worker.js` — instantiate WASM, host Go runtime, relay messages
- Modify `wasm/app.js` — replace direct `window.convert()` with worker message passing
- Modify `wasm/index.html` — load worker instead of wasm_exec.js directly
- Modify `cmd/wasm/main.go` — minor adjustments if needed (ensure `select {}` works with repeated calls)

### Risk
- Go's WASM runtime processes the JS event loop internally — `onmessage` fires during Go callbacks. This is tested and works.
- 22MB WASM binary loaded once in the worker. Each additional worker duplicates this (~44MB for 2 workers).
- Memory leak after conversion — Go's GC doesn't return WASM memory to the OS. Mitigation: `runtime.GC()` after each conversion.

---

## Enhancement 3: Batch Conversion — Multi-File Drag-Drop

**Effort:** 2 days | **Dependencies:** Strongly benefits from Web Workers

### Goal
Accept multiple files and convert them sequentially. Show per-file progress.

### Actions
1. Update file input to accept `multiple`
2. Switch from `handleFile(file)` to `handleFiles(files[])` with a file queue
3. Implement sequential queue processing in `doConvert()`
4. Add file list UI showing status per file (pending/converting/done/error)
5. With Web Workers: send files through the worker queue (non-blocking, one at a time)
6. Optional: "Download all as ZIP" using JSZip library

### Without Web Workers
Batch conversion without workers blocks the UI for the entire batch duration. Acceptable for small batches but not ideal. **Recommend implementing after Web Workers.**

---

## Enhancement 4: Service Worker — Offline PWA

4. For CBR: more complex — source uses `rardecode.List(filename)` for sorting, which requires file paths. Would need to refactor to use `NewReader()`-based iteration. Skip CBR streaming in initial pass.
5. For PDF: `pdfread.LoadBytes(data)` exists and accepts a byte slice directly. Can skip virtual FS write, similar to CBZ.
Add offline support so the converter works after first visit. Cache the 22MB WASM binary and all static assets.

### Actions
1. Create `wasm/manifest.json` — PWA manifest with icons, display mode
2. Create `wasm/service-worker.js` — cache-first for main.wasm, stale-while-revalidate for HTML/JS
3. Add SW registration and manifest link to `wasm/index.html`
4. Add PWA icons (192×192, 512×512)

### Caching strategy
- `main.wasm` (6MB gzipped): cache-first with versioned URL (`main.v1.wasm`)
- `app.js`, `index.html`: network-first (update on new version)
- `wasm_exec.js`: cache-first (versioned with Go)

---

## Enhancement 5: Streaming I/O — Reduce Memory Copies

**Effort:** 3 days | **Dependencies:** None

### Goal
Eliminate the double-copy of input bytes through the virtual filesystem. Currently: JS→Go memory→virtual FS write→zip reader read. Target: JS→Go memory→zip reader (skip FS write).

### Approach
1. Add `source.NewFromBytes(data []byte, name string, sortMode int)` constructor
2. For CBZ: use `archive/zip.NewReader(bytes.NewReader(data), int64(len(data)))` — note the `size` parameter is required by `NewReader(r io.ReaderAt, size int64)`. No FS write needed.
3. For CBR: more complex — `cbrSource` uses `rardecode.List(filename)` for sorting and discovery, which requires a file path. Would need to refactor to `NewReader()`-based iteration with a different sort strategy. Skip CBR streaming in initial pass.
4. For PDF: `pdfread.LoadBytes(data)` exists and accepts a byte slice directly. Can skip virtual FS write, same as CBZ.
5. Thread through `cmd/wasm/main.go`: pass bytes directly to processor instead of writing to virtual FS.

### Impact
Reduces memory copy from 3x to 1x for CBZ and PDF input. Modest performance gain (CPU-bound image processing dominates). Main win is memory — avoids duplicating input in both JS heap and virtual FS.

**Do not attempt direct `Uint8Array` wrapping** — WASM memory can grow and invalidate JS views. The safety guarantees are insufficient.

---

## Enhancement 6: HTML Inline Preview

**Effort:** 1 day | **Dependencies:** None

### Goal
Add an option to preview HTML output in the browser instead of downloading it.

### Approach
1. Add "HTML Preview" entry to the output format dropdown
2. After conversion, display the HTML in an `<iframe>` instead of triggering a download
3. Show a download button alongside the preview

### Limitation
HTML output embeds images as base64 — a 200-page comic produces a very large HTML file. For previews, only render the first N pages (configurable, default 5) to keep memory bounded.

---

## Implementation Roadmap

### Phase 1 — Feature parity + Workers (5–6 days)
1. Test + document CBR/PDF/KEPUB/CBZ/HTML (1 day)
2. Implement Web Workers (4 days)
3. Add batch conversion (1 day, building on workers)

### Phase 2 — UX + offline (2–3 days)
4. Add Service Worker + PWA manifest (2 days)
5. Add HTML inline preview (1 day)

### Phase 3 — Performance (choose when needed)
6. Implement Streaming I/O for CBZ/CBR (3 days)
