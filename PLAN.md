# PLAN: WASM Browser App

**Status:** Draft — feasibility confirmed, all dependencies are pure Go and compile to WASM.
**Goal:** Browser-based comic converter using Go WASM — user drops a CBZ, tweaks options, downloads an EPUB.

---

## Feasibility (verified)

`GOOS=js GOARCH=wasm go build .` compiles the entire project with zero changes.
- All dependencies are pure Go (no CGo)
- Image codecs (JPEG, PNG, WebP), ZIP (archive/zip), XML, templates — all work in WASM
- The conversion pipeline (`epub.New(opts).Write()`) runs unchanged

**What runs in WASM:**
- CBZ/ZIP reading, image decode, all filter transforms, image encode
- EPUB assembly (ZIP + XML + XHTML templates)
- Recipe system, ComicInfo.xml, all output formats

**What does NOT run in WASM:**
- HTTP server mode (`-serve`) — `net.Listen` fails in browser WASM
- Watch mode (`-watch`) — no filesystem notifications in browser
- CBR/RAR untested but expected to work (`nwaples/rardecode` is pure Go)
- PDF untested but expected to work (`raff/pdfreader` is pure Go)

---

## Architecture

```
┌──────────────────────────────────────────────────┐
│  Browser (HTML + CSS + JS)                       │
│  ┌──────────┐  ┌──────────────┐  ┌───────────┐   │
│  │ File drop │  │ Options form │  │ Download  │   │
│  │ zone      │  │ (~30 fields) │  │ button    │   │
│  └─────┬─────┘  └──────┬───────┘  └─────▲─────┘   │
│        │               │                 │         │
│        ▼               ▼                 │         │
│  ┌────────────────────────────────────────────┐   │
│  │  Go WASM (main.wasm, ~8–15 MB)             │   │
│  │                                             │   │
│  │  jsBridge.convert(inputBytes, options) {    │   │
│  │    // Write input to virtual filesystem     │   │
│  │    os.WriteFile("/in/file.cbz", input, 0644)│   │
│  │    // Build EPUBOptions from JS options     │   │
│  │    opts := buildOptions(options)             │   │
│  │    opts.Input = "/in/file.cbz"              │   │
│  │    opts.Output = "/out/file.epub"           │   │
│  │    // Run pipeline (unchanged)              │   │
│  │    err := epub.New(opts).Write(ctx)          │   │
│  │    // Read output back to JS                │   │
│  │    data, _ := os.ReadFile("/out/file.epub") │   │
│  │    return data as Uint8Array                │   │
│  │  }                                          │   │
│  └────────────────────────────────────────────┘   │
│                                                   │
│  wasm_exec.js (Go JS runtime, from GOROOT)        │
└──────────────────────────────────────────────────┘
```

### Data flow

1. User drops `.cbz` file (or selects via `<input type="file">`)
2. JavaScript reads file as `Uint8Array`
3. Calls `window.convert(inputBytes, options)` (Go function exported via `syscall/js`)
4. Go WASM writes input to its virtual filesystem, runs the existing `epub.New(opts).Write()` pipeline
5. Go WASM reads the output EPUB from virtual filesystem, returns as `Uint8Array`
6. JavaScript creates a Blob and triggers a download link

### WASM entry point (`main.go`)

```go
package main

import (
    "context"
    "os"
    "syscall/js"
    "github.com/druzn3k/go-comic-converter/v3/pkg/epub"
    "github.com/druzn3k/go-comic-converter/v3/pkg/epuboptions"
)

func main() {
    c := make(chan struct{}, 0)
    js.Global().Set("convert", js.FuncOf(func(this js.Value, args []js.Value) any {
        inputBytes := js.CopyBytesToGo(make([]byte, args[0].Get("length").Int()), args[0])
        os.WriteFile("/input/file.cbz", inputBytes, 0644)

        opts := epuboptions.EPUBOptions{
            Input:  "/input/file.cbz",
            Output: "/output/file.epub",
            // Options from JS args[1]...
        }

        if err := epub.New(opts).Write(context.Background()); err != nil {
            return err.Error()
        }
        data, _ := os.ReadFile("/output/file.epub")
        dst := js.Global().Get("Uint8Array").New(len(data))
        js.CopyBytesToJS(dst, data)
        return dst
    }))
    <-c
}
```

### Bridge optimization

The naive approach copies every byte through WASM linear memory twice. For a 300MB output this is 600MB of traffic through the JS/WASM boundary — acceptable for desktop but slow on mobile. Future optimization: stream through `io.Reader`/`io.Writer` backed by JS `Uint8Array` to avoid intermediate buffers.

---

## Implementation plan

### Phase 1 — Scaffold (~1 day)

| Task | Files | Effort |
|------|-------|--------|
| WASM entry point | `cmd/wasm/main.go` | 0.5 d |
| Build script | `Makefile` target for `GOOS=js GOARCH=wasm` | 0.25 d |
| Static assets | `wasm/` directory with `index.html`, `wasm_exec.js` (copied from GOROOT) | 0.25 d |

### Phase 2 — CBZ→EPUB bridge (~2 days)

| Task | Files | Effort |
|------|-------|--------|
| File upload + JS → WASM bridge | `wasm/index.html` + `wasm/app.js` | 1 d |
| Convert button + download | `wasm/app.js` | 0.5 d |
| Progress indicator | `wasm/index.html` CSS + JS polling | 0.5 d |

### Phase 3 — Options form (~2 days)

| Task | Effort |
|------|--------|
| Basic options (quality, grayscale, format, crop, resize, profile) | 1 d |
| Advanced options (brightness, contrast, auto-rotate, split, manga, etc.) | 0.5 d |
| Recipe selector (builtin dropdown) | 0.5 d |

### Phase 4 — Polish (~1 day)

| Task | Effort |
|------|--------|
| Error handling (invalid CBZ, corrupt images) | 0.5 d |
| Mobile-responsive layout | 0.25 d |
| CI: build + deploy wasm to GitHub Pages | 0.25 d |

**Total: ~5 days for basic CBZ→EPUB, ~6 days with full options.**

---

## Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Binary size (8–15MB) | High | Slow first load | Compress with `wasm-opt`, show loading bar |
| Memory (500MB+ for large CBZs) | Medium | OOM on 4GB devices | Show memory estimate before conversion |
| CBR/PDF untested in WASM | Low | Feature gap | Test and document; ship CBZ-first |
| Single-threaded perf | Medium | 2–3x slower conversion | Acceptable for browser use; add Web Worker support later |
| Go 1.26 WASM API changes | Low | Breakage | Pin Go version in CI |

---

## Future enhancements

- **Web Workers**: run conversion off the main thread (non-blocking UI)
- **Streaming I/O**: avoid double-copy through WASM memory via direct `Uint8Array` buffers
- **Drag-n-drop multiple files**: batch conversion
- **More input formats**: CBR, PDF, directory (drag a folder)
- **More output formats**: KEPUB, CBZ, HTML preview in browser
- **Service Worker**: offline-capable PWA
