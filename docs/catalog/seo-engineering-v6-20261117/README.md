# SEO v6 第一阶段审阅包

后续状态：2026-11-18 已按店主新授权限定上线 cart 规则和两处内容修改，见 [正式上线记录](../../seo-v6-release-20261118.md)。成品号页面、弹窗、分类表及截图草稿仍未发布。以下为第一阶段的历史审阅记录；原计划和审批来源保留，不把全部草稿视为获准上线。

分支：`codex/seo-engineering-v6-20261117`。执行时 fetch 到的最新 main / merge-base：
`781fcd1062bbccfa4a25fe5d0f7dad051a5b8885`（2026-11-17）。

任务来源为用户指定 UTF-8 JSON 的 content：qzweb-seo-engineering-report-v6.md，428 行，truncated=false，已完整读取。只完成第一阶段；本分支不合 main、不部署、不写生产库、不操作 Search Console。v2–v5、已发布菲区教程、原商品 URL 以及旧址 301 保持。原主目录 `codex/glass-header-footer / 09dae2114` 既有改动未动。

## 阅读入口

- [可直接打开的 HTML 预览](preview.html)；本次本地入口 http://127.0.0.1:5186/preview.html。
- [Plus 来源/前端/SEO 盘点](plus-inventory.md)。
- [店主确认、原四问、真实素材和成品号边界](owner-questions.md)。
- [截图取材说明与真实查单入口](screenshot-materials.md)。
- [首页弹窗实测与轻提示提案](popup-review.md)。
- [后续发布与定向回退纪律](publication-plan.md)。
- [站外人工清单](manual-outreach.md)。

## 可独立审核的 cart 代码

运行代码唯一修改处是 `internal/modules/sitemap/application/service.go` 的 GenerateRobots。保留 `Disallow: /cart`，新增：

```text
Allow: /cart$
Allow: /cart?
Allow: /cart/$
Allow: /cart/?
```

`?` 是 URL 查询分隔符，不是通配符；`$` 锚定末尾。最长匹配及相同长度 Allow 优先，让入口及其查询串可抓取，同时继续拦住 `/cart/checkout`、`/cart/api`、`/cart/extra`、`/cartoon`、`/cart-old` 等。查询串中出现 /checkout 不会把路径变成 checkout。

现网匿名 GET/HEAD 实测 /cart、/cart/、两者查询串都是 200，均带 `X-Robots-Tag: noindex, nofollow`、`Cache-Control: no-cache`。三种 cart 子路径是 404。没有因为尾斜杠额外改路由或 Nginx。robots 无法按登录状态鉴权；放行只是让抓取工具读到匿名公共壳的 noindex，原会话/API 权限没有变。

