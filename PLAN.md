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
| CBR/RAR input | ⚠️ Compiled, needs testing |
| PDF input | ⚠️ Compiled, needs testing |
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

## Future enhancements

See [PLAN-WASM-ENHANCEMENTS.md](PLAN-WASM-ENHANCEMENTS.md) for the detailed implementation roadmap.

Planned enhancements (by priority):
1. CBR/PDF/KEPUB/CBZ/HTML — test and document (already compiled)
2. Web Workers — non-blocking conversion
3. Batch conversion — multi-file drag-drop
4. Service Worker — offline PWA
5. Streaming I/O — reduce memory copies
6. HTML inline preview
