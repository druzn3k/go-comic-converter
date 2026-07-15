// memfs.js — In-memory filesystem for Go WASM in the browser.
// Load AFTER wasm_exec.js; monkey-patches only the broken globalThis.fs methods.
// Preserves stdout/stderr logging, constants, process, and all working stubs.

(function () {
  "use strict";

  const fs = globalThis.fs;
  if (!fs) {
    console.error("memfs: wasm_exec.js not loaded first — globalThis.fs missing");
    return;
  }

  // ---- In-memory store ----
  const store = new Map();
  const fds = new Map();
  let nextFd = 3;

  function enosys() {
    const e = new Error("not implemented");
    e.code = "ENOSYS";
    return e;
  }

  const origWrite = fs.write.bind(fs);

  function statResult(path) {
    const entry = store.get(path);
    if (!entry) return null;
    const isDir = entry.data === null;
    return {
      dev: 1, nlink: 1, rdev: 0, blksize: 4096, ino: 0,
      mode: isDir ? 0o755 | 0o40000 : (entry.mode || 0o644) | 0o100000,
      uid: 0, gid: 0, size: isDir ? 0 : entry.data.length,
      atimeMs: Date.now(), mtimeMs: Date.now(), ctimeMs: Date.now(),
      isDirectory() { return isDir; },
    };
  }

  fs.open = function (path, flags, mode, callback) {
    const existing = store.get(path);
    const exists = !!existing;
    if ((flags & 64) === 0 && !exists) {
      const e = new Error("ENOENT: " + path); e.code = "ENOENT"; callback(e); return;
    }
    if ((flags & (64 | 128)) === (64 | 128) && exists) {
      const e = new Error("EEXIST: " + path); e.code = "EEXIST"; callback(e); return;
    }
    const fd = nextFd++;
    let data = exists ? existing.data : new Uint8Array(0);
    if (!exists) store.set(path, { data, mode: mode || 0o644 });
    if (flags & 512) {
      data = new Uint8Array(0);
      store.set(path, { data, mode: mode || 0o644 });
    }
    fds.set(fd, { path, offset: 0, flags, mode: mode || 0o644 });
    callback(null, fd);
  };

  fs.close = function (fd, callback) {
    if (fd >= 3) fds.delete(fd);
    callback(null);
  };

  fs.read = function (fd, buffer, offset, length, position, callback) {
    if (fd < 3) { callback(enosys()); return; }
    const entry = fds.get(fd);
    if (!entry) { callback(new Error("bad fd: " + fd)); return; }
    const fileEntry = store.get(entry.path);
    if (!fileEntry || fileEntry.data === null) { callback(new Error("ENOENT")); return; }
    const pos = position === null ? entry.offset : position;
    const fileData = fileEntry.data;
    const end = Math.min(pos + length, fileData.length);
    const bytesToCopy = Math.max(0, end - pos);
    for (let i = 0; i < bytesToCopy; i++) buffer[offset + i] = fileData[pos + i];
    if (position === null) entry.offset += bytesToCopy;
    callback(null, bytesToCopy);
  };

  fs.write = function (fd, buffer, offset, length, position, callback) {
    if (fd < 3) { origWrite(fd, buffer, offset, length, position, callback); return; }
    const entry = fds.get(fd);
    if (!entry) { callback(new Error("bad fd: " + fd)); return; }
    const fileEntry = store.get(entry.path);
    if (!fileEntry) { callback(new Error("ENOENT")); return; }
    const pos = position === null ? entry.offset : position;
    const newLen = Math.max(fileEntry.data.length, pos + length);
    const newData = new Uint8Array(newLen);
    newData.set(fileEntry.data, 0);
    newData.set(buffer.subarray(offset, offset + length), pos);
    fileEntry.data = newData;
    if (position === null) entry.offset += length;
    callback(null, length);
  };

  fs.stat = function (path, callback) {
    const r = statResult(path);
    if (!r) { const e = new Error("ENOENT: " + path); e.code = "ENOENT"; callback(e); return; }
    callback(null, r);
  };

  fs.lstat = function (path, callback) { fs.stat(path, callback); };

  fs.fstat = function (fd, callback) {
    if (fd < 3) { callback(enosys()); return; }
    const entry = fds.get(fd);
    if (!entry) { callback(new Error("bad fd")); return; }
    const r = statResult(entry.path);
    if (!r) { callback(new Error("ENOENT")); return; }
    callback(null, r);
  };

  fs.mkdir = function (path, perm, callback) {
    if (store.has(path)) {
      const e = new Error("EEXIST: " + path); e.code = "EEXIST"; callback(e); return;
    }
    store.set(path, { data: null, mode: perm || 0o755 });
    const parts = path.replace(/^\/+/, "").split("/");
    let acc = "";
    for (let i = 0; i < parts.length - 1; i++) {
      if (!parts[i]) continue;
      acc += "/" + parts[i];
      if (!store.has(acc)) store.set(acc, { data: null, mode: 0o755 });
    }
    callback(null);
  };

  fs.readdir = function (path, callback) {
    const prefix = path.endsWith("/") ? path : path + "/";
    const entries = [];
    for (const key of store.keys()) {
      if (key.startsWith(prefix)) {
        const rest = key.slice(prefix.length);
        if (rest && !rest.includes("/")) entries.push(rest);
      }
    }
    callback(null, entries);
  };

  fs.unlink = function (path, callback) {
    store.delete(path);
    for (const [fd, entry] of fds) {
      if (entry.path === path) fds.delete(fd);
    }
    callback(null);
  };

  console.log("memfs: in-memory filesystem active");
})();
