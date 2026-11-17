# SEO v5 顾客文案定向修复（2026-11-17）

用户要求立即清除公开页面中的“需店主确认”，仅更新指定的 8 款商品及其被引用的摘要。生产文案已修复并通过公开检查；本次没有重新部署后端、商城静态资源或 Nginx。

## 修改范围

以下链接均为当前规范地址：

| 商品 | 公开地址 |
| --- | --- |
| Telegram 3 个月 | https://www.qzweb.cloud/products/telegram-premium-3m |
| Telegram 12 个月 | https://www.qzweb.cloud/products/telegram-premium-12m |
| Pro 100 菲区 | https://www.qzweb.cloud/products/chatgpt-pro-100-ph |
| Pro 200 菲区 | https://www.qzweb.cloud/products/chatgpt-pro-200-ph |
| Pro 500 菲区 | https://www.qzweb.cloud/products/chatgpt-pro-500-ph |
| Pro 100 埃及 | https://www.qzweb.cloud/products/chatgpt-pro-100-egypt |
| Go 埃及兑换码 | https://www.qzweb.cloud/products/chatgpt-go-code-egypt |
| Go 人工代充 | https://www.qzweb.cloud/products/chatgpt-go-manual |

共替换 13 处文本：11 处正文、Go 埃及的商品摘要与 SEO 摘要各 1 处。把内部审核用语改为顾客可执行的“购买前请联系客服核实”，保留账号条件以兑换预检为准的限制，没有补造官方权益、优惠资格或时效承诺。原退款公式和其余正文保持。

Go 埃及的新摘要同时用于首页、商品中心和 GPT 充值分类。商品标题、售价、库存、图片、支付和卡密不在本次写入范围。8 个商品的 `updated_at` 反映本次真实修改，sitemap 对应 `lastmod` 为 `2026-11-17`。

维护文件为 `docs/catalog/seo-v5/customer-copy-fix-20261117.json` 和 `scripts/seo_v5_customer_copy.py`，8 份 HTML 源稿及 `copy.json` 同步修正，避免以后内容发布重新带入内部标记。审批清单 SHA256 为 `e7e45490cc02a1b14538b5110c53bd8e656ad3bbb6b9f2dd051ce84a2964e94f`。

## 生产验证与实际异常

- 定向事务修改 8 款商品的 10 个 JSON 字段及 8 个更新时间，共 18 个字段。对 75 张表执行事务内非目标字段核验，其他值保持；重复计划没有待写字段。
- 第一次写入于北京时间 10:14 提交，随后报告序列化失败；公开检查还发现 JSON 从 SQLite BLOB 写成 TEXT，导致这 8 页正文暂时无法由后端读取。这是实际生产异常，不能把首次执行记为完整成功。
- 10:19 已把这 10 个 JSON 字段恢复为原有 BLOB 存储类型，保留批准的新文案及更新时间；修复事务再次核验 75 张表其余值保持。没有重跑整份生产文案写入。
- 修正脚本保留原存储类型，并让备份报告以可逆的十六进制记录 BLOB。随后在生产原始备份副本上完整执行 CLI，报告、类型和业务数据保护检查通过；生产最终计划为零修改，实际运行版本与进程保持。
- 最终公开检查覆盖 8 个商品全文/API/普通访问/Googlebot、3 处列表摘要、16 个获准提交 URL 的 HTTP 状态和自指 canonical，以及 sitemap 日期。8 页均无内部标记，正文、FAQ 和 BreadcrumbList 正常。Go 埃及真实浏览器展示也已复核，原购买区、售后全文和问答正常。
- 新增 3 项回归测试：BLOB 存储及其他语言/业务字段保护、并发人工改文案拒绝覆盖、完整 CLI 可逆报告。首轮测试遇到 Windows 子进程文本编码和 SQLite 连接未关闭的问题，修正测试后全部通过，失败日志保留。

此前已开启的 gzip 保持。Brotli 仍未启用，只登记，不重编译 Nginx。未创建订单、真实充值、付款、采购或退款。

## 备份与回退

备份目录：`/srv/zhixiang/backups/before-seo-v5-customer-copy-20261117-101312`。

文案首次修改前的数据库一致性备份和最终可逆报告位于其 `apply/customer-copy-20261117T021406437895Z/`，根目录另有 `final-report.json` 和存储类型修复记录。原失败报告与日志保留。

如需回退，必须根据最终 `report.json` 对这 18 个字段逐项比较当前值是否仍等于 `after`，BLOB 值按 `blob` 十六进制解码，再恢复相应 `before`；发现后续编辑应停止并人工核对。不得回灌整库、回退价格库存，或切换旧程序版本。

## Search Console 与复检

站点地图 `https://www.qzweb.cloud/sitemap.xml` 已于 2026-11-17 重新提交，界面明确显示提交成功。提交成功不表示 Google 已完成新一轮抓取。

上述 8 页继续等待用户复检，本次不逐页请求索引。其余用户允许的 16 个规范 URL 按界面结果记录到 `docs/history/seo-v5-customer-copy-20261117.json`；索引请求被接受不等于已经收录或排名提升。没有提交旧网址。

本次成功请求 7 页：`/terms`、`/privacy`、`/blog`、`/notice`、`/about`、`/categories/gemini`、`/categories/grok`。其中 Gemini 分类的索引快照仍显示 11 月 16 日的旧重复网页判断，Google 当时选择 `/blog` 为规范页；这次现网自指 canonical 已验证，新索引请求已接受，不能把旧抓取快照当成修复后的结果。

在提交 Telegram 分类时 Google 明确提示“超出了配额”，要求次日再试，已停止继续提交。剩余 9 个已获准 URL 为：

- `/categories/telegram-premium-links`（本次被每日配额拒绝，检查显示此前已收录）
- `/categories/gpt-overwrite-topup`
- `/products/telegram-premium-6m`
- `/products/chatgpt-pro-500-egypt`
- `/products/chatgpt-pro-200-renew-ph`
- `/products/chatgpt-pro-100-manual`
- `/products/chatgpt-pro-200-manual`
- `/products/gemini-pro-18m-warranty`
- `/products/gemini-pro-18m-no-warranty`

这些待提交项与需用户先复检的 8 页分别记录，没有把尚未提交的项目记成成功，也没有创建定时重试。

完整私有证据与截图保留在工作区 `work/seo-v5-customer-copy/`；命令日志在 `work/codex-logs/`。上述生产验收完成时，维护源码和文档尚未单独提交、推送。

## 源码归档与复检

用户随后明确授权把本次文案修复源码和记录提交、推送至 GitHub，供 Grok 复检，并要求不要重复部署。本次归档分支为 `codex/seo-v5-customer-copy-20261117`，基于 main `71c7357018592eb537fc75b97467152b6ad46730`，仅包含本文所列的 8 份商品正文、Go 摘要、替换清单、修复脚本、3 项测试及验收记录，共 14 个文件。

源码归档不合并 main、不创建发布标签、不执行生产 `--apply`、不重新部署，也不再次提交 GSC；8 个商品网址仍等待用户复检通过。
