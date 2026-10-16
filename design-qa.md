# 推广返利规则说明（2026-10-16）

结果：通过。用户批准预览位于 `work/codex-runtime/affiliate-rules-preview-20261016/` 的 desktop/mobile/detail 图片；实现截图位于 `work/codex-runtime/affiliate-rules-20261016/` 的 local-final-1920/390/320、built-1920/390/320 和 result-detail。

桌面保持既有卡片宽度、色彩和间距，在推广链接前增加淡绿三列规则摘要；手机依次显示比例、示例和确认期，统计标签与数值同行。修正了首轮实现中手机统计卡过高和重复角标的问题。原有个人资料头、导航、佣金和历史提现记录保留。规则摘要与参与条件卡的层级、字号、间距和边框已对照批准预览，未发现需进一步修正的实质偏差。

经典/Vault、简/繁/英语、亮暗、1920/390/320px 验证通过。正式站已有用户会话中实际数值、DOM 排版宽度及刷新正常，未发现脚本错误；Chrome 截图接口超时，正式资源通过哈希与本机视觉验收包对应，正式截图未生成。

---

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


## 正式产物与发布复核

干净构建 `a4beda922912f4e2221a6584b8ba6745fa55e652` 和正式发布 `20261016-100417` 已分别完成 6 项、11 项实际浏览器检查。P1 与 `built-desktop.jpg` 已在同一比较输入中再次核对；`production-desktop.jpg`、`production-gpt.jpg`、`production-codex.jpg`、`production-mobile390.jpg`、`production-mobile320.jpg` 为最终线上截图。原截图为登录状态，线上验收为游客状态，价格按各自身份由 API 返回，本次没有调价。正式目录为 18 款，GPT/Codex/Gemini 为 8/6/4，20x 搜索 2 款、20x 续费 1 款；无脚本错误及横向溢出。final result: passed。

接续发布 `20261016-100816`（源码 `149b049921245d18cebd05d8b5d7f8a2617166ad`）已保留本次商品中心源码，另验当前产物 256 个文件哈希以及桌面、GPT 分类、320px 三项浏览器检查；全部通过。当前截图为 `current-production-desktop.jpg`、`current-production-mobile320.jpg`。

---

# 商品详情页：已批准第 1 版（2026-10-16）

final result: passed

正式发布 `20261016-100816` 已核验。最终合并构建截图 `final-built-desktop.jpg`、`final-built-390.jpg`、`final-built-320.jpg`；正式站截图 `production-desktop.jpg`、`production-390.jpg`、`production-320.jpg`。7 项正式页面检查无错误，并确认商品中心 18 款展示保持。

Source visual truth: `C:/Users/admin/.codex/generated_images/01a1425e-4b8d-7d62-a180-d18b83cd477f/exec-dd1deb73-a460-432f-a2b4-f7a11db3d666.png`。

Implementation evidence: `D:/卡网/work/codex-runtime/product-detail-preview-20261016/` 内的 `local-desktop-v2.jpg`、`built-desktop.jpg`、`built-390.jpg`、`built-320.jpg` 与 `local-verified.json`。

## 对照状态与归一化

- 经典模板、简体中文、浅色、未登录、`/products/qzweb-5`；实现 CSS 视口 1920×1080，DPR 1，完整截图 1910×2203（浏览器滚动条占 10px）。预览原图 1448×1086，并非真实浏览器截图。
- 全页配对证据 `comparison-full.jpg`：源图和实现各按比例缩至 960px 宽后放入同一图像。完整商品正文与真实页脚比生成预览更长，不能将总页高作为一比一目标。
- 重点配对证据 `comparison-overview.jpg`：预览裁切 (243,159)-(1206,802)，实现裁切 (367,227)-(1543,1064)，两者保持比例归一至 960px 宽。图片、标题、价格、规格、条件、数量及按钮均清晰可比较。
- 预览采用正式公开价 130.00 CNY，本机独立数据为 114.80 CNY；库存随环境不同。这些数据差异不作为视觉偏差，也未为了匹配预览调价。

## 发现、修正与再验收

- **P2，首屏信息密度，已修正。** 首轮 `local-desktop-v1.jpg` 中分类和渠道占两行，购买按钮靠近首屏下缘。提交 `3b6da89` 将分类/渠道对齐同一行，缩短面包屑和步骤条间距。`local-desktop-v2.jpg` 与干净构建 `built-desktop.jpg` 确认按钮 y=953–1001，开票提示也在 1080px 首屏内。
- 原图库被右侧高度撑至约 1348px，图片约 522×313px。最终左侧内容面板约 640×611px，图片约 606×364px，保持完整 5:3 图；两列独立按内容确定高度，消除图下的大块填充空白。
- **接受的内容差异。** 用户明确要求商品内容不变：保留完整后台 HTML、所有质保与账号条件、原客服提示、全部兑换步骤。预览中提炼的“如何兑换”仍保留在原有完整正文中，左侧展示现有兑换链接与交付说明。没有将后台正文改成生成预览中的简写。真实左侧因此短于右侧，卡片不再强制等高。
- 不存在待处理 P0/P1/P2。原站点导航、原币种格式、库存实时值和字体栅格化差异属于已接受的生产约束。

