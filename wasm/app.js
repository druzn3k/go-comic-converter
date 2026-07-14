// app.js — Go WASM bridge for go-comic-converter

let wasmReady = false;
let selectedFile = null;

// Initialize Go WASM
async function initWasm() {
    const go = new Go();
    try {
        const result = await WebAssembly.instantiateStreaming(
            fetch("main.wasm"),
            go.importObject
        );
        go.run(result.instance);
        wasmReady = true;
        document.getElementById("convertBtn").disabled = false;
        document.getElementById("dropzone-text").textContent = "Drop a comic file here, or click to browse";
        console.log("WASM initialized successfully");
    } catch (err) {
        // Fallback: try instantiate (for browsers that don't support streaming)
        try {
            const resp = await fetch("main.wasm");
            const bytes = await resp.arrayBuffer();
            const result = await WebAssembly.instantiate(bytes, go.importObject);
            go.run(result.instance);
            wasmReady = true;
            document.getElementById("convertBtn").disabled = false;
            console.log("WASM initialized (fallback)");
        } catch (err2) {
            document.getElementById("dropzone-text").textContent =
                "Failed to load WASM module: " + err2.message;
            console.error("WASM init error:", err2);
        }
    }
}

// File handling
document.getElementById("fileInput").addEventListener("change", (e) => {
    if (e.target.files.length > 0) {
        handleFile(e.target.files[0]);
    }
});

document.getElementById("dropzone").addEventListener("dragover", (e) => {
    e.preventDefault();
    e.currentTarget.classList.add("dragover");
});

document.getElementById("dropzone").addEventListener("dragleave", (e) => {
    e.preventDefault();
    e.currentTarget.classList.remove("dragover");
});

document.getElementById("dropzone").addEventListener("drop", (e) => {
    e.preventDefault();
    e.currentTarget.classList.remove("dragover");
    if (e.dataTransfer.files.length > 0) {
        handleFile(e.dataTransfer.files[0]);
    }
});

function handleFile(file) {
    selectedFile = file;
    const dz = document.getElementById("dropzone");
    dz.classList.add("has-file");
    document.getElementById("dropzone-text").innerHTML =
        'Selected: <span class="filename">' + escapeHtml(file.name) +
        '</span> (' + formatSize(file.size) + ')';

    // Auto-fill title from filename
    const titleInput = document.getElementById("title");
    if (!titleInput.value) {
        titleInput.value = file.name.replace(/\.(cbz|zip|cbr|rar|pdf)$/i, "");
    }

    document.getElementById("convertBtn").disabled = !wasmReady;
}

function formatSize(bytes) {
    if (bytes < 1024) return bytes + " B";
    if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + " KB";
    return (bytes / (1024 * 1024)).toFixed(1) + " MB";
}

function escapeHtml(str) {
    const div = document.createElement("div");
    div.textContent = str;
    return div.innerHTML;
}

// Progress callback
function onWasmProgress(msg) {
    document.getElementById("progress-text").textContent = msg;
}

// Conversion
async function doConvert() {
    if (!selectedFile) {
        showResult("Please select a file first.", "error");
        return;
    }
    if (!wasmReady) {
        showResult("WASM module not yet loaded. Please wait.", "error");
        return;
    }

    // Collect form options
    const options = {
        inputName: selectedFile.name,
        outputFormat: document.getElementById("outputFormat").value,
        imageFormat: document.getElementById("imageFormat").value,
        quality: parseInt(document.getElementById("quality").value) || 85,
        grayscale: document.getElementById("grayscale").checked,
        grayscaleMode: 0,
        crop: document.getElementById("crop").checked,
        cropLeft: 1,
        cropUp: 1,
        cropRight: 1,
        cropBottom: 3,
        cropLimit: 0,
        brightness: parseInt(document.getElementById("brightness").value) || 0,
        contrast: parseInt(document.getElementById("contrast").value) || 0,
        autoContrast: document.getElementById("autoContrast").checked,
        autoRotate: document.getElementById("autoRotate").checked,
        autoSplitDouble: document.getElementById("autoSplitDouble").checked,
        keepDoubleIfSplit: true,
        keepSplitAspect: true,
        noBlankImage: document.getElementById("noBlankImage").checked,
        manga: document.getElementById("manga").checked,
        hasCover: document.getElementById("hasCover").checked,
        resize: document.getElementById("resize").checked,
        profile: document.getElementById("profile").value,
        aspectRatio: parseFloat(document.getElementById("aspectRatio").value) || -1,
        portraitOnly: document.getElementById("portraitOnly").checked,
        titlePage: 1,
        limitMb: 0,
        title: document.getElementById("title").value,
        author: document.getElementById("author").value || "GO Comic Converter",
        series: document.getElementById("series").value,
        number: document.getElementById("number").value,
        genre: document.getElementById("genre").value,
        mangaTag: document.getElementById("mangaTag").checked,
        recipe: document.getElementById("recipe").value,
    };

    // Show loading
    document.getElementById("loading").classList.add("active");
    document.getElementById("progress-text").textContent = "Starting conversion…";
    document.getElementById("result").style.display = "none";
    document.getElementById("convertBtn").disabled = true;

    try {
        // Read file as ArrayBuffer
        const fileBytes = await selectedFile.arrayBuffer();
        const inputData = new Uint8Array(fileBytes);

        // Call Go WASM
        const result = window.convert(inputData, JSON.stringify(options));

        if (typeof result === "string" && result.startsWith("error:")) {
            showResult(result.substring(6).trim(), "error");
            return;
        }

        // Result is an object with data (Uint8Array) and filename
        const outputBytes = result.data;
        const outputFilename = result.filename || "output.epub";

        // Trigger download
        const blob = new Blob([outputBytes], {
            type: "application/octet-stream"
        });
        const url = URL.createObjectURL(blob);
        const a = document.createElement("a");
        a.href = url;
        a.download = outputFilename.replace(/^\/output\//, "");
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        URL.revokeObjectURL(url);

        const displayName = outputFilename.replace(/^\/output\//, "");
        showResult(
            'Conversion complete! <a href="' + url + '" download="' + displayName + '">Download ' +
            escapeHtml(displayName) + '</a> (' + formatSize(outputBytes.byteLength) + ')',
            "success"
        );
    } catch (err) {
        showResult("Conversion failed: " + err.message, "error");
        console.error(err);
    } finally {
        document.getElementById("loading").classList.remove("active");
        document.getElementById("convertBtn").disabled = false;
    }
}

function showResult(msg, type) {
    const el = document.getElementById("result");
    el.className = type;
    el.innerHTML = msg;
    el.style.display = "block";
}

// Start WASM loading
initWasm();
