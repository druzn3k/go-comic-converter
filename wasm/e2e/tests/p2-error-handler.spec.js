// @ts-check
const { test, expect } = require('@playwright/test');

/**
 * Helper: upload a single file blob via the dropzone.
 * @param {import('@playwright/test').Page} page
 * @param {{ name: string, data: Buffer, mimeType: string }} file
 */
async function uploadFile(page, file) {
  const input = page.locator('#fileInput');
  await input.setInputFiles([
    { name: file.name, mimeType: file.mimeType, buffer: file.data },
  ]);
  await page.waitForSelector('.file-item');
}

/**
 * Helper: click convert and wait for queue to finish.
 */
async function convert(page) {
  await page.click('#convertBtn');
  // Wait for the loading indicator to disappear (queue finished)
  await page.waitForFunction(() => {
    const el = document.getElementById('loading');
    return !el.classList.contains('active');
  }, { timeout: 30000 });
}

test.describe('P2 error handler', () => {
  test('per-file error does not show global banner', async ({ page }) => {
    await page.goto('/');

    // Upload a 0-byte file to trigger a per-file error.
    await uploadFile(page, { name: 'broken.cbz', data: Buffer.alloc(0), mimeType: 'application/octet-stream' });

    // Wait for convert to show up as enabled.
    await page.waitForFunction(() => !document.getElementById('convertBtn').disabled, { timeout: 15000 });

    await convert(page);

    // The global #result element should NOT be visible for per-file errors.
    const result = page.locator('#result');
    await expect(result).toBeHidden();
  });

  test('per-file error message is in the file list', async ({ page }) => {
    await page.goto('/');

    await uploadFile(page, { name: 'broken.cbz', data: Buffer.alloc(0), mimeType: 'application/octet-stream' });
    await page.waitForFunction(() => !document.getElementById('convertBtn').disabled, { timeout: 15000 });
    await convert(page);

    // The file list should contain a .file-error element with a message.
    const fileError = page.locator('.file-error');
    await expect(fileError).toBeVisible();
    const text = await fileError.textContent();
    expect(text.length).toBeGreaterThan(0);
  });

  test('batch continues after a per-file error', async ({ page }) => {
    await page.goto('/');

    // Upload a broken file and a valid CBZ file.
    await uploadFile(page, { name: 'broken.cbz', data: Buffer.alloc(0), mimeType: 'application/octet-stream' });
    await uploadFile(page, { name: 'sample.cbr', data: require('fs').readFileSync('fixtures/sample.cbr'), mimeType: 'application/octet-stream' });

    await page.waitForFunction(() => !document.getElementById('convertBtn').disabled, { timeout: 15000 });
    await convert(page);

    // At least one file should show 'error' and one should show 'done'.
    const fileItems = page.locator('.file-item');
    const count = await fileItems.count();
    expect(count).toBe(2);

    let errorCount = 0;
    let doneCount = 0;
    for (let i = 0; i < count; i++) {
      const cls = await fileItems.nth(i).getAttribute('class');
      if (cls.includes('error')) errorCount++;
      if (cls.includes('done')) doneCount++;
    }
    expect(errorCount).toBeGreaterThanOrEqual(1);
    expect(doneCount).toBeGreaterThanOrEqual(1);
  });
});