## 五项视觉检查

- 字体：沿用项目字体栈；桌面标题 32px、行高 1.3、750 字重，手机 25px；完整换行无省略。价格延续原格式与等宽数字，尺寸 38px，手机按空间缩放。小字可读，英语条件段落正常换行。
- 布局：宽屏 1.25:1 双栏，24px 栏间距，独立圆角白卡；图片完整显示，描述及兑换说明置于左侧。手机顺序为图片→完整购买区→描述/兑换→完整正文。1920/768/390/320px 无横向溢出。
- 色彩：保留暖白背景、深绿色主按钮、浅绿条件块和原主题变量；暗色可读且无白底文字冲突。
- 图片：复用现有派大星 Logo、商品图和 Lucide 图标，未生成或重绘运行素材；经典图库 contain，图片实际加载成功。Vault 原有图片样式保持原状。
- 文案：全部商品字段、规则函数及三语文本未修改。GPT/Codex 兑换链接仍为 cdk.bar，Gemini 按订单交付链接处理；未知/缺货商品不被套用其他条件。

## 功能证据

- 138 项既有商城测试通过，类型检查及生产构建通过。
- 日常 5173 检查新开通、续费、Codex、Gemini 18 个月、iOS 缺货和 DEMO；缺货购买按钮禁用，长标题不截断。
- 数量从 1 改为 2，“立即购买”进入结账后为原规格与 229.60 CNY；刷新后仍正确。加入购物车显示 1 件、114.80 CNY，测试项已用界面移除。未提交订单。
- 暗色、简/繁/英、Vault 桌面和手机、768px 平板通过；浏览器无脚本错误。本机 6 款 API 完整快照和 6 张业务/配置表摘要与改动前一致。
- 最终构建另在只读预览服务检查三种宽度；该服务拒绝 POST/PUT/PATCH/DELETE。没有实际付款、采购或改动后端。

Checklist: 双栏/图片比例完成；完整文案保留；条件在按钮前；数量/购物车/立即购买确认；多尺寸及干净构建验收完成。


---

以下保留此前商品中心和付款页验收记录。

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

## 小π悬浮客服本地实现（2026-10-16）

final result: passed

- 目标：本会话批准的 `patrick-motion-occlusion.html` 原画派大星、挥手/墨镜反光/托腮动作及客服小窗。使用同一原画网格与头像资产；主入口挂载到独立客服分支，未部署。
- 外观：核对批准的小窗截图与实现的 390×500px 小窗，保留头像、标题、四个话题、输入区、人工入口、圆角和绿色调。把预览文字输入占位变成实际输入框，明确标识演示问答；手机避开底栏，短屏优先展示聊天。
- 动作：生产组件渲染器与批准预览的 7 个关键帧逐像素一致（思考 0/350/700/2240/3825/4200ms，挥手 560ms），差异像素均为 0。托腮前臂在身体前方，肩部连续。
- 功能：快捷问题、自由输入、未知问题说明、纯文本转义、微信复制、QQ/Telegram 联系方式、Escape 收起与焦点返回均通过；检查期间没有业务写请求和页面脚本错误。
- 布局：经典/Vault 在 1920×1080、390×844、320×844 下均无横向溢出和小窗裁切；另检查 390×500 短屏、真实触摸上下文、三语言和暗色主题。已修复暗色选择器和窄屏对称留白。
- 降级：减少动态效果、WebGL 不可用、WebGL 上下文丢失/恢复通过；无法播放时仍显示原画和可用的客服问答。
- 自动检查：149 项商城测试（包含 6 项演示回复边界测试）、Vue/TypeScript 类型检查及最终 Vite 构建通过。
- 私有截图与报告：日常目录 `work/codex-runtime/customer-support-planning/implementation/qa/` 的 `report.json`、`lifecycle-report.json`、`approved-window.png`、`final-window.png`、`final-desktop.png`、`final-mobile.png` 和 `final-dark.png`。
- 范围：演示问答只在内存中选择预设回复，不连接模型或客服后端，不查询订单、不读取支付状态、不承诺返利。没有推送或部署。分支仍基于 `4243550352902139cd9c4d7530615b6a5131c9f2`；后续拟合入时需要同步 main 的后续商品展示、客服和隐私改动，不能直接把此基线作为线上发布包。

## 小π客服入口鼠标拖动（2026-10-16）

final result: passed

