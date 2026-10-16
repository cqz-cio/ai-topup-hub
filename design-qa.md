# 商品中心：同步已确认的首页目录（2026-10-16）

final result: passed

Source visual truth: 用户提供的 P1 首页截图，私有副本 `work/codex-runtime/product-center-display/reference-home.png`。P2/P3 为修复前商品中心对照。

Implementation screenshots: 同目录 `local-desktop-all.jpg`、`local-desktop-gpt.jpg`、`local-desktop-codex.jpg`、`local-mobile-390.jpg`、`local-mobile-320.jpg`、`local-mobile-codex-390.jpg`、`local-home-catalog.jpg`、`local-desktop-dark.jpg`、`local-english-320.jpg`。

## 比较证据与状态

- P1 与实现桌面截图已在同一个比较输入中打开。源图 1920×911；实现 CSS 视口 1920×911、DPR 1，浏览器截图保留其实际像素尺寸，没有重绘或缩放设计素材。移动视口 390×844、320×740。
- 比较区域为目录工具栏、侧栏及第一排三张商品卡。完整图中图片、标题、信息标签和按钮均可读，故无需单独裁切。
- 实现为经典模板、简体中文、浅色、全部商品。本机独立商品目录为 23 款，含原有本机附加分类和测试售价；P1 为正式站 18 款及已登录状态。这些数据和登录导航差异不用于视觉偏差判定，也没有为了匹配截图修改数据。

## 发现、修正与最终检查

- 原 P3 四列窄卡导致商品图裁切与名称省略，原 P2 在全部商品中提前展示 Codex。已让商品中心直接共用首页目录组件及分组逻辑，三列、完整 5:3 商品图、完整换行标题和两类操作均与 P1 对齐。
- 新实现首次桌面对照没有发现需修正的 P0/P1/P2 偏差。顶部导航登录状态、浏览器字体栅格化和数像素的视口滚动条差异属于已接受差异。
- 字体与排版：沿用项目字体及原卡片 16px 名称、12px 标签、26px 价格，商品中心标题采用 h1 并保留首页 h2 的视觉尺寸。
- 布局与间距：复用 175px 分类侧栏、30px 栏间距、三列 18px 网格间距和 18px 卡片内边距；窄屏使用首页相同的单列及折行分类按钮。三种视口无横向溢出。
- 色彩：共用既有绿色、白色卡片、主题变量与水母背景；暗色模式仍使用原主题变量。
- 图片：沿用原 Logo、背景、商品图片与 Codex 组件；图片 object-fit 为 contain，前三张商品图均已实际加载，未增加替代素材。
- 文案：保留商品名称、交付信息、库存、Codex 资格与兑换说明、客服微信，以及使用期限/质保提示；价格继续取各环境 API。

## 功能与验收

- 138 项商城测试、类型和生产构建通过；pnpm 联网版本检查停滞后已终止，改用同一项目已安装的 Node、vue-tsc 与 Vite 完成上述命令。
- 日常本机 5173 实际检查：所有分类排序、GPT 刷新、Codex 六档、Gemini 四款、全角“２０Ｘ 续费”搜索一款、无结果恢复、选购详情、首页回归、简/繁/英语与暗色，浏览器无脚本错误。
- 本次只涉及经典商品中心展示，Vault/列表模式、订单创建、付款与真实采购不在变更范围。没有进行付款或采购测试。

Checklist: 共用目录完成；分类地址及搜索确认；桌面/手机对照完成；相关测试与构建通过。无待处理 P0/P1/P2，字体栅格化细微差异作为 P3 接受。

---

以下保留此前付款页验收记录。

# Payment page — selected option 1

final result: passed

Source visual truth: `C:/Users/CHENZIQIAO/.codex/generated_images/01a13fa7-82bc-7060-b397-39400c050a58/exec-122567e3-2d6d-43c4-92ad-5d0092a4a124.png` (the first displayed option).

Implementation: clean Git archive of `3f9c00349214520784e548ece780163a1b5a6a9e`, built with Vue type checking and Vite. Browser-rendered evidence is in the private outer-workspace directory `deployment/local/payment-redesign-release/`.

## Comparison evidence

