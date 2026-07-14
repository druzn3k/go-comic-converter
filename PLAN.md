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
2. **Go side** writes input to the virtual filesystem (`/input/<name>.cbz`), constructs `epuboptions.EPUBOptions` from the JSON, runs the existing pipeline (`epub.New(opts).Write(ctx)` or `comic.New(opts).Convert(ctx)`)
3. **Go side** reads the output from the virtual filesystem, returns it as `Uint8Array` to JS
4. **JS side** creates a Blob and triggers a download

Progress is reported via `window.onWasmProgress(msg)`.

### Files

| File | Purpose |
|------|---------|
| `cmd/wasm/main.go` | Go WASM entry point (`//go:build js`). Reads input bytes + JSON options from JS, runs pipeline, returns output bytes. Supports EPUB, KEPUB, CBZ, HTML formats, recipe system, all filter options. |
| `wasm/index.html` | Full options form: file drop zone, 30+ options organized in sections (Output, Image Processing, Metadata, Recipe), download result. |
| `wasm/app.js` | JS glue: loads WASM module, handles drag-n-drop, collects form state, calls Go convert(), manages progress/error display, triggers download. |
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
| ComicInfo.xml metadata (series, number, genre, etc.) | ✅ |
| Device profiles (8 profiles) | ✅ |
| CBR/RAR input | ⚠️ Untested (expected to work — pure Go library) |
| PDF input | ⚠️ Untested (expected to work — pure Go library) |
| Drag-n-drop file upload | ✅ |
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

- **Web Workers**: run conversion off the main thread (non-blocking UI)
- **Streaming I/O**: avoid double-copy through WASM memory via direct `Uint8Array` buffers
- **Drag-n-drop multiple files**: batch conversion
- **More input formats**: CBR, PDF, directory (drag a folder)
- **More output formats**: KEPUB, CBZ, HTML preview in browser
- **Service Worker**: offline-capable PWA
