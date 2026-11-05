# SEO P0 基础修复（2026-11-05，仅本地）

本次只修复规范站点地址、sitemap 和 robots 三项。未提交、推送或部署，未修改正式配置及业务数据。

后续状态：用户已追加授权商品和文章正文优化，详见 [公开正文维护说明](seo-public-content.md)。以下是 P0 阶段历史记录；当前 5180 本地预览由 `work/codex-logs/seo-content-20261105/` 管理，原 P0 预览进程已停止。最新完整验证包含此前三项修复。

当前基线已于 2026-11-05 通过 `git fetch origin main` 和快进合并同步到 `39e109a00fbe88d4ab8ae29008f293e159e0cc51`（2026-10-28，含空分类与分类路由修复），HEAD 与 origin/main 一致。初次验收基线为 `0ad9f79`，落后 4 个提交；以下结果已在最新基线上重新执行，预览也已重建。同步前保留的 28 份文件逐字节一致，备份见 `work/codex-logs/sync-latest-20261105/before-sync/`。

## 行为

- `internal/platform/http/siteurl` 统一解析 SEO 使用的站点根地址。合法的公网 HTTP(S) 配置仍优先使用；本机、私网、带凭据或路径等无效配置回退到当前请求的 Host 与协议。网关须保留 Host 并设置 `X-Forwarded-Proto`，不采用任意 `X-Forwarded-Host`。
- 公开配置在缓存读取后、缓存写入后分别处理返回值，克隆 brand 字段，避免不同域名的请求互相污染缓存。不改写数据库中的 brand 配置。前端现有 canonical、`og:url`、商品 JSON-LD 自动复用修正后的地址。
- 商城网关精确代理 `/sitemap.xml` 和 `/robots.txt` 到后端，避免落入 SPA 首页回退。沿用现有代理的错误处理和后端缓存头。
- robots 保留私有 API 和交易路径限制，精确允许页面渲染所需的公开配置、商品、分类、文章、横幅及会员等级接口。`Sitemap` 指向同一规范域名。

## 本地验收

证据目录：`work/codex-logs/seo-p0-20261105/`。

- 788 项 Go 测试（含子用例）、190 项商城测试、类型检查及两端构建通过；架构检查及 `git diff --check` 通过。
- 93 项 HTTP 检查通过。Nginx 真正转发到本次构建的 Go 后端，数据库为原本地 SQLite 的副本，刻意保留 `http://127.0.0.1:4170` 错误地址以复现问题。
- 本地网关模拟正式 Host 和 HTTPS 转发头，公开配置、XML 地址及 robots Sitemap 均使用 `https://www.qzweb.cloud`。46 条本地 sitemap URL 可访问，覆盖本地 29 款在售商品。
- Googlebot、bingbot、OAI-SearchBot 请求均得到正确 XML/文本。模拟上游不可用时两个资源返回 502，不回退为 200 首页。
- Chrome 实查首页、分类、商品详情、博客列表、文章详情、关于共 6 类页面；每页只有一个正确 canonical，`og:url` 一致，商品 JSON-LD URL 正确，无控制台错误。

预览：`http://127.0.0.1:5180/`。临时后端为 API 模式，采购关闭，队列、支付链扫描、邮件均关闭；原本地配置和业务库未覆盖。进程及日志记录在 `preview-state.json`，关闭入口为同目录 `stop-preview.ps1`。

这是本地验证，使用本地目录和数据；不代表正式站已生效，也不验证生产 TLS 或搜索引擎实际收录。SSR、内容优化等其他审计项不在本次范围。后续上线需要同时包含后端代码和这两条 Nginx 路由，不能只更新前端。

首次沙箱测试因 60 秒无输出中止；改用带编译日志的有限时测试后通过。一次架构检查发现新增测试文件超出目录预算，已合并到现有文件并重跑通过。临时 Windows Nginx 的 include 路径和启动脚本退出码读取问题均已修正，最终 `nginx -t` 与 HTTP 检查通过。
