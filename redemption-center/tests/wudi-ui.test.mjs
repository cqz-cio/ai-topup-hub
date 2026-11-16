import test, { after, afterEach } from "node:test";
import assert from "node:assert/strict";
import { mkdir, rm } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import { resolve } from "node:path";
import { build } from "esbuild";
import { JSDOM } from "jsdom";
import React, { act } from "react";

const dom = new JSDOM('<div id="root"></div>', { url: "http://localhost/" });
for (const key of [
  "window",
  "document",
  "navigator",
  "HTMLElement",
  "Event",
  "MouseEvent",
]) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: dom.window[key],
  });
}
globalThis.IS_REACT_ACT_ENVIRONMENT = true;
const { createRoot } = await import("react-dom/client");
const directory = fileURLToPath(
  new URL("../node_modules/.cache/wudi-ui-tests/", import.meta.url),
);
await mkdir(directory, { recursive: true });
const output = resolve(directory, `components-${process.pid}.mjs`);
await build({
  stdin: {
    contents:
      "export { WudiV2Flow } from './src/WudiV2Flow.jsx'; export { LinkRecharge } from './src/WudiLinkFlow.jsx';",
    resolveDir: fileURLToPath(new URL("../", import.meta.url)),
    loader: "jsx",
  },
  outfile: output,
  bundle: true,
  platform: "node",
  format: "esm",
  jsx: "automatic",
  external: ["react", "react-dom", "lucide-react", "xlsx"],
  logLevel: "silent",
});
const { WudiV2Flow, LinkRecharge } = await import(pathToFileURL(output));
const card = "PAIPLUS-AAAAAAAAAA-BBBBBBBBBB";
const otherCard = "PAIPLUS-CCCCCCCCCC-DDDDDDDDDD";
const secret = JSON.stringify({
  user: { email: "mock@example.test" },
  sessionToken: "SYNTHETIC-UI-ONLY",
});
const baseResult = {
  attempt_id: "mock-attempt",
  state: "awaiting_customer",
  code: "PAIPLUS-…BBBB",
  provider: "wudi",
  can_check: true,
  card: {
    plan_name: "ChatGPT Plus",
    region_name: "测试地区",
    require_plus_promo: false,
  },
};
const ready = () => ({
  ...baseResult,
  confirmation: "synthetic-confirmation",
  preflight: {
    status: "ready",
    expiresAt: new Date(Date.now() + 60000).toISOString(),
    subscription: {
      email: "mock@example.test",
      accountCount: 1,
      accounts: [
        {
          selected: true,
          plan: "free",
          subscriptionPlan: "free",
          active: false,
          willRenew: false,
          plusPromoEligible: true,
        },
      ],
    },
  },
});
let root,
  calls = [],
  handler,
  originalFetch = globalThis.fetch;
globalThis.fetch = async (url, options) => {
  const path = String(url).replace("/api/", "");
  const body = options.body ? JSON.parse(options.body) : undefined;
  calls.push({ path, body });
  const value = await handler(path, body);
  return {
    ok: value?.ok !== false,
    json: async () => (value?.ok === false ? value : { ok: true, data: value }),
  };
};
async function mount(tool = "recharge", props = {}) {
  calls = [];
  root = createRoot(document.getElementById("root"));
  await act(async () =>
    root.render(
      React.createElement(WudiV2Flow, {
        tool,
        config: { mode: "mock", examples: [], recent: [] },
        ...props,
      }),
    ),
  );
}
const text = () => document.body.textContent;
const button = (name) =>
  [...document.querySelectorAll("button")].find(
    (element) => element.textContent.trim() === name,
  );
async function click(element) {
  assert.ok(element);
  await act(async () => element.click());
}
async function fill(selector, value) {
  const element = document.querySelector(selector);
  assert.ok(element, selector);
  const prototype =
    element.tagName === "TEXTAREA"
      ? dom.window.HTMLTextAreaElement.prototype
      : dom.window.HTMLInputElement.prototype;
  await act(async () => {
    Object.getOwnPropertyDescriptor(prototype, "value").set.call(
      element,
      value,
    );
    element.dispatchEvent(new dom.window.Event("input", { bubbles: true }));
  });
}
async function validate() {
  await fill("#wudi-card", card);
  await click(button("验证卡密"));
}
async function preflight() {
  await fill("#wudi-session", secret);
  assert.equal(button("开始账号预检").disabled, true);
  await click(document.querySelector(".wudi-check input[type=checkbox]"));
  await click(button("开始账号预检"));
}
afterEach(async () => {
  if (root) await act(async () => root.unmount());
  root = null;
  handler = undefined;
  dom.window.localStorage.clear();
  dom.window.sessionStorage.clear();
});
after(async () => {
  globalThis.fetch = originalFetch;
  await rm(output, { force: true });
  dom.window.close();
});

