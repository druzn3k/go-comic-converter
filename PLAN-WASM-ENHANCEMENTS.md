# WASM Browser App — Enhancement Roadmap

> Generated: 2026-07-15  
> Revision: 3 — incorporates two architect review cycles  
> Status: Implemented 2026-07-15 — all phases (0–6) complete

---

## Critical Discovery: Virtual Filesystem Is Broken in Browser

The original plan assumed Go's virtual filesystem (`os.MkdirAll`, `os.WriteFile`, `os.ReadFile`, `os.Stat`) works in the browser. **It does not.**

Go's standard `wasm_exec.js` stubs every filesystem operation to `enosys()` — `open`, `mkdir`, `stat`, `read`, `close`, `readdir` all return "not implemented". Only `write`/`writeSync` (stdout/stderr logging) work:

```js
// wasm/wasm_exec.js (Go 1.26)
mkdir(path, perm, callback) { callback(enosys()); },
open(path, flags, mode, callback) { callback(enosys()); },
read(fd, buffer, offset, length, position, callback) { callback(enosys()); },
stat(path, callback) { callback(enosys()); },
close(fd, callback) { callback(enosys()); },
fstat(fd, callback) { callback(enosys()); },
lstat(path, callback) { callback(enosys()); },
unlink(path, callback) { callback(enosys()); },
// ...
```

**Consequence:** `cmd/wasm/main.go` fails on its first `os.MkdirAll("/input", 0755)` call. Every source dispatch (`zip.OpenReader`, `rardecode.List`, `pdfread.Load`) and all output reading (`os.ReadFile`) fail transitively. **The WASM app cannot function in a browser as-is.**

**Solution:** A memfs polyfill monkey-patching the broken `globalThis.fs` methods with in-memory storage is a prerequisite for all other work.

---

## Priority Ranking

| # | Enhancement | Effort | Dependencies | Value |
|---|-------------|--------|-------------|-------|
| 0 | **Memfs polyfill** — make FS work in browser | 1–2d | None (prerequisite for everything) | **Critical — unblocks all** |
| 1 | **CBR/PDF/KEPUB/CBZ/HTML** — test + fix output path bug | 1d | #0 | High (feature parity) |
| 2 | **Web Workers** — non-blocking conversion | 5d | #0 | High (UI responsiveness) |
| 3 | **Batch conversion** — multi-file drag-drop | 2d | #0 (benefits from #2) | Medium |
| 4 | **Service Worker** — offline PWA | 2d | #0, build tooling | Medium |
| 5 | **Streaming I/O** — reduce memory copies | 5d | #0 | Medium (performance) |
| 6 | **HTML inline preview** — view before download | 2d | #0 | Low (nice-to-have) |

**Total effort:** 13–18 days for all 7 items.

---

## Phase 0: Memfs Polyfill (Prerequisite)

**Effort:** 1–2 days | **Dependencies:** None

### Goal
Provide a working in-memory filesystem so the Go WASM binary can read/write files in the browser. Without this, nothing else works.

### Approach: Monkey-patch after wasm_exec.js

Load `wasm_exec.js` **first** — it sets up `globalThis.fs` with all stubs, `constants`, `process`, `path`, and the working `writeSync`/`write` for stdout/stderr. Then load `memfs.js` which overrides only the broken methods with in-memory implementations.

This preserves:
- `fs.constants` (O_WRONLY, O_RDWR, O_CREAT, etc.) — Go's syscall init reads these at package init time; missing them causes a panic
- `fs.writeSync` / `fs.write` for fds 1 and 2 (stdout/stderr logging)
- `process`, `path`, `crypto` objects

### Syscall surface Go's runtime needs

| Method | Required by | Notes |
|--------|-------------|-------|
| `open(path, flags, mode)` → fd | `os.WriteFile`, `os.ReadFile`, `os.Open`, `os.Stat` (via open + fstat) | Parse flags to create/truncate/append |
| `close(fd)` | cleanup after every open | |
| `read(fd, buf, offset, length, position)` | `os.ReadFile`, `zip.OpenReader` | `position` is `null` for sequential (`os.Read`), number for random access (`zip.Reader`) |
| `write(fd, buf, offset, length, position)` | `os.WriteFile` | Same position semantics as read |
| `stat(path)` → result | `source.New` (`os.Stat` dispatch) | Must return mode, size, mtime |
| `fstat(fd)` → result | Go runtime internals, zip/rar readers | Must return mode, size, mtime |
| `lstat(path)` → result | Go runtime internals | Same as stat |
| `mkdir(path, perm)` | `os.MkdirAll` | Must create parent dirs implicitly |
| `readdir(path)` | directory source (rare in WASM) | Returns file names in dir |
| `unlink(path)` | cleanup temp files | |
| `fsync(fd)` | — | Already stubbed as `callback(null)` — keep as-is |