依据：[Google noindex 说明](https://developers.google.com/search/docs/crawling-indexing/block-indexing?hl=zh-cn)、[robots 路径匹配与优先级](https://developers.google.com/crawling/docs/robots-txt/robots-txt-spec)。没有往 robots 写不支持的 noindex 指令。

## 其它 robots 匹配范围（仅报告，不修改）

| 原规则 | 本轮测试/发现 | 处理 |
| --- | --- | --- |
| /checkout、/pay 前缀 | 本体、子路径与查询串均禁止；/payment-guide 等同前缀也被禁止 | 保持，是否缩小另审 |
| /me/、/guest/、/admin/、/auth/、/orders/、/recharge-orders/ | 带斜杠路径及后代禁止；裸路径 /me、/guest、/admin、/auth、/orders、/recharge-orders 没有被这些规则命中 | 潜在遗漏列给审核；robots 不是访问控制，未改鉴权或扩展此次规则 |
| /api/ | 默认禁止；config/products/categories/posts/post-categories/banners/member-levels 精确与查询例外保留 | 不调整例外 |
| public products/、posts/ | 既有详情前缀 Allow 继续覆盖其后缀 | 不顺手改范围 |
| /sitemap.xml | 允许；现网 47 条，不含任何 cart URL | 不加 cart、不伪造 lastmod |

## 修改清单与草稿边界

| 文件/目录 | 目的 |
| --- | --- |
| sitemap application/service.go | 唯一运行变更，四条 cart 精确例外 |
| sitemap integrationtest/service_test.go | 实际规则匹配矩阵、公共 API 例外、sitemap 排除 cart |
| web/user_routes_test.go | 匿名 GET/HEAD 同头、静态壳、无缓存、无 Set-Cookie，查询不能变出用户数据 |
| scripts/verify_seo_v6_public.py | 可重跑匿名公开 GET/HEAD、页面/JSON-LD、25 商品和 sitemap 取证；无凭据/写操作 |
| scripts/test_seo_v6_review.py | 独立 REP 匹配夹具（最长匹配、相同规则 Allow、查询与 $、*） |
| scripts/build_seo_v6_preview.py | 从四份 docs 草稿重建独立 noindex 预览 |
| scripts/check-seo-v6-drafts.mjs | 复用现有 3-gram Jaccard 去噪算法检查增量草稿 |
| 本目录 draft-*.html | 分类、Plus、菲区教程的增量段及成品号草稿；不是可直接导入的完整正式稿 |
| 本目录说明与 evidence/ | 来源、待确认问题、实测截图与输出 |

店主后续确认“按商品信息来，去掉吧”，以及成品号 Plus 一个月订阅、订阅质保一个月、115 元/个。分类草稿的待答复单元格已替换为原商品资格/预检说明；教程草稿补充套餐区分句，去标签计划只针对 #30。确认原文见 [owner-confirmation.json](owner-confirmation.json)，字段计划见 [content-change-plan.json](content-change-plan.json)。这些更新未导入生产。

Plus 原退款/质保正文与三个 FAQ、菲区教程六个 FAQ/BlogPosting/BreadcrumbList 均未替换。没有新路由、SKU、媒体覆盖或内容迁移。未把去标签解释为全球通用，也未给成品号新增退款、换号或安全承诺。最终全文仍须按原去噪 3-gram Jaccard 算法复核，当前增量比较见 evidence/draft-similarity.json。

## 验证与限制

- 商城前端：278 / 278 测试通过。
- Go 专项：sitemap 全部包、web、architecture，133 个顶层测试 / 799 个含子测试通过，文件预算通过；没有向顶格的 sitemap transport/http 新增文件。
- 全量 Windows `go test ./...` 有两个包失败：logger 的 Windows 文件占用清理；selfupdate 的 unsupported_os、POSIX 权限及锁语义。保留失败记录，未改这些模块，也不把此轮全量说成通过；Linux CI 结果在 PR 检查中。
- 两项 Python 离线匹配测试、草稿相似度及 `git diff --check` 通过。
- 两轮公开取证各 27 个 GET/HEAD 请求，errors 为空，25 款商品、47 个 sitemap URL；**当前正式 robots 仍阻止 cart，这是未部署的预期现状**。
- 两轮公开商品售价、有效售价、上下架及 stock_status 一致。#55 SKU55 的 manual_stock_total 42→41，#57 SKU57 的 172→171，其余字段一致；只读观察到库存变动，未查交易/同步日志，不能确定具体来源。没有为对齐快照回写库存或改价。
- 实际浏览器证实 Plus 标签可见、首页弹窗遮挡；关闭、今日、永久、错误存储值及旧版本行为留档。预览桌面/390/320、展开提示验证通过；预览无 console entries。
- 未登录后台或 GSC、未新建订单、未支付、未采购；没有新的收录/排名验证。

详见 [测试输出](evidence/test-output.txt)、[浏览器记录](evidence/browser.json)、[前快照](evidence/public-before.json)、[后快照](evidence/public-after.json) 与 [相似度](evidence/draft-similarity.json)。

店主补充后的本轮只改 docs 和预览生成器，未再改运行代码。重新生成预览、检查六对草稿相似度及 diff 格式；浏览器再次核对桌面、390/320 宽度、成品号三项事实和分类待答复单元格已移除。新增公开查单入口截图（无输入、无提交）及成品号 390px 预览，见 [本轮记录](evidence/owner-followup.json)。此前前端/Go 成绩不当作本轮重新执行；新提交的 CI 以 PR 检查为准。

## 重跑（在独立工作区）

```text
go test -v ./internal/modules/sitemap/... ./internal/web ./internal/architecture
go test ./...
cd frontend/user
pnpm install --frozen-lockfile
pnpm test
cd ../..
python -m unittest discover -s scripts -p test_seo_v6_review.py -v
node --experimental-strip-types scripts/check-seo-v6-drafts.mjs
python scripts/build_seo_v6_preview.py
python -m http.server 5186 --bind 127.0.0.1 --directory docs/catalog/seo-engineering-v6-20261117
python scripts/verify_seo_v6_public.py --expect-cart blocked --output work/v6/recheck.json --compare docs/catalog/seo-engineering-v6-20261117/evidence/public-before.json
```

公开核验器只用 GET/HEAD；生产仍未部署时必须预期 blocked。第二阶段经批准部署后才使用 `--expect-cart allowed`。出现差异时报告，不用重试写操作或调库存来消除差异。

到此交店主转 Grok 审阅。Plus 事实与内容审批不阻塞 cart 独立送审；收到明确“审核通过，可以合入/部署”前停止。