- Reference and `final-desktop.png` were opened together in the same comparison input at original resolution. Both are 1487 × 1058 pixels; implementation viewport is 1487 × 1058 CSS pixels at DPR 1. No density scaling or device frame was used.
- State: light theme, classic storefront, pending BEP20 payment, amount `12.345678 USDT`, order total `82.10 CNY`, deliberately non-payable demonstration QR/address. Countdown and expiry are dynamic.
- The full-resolution pair makes the amount, network, address, copy controls and guide text readable; separate cropped comparison was unnecessary. The real existing header/footer are preserved rather than reproducing the concept's abbreviated navigation/footer.
- Responsive evidence: `final-classic-1920.png`, `final-classic-768.png`, `final-classic-390.png`, `final-classic-320.png` and the corresponding four `final-vault-*` images. Measurements are in `final-checks.json`.

## Findings and fixes

1. **P2, mobile first-screen QR visibility, fixed.** Initial `mobile-v1.png` placed the QR bottom at y=802 on a 390 × 844 viewport, behind the fixed bottom navigation. Reduced heading spacing and used a 192 px mobile QR. Final QR bottom is y=742; both tutorials and the complete QR are visible above the navigation at 390 px. On a 320 × 740 screen the QR requires scrolling, with no clipping or horizontal overflow.
2. **P2, desktop detail density, fixed.** Initial `desktop-v1.png` repeated fee-help and order details across additional rows. Moved the full fee explanation into the visible warning's disclosure and aligned expiry with the amount-breakdown disclosure. `desktop-v2.png` and the final clean-build comparison retain the flat two-column composition.
3. **Expected production differences.** The amount uses the existing authoritative currency formatter (`82.10 CNY` rather than a decorative yuan symbol). Real full order numbers, expiry, discounts, wallet deduction and fee breakdown remain available. These require slightly more metadata height than the concept. Vault uses its established theme tokens. No P0/P1/P2 issues remain.

## Fidelity surfaces

- **Typography:** existing Inter / PingFang SC / Microsoft YaHei stack; 52 px desktop heading, 54 px amount, 27 px QR heading, compact readable labels. Amount digits retain all precision and use tabular numerals. English, Simplified and Traditional Chinese were inspected, including 320 px English wrapping.
- **Spacing/layout:** 1224 px content region, 76 px tutorial band, approximately 490/656 px desktop columns separated by a hairline and generous gutter. Mobile reorders to tutorials → amount/network → QR → address/actions. No horizontal overflow at 1920, 1487, 768, 390 or 320 px.
- **Color/tokens:** existing warm page background, dark green accent, white QR/address controls, amber fee warning, and existing dark-theme tokens. Focus indicators and contrast remain visible in dark English capture `dark-english-320.png`.
- **Image quality:** supplied brand logo reused; QR rendered from real payment data by the existing QR renderer, never a generated picture. Lucide icons match the reference's line icons. No custom decorative imagery or placeholder assets were added to production.
- **Copy/content:** both tutorials remain prominent and open in new tabs. Exact receipt amount and separately paid withdrawal fees remain visible; expanded help preserves the full prior warning. Payment amounts, network and address are sourced from the existing composable. No financial claims or new payment logic were introduced.

## Functional evidence

- Working source passed 138 frontend tests and a typechecked production build. The clean archive passed its own typechecked production build; utility CSS and actual rendered layouts were verified.
- Actual clipboard checks returned `12.345678` exactly and the complete displayed address, including a long-address fixture. Copy feedback, keyboard focus, fee/amount disclosures, refresh, payment-method selection and reload were checked.
- Both tutorial links opened their correct articles in new tabs and retained the payment tab. Daily local `5173` payment entry and published guide article were inspected against the actual local backend.
- Classic and Vault were checked for BEP20, TRON, Plasma/USDT0 and WeChat. WeChat fixture displayed order `82.10 CNY`, fee `1.32 CNY`, payable `83.42 CNY`, without U tutorials. Two-decimal U amounts, wallet deductions, QR fallback and automatic redirect fixtures were checked.
- Expired and canceled orders do not show a payable QR. The isolated fixture server rejects all POSTs, so creation/recreation of payments was deliberately not exercised; the existing payment composable is unchanged.
- Final clean-build browser console: no errors. No live order, transfer, purchase, paid-state mutation or backend restart was used for UI validation.

## Implementation checklist

- [x] Reuse one presentation component in both payment templates.
- [x] Preserve existing payment state, polling, expiry and copy operations.
- [x] Keep tutorials above the QR and in the first screen.
- [x] Compare final clean-build screenshots with the selected source.
- [x] Verify responsive layouts, non-U payment and important alternate states.

Follow-up polish: minor font rasterization and existing shared navigation/footer differences are accepted; no blocking visual work remains.