### Per-fd offset tracking for sequential reads/writes

When `position` is `null` (sequential mode), each fd has a current offset that advances with every read/write. When `position` is a number (random access / Pread/Pwrite), use it directly without modifying the fd offset.

The wasm_exec.js `write` stub explicitly checks `position !== null` before returning ENOSYS, confirming this parameter carries meaningful semantics.

### Implementation sketch

```js
// wasm/memfs.js — loaded AFTER wasm_exec.js
// Overrides only the broken globalThis.fs methods with in-memory storage.
(function() {
  const fs = globalThis.fs;
  if (!fs) { console.error('memfs: wasm_exec.js not loaded first'); return; }

  const store = new Map();    // path → { data: Uint8Array, mode }
  const fds = new Map();      // fd → { path, offset, flags, mode }
  let nextFd = 3;

  function enosys() { const e = new Error('not implemented'); e.code = 'ENOSYS'; return e; }

  // Preserve existing write/writeSync for fds 0-2 (stdin/stdout/stderr)
  const origWrite = fs.write.bind(fs);
  const origWriteSync = fs.writeSync.bind(fs);

  fs.open = function(path, flags, mode, callback) {
    // Flags: O_RDONLY=0, O_WRONLY=1, O_RDWR=2, O_CREAT=64, O_TRUNC=512, O_APPEND=1024
    const fd = nextFd++;
    const existing = store.get(path);
    let data = existing ? existing.data : new Uint8Array(0);
    if ((flags & 512) !== 0) data = new Uint8Array(0); // O_TRUNC
    store.set(path, { data, mode });
    fds.set(fd, { path, offset: 0, flags, mode });
    callback(null, fd);
  };

  fs.close = function(fd, callback) {
    if (fd < 3) { callback(enosys()); return; }
    fds.delete(fd);
    callback(null);
  };

  fs.read = function(fd, buffer, offset, length, position, callback) {
    if (fd < 3) { callback(enosys()); return; }
    const entry = fds.get(fd);
    if (!entry) { callback(new Error('bad fd')); return; }
    const pos = (position === null) ? entry.offset : position;
    const fileData = store.get(entry.path).data;
    const end = Math.min(pos + length, fileData.length);
    const chunk = fileData.slice(pos, end);
    buffer.set(chunk, offset);
    if (position === null) entry.offset += chunk.length;
    callback(null, chunk.length);
  };

  fs.write = function(fd, buffer, offset, length, position, callback) {
    if (fd < 3) { origWrite(fd, buffer, offset, length, position, callback); return; }
    const entry = fds.get(fd);
    if (!entry) { callback(new Error('bad fd')); return; }
    const pos = (position === null) ? entry.offset : position;
    const fileData = store.get(entry.path);
    const newLen = Math.max(fileData.data.length, pos + length);
    const newData = new Uint8Array(newLen);
    newData.set(fileData.data, 0);
    newData.set(buffer.slice(offset, offset + length), pos);
    fileData.data = newData;
    if (position === null) entry.offset += length;
    callback(null, length);
  };

  function statResult(path) {
    const entry = store.get(path);
    if (!entry) return null;
    return {
      dev: 1, nlink: 1, rdev: 0, blksize: 4096, ino: 0,
      mode: entry.mode || 0o644,
      uid: 0, gid: 0, size: entry.data.length,
      atimeMs: Date.now(), mtimeMs: Date.now(), ctimeMs: Date.now(),
      isDirectory: () => false,
    };
  }

  fs.stat = function(path, callback) {
    const r = statResult(path);
    if (!r) { const e = new Error('ENOENT'); e.code = 'ENOENT'; callback(e); return; }
    callback(null, r);
  };

  fs.lstat = function(path, callback) {
    fs.stat(path, callback);
  };

  fs.fstat = function(fd, callback) {
    if (fd < 3) { callback(enosys()); return; }
    const entry = fds.get(fd);
    if (!entry) { callback(new Error('bad fd')); return; }
    const r = statResult(entry.path);
    if (!r) { callback(new Error('ENOENT')); return; }
    callback(null, r);
  };

  fs.mkdir = function(path, perm, callback) {
    // Create dirs implicitly — Go often does MkdirAll
    const parts = path.replace(/^\/+/, '').split('/');
    let acc = '';
    for (const p of parts) {
      if (!p) continue;
      acc += '/' + p;
      if (!store.has(acc)) {
        store.set(acc, { data: new Uint8Array(0), mode: perm || 0o755 });
      }
    }
    callback(null);
  };

  fs.readdir = function(path, callback) {
    const entries = [];
    const prefix = path.endsWith('/') ? path : path + '/';
    for (const key of store.keys()) {
      if (key.startsWith(prefix) && key.indexOf('/', prefix.length) === -1) {
        entries.push(key.slice(prefix.length));
      }
    }
    callback(null, entries);
  };

  fs.unlink = function(path, callback) {
    store.delete(path);
    callback(null);
  };

  // All other methods (chmod, chown, lchown, rename, truncate, link,
  // symlink, readlink, rmdir, utimes) keep their existing enosys stubs
  // from wasm_exec.js — they are not called by Go's syscall path in WASM.
})();
```

