# Plus 引用盘点

基线：`781fcd1062bbccfa4a25fe5d0f7dad051a5b8885`。现网取证于 2026-11-17 22:39 起（北京时间），只用匿名公开 GET/HEAD；完整正文和结构化数据见 [public-before.json](evidence/public-before.json)。没有读取生产数据库或后台。

## 属性与 SKU

| 商品 ID / SKU ID | 当前规范 slug | API tags | 标题 |
| --- | --- | --- | --- |
| 30 / 30 | chatgpt-plus-code | ChatGpt 菲区 | ChatGPT Plus 兑换码｜自动发码（没有地区字样） |
| 45 / 45 | chatgpt-pro-100-ph | ChatGpt 菲区 | Pro 100美金、菲区、自动发码 |
| 31 / 31 | chatgpt-pro-200-ph | ChatGpt 菲区 | Pro 200美金、菲区、自动发码 |
| 33 / 33 | chatgpt-pro-500-ph | ChatGpt 菲区 | Pro 500美金、菲区、自动发码 |
| 32 / 32 | chatgpt-pro-200-renew-ph | ChatGpt 菲区 | Pro 200美金续费绑卡、菲区、自动发码 |

此为商品级 tags，SKU 接收同商品展示，并不是分类名或独立 SKU 地区字段。全部公开目录共 25 款；带“菲”标签的只有以上五款。埃及标签保留，未纳入纠正范围。分类 #9 的名称仍是 GPT（卡冲），slug 是 gpt-recharge。

## 正文、教程与 SEO

- Plus 的 title/H1、description、seo_meta title/description/keywords、SSR 商品正文没有把该商品描述为菲区。摘要写自有账号充值、PAIPLUS 自动发码、领取后预检兑换。条件为新账号或原有订阅已到期、掉订阅至少 10 分钟，最终以预检为准。
- 正文三个可见 FAQ：未到期能否使用、自动发码是否等于开通、不知道选择哪种方式；SSR 与 hydrated FAQPage 均是三条。
- SSR 与 hydrated Product：name 是 Plus 兑换码；category 是 GPT（卡冲）；description 不含地区。Offer 保留 CNY、价格、availability、商家及退货政策引用；image 是栅格分享图。取证时 130.00 / InStock 仅为快照，不写入候选长期正文。没有用 tags 生成 Product 的地区或权益字段。
- self-canonical 为 `/products/chatgpt-plus-code`。og/twitter 使用 SEO 文案及分享图，不从 tags 拼接。“菲区”在 SSR 文末其他 Pro 商品链接中出现，不代表 Plus 的供货事实。
- 已发布菲区教程正文存在“本站 Plus 兑换码没有标注菲区”。与公开 API 和实际 hydrated 详情标签不一致。它的位置也在仓库 `docs/catalog/blog-ph-chatgpt-20261117/{chatgpt-ph-recharge-guide.md,chatgpt-ph-recharge-guide.html,preview.html}`，这些发布记录本轮未改。
- 现网旧文 `chatgpt-plus-code-guide`、`chatgpt-code-region-check`、`chatgpt-payment-methods` 已匿名读取。未发现同一句“Plus 没有标注菲区”的断言；它们提供通用选择/地区核对和教程链接，不能当作 #30 的地区证明。取证 JSON 留有全文。
- 主站 GPT 专题源码 `internal/platform/publiccontent/gpt-recharge.json` 用独立身份映射解析规范链接；没有按该标签把 Plus 列为菲区。保留现有身份映射、SSR/Vue 同步和当前 slug。

## 前端引用

| 位置 | 对 tags 的处理 | 本轮结论 |
| --- | --- | --- |
| `frontend/user/src/views/ProductDetail.vue:70` | `product-meta` 中渲染全部标签 | **浏览器实测可见**“分类 · GPT（卡冲） / ChatGpt 菲区”，见截图。 |
| `templates/vault/ProductDetail.vue:60` | 渲染全部标签 | 源码确认；本轮未切换真实分销站做浏览器验收。 |
| `components/ProductCard.vue:38`、`ProductListItem.vue:40` | 有货卡片显示至多 maxTags / 列表一条 | 公共复用组件，不能只改详情隐藏标签来消除事实冲突。 |
| `templates/vault/components/VaultProductCard.vue:11` | 有货卡片前两条 | 源码确认。 |
| `views/Storefront.vue`、`components/commerce/StorefrontProductCard.vue` | 当前主站首页、商品中心/分类卡片没有传入/渲染 tags | 不等于所有模板不展示 tags。 |
| `utils/productSearch.ts:18` | 搜索标题、分类及调用方补充的交付描述，不搜索 tags | 主站“菲区”搜索命中标题含词的 Pro，不能靠搜索结果推断 Plus 地区。 |
| `composables/useProductDetail.ts:612–683`、`usePageSeo.ts`、`utils/shareSeo.ts` | 标题、摘要、图片、FAQ/Offer；不从商品 tags 拼接地区 | 保持两个 SEO 渲染端一致。 |
| `internal/platform/http/publicpage/`、`internal/bootstrap/publicpage/` | SSR 用标题/SEO/正文生成公开元信息 | 本轮未改变。 |
| OrderDetail、GuestOrderDetail、VaultOrderItem | 使用订单项 tags 快照 | 仅盘点，不改订单、历史快照或交付字段。 |

“前端 meta tags”与“商品 tags”是两个概念；不得混用。当前事实证明“Plus 完全没标菲区”不成立，但不能由此判定真实供货地区。

## 后续

店主的四个原问题及条件 A/B 在 [owner-questions.md](owner-questions.md)。未获答复，商品字段、标签、教程、SEO/结构化数据全部保持现状；cart 代码可独立审核。