- 默认入口位置和原画动作保留。新增鼠标按住拖动、6px 点击容差、拖动后不误触开关、指针取消/失焦清理；手机仍为轻点打开，不通过滑动移动入口。
- 拖动位置只保存在当前页面，刷新恢复默认；缩放后限制在可见区域。小窗根据可用空间在角色上方、下方或侧面展开，窄屏优先显示聊天内容；气泡自动换边并避开小窗。
- 31 项实际浏览器检查通过：默认位置、轻微手抖点击、四角与中间拖动、展开时拖动、取消/失焦后的下一次点击、Escape/Enter、页面滚动、刷新、经典/Vault、演示回复，以及 1920/768/390/320px 和 390×500 短屏。手机触摸轻点展开收起、鼠标拖动后切换触摸点击正常，无页面脚本错误或业务写请求。
- 已修正拖到顶部时导航盖住角色、短屏导航挡住关闭按钮的问题。拖动/展开层级位于导航上方，仍低于公告与付款拦截弹窗。核对顶部、中间及短屏截图，角色完整、输入区与关闭按钮可用。
- 最终 Vue/TypeScript 类型检查及 Vite 构建通过。原有网格动画、预设回复及业务接口没有修改，本次未重复计入上一轮 149 项测试。
- 私有证据：日常目录 work/codex-runtime/customer-support-planning/implementation/qa-drag/report.json 及同目录 top-left.png、middle.png、drag-open.png、mobile-500.png、touch-open.png。
- 保存在原 codex/customer-support-agent 独立分支；没有合入主目录、推送、接入模型或部署。

## 小π客服第一阶段实际对话（2026-10-16）

final result: passed

- 按用户选择优先实现商品选购、充值与使用。使用本机公开目录的商品名，记住当前商品、iOS 渠道及对话步骤；区分 GPT 订阅、Codex 点数、Gemini 和商城钱包。付款、返利和开店仍保留基础指引。
- 新增站内商品、订单、游客查单和钱包链接；只跳转现有页面，不在客服中查询私有订单或执行业务操作。具体兑换入口只引导到付款后的订单交付说明。
- 长回复从首个步骤开始展示，修复窄屏商品列表只看到末尾的问题；保持批准的小窗外观、角色动作与拖动交互。
- 174 项商城测试通过，其中本轮新增 25 项多轮规则及公开目录边界测试；Vue/TypeScript 类型检查和最终 Vite 构建通过。构建仍有已有 Browserslist 数据版本提示，无新增依赖。
- 16 项浏览器检查通过：实际目录读取、GPT 多轮选择与兑换、iOS 上下文、商城钱包、Codex 点数、人工入口、纯文本转义、商品详情上下文、实际游客查单导航与返回、拖动保留会话、目录失败重试、经典/Vault 两模板 390/320px 三语言，以及切换语言取消旧请求。没有页面脚本错误和业务写请求。
- 私有证据位于日常目录 work/codex-runtime/customer-support-planning/conversation/qa/：report.json、gpt-conversation.png、classic-zh-CN-390.png、vault-zh-CN-320.png、classic-en-US-390.png 和 vault-zh-TW-320.png。已查看中文桌面、手机和 320px 长列表截图。
- 会话只保存在组件内存；规则返回已维护的步骤，不连接大模型。开发保存在原 codex/customer-support-agent 分支，没有合入主目录、推送或部署。分支较旧基线的合入约束沿用上文。


## 2026-10-16 小π客服 DeepSeek 本地接入

在原 `codex/customer-support-agent` 分支完成；5175 小窗调用独立回环 8085 客服服务，现有本机商城 8080 保持运行。仅客服测试消息写入模型用量账本，没有订单、支付、钱包或供应商写入，也没有生产访问、推送或部署。

- 177 项前端测试、TypeScript 检查和 Vite 构建通过；Go 客服测试覆盖 18 个用例（含子用例），只使用模拟模型和临时预算库。
- 真实 DeepSeek API 验证：充值指引、自然追问、复杂越界拒答；明确天气问法返回 local，未调用模型。多轮回复格式问题已修复并回归。
- 浏览器 9 项检查通过：真实钱包/GPT 多轮对话、拖动后聊天保留、天气本地拒答、额度错误后输入恢复、人工入口、商品上下文，以及经典/Vault 的 390/320px 与三语言。没有脚本错误或商城业务写入。
- 商品页补充 2 轮真实检查：Codex 250 点数页“这个充完会延长我的 Plus 时间吗？”与“所以只是多一些点数，对吧？”，均直接回答当前商品而非重复询问商品名称。
- 已检查桌面和手机截图，保留已批准的 UI 和动画。前端源码、构建产物及本次日志做真实密钥精确扫描；密钥、用量库均在 Git 忽略目录，ACL 仅当前用户与 SYSTEM。

私有证据：`D:\卡网\work\codex-runtime\customer-support-planning\deepseek\qa\` 内 `api-report.json`、`browser-report.json`、`page-context-report.json`、`private-audit.json`、`desktop-live-chat.png`、`mobile-live-chat.png`。完整运行及上线前限制见 `docs/customer-support-deepseek.md`。本轮限额 UI 为模拟触发，真实服务硬额度由临时账本测试验证；不消耗整日预算做验收。