### Actions
1. Create `wasm/memfs.js` with the above monkey-patch implementation
2. Add `<script src="memfs.js"></script>` to `wasm/index.html` **after** `wasm_exec.js`
3. Smoke-test: open the app in browser, verify `convert()` runs without FS errors, produces a download
4. If successful, all subsequent enhancements can be validated in-browser

### Verification
- Browser devtools console shows no ENOSYS errors
- Conversion produces a downloadable file
- `os.MkdirAll`, `os.WriteFile`, `os.ReadFile` in Go side complete without error
- Progress callbacks appear in console (stdout logging preserved)

---

## Phase 1: CBR, PDF, KEPUB, CBZ, HTML — Test and Fix Output Path Bug

**Effort:** 1 day | **Dependencies:** Phase 0 (memfs)

All format libraries are pure Go and compiled into the WASM binary:
- `nwaples/rardecode/v2` — RAR/CBR reading (pure Go)
- `raff/pdfreader` — PDF reading (pure Go)
- `pkg/comic/output/kepub.go`, `cbz.go`, `html.go` — registered via `init()`

### Known bug: Output extension mismatch in cmd/wasm/main.go

`cmd/wasm/main.go` line 82 always sets `outputPath := "/output/" + baseName + ".epub"`. The CBZ and HTML output writers write directly to `opts.Output` — which is `*.epub`. But the readback code (lines ~214–221) adjusts the read path to `*.cbz` or `*.html`, causing `os.ReadFile` to fail because the file exists at `*.epub`.

KEPUB is unaffected because its writer strips the extension and appends `.kepub.epub` regardless.

**Fix:** Set `opts.Output` with the correct extension per format before calling the writer, or use the return value of `Writer.Write()` (which returns `[]string` of output paths) for the readback path.

### Actions
1. Create small test fixtures (CBZ, CBR, PDF with 2–3 images each) in `wasm/testdata/`
2. Test each format through the WASM bridge in-browser
3. **Fix the output extension mismatch** in `cmd/wasm/main.go` so CBZ and HTML output can be read back
4. Update PLAN.md to remove "untested" caveats

---

## Phase 2: Web Workers — Non-Blocking Conversion

**Effort:** 5 days | **Dependencies:** Phase 0

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
- **Must verify** that `select {}` allows repeated `js.FuncOf` callbacks; if not, fall back to `for { runtime.Gosched() }` or a `time.Sleep` polling loop

### Files changed
- Create `wasm/worker.js` — instantiate WASM, host Go runtime, relay messages
- Modify `wasm/app.js` — replace direct `window.convert()` with worker message passing
- Modify `wasm/index.html` — load worker instead of wasm_exec.js directly
- Modify `cmd/wasm/main.go` — ensure `select {}` (or equivalent) works with repeated calls