test("v2 requires account consent and separate confirmation, and clears Session on success", async () => {
  handler = async (path) =>
    path === "partner-link"
      ? baseResult
      : path === "partner-check"
        ? ready()
        : {
            ...baseResult,
            state: "succeeded",
            can_check: false,
            confirmation: null,
          };
  await mount();
  await validate();
  await preflight();
  assert.match(text(), /mock@example.test/);
  assert.match(text(), /ChatGPT Plus/);
  assert.equal(document.querySelector("#wudi-session").value, "");
  assert.equal(button("确认兑换").disabled, true);
  assert.deepEqual(
    calls.map((c) => c.path),
    ["partner-link", "partner-check"],
  );
  assert.equal(calls[1].body.acknowledged, true);
  await click(document.querySelector(".wudi-preflight input[type=checkbox]"));
  await click(button("确认兑换"));
  assert.equal(calls[2].body.confirmed, true);
  assert.equal(calls[2].body.confirmation, "synthetic-confirmation");
  assert.equal("session" in calls[2].body, false);
  assert.match(text(), /已完成/);
  assert.equal(
    dom.window.localStorage.length + dom.window.sessionStorage.length,
    0,
  );
});

test("an expired or rejected preflight cannot offer a confirmation", async () => {
  handler = async (path) =>
    path === "partner-link"
      ? baseResult
      : {
          ...ready(),
          preflight: {
            ...ready().preflight,
            expiresAt: "2000-01-01T00:00:00Z",
          },
        };
  await mount();
  await validate();
  await preflight();
  assert.equal(button("确认兑换"), undefined);
  assert.match(text(), /预检授权已失效/);
  assert.ok(calls.every((c) => c.path !== "partner-confirm"));
});

test("changing card invalidates old account authorization and does not submit", async () => {
  handler = async (path) => (path === "partner-link" ? baseResult : ready());
  await mount();
  await validate();
  await preflight();
  await fill("#wudi-card", otherCard);
  assert.equal(button("确认兑换"), undefined);
  assert.doesNotMatch(text(), /mock@example.test/);
  assert.equal(calls.length, 2);
});

test("late preflight cannot populate another tool after navigation", async () => {
  let finish;
  handler = async (path) =>
    path === "partner-link"
      ? baseResult
      : new Promise((resolve) => {
          finish = resolve;
        });
  await mount();
  await validate();
  await preflight();
  await act(async () =>
    root.render(
      React.createElement(WudiV2Flow, { tool: "lookup", config: {} }),
    ),
  );
  await act(async () => finish(ready()));
  assert.match(text(), /查询结果/);
  assert.doesNotMatch(text(), /mock@example.test|确认兑换/);
});

test("promo page explains eligibility and never charges during validation", async () => {
  handler = async () => ({
    ...baseResult,
    card: { ...baseResult.card, require_plus_promo: true },
  });
  await mount("promo");
  await validate();
  assert.match(text(), /是否需要半价资格，以卡密实际套餐及预检结果为准/);
  assert.deepEqual(
    calls.map((c) => c.path),
    ["partner-link"],
  );
});

test("order pagination keeps the original card and appends records", async () => {
  handler = async (_path, body) => ({
    orders: [
      {
        ...baseResult,
        attempt_id: body.before_id ? "older" : "newer",
        message: body.before_id ? "较早订单" : "最新订单",
      },
    ],
    next_before_id: body.before_id ? null : 12,
  });
  await mount("orders");
  await fill("#wudi-query-card", card);
  await click(button("查询"));
  await click(button("加载更多订单"));
  assert.match(text(), /最新订单/);
  assert.match(text(), /较早订单/);
  assert.deepEqual(calls, [
    { path: "partner-orders", body: { code: card } },
    { path: "partner-orders", body: { code: card, before_id: 12 } },
  ]);
});

test("query failure remains visible and never starts redemption", async () => {
  handler = async () => ({
    ok: false,
    error: { code: "LIMIT", message: "查询频繁，请稍后重试。" },
  });
  await mount("lookup");
  await fill("#wudi-query-card", card);
  await click(button("查询"));
  assert.match(text(), /查询频繁/);
  assert.deepEqual(
    calls.map((c) => c.path),
    ["result-by-code"],
  );
});

