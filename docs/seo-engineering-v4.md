# Grok 商品正文与公开链接维护（SEO v4）

基线 `main 282c6aa94a18a2b6e21cc0b91bee5048fe05ffa0`；分支 `codex/seo-engineering-v4-20261115`。目标是八款 Grok 商品的正文区分度和现有新地址的可发现性。页面 self-canonical、v3 的 SSR、301、分享大图、slug 与标题机制沿用原实现。

## 内容确认与回填

`catalog/seo-v4/grok-copy.json` 和同目录八份 HTML 是**待运营确认的草稿**。中文摘要、SEO 标题/描述及正文逐条不同；已经确认的依据限于原公开记录和 v4 报告中的档位、周期标签、订单领码流程与非成品账号属性。运营必须确认这八款仍适用自有账号兑换流程，并逐条审阅全文。新账号/到期账号资格、叠加续费、模型权限、额度、地区、生效时间、具体质保和退款比例均未获得新增承诺，草稿不补写这些内容。

运营可以从 [可填写模板](catalog/seo-v4/operator-template.html) 开始补充事实，但不能给八款套用同一正文后只替换套餐名。每款应写清相邻档位和另一周期的区别、适用条件、领取与结果核对，以及至少三条独立 FAQ。`description` 与 `seo_meta.description` 也要分别撰写。

一次性工具默认只生成计划：

```powershell
python scripts/seo_grok_v4_content.py --database work/v4-preview/shop.db
node scripts/check-product-content.mjs
```

`--apply-local` 只接受本工作区 `work/` 下的数据库，可预览未发布草稿。正式 `--apply` 必须提供运营对**确切文案摘要**的确认文件；未确认或摘要不匹配会拒绝执行。确认文件只在收到实际确认后制作，不作为程序生成的自动批准：

```json
{"status":"approved","copy_sha256":"计划输出的确切摘要","confirmed_by":"实际确认人及确认记录"}
```

```sh
python3 scripts/seo_grok_v4_content.py --database /srv/zhixiang/shared/db/shop.db \
  --apply --approval-file /受限目录/grok-copy-approval.json \
  --backup-root /srv/zhixiang/backups
```

工具固定校验商品 ID 66–73、slug、分类 23、标题及在售状态，只更新中文 `description_json`、`content_json`、`seo_meta_json` 中的标题/描述及 `updated_at`。其他语言、分享图与其他 SEO 字段保持。已有文案若与抓取基线及批准稿均不一致则整批拒绝，不能覆盖较新的运营修改。每次写入先独立备份，单事务比较全部业务表及受保护字段，保存变更日志；重复执行不改变内容或更新时间。Plus 月付缺货状态、售价、SKU、映射与历史记录不参与更新。

## 公开页面与后台提示

- SSR 与两套商城详情模板使用同一八款身份集合。每页显示其他七款的当前标题、匿名价格和当前规范链接，排除自身；别名仅用于识别改名后的同一商品，不写进新链接。分销站不显示该固定集合。
- `DeliveryAndReturns().AppendLink` 和商城对应逻辑共享一句“售后与退款流程见交付与售后说明。”，正文已有目标 path 时不追加。
- Product JSON-LD 使用现有栅格分享图候选或站点默认 JPG，页面 SVG 插图保持。Offer 的价格、币种、库存及退货政策引用保留，不新增评分。
- 后台详情编辑区对同分类在售商品规范化正文，用字符三元组 Jaccard 比较；去除 HTML、数字、价格符号、档位和周期词后超过 0.85 提醒。短于 60 字的规范化文本不参与；最多读取前 100 条，超出或读取失败会显示范围提示。提示不阻止保存，也不能证明 Google 会收录。
- 原 `products.updated_at` 到 sitemap UTC `lastmod` 的机制已有测试覆盖；不为没有改内容的 11 个新地址伪造更新时间。

## 本地与发布后验证

本地预览使用仅含公开目录和测试账号的独立数据库，没有订单、卡密或供货连接。待确认草稿只写该副本。

