// @ts-check
const { test, expect } = require('@playwright/test');
const fs = require('fs');

test.describe('CBR input', () => {
  test('happy path — converts sample.cbr to epub', async ({ page }) => {
    await page.goto('/');

    // Upload the CBR fixture.
    const input = page.locator('#fileInput');
    await input.setInputFiles([
      { name: 'sample.cbr', mimeType: 'application/octet-stream', buffer: fs.readFileSync('fixtures/sample.cbr') },
    ]);
    await page.waitForSelector('.file-item');

    // Set output format to epub (default).
    await page.selectOption('#outputFormat', 'epub');

    // Wait for convert button to be enabled.
    await page.waitForFunction(() => !document.getElementById('convertBtn').disabled, { timeout: 15000 });

    // Set up download promise before clicking convert.
    const downloadPromise = page.waitForEvent('download', { timeout: 30000 });
    await page.click('#convertBtn');

    // Wait for the download.
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/\.epub$/i);
    expect(download.suggestedFilename().length).toBeGreaterThan(0);

    // Verify the file is non-zero.
    const path = await download.path();
    expect(path).not.toBeNull();
    if (path) {
      const stat = fs.statSync(path);
      expect(stat.size).toBeGreaterThan(0);
    }
  });
});