### Risk
- The `select {}` + `postMessage` / `js.FuncOf` callback cycle must be explicitly tested with the 22MB binary — Go WASM's event loop integration is real but this specific composition hasn't been verified in this codebase
- 22MB WASM binary loaded once in the worker. Each additional worker duplicates this (~44MB for 2 workers).
- Memory bloat after repeated conversions — Go's GC doesn't return WASM heap to the OS. Mitigation: `runtime.GC()` after each conversion.
- `memfs.js` state is per-worker and must be independent for each worker scope.

---

## Phase 3: Batch Conversion — Multi-File Drag-Drop

**Effort:** 2 days | **Dependencies:** Phase 0 (benefits from Phase 2 but not blocked by it)

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
Batch conversion without workers blocks the UI for the entire batch duration. Acceptable for small batches but not ideal. **Implement after Web Workers if UI responsiveness matters.**

---

## Phase 4: Service Worker — Offline PWA

**Effort:** 2 days | **Dependencies:** Phase 0, build tooling

### Goal
Add offline support so the converter works after first visit. Cache the 22MB WASM binary and all static assets.

### Caching strategy
- `main.wasm` (6MB gzipped): cache-first with **content-hash URL** (`main.<sha256>.wasm`)
- `app.js`, `index.html`: network-first (update on new version)
- `wasm_exec.js`, `memfs.js`: cache-first (versioned with Go)

### Build tooling changes
Manual versioning like `main.v1.wasm` is brittle and prone to stale-cache bugs. Instead:
- Modify `Makefile` to compute SHA256 of `main.wasm` and copy as `main.<hash>.wasm`
- Generate `wasm/version.json` containing the hash mapping for the SW to resolve
- The service worker reads `version.json` to know which binary URL to cache-first

### Actions
1. Create `wasm/manifest.json` — PWA manifest with icons, display mode
2. Create `wasm/service-worker.js` — cache-first for main.wasm, stale-while-revalidate for HTML/JS
3. Add SW registration and manifest link to `wasm/index.html`
4. Add PWA icons (192×192, 512×512)
5. Modify `Makefile` to content-hash the wasm binary and produce `version.json`

### Risk
- 6MB gzipped may exceed some browser cache quotas (typically 50MB+ on modern browsers, but Safari is conservative)
- Service workers cannot cache cross-origin resources without the right CORS headers
- SW install/activate lifecycle can race with initial WASM loading — must handle the case where SW isn't yet active on first visit

---

## Phase 5: Streaming I/O — Reduce Memory Copies

**Effort:** 5 days | **Dependencies:** Phase 0

### Goal
Eliminate the double-copy of input bytes through the virtual filesystem. The original plan only addressed input; this revision covers both sides.

### Current data flow (6 memory copies)
```
JS Uint8Array ──copy1──▶ Go []byte
Go []byte ──copy2──▶ memfs (os.WriteFile)
memfs ──copy3──▶ zip/rar/pdf reader (os.Open)
                         │
                 [image processing]
                         │
                 Output write (os.WriteFile) ──copy4──▶ memfs
                 memfs ──copy5──▶ os.ReadFile
                 Go []byte ──copy6──▶ JS Uint8Array
```

### Input side (target: skip copies 2–3)

Add `source.NewFromBytes(data []byte, name string, sortMode int)` constructor (or modify dispatch.go to accept bytes).

- **CBZ:** `archive/zip.NewReader(bytes.NewReader(data), int64(len(data)))` — `bytes.NewReader` implements `io.ReaderAt`. **No FS write needed.**
- **CBR:** `rardecode.NewReader(bytes.NewReader(data), ...opts)` accepts an `io.Reader`. **Full rewrite of `cbrSource.Load` required** — the current code uses `rardecode.List(filename)` (file path) and `rardecode.OpenReader(filename)` (file path) with different solid/non-solid paths. The two-pass strategy:
  1. Pass 1: `NewReader` → `r.Next()` loop to collect file names
  2. Sort names
  3. Pass 2: new `NewReader` from same `[]byte` → extract in sorted order
  - Solid archives: pass 2 is sequential anyway (all entries read from one stream); sorting only affects the final output order, not extraction order
  - Non-solid: `Reader.File` entries can be opened individually from the second reader
  - This is effectively a rewrite of the ~90-line `Load()` method including the feeder goroutine
