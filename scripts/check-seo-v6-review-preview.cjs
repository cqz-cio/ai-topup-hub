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
      assert.equal(await page.locator("#notice-proposal, a[href='#notice-proposal']").count(), 0);
      const account = page.locator("#account");
      const copy = await account.innerText();
      for (const term of ["115 元", "订阅质保一个月", "自动发货", "卡密和账号信息", "按剩余天数退还", "按优惠价补购"]) {
        assert.ok(copy.includes(term), term);
      }
      assert.equal(await account.locator("img").count(), 0);
      await page.close();
      console.log(width, "passed");
    }
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