```sh
go test ./internal/platform/http/publicpage ./internal/platform/publiccontent ./internal/bootstrap/publicpage ./internal/modules/sitemap/... ./internal/modules/catalog/...
cd frontend/user && pnpm test && pnpm build
cd ../admin && pnpm test && pnpm build
```

在仓库根目录执行正文检查、事务回归和只读验收：

```sh
node scripts/check-product-content.mjs
python -m unittest discover -s scripts/tests -p test_seo_grok_v4_content.py
python scripts/verify_seo_v4_public.py --base http://127.0.0.1:8080
python scripts/verify_seo_v4_public.py --base https://www.qzweb.cloud --content-date YYYY-MM-DD --out work/v4-production-public.json
```

`--content-date` 填批准文案实际写入的 UTC 日期。验收脚本在文案仍为旧版时会失败，不可把失败改成通过。它检查八页 200、自身 canonical、标题/H1、独立摘要和 FAQ、七个互链、短政策链接、Product 栅格图和 API 价格/库存一致、Plus 月付缺货、HEAD、分享卡、sitemap 日期、28 组正文相似度，以及 11 个新地址的分类入口。报告同时记录 UTC 与北京时间。另须在浏览器确认 hydration 后元数据和正文仍一致、互链可跳转、手机无溢出。

部署只发布审核过的后端及商城/后台构建，保留共享脚本、私有配置和历史静态资源。v4 无网关规则变化，Nginx 不更新。正式内容确认后再定向回填，清理仅 `sitemap:xml:*` 缓存键；绝不清空整个 Redis。最终记录公开验收及库存服务最新成功状态。不得用真实下单、付款或采购验收。

回退代码通过版本目录软链接；文案回退仅恢复变更日志中的八款本次公开字段，并在恢复前确认当前值仍是该批发布值。回退后刷新 sitemap；不得整库回灌，也不得恢复已经被运营再次修改的字段。

## 站长 Search Console 清单

**仅在批准的独立正文已上线且公开验收通过后执行。** 使用 `https://www.qzweb.cloud/` 属性，重新提交 `https://www.qzweb.cloud/sitemap.xml`。对以下 11 个当前地址逐条做“网址检查 → 请求编入索引”：

| 当前商品路径 | 现有分类入口 |
| --- | --- |
| `/products/chatgpt-pro-200-renew-ph` | `/categories/gpt-recharge` |
| `/products/telegram-premium-12m` | `/categories/telegram-premium-links` |
| `/products/chatgpt-pro-100-ph` | `/categories/gpt-recharge` |
| `/products/chatgpt-pro-200-ph` | `/categories/gpt-recharge` |
| `/products/chatgpt-pro-500-ph` | `/categories/gpt-recharge` |
| `/products/chatgpt-pro-500-egypt` | `/categories/gpt-recharge` |
| `/products/chatgpt-pro-100-egypt` | `/categories/gpt-recharge` |
| `/products/gemini-pro-18m-warranty` | `/categories/gemini` |
| `/products/gemini-pro-18m-no-warranty` | `/categories/gemini` |
| `/products/chatgpt-plus-manual` | `/categories/gpt-overwrite-topup` |
| `/products/chatgpt-go-manual` | `/categories/gpt-overwrite-topup` |

再检查 `ellaix-grok-plus-monthly`、`ellaix-grok-plus-yearly`、`ellaix-grok-premium-monthly`、`ellaix-grok-premium-yearly`、`ellaix-grok-super-monthly`、`ellaix-grok-super-yearly`、`ellaix-grok-ultra-monthly`、`ellaix-grok-ultra-yearly` 八个 `/products/` 地址的“Google 选择的规范网址”。若仍选公告或政策页，先看最新抓取是否包含批准的新正文，再请求索引。旧 `qzweb-*`、`86gamestore-*` 地址继续 301，不提交旧址；已经索引且本次没改正文的六个改名页无需重复请求。

几天后复查重复页与“已发现尚未抓取”状态。公开 HTTP、schema 与链接验收不等于 Google 已重新抓取、改选 canonical 或完成索引；人工 GSC 结果需另行记录。