- **PDF:** `pdfread.LoadBytes(data)` exists and accepts `[]byte`. **No FS write needed.**

### Output side (target: skip copies 4–5)

Even with input streaming, the output pipeline writes to `opts.Output` (a file path) and is read back via `os.ReadFile`.

- **Option A (simpler):** Keep memfs for output — the 2 output copies remain but are unavoidable without API changes. After Phase 0 this already works.
- **Option B (better, more effort):** Add an output-to-bytes path. Either:
  - Add `WriteToWriter(ctx, parts, opts, io.Writer)` to `OutputWriter` interface
  - Or thread a `bytes.Buffer` through the pipeline instead of a file path

**Recommendation:** Start with Option A. Revisit Option B if output memory becomes a bottleneck. The main win is input streaming.

### Impact
Input side: CBZ/PDF input copies reduce from 2x (JS→Go + FS) to 1x (JS→Go). CBR also possible after source rewrite.
Output side: no change initially (2x through memfs).

**Do not attempt direct `Uint8Array` wrapping** — WASM memory can grow and invalidate JS views.

---

## Phase 6: HTML Inline Preview

**Effort:** 2 days | **Dependencies:** Phase 0

### Goal
Add an option to preview HTML output in the browser instead of downloading it.

### Approach
1. Add "HTML Preview" entry to the output format dropdown
2. After conversion, display the HTML in an `<iframe srcdoc="...">` instead of triggering a download
3. Show a download button alongside the preview

### Page-limiting constraint
The plan proposes rendering only the first N pages (default 5) to keep memory bounded. The pipeline currently processes ALL images before writing — there is no mechanism to stop early.

- **Option A (simpler, wasted CPU):** Process all pages (full conversion), truncate `[]OutputPart` slice before `Write()`. Wasted CPU for large comics but requires zero pipeline changes.
- **Option B (harder, correct):** Add a `MaxPages` field to `epuboptions.EPUBOptions`. The processor's `Load()` or source iteration stops early. More effort, saves CPU.

**Recommendation:** Start with Option A, optimize to Option B if page-limiting proves valuable.

### Limitation
HTML output embeds images as base64 — a 200-page comic produces >50MB HTML. With 5-page limit this is ~1–2MB. The iframe `srcdoc` approach works for this size.

---

## Implementation Roadmap

### Phase 0 — Foundation (1–2 days)
0. Implement memfs polyfill (monkey-patch after wasm_exec.js), smoke-test base app

### Phase 1 — Feature Parity (1 day)
1. Test CBR/PDF/KEPUB/CBZ/HTML + fix output extension mismatch in `cmd/wasm/main.go`

### Phase 2 — Responsiveness (5 days)
2. Implement Web Workers (non-blocking conversion)
3. Batch conversion on top of workers

### Phase 3 — UX + Offline (3–4 days)
4. Service Worker + PWA manifest
5. HTML inline preview (Option A first)

### Phase 4 — Performance (5 days, deferrable)
6. Streaming I/O for CBZ/CBR/PDF input (output via memfs)
7. Optionally add output-to-bytes path

---

## Key Risks Summary

| Risk | Impact | Mitigation |
|------|--------|------------|
| Memfs polyfill may miss edge cases Go's runtime exercises (e.g., `constants`, null position) | All phases blocked | Start with the monkey-patch approach above, add syscalls as Go errors reveal them |
| Web Worker `select{}` + repeated `js.FuncOf` calls untested in this codebase | Phase 2 blocked | Prototype standalone first; fallback to `for { runtime.Gosched() }` busy loop |
| 22MB WASM + memfs memory pressure in browser tabs | Poor UX on low-end devices | Profile after Phase 0; consider lazy loading or smaller WASM via dead-code elimination |
| CBR two-pass streaming requires full `cbrSource.Load` rewrite (~90 lines) | Phase 5 effort understated | Budget 2 extra days for CBR streaming vs estimate |
| Service Worker cache quotas on Safari/iOS | Offline fails on some devices | Detect via `navigator.storage.estimate()`; show warning if insufficient |
| Content-hash build tooling adds complexity to Makefile | Build pipeline changes | Keep it simple: `sha256sum main.wasm | head -c 16` for short hash |
