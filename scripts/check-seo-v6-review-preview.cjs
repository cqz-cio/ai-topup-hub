const { chromium } = require("playwright");
const { resolve, join } = require("node:path");
const { pathToFileURL } = require("node:url");
const assert = require("node:assert/strict");
const dir = resolve(__dirname, "../docs/catalog/seo-engineering-v6-20261117");

(async () => {
  const browser = await chromium.launch({ channel: "chrome", headless: true, timeout: 10000 });
  try {
    for (const width of [1280, 390, 320]) {
      const page = await browser.newPage({ viewport: { width, height: 880 }, offline: true });
      page.setDefaultTimeout(5000);
      await page.goto(pathToFileURL(join(dir, "preview.html")).href);
      assert.equal(await page.locator("#category tbody tr").count(), 5);
      assert.equal(await page.locator('a[href$="/chatgpt-plus-manual"]').count(), 0);
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth), width);
      const table = page.locator("#category .table-scroll");
      if (width < 400) {
        assert.ok(await table.evaluate(el => { el.scrollLeft = el.scrollWidth; const moved = el.scrollLeft > 0; el.scrollLeft = 0; return moved; }));
      }
      await page.locator("#category").screenshot({ path: join(dir, "evidence", "followup-category-" + width + ".png") });
      await page.locator("#notice-proposal summary").click();
      assert.equal(await page.locator("#notice-proposal details").getAttribute("open"), "");
      await page.close();
      console.log(width, "passed");
    }
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