test("subscription query needs no card, clears Session and displays actual subscription fields", async () => {
  handler = async () => ({
    status: "success",
    message: "查询完成",
    result: ready().preflight.subscription,
  });
  await mount("billing");
  await fill("#wudi-subscription-session", secret);
  await click(button("查询订阅"));
  assert.deepEqual(calls, [
    { path: "subscription-query", body: { session: secret } },
  ]);
  assert.equal(document.querySelector("textarea").value, "");
  assert.match(text(), /半价资格/);
  assert.match(text(), /mock@example.test/);
  assert.equal(dom.window.localStorage.length, 0);
});

test("batch imports are local and each card requires its own validation and confirmation", async () => {
  handler = async () => baseResult;
  await mount("batch");
  await fill("#wudi-batch", `${card}\n${otherCard}`);
  await click(button("读取卡密"));
  assert.equal(calls.length, 0);
  await click([...document.querySelectorAll(".wudi-batch-rows button")][0]);
  assert.equal(document.querySelector("#wudi-card").value, card);
  await click(button("验证卡密"));
  await click([...document.querySelectorAll(".wudi-batch-rows button")][1]);
  assert.equal(document.querySelector("#wudi-card").value, otherCard);
  assert.equal(document.querySelector("#wudi-session"), null);
  assert.deepEqual(
    calls.map((c) => c.path),
    ["partner-link"],
  );
});

test("duplicate batch cards are rejected before any network request", async () => {
  handler = async () => assert.fail("unexpected network");
  await mount("batch");
  await fill("#wudi-batch", `${card}\n${card}`);
  await click(button("读取卡密"));
  assert.match(text(), /第 2 行卡密重复/);
  assert.equal(calls.length, 0);
});

test("legacy link protocol preserves its separate no-Session flow", async () => {
  calls = [];
  handler = async () => ({
    ...baseResult,
    state: "succeeded",
    can_open: true,
    redemption_code: "SYNTHETIC-LINK-CODE",
    redeem_url: "https://example.test/redeem",
  });
  root = createRoot(document.getElementById("root"));
  await act(async () =>
    root.render(
      React.createElement(LinkRecharge, {
        config: { mode: "mock", examples: [], recent: [] },
        onChange: () => {},
      }),
    ),
  );
  await fill("#partner-card", card);
  await click(button("获取兑换链接"));
  assert.match(text(), /SYNTHETIC-LINK-CODE/);
  assert.equal(button("模拟链接 · 不跳转").disabled, true);
  assert.equal(document.querySelector("textarea"), null);
  assert.deepEqual(
    calls.map((c) => c.path),
    ["partner-link"],
  );
});

test("asynchronous preflight polls the same attempt before offering confirmation", async () => {
  handler = async (path) => {
    if (path === "partner-link") return baseResult;
    if (path === "partner-check")
      return {
        ...baseResult,
        preflight: { status: "queued", message: "模拟排队预检" },
      };
    if (path === "partner-check-status") return ready();
    assert.fail("Unexpected request: " + path);
  };
  await mount();
  await validate();
  await preflight();
  assert.equal(button("确认兑换"), undefined);
  await act(async () => new Promise((resolve) => setTimeout(resolve, 4200)));
  assert.ok(button("确认兑换"));
  assert.equal(button("确认兑换").disabled, true);
  assert.deepEqual(
    calls.map((c) => c.path),
    ["partner-link", "partner-check", "partner-check-status"],
  );
  assert.deepEqual(calls[2].body, {
    code: card,
    attempt_id: baseResult.attempt_id,
  });
});

test("subscription expiry clears the stale query without submitting any redemption", async () => {
  handler = async (path) =>
    path === "subscription-query"
      ? { query_id: "synthetic-query", status: "queued", message: "模拟查询中" }
      : {
          ok: false,
          error: { code: "QUERY_EXPIRED", message: "查询已过期，请重新查询。" },
        };
  await mount("billing");
  await fill("#wudi-subscription-session", secret);
  await click(button("查询订阅"));
  await act(async () => new Promise((resolve) => setTimeout(resolve, 4200)));
  assert.match(text(), /查询已过期/);
  assert.deepEqual(
    calls.map((c) => c.path),
    ["subscription-query", "subscription-status"],
  );
  assert.equal(document.querySelector("textarea").value, "");
});
