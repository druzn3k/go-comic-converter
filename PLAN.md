# WASM Browser App — Implementation Status

**Status:** Implemented ✅  
**Binary:** `wasm/main.wasm` (22MB uncompressed, ~6MB gzipped)  
**Files:** `cmd/wasm/main.go`, `wasm/index.html`, `wasm/app.js`, `wasm/wasm_exec.js`  
**Build:** `make wasm`  
**Run locally:** `make wasm-serve` then open http://localhost:8080  

---

## Architecture

The Go WASM binary exposes a `window.convert(inputBytes, optionsJSON)` function:

1. **JS side** reads the uploaded file as `Uint8Array`, collects form options as JSON
2. **Go side** writes input to the virtual filesystem, constructs `epuboptions.EPUBOptions` from the JSON, runs the existing pipeline
3. **Go side** reads the output from the virtual filesystem, returns it as `Uint8Array` to JS
4. **JS side** creates a Blob and triggers a download

Progress is reported via `window.onWasmProgress(msg)`.

### Files

| File | Purpose |
|------|---------|
| `cmd/wasm/main.go` | Go WASM entry point (`//go:build js`). Handles all formats, recipes, options. |
| `wasm/index.html` | Full options form: file drop, 30+ options, profile selector, recipe dropdown. |
| `wasm/app.js` | JS glue: WASM loading, drag-n-drop, form collection, progress, download. |
| `wasm/main.wasm` | Compiled WASM binary (~22MB). |
| `wasm/memfs.js` | In-memory filesystem polyfill for Go WASM |
| `wasm/worker.js` | Web Worker hosting Go WASM runtime |
| `wasm/service-worker.js` | Offline PWA cache (cache-first for WASM) |
| `wasm/manifest.json` | PWA manifest |
| `wasm/version.json` | Content-hash WASM URL mapping (build artifact) |
| `wasm/icons/` | PWA icons (192×192, 512×512) |
| `wasm/wasm_exec.js` | Go WASM runtime (copied from `$(go env GOROOT)/lib/wasm/`). |

### Makefile

```
make wasm        # Build wasm/main.wasm + copy wasm_exec.js
make wasm-serve  # Start local HTTP server on :8080
```

---

## What works

| Feature | Status |
|---------|--------|
| CBZ → EPUB | ✅ |
| CBZ → CBZ | ✅ |
| CBZ → KEPUB | ✅ |
| CBZ → HTML | ✅ |
| All image formats (JPEG, PNG, WebP) | ✅ |
| All filter options (crop, contrast, resize, grayscale, etc.) | ✅ |
| Recipe system (5 built-in recipes) | ✅ |
| ComicInfo.xml metadata | ✅ |
| Device profiles (8 profiles) | ✅ |
| CBR/RAR input | ✅ Tested — `cbr_load_test.go` + fixture |
| PDF input | ✅ Tested — `pdf_load_test.go`, `pdf_safety_test.go` + fixture |
| Drag-n-drop upload | ✅ |
| Progress indicator | ✅ |
| Error handling | ✅ |
| Mobile-responsive layout | ✅ |

---

## Binary size

| Compression | Size |
|-------------|------|
| Uncompressed | 22 MB |
| gzip | ~6 MB |
| brotli | ~5 MB |

First load downloads ~5–6 MB. After browser cache, subsequent loads are instant.

---

## Implemented enhancements

All phases from [PLAN-WASM-ENHANCEMENTS.md](PLAN-WASM-ENHANCEMENTS.md) are implemented:

| Phase | Enhancement | Status |
|-------|-------------|--------|
| 0 | Memfs polyfill — in-memory filesystem for browser | ✅ |
| 1 | Output path fix — correct extension per format | ✅ |
| 2 | Web Workers — non-blocking conversion | ✅ |
| 3 | Batch conversion — multi-file drag-drop queue | ✅ |
| 4 | Service Worker + PWA — offline support | ✅ |
| 5 | Streaming I/O — bytes directly to source readers | ✅ |
| 6 | HTML inline preview — sandboxed iframe | ✅ |

---

## WASM E2E (Playwright)

End-to-end tests for the WASM browser app live in `wasm/e2e/`. They verify:

- P2 error handler: per-file errors don't leak to the global banner
- CBR input: happy-path conversion to EPUB with download
- PDF input: happy-path conversion and graceful handling of unsupported encodings

**Setup** (requires a built WASM binary):

    make wasm
    cd wasm/e2e && npm ci && npx playwright install --with-deps chromium

**Run locally:**

    # Terminal 1: serve the app
    make wasm-serve

    # Terminal 2: run tests
    cd wasm/e2e && npx playwright test

**CI:** the `wasm-e2e` job in `.github/workflows/ci.yml` builds the WASM binary,
generates fixtures, installs Playwright, and runs the full suite against
Chromium. On failure, the Playwright report is uploaded as an artifact.
