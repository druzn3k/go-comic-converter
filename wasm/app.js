// app.js — Go WASM bridge for go-comic-converter (Web Worker edition)
//
// Phases implemented:
//   2 — Web Worker keeps the UI responsive during conversion.
//   3 — Batch conversion with file list, sequential queue, status per file.
//   6 — HTML inline preview via sandboxed iframe.
 
// ── state ──────────────────────────────────────────────────────────────────
 
let worker = null;
let workerReady = false;
let isProcessing = false;

let fileIdCounter = 0;
/** @type {Map<number, {name:string, size:number, status:string, error?:string, file:File}>} */
let fileStatuses = new Map();

// ── worker lifecycle ──────────────────────────────────────────────────────

function initWorker() {
  worker = new Worker('worker.js');

  worker.addEventListener('message', function (e) {
    var msg = e.data;

    switch (msg.type) {
      case 'ready':
        workerReady = true;
        document.getElementById('convertBtn').disabled = false;
        document.getElementById('dropzone-text').textContent =
          'Drop comic files here, or click to browse';
        console.log('Worker ready');
        break;

      case 'progress':
        document.getElementById('progress-text').textContent = msg.message;
        break;

      case 'error':
        // Skip per-file errors (have id) — handled by processQueue's per-item listener
        if (msg.id) break;
        showResult('Error: ' + (msg.error || msg.message || 'unknown'), 'error');
        console.error('Worker error:', msg.error || msg.message);
        break;
     }
  });

  worker.addEventListener('error', function (e) {
    console.error('Worker fatal error:', e.message);
    workerReady = false;
    showResult('WASM worker crashed: ' + (e.message || 'unknown'), 'error');
    if (typeof worker.terminate === 'function') worker.terminate();
  });

  worker.addEventListener('messageerror', function (e) {
    console.error('Worker message error:', e.data);
    workerReady = false;
    showResult('WASM worker message error: ' + (e.data || 'unknown'), 'error');
    if (typeof worker.terminate === 'function') worker.terminate();
  });
 }
 
// ── file handling – multiple files ────────────────────────────────────────

document.getElementById('fileInput').addEventListener('change', function (e) {
  if (e.target.files.length > 0) {
    handleFiles(Array.from(e.target.files));
    e.target.value = ''; // allow re-selection of same file(s)
  }
 });
 
document.getElementById('dropzone').addEventListener('dragover', function (e) {
  e.preventDefault();
  e.currentTarget.classList.add('dragover');
 });
 
document.getElementById('dropzone').addEventListener('dragleave', function (e) {
  e.preventDefault();
  e.currentTarget.classList.remove('dragover');
 });
 
document.getElementById('dropzone').addEventListener('drop', function (e) {
  e.preventDefault();
  e.currentTarget.classList.remove('dragover');
  if (e.dataTransfer.files.length > 0) {
    handleFiles(Array.from(e.dataTransfer.files));
  }
 });
 
function handleFiles(files) {
  files.forEach(function (file) {
    var id = ++fileIdCounter;
    fileStatuses.set(id, { name: file.name, size: file.size, status: 'pending', file: file });
  });

  updateFileList();
  updateDropzone();
 
  // Auto-fill title from the first file if empty
  if (fileStatuses.size > 0) {
    var first = fileStatuses.values().next().value;
    var titleInput = document.getElementById('title');
     if (!titleInput.value) {
      titleInput.value = first.name.replace(/\.(cbz|zip|cbr|rar|pdf)$/i, '');
     }
  }

  document.getElementById('convertBtn').disabled = !workerReady;
}

function clearFiles() {
  fileStatuses.clear();
  updateFileList();
  updateDropzone();
}
 
function updateDropzone() {
  var dz = document.getElementById('dropzone');
  var count = fileStatuses.size;
  if (count > 0) {
    dz.classList.add('has-file');
    document.getElementById('dropzone-text').innerHTML =
      count + ' file' + (count > 1 ? 's' : '') + ' selected';
  } else {
    dz.classList.remove('has-file');
    document.getElementById('dropzone-text').textContent =
      'Drop comic files here, or click to browse';
  }
 }
 
