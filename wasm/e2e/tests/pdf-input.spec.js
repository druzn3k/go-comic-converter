// @ts-check
const { test, expect } = require('@playwright/test');
const fs = require('fs');

test.describe('PDF input', () => {
  test('happy path — converts sample.pdf to epub', async ({ page }) => {
    await page.goto('/');

    // Upload the PDF fixture.
    const input = page.locator('#fileInput');
    await input.setInputFiles([
      { name: 'sample.pdf', mimeType: 'application/pdf', buffer: fs.readFileSync('fixtures/sample.pdf') },
    ]);
    await page.waitForSelector('.file-item');

    await page.selectOption('#outputFormat', 'epub');
    await page.waitForFunction(() => !document.getElementById('convertBtn').disabled, { timeout: 15000 });

    const downloadPromise = page.waitForEvent('download', { timeout: 60000 });
    await page.click('#convertBtn');

    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/\.epub$/i);
    const path = await download.path();
    expect(path).not.toBeNull();
    if (path) {
      const stat = fs.statSync(path);
      expect(stat.size).toBeGreaterThan(0);
    }
  });

  test('unsupported encoding does not crash the app', async ({ page }) => {
    // This test verifies that the Phase 1 safety wrapper prevents
    // process-killing log.Fatal calls. We upload a PDF with an
    // unsupported image encoding and confirm the app survives.
    await page.goto('/');

    // Build a minimal PDF with a known log.Fatal trigger:
    // /FlateDecode /DeviceGray /BitsPerComponent 3 (not in {1,2,4,8}).
    // We inject it as a raw buffer.
    const pdfBytes = buildUnsupportedPDF();
    const input = page.locator('#fileInput');
    await input.setInputFiles([
      { name: 'bad-image.pdf', mimeType: 'application/pdf', buffer: pdfBytes },
    ]);
    await page.waitForSelector('.file-item');

    await page.waitForFunction(() => !document.getElementById('convertBtn').disabled, { timeout: 15000 });
    await page.click('#convertBtn');

    // Wait for queue to finish.
    await page.waitForFunction(() => {
      const el = document.getElementById('loading');
      return !el.classList.contains('active');
    }, { timeout: 30000 });

    // The page should NOT be closed (the WASM worker did not crash).
    expect(page.isClosed()).toBe(false);

    // The file row should show error status.
    const fileItem = page.locator('.file-item');
    const cls = await fileItem.getAttribute('class');
    expect(cls).toContain('error');

    // The global error banner should NOT show for a per-file error.
    const result = page.locator('#result');
    await expect(result).toBeHidden();
  });
});

/**
 * Builds a minimal PDF with an image XObject using
 * /FlateDecode /DeviceGray /BitsPerComponent 3 — a combination
 * that triggers log.Fatal in pdfimage.Extract.
 */
function buildUnsupportedPDF() {
  // The document is hand-crafted with correct byte offsets for xref.
  const lines = [];
  lines.push('%PDF-1.4');

  // Object 1: Catalog
  const o1 = byteOffset(lines);
  lines.push('1 0 obj');
  lines.push('<< /Type /Catalog /Pages 2 0 R >>');
  lines.push('endobj');

  // Object 2: Pages
  const o2 = byteOffset(lines);
  lines.push('2 0 obj');
  lines.push('<< /Type /Pages /Kids [3 0 R] /Count 1 >>');
  lines.push('endobj');

  // Object 3: Page with Resources
  const o3 = byteOffset(lines);
  lines.push('3 0 obj');
  lines.push('<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]');
  lines.push('   /Resources << /XObject << /Im0 4 0 R >> >>');
  lines.push('>>');
  lines.push('endobj');

  // Object 4: Image XObject with unsupported encoding
  const o4 = byteOffset(lines);
  const junkData = Buffer.from('x\x9c\x00\x00\x00\x00\x00\x00\x00\x00', 'binary'); // garbage zlib
  lines.push('4 0 obj');
  lines.push('<< /Type /XObject /Subtype /Image /Width 4 /Height 4');
  lines.push('   /ColorSpace /DeviceGray /BitsPerComponent 3');
  lines.push('   /Filter /FlateDecode /Length ' + junkData.length);
  lines.push('>>');
  lines.push('stream');
  lines.push(junkData.toString('binary'));
  lines.push('endstream');
  lines.push('endobj');

  // Objects total: 4 + null = 5 entries in xref
  const xref = byteOffset(lines);
  lines.push('xref');
  lines.push('0 5');
  lines.push('0000000000 65535 f ');
  lines.push(pad10(o1) + ' 00000 n ');
  lines.push(pad10(o2) + ' 00000 n ');
  lines.push(pad10(o3) + ' 00000 n ');
  lines.push(pad10(o4) + ' 00000 n ');
  lines.push('trailer');
  lines.push('<< /Size 5 /Root 1 0 R >>');
  lines.push('startxref');
  lines.push('' + xref);
  lines.push('%%EOF');

  return Buffer.from(lines.join('\n'), 'binary');
}

function byteOffset(lines) {
  // Calculate the byte offset of the NEXT line to be added.
  const text = lines.join('\n');
  return text.length + 1; // +1 for the newline at the end
}

function pad10(n) {
  return String(n).padStart(10, '0');
}
