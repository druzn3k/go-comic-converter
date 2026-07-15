// worker.js — Go WASM Web Worker for go-comic-converter
// Runs go-comic-converter off the main thread so the UI stays responsive.

importScripts('wasm_exec.js', 'memfs.js');

let goInstance = null;
let wasmReady = false;

// ---- progress callback for Go side ----
// Go calls js.Global().Call("onWasmProgress", msg) which resolves to this.
globalThis.onWasmProgress = function (msg) {
  self.postMessage({ type: 'progress', message: String(msg) });
};

async function initWasm() {
  try {
    const go = new Go();
    goInstance = go;

    // Fetch using the versioned WASM binary if available, otherwise main.wasm
    let wasmUrl = 'main.wasm';
    try {
      const resp = await fetch('version.json');
      const version = await resp.json();
      if (version.wasm) wasmUrl = version.wasm;
    } catch (_) {
      // No version.json — use default
    }

    const result = await WebAssembly.instantiateStreaming(
      fetch(wasmUrl),
      go.importObject
    );
    go.run(result.instance);
    wasmReady = true;
    self.postMessage({ type: 'ready' });
  } catch (err) {
    // Fallback for browsers without streaming support
    try {
      const resp = await fetch('main.wasm');
      const bytes = await resp.arrayBuffer();
      const result = await WebAssembly.instantiate(bytes, go.importObject);
      go.run(result.instance);
      wasmReady = true;
      self.postMessage({ type: 'ready' });
    } catch (err2) {
      self.postMessage({ type: 'error', error: 'Failed to load WASM: ' + err2.message });
    }
  }
}

self.addEventListener('message', function (e) {
  const msg = e.data;

  if (msg.type === 'convert') {
    if (!wasmReady) {
      self.postMessage({ type: 'error', id: msg.id, error: 'WASM not ready yet' });
      return;
    }

    try {
      // Parse options — main thread sends them as a JSON string
      var opts;
      if (typeof msg.options === 'string') {
        opts = JSON.parse(msg.options);
      } else if (typeof msg.options === 'object' && msg.options !== null) {
        opts = msg.options;
      } else {
        opts = {};
      }
      opts.inputName = msg.filename || 'input';

      // Read input data — main thread sends it as inputData (ArrayBuffer)
      var inputBytes = msg.inputData || msg.file;
      var inputArray = new Uint8Array(inputBytes);

      // Call the Go convert function
      var result = globalThis.convert(inputArray, JSON.stringify(opts));

      if (typeof result === 'string' && result.startsWith('error:')) {
        self.postMessage({ type: 'error', id: msg.id, error: result.substring(6) });
        return;
      }

      // Read result: { data: Uint8Array, filename: string }
      var data = result.data;
      var filename = result.filename || 'output.epub';

      // Transfer the ArrayBuffer to main thread (zero-copy)
      self.postMessage(
        {
          type: 'result',
          id: msg.id,
          data: data.buffer,
          filename: filename,
          mimeType: getMimeType(filename),
        },
        [data.buffer]
      );
    } catch (err) {
      self.postMessage({ type: 'error', id: msg.id, error: err.message });
    }
  }
});

function getMimeType(filename) {
  if (filename.endsWith('.epub') || filename.endsWith('.kepub.epub')) return 'application/epub+zip';
  if (filename.endsWith('.cbz')) return 'application/vnd.comicbook+zip';
  if (filename.endsWith('.html')) return 'text/html';
  return 'application/octet-stream';
}

initWasm();