function updateFileList() {
  var listEl = document.getElementById('fileList');
  var summaryEl = document.getElementById('progressSummary');

  if (fileStatuses.size === 0) {
    listEl.innerHTML = '';
    summaryEl.textContent = '';
    return;
  }

  var html = '';
  var done = 0;
  var total = fileStatuses.size;
  var icons = { pending: '', converting: '', done: '', error: '' };

  fileStatuses.forEach(function (st) {
    var icon = icons[st.status] || icons.pending;
    html += '<div class="file-item ' + st.status + '">' +
      '<span class="file-icon">' + icon + '</span>' +
      '<span class="file-name">' + escapeHtml(st.name) + '</span>' +
      '<span class="file-size">' + formatSize(st.size) + '</span>';
    if (st.error) {
      html += '<span class="file-error">' + escapeHtml(st.error) + '</span>';
    }
    html += '</div>';
    if (st.status === 'done') done++;
  });

  listEl.innerHTML = html;
  summaryEl.textContent = total > 0 ? done + '/' + total + ' done' : '';
}

// ── helpers ────────────────────────────────────────────────────────────────

 function formatSize(bytes) {
  if (bytes < 1024) return bytes + ' B';
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB';
  return (bytes / (1024 * 1024)).toFixed(1) + ' MB';
 }
 
 function escapeHtml(str) {
  var div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
 }
 
function showResult(msg, type) {
  var el = document.getElementById('result');
  el.className = type;
  el.innerHTML = msg;
  el.style.display = 'block';
}

// ── form options ───────────────────────────────────────────────────────────

function getOptions() {
  // Map "html-preview" → "html" for the Go backend — preview is a front-end
  // delivery choice; Go only needs to know to produce HTML.
  var rawFormat = document.getElementById('outputFormat').value;
  var backendFormat = rawFormat === 'html-preview' ? 'html' : rawFormat;

  return {
    inputName: '',
    outputFormat: backendFormat,
    imageFormat: document.getElementById('imageFormat').value,
    quality: parseInt(document.getElementById('quality').value) || 85,
    grayscale: document.getElementById('grayscale').checked,
    grayscaleMode: 0,
    crop: document.getElementById('crop').checked,
    cropLeft: 1,
    cropUp: 1,
    cropRight: 1,
    cropBottom: 3,
    cropLimit: 0,
    brightness: parseInt(document.getElementById('brightness').value) || 0,
    contrast: parseInt(document.getElementById('contrast').value) || 0,
    autoContrast: document.getElementById('autoContrast').checked,
    autoRotate: document.getElementById('autoRotate').checked,
    autoSplitDouble: document.getElementById('autoSplitDouble').checked,
    keepDoubleIfSplit: true,
    keepSplitAspect: true,
    noBlankImage: document.getElementById('noBlankImage').checked,
    manga: document.getElementById('manga').checked,
    hasCover: document.getElementById('hasCover').checked,
    resize: document.getElementById('resize').checked,
    profile: document.getElementById('profile').value,
    aspectRatio: parseFloat(document.getElementById('aspectRatio').value) || -1,
    portraitOnly: document.getElementById('portraitOnly').checked,
    titlePage: 1,
    limitMb: 0,
    title: document.getElementById('title').value,
    author: document.getElementById('author').value || 'GO Comic Converter',
    series: document.getElementById('series').value,
    number: document.getElementById('number').value,
    genre: document.getElementById('genre').value,
    mangaTag: document.getElementById('mangaTag').checked,
    recipe: document.getElementById('recipe').value,
  };
 }
 
// ── sequential queue processor ────────────────────────────────────────────

/**
 * Posts one file to the worker and returns a Promise that resolves with the
 * worker's {type:'result', data:ArrayBuffer, filename} or rejects on error.
 * The fileId parameter is the correlation key: the worker echoes it as msg.id
 * in its {type:'result'|'error'} response, so convertOne filters messages
 * belonging to other concurrent conversions.
 */
function convertOne(fileId, file, optionsJson) {
  return new Promise(function (resolve, reject) {
    function handler(e) {
      var msg = e.data;
      // Filter messages by fileId — only respond to our own conversion
      if (msg.id !== fileId) return;


      if (msg.type === 'progress') {
        document.getElementById('progress-text').textContent = msg.message;
         return;
      }

      if (msg.type === 'result') {
        worker.removeEventListener('message', handler);
        resolve(msg);
        return;
      }

      if (msg.type === 'error') {
        worker.removeEventListener('message', handler);
        reject(new Error(msg.error || msg.message || 'Unknown error'));
         return;
      }
    }

    worker.addEventListener('message', handler);

    // Read file and transfer its ArrayBuffer to the worker
    file.arrayBuffer().then(function (buffer) {
      worker.postMessage(
        { type: 'convert', id: fileId, inputData: buffer, options: optionsJson, filename: file.name },
        [buffer]
      );
    }).catch(function (err) {
      worker.removeEventListener('message', handler);
      reject(err);
    });
  });
}

