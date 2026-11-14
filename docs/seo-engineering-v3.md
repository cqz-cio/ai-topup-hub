# 2026-11-14 SEO 工程 v3

基线：`dc946e3a9d3e77a99fbfb2d5d7025f92f6197630`。开发分支：`codex/seo-engineering-v3-20261114`。范围来自桌面的 v3 工程报告；既有 P0 路由、HEAD、404、favicon、gzip 和尾斜杠规则继续保留。

## 实现

- Go SSR 与 Vue 使用相同的分享图清单。默认 JPG 和 50 张目录分享图均为 1200×630；后台商品、分类编辑器上传分享图时生成留白 JPEG。普通长图没有确认尺寸时不伪造宽高。Logo 的 JSON-LD 身份独立保留。
- 商品和分类规范 slug 使用小写 kebab，可按标题生成拼音，冲突追加数字。别名永久绑定原 ID，规范名也预留；下架、删除或不可见实体不会借别名跳转到其他商品。公开接口返回当前 slug，客户端替换旧地址并保留查询串和片段。
- 固定专题引用清单以旧 alias 标识实体，一次 SQL 批量解析当前 URL。SSR 把同一份映射交给 Vue；配置接口在共享配置缓存之外解析。缺失实体移除相应链接，分销站不应用主站专题。测试覆盖被其他实体占名时的 `-2` 和后续人工改名。
- 首页 H1、商品中心、博客、公告、关于和单词分类标题共享中文文案；三份 locale 同步。手机首页专题入口移出原来只容纳排序按钮的绝对定位区域。
- 新场景资源位于 `/media/storefront-20261114/`。背景为 AVIF/WebP 两种尺寸，renderer 保持原艺术尺寸、坐标和动画参数。原媒体字节不变，clean-plate/foreground alpha 逐像素一致。
- 仅匿名、200、非 noindex、无 search 参数的公开 HTML 使用 `public, max-age=60, s-maxage=120, stale-while-revalidate=300`。Cookie/Authorization、404、搜索和业务 SPA 保留禁止共享缓存的规则。商品/会员价格 JSON 仍为 private/no-store。
- 首页/商品中心改用匿名列表批量投影，删除逐 slug 的完整商品详情查询及重复库存汇总；价格装饰算法不改。促销/上游库存装饰仍使用既有逻辑，不宣称全部 SQL 查询已变成常数。

## 迁移与边界

`cmd/catalog-seo-migrate` 只创建两张 alias 表、分类 SEO 字段及一次性迁移标记，修改 products/categories 的 slug。默认事务回滚，显式 `-apply` 才提交。应用启动注册相同迁移，标记完成后不会重跑或覆盖人工改名。

正式库在线备份副本于北京时间 2026-11-14 23:04 验证：19 个 URL 修改（17 商品、2 分类），73 张既有表除获准字段外摘要一致；dry-run 不留改动，第二次 apply 无变化。备份目录记录于私有发布报告，不提交数据库或凭据。

没有修改支付、结账、订单状态机、卡密、采购、后台登录或鉴权代码；没有改售价、成本、库存规则、供应商映射或历史快照。原工作区及其未提交内容保持。

## 本地验证

- Linux Go 1.26.5：`go test -timeout 180s ./...` 和 `go vet ./...` 通过，含架构预算检查。Windows 全量测试中的文件权限/锁兼容性失败用 Linux 环境复验，不改自更新或日志模块。
- 商城 267 项、后台 23 项测试通过，两套类型检查及生产构建通过。售后链接去重测试保留原断言。
- 5173 独立 Chromium 检查首页、分类/商品旧地址、博客、关于、公告，以及 390/320px 和禁用 JS。没有 JS 异常或资源 404；SSR 与客户端分享卡均为 summary_large_image。旧分类查询串保留，专题落到规范 URL。
- 本地会员登录、购物车及结算页导航通过，未提交订单；后台登录、按名称生成 slug 及图片上传预览通过，下载的上传产物为 1200×630 JPEG。公开产物扫描覆盖商城 492 个、后台 124 个文件，没有敏感交付内容命中。
- 本地预览仅导入公开目录/文章和公开图片，使用独立数据库、Redis 和测试会员；采购关闭，不包含正式支付配置、订单或卡密。预览自动发码商品没有复制正式库存。
- 51 张分享卡为真正的 1200×630 JPEG；新 1280 WebP 97,828 B，1672 WebP 132,118 B，AVIF 分别 75,179/104,787 B；旧背景约 1.6 MB。体积变化不等于实测 LCP 提升。

私有证据位于独立工作区 `work/seo-*.log`、`work/seo-browser-local/`、`work/seo-migration-preflight.json` 和 `work/seo-media-report.json`。

## Nginx 与性能

生产 nginx 1.24.0 没有 Brotli 模块，按报告保留 gzip，不安装或重编译。只应用 `ops/tencent/frontend-gateway/seo-v3-nginx.patch` 中两段新媒体 location，检查现网配置摘要后打补丁、`nginx -t` 并平滑重载。补丁基于实际 `/etc/nginx/conf.d/cardnet-qzweb-www.conf`，不使用仓库整份模板；现网 root 是 `/srv/zhixiang-front/current/user`。没有 proxy_cache。

发布前北京时间 22:26，在生产服务器分别测 5 次：首页后端 TTFB 22–28 ms、中位数 23.55 ms；HTTPS 本机网关 48–56 ms、中位数 52.09 ms。商品列表中位数为 24.34/53.79 ms；条款页 2.93/29.99 ms。这里不包含用户网络往返，不能与外网 0.8–0.9 秒直接比较或据此宣称端到端提升。后续数字记录在发布报告。

## 发布及回退

顺序：合并且 CI 通过 → 新一致性备份 → 在备份副本复验 → 定向迁移 → 已验证干净提交构建的后端、商城和后台编辑器资源 → Nginx 差异 → 仅清 `sitemap:xml:` 前缀 → 公开 GET/HEAD、图片、缓存、页面及库存 service 检查。

上传采用新 releases 目录；保留共享 config、env、uploads、scripts、drop-in、Redis、定时器和旧静态资源。记录源码 SHA、程序 SHA256、原发布目录及全部产物清单。构建须有 `vcs.modified=false`；不存在仅凭推送成功就标记上线的步骤。

回退先恢复原程序/静态目录软链接及本次 Nginx 备份，并检查配置后重载。如旧程序不识别 alias，按本次报告的 ID 和 before/after 定向恢复 slug，仅在当前值仍为 after 时执行；保留 alias 表供重新发布。不得整库回灌。恢复迁移前状态时须同步处理本次标记，避免旧版本启动产生意外重复迁移。

公开验收覆盖 robots/sitemap HEAD、真实 404、空分类、旧 slug GET/HEAD 301、尾斜杠最多两跳、查询串、canonical/sitemap 新 URL、分享图尺寸/MIME、gzip、新媒体长期缓存以及带 Cookie/Authorization 的 HTML 禁止共享缓存。登录、购物车、结算只验证页面与导航，不创建正式验收订单或执行付款采购。