async function processQueue() {
  if (isProcessing) return;
  if (fileStatuses.size === 0) return;

  isProcessing = true;

  document.getElementById('loading').classList.add('active');
  document.getElementById('convertBtn').disabled = true;
  document.getElementById('result').style.display = 'none';

  // Snapshot the ordered list of pending entries
  var entries = [];
  fileStatuses.forEach(function (st, id) {
    if (st.status === 'pending' || st.status === 'error') {
      entries.push({ id: id, status: st });
     }
  });

  for (var i = 0; i < entries.length; i++) {
    var entry = entries[i];
    var st = entry.status;
    var id = entry.id;
    if (!workerReady) {
      st.status = 'error';
      st.error = 'Worker terminated unexpectedly';
      updateFileList();
      break;
    }

 
    st.status = 'converting';
    delete st.error;
    updateFileList();

    document.getElementById('progress-text').textContent =
      'Converting: ' + st.name;

    var options = getOptions();
    options.inputName = st.name;
 
     try {
      var msg = await convertOne(id, st.file, JSON.stringify(options));
      st.status = 'done';
      updateFileList();

      handleFileResult(st.name, msg);
     } catch (err) {
      st.status = 'error';
      st.error = err.message;
      updateFileList();
      console.error('Convert error for ' + st.name + ':', err);
     }
  }

  document.getElementById('loading').classList.remove('active');
  document.getElementById('convertBtn').disabled = false;
  document.getElementById('progress-text').textContent = 'Done';
  isProcessing = false;
 }
 
function doConvert() {
  if (!workerReady) {
    showResult('WASM module not yet loaded. Please wait.', 'error');
    return;
  }
  if (fileStatuses.size === 0) {
    showResult('Please select files first.', 'error');
    return;
  }
  processQueue();
}

// ── result handling ───────────────────────────────────────────────────────

function handleFileResult(inputName, msg) {
  var outputFormat = document.getElementById('outputFormat').value;
  var outputBytes = new Uint8Array(msg.data);
  var outputFilename = (msg.filename || '').replace(/^\/output\//, '') || 'output';

  if (outputFormat === 'html-preview') {
    showHtmlPreview(outputBytes, outputFilename);
  } else {
    triggerDownload(outputBytes, outputFilename);
    showResult(
      'Conversion complete! Downloaded: ' + escapeHtml(outputFilename) +
      ' (' + formatSize(outputBytes.byteLength) + ')',
      'success'
    );
  }
}

function triggerDownload(bytes, filename) {
  var blob = new Blob([bytes], { type: 'application/octet-stream' });
  var url = URL.createObjectURL(blob);
  var a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  document.body.removeChild(a);
  setTimeout(function () { URL.revokeObjectURL(url); }, 1000);
}

function showHtmlPreview(bytes, filename) {
  var htmlContent = new TextDecoder().decode(bytes);

  var container = document.getElementById('previewContainer');
  var frame = document.getElementById('previewFrame');
  var downloadBtn = document.getElementById('previewDownloadBtn');

  // Set the iframe content (sandboxed by the sandbox attribute in HTML)
  frame.srcdoc = htmlContent;

  // Wire the download button
  var blob = new Blob([bytes], { type: 'text/html' });
  var url = URL.createObjectURL(blob);
  downloadBtn.href = url;
  downloadBtn.download = filename;
  downloadBtn.textContent = '\u2B07 Download ' + escapeHtml(filename) +
    ' (' + formatSize(bytes.byteLength) + ')';

  // Clean up the blob URL after a reasonable time
  var oldUrl = downloadBtn._blobUrl;
  if (oldUrl) URL.revokeObjectURL(oldUrl);
  downloadBtn._blobUrl = url;

  container.style.display = 'block';

  showResult('Preview ready: ' + escapeHtml(filename), 'success');
 }
 
// ── bootstrap ─────────────────────────────────────────────────────────────

initWorker();
