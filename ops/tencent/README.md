# 上海前端与新加坡后端

π星卡社的[商城](https://www.qzweb.cloud/)和[后台](https://124.220.2.69:18888/login)前端运行于上海 `124.220.2.69`，目录 `/srv/zhixiang-front`。新加坡 `43.134.141.10` 的 `/srv/zhixiang` 保留 Go 后端、生产数据库、持久化 Redis、支付监听、采购交付和库存定时任务。

2026-10-12 16:30 完成 `www A 124.220.2.69` 的腾讯云身份验证及解析保存，根域 `@` 解析未修改。16:53 核验正常域名访问为新版 π星卡社，HTTPS 链与域名验证通过；管理端使用独立 IP 端口，`www` 下的 `/zxa-management/` 返回 404。旧新加坡 HTTP 入口仍可用于兼容，但日常登录和访问使用上海 HTTPS 入口。

## 上海入口维护

2026-10-13 15:38 已按批准的 B 方案上线[后台商品缩略图修复](../../docs/admin-product-thumbnails.md)：100×60 完整横图、加载失败占位，并恢复后台对商城 `/catalog-products/` 及 `/products/` 图片文件的访问。上海发布 `20261013-153730`，源码 `471ded02e1b265520a53485288bdcc4f47dbece9`；只替换后台资源及后台 Nginx 配置并平滑重载。新加坡程序 `20261013-114011` 与商城资源来源保持原基准。

本机 58 项后台测试、类型构建、50 项 HTTP 和 7 项实际浏览器检查通过；正式 49 项服务器检查、9 项浏览器检查通过，40 个商品图片地址及 44 条列表记录均正常，119 个后台文件和 1468 个保留商城文件核验通过。320px 且有系统滚动条时，原顶部控件有 5px 溢出，已记录；本次未改顶栏布局。两地备份分别为 `/srv/zhixiang/backups/before-product-thumbnails-20261013-153730`、`/srv/zhixiang-front/backups/before-product-thumbnails-20261013-153730`，报告为上海 `shared/work/product-thumbnails-20261013-153730.json`，本机证据位于 `work/codex-runtime/product-thumbnails/`。售价、成本、商品图地址、支付与站点配置、个人专属价及后端和 Redis 进程与切换前备份一致，未上传本机库、修改业务数据或进行付款采购测试。

2026-10-13 14:43 已修复商品列表直接访问的 403 和服务器错误页残留旧图标。`/products` 与商城静态图片目录同名，原 `try_files $uri $uri/ /index.html` 优先命中目录并触发禁止目录列表；现改为 `try_files $uri /index.html`，真实图片仍直接读取，商品列表及带尾斜杠、查询参数的地址交给 Vue 路由。根域继续跳转到 WWW。

2026-10-13 14:52 已补齐后台错误页标签图标的响应类型修复，配置源码 `7a669ccbf6d6165a26daf82e5a4aa3cb06e52ff9`：缺失 `.js`、`.css`、`.png` 等资源时，命名错误位置清空扩展名 MIME 映射并明确返回 HTML，浏览器才能解析图标声明。错误状态码、版本化派大星图标、API 错误透传保留。本机 37 项 HTTP 和 22 项浏览器检查、正式站 16 项浏览器和 12 项服务器 HTTP 检查通过；本机覆盖六类错误状态，正式站只读验证 403/404、登录页、浏览器内模拟接口失败和失效登录，未制造正式服务中断。两地备份均为各自 `backups/before-admin-error-icon-20261013-145235`；报告 `/srv/zhixiang-front/shared/work/admin-error-icon-20261013-145235.json`，本机证据 `work/codex-runtime/admin-error-icon/`。仅更新共享错误片段并平滑重载 Nginx，现有 1960 个前端文件及程序发布、后端与 Redis 进程、价格和收款配置保持一致。

商城和后台共同加载 [错误页与图标配置](frontend-gateway/nginx-errors.conf)，安装位置 `/etc/nginx/snippets/qzweb-errors.conf`。默认 `/favicon.ico` 直接返回现有派大星 PNG，明确图片类型并要求重新校验缓存；403、404、500、502、503、504 错误页在 HTML 中指定带版本的图标地址，保留原状态码和错误正文结构，不依赖 Vue、API 或旧图标缓存。后端 API 错误响应不拦截。以后安装或更新 [商城配置](frontend-gateway/nginx-www.conf)、[后台配置](frontend-gateway/nginx-admin-ip.conf) 时，须先安装这一共享片段，再执行 `nginx -t` 和平滑重载；若更换默认 Logo，同时更新错误页图标 URL 的版本值。

配置源码 `9fd00866eb655b0da9e95c8fdaca20152c73911e` 先通过本机 Nginx 1.24 的 35 项检查及日常 `5173/5174/8080` 入口核验，再推送部署。正式服务器 15 项、公网 16 项 HTTP 检查通过；实际浏览器复查商品列表、刷新、403 与 404 页面的图标引用。新加坡一致性备份 `/srv/zhixiang/backups/before-favicon-route-20261013-144241`，上海配置与证书备份 `/srv/zhixiang-front/backups/before-favicon-route-20261013-144241`，服务器报告 `/srv/zhixiang-front/shared/work/favicon-route-20261013-144241.json`，本机报告和截图在忽略目录 `work/codex-runtime/favicon-repair/`。仅安装三份 Nginx 配置并平滑重载：上海发布仍为 `20261013-143008`，现有 1960 个前端文件哈希不变；新加坡程序仍为 `20261013-114011`，后端、Redis、库存定时器及 storefront 模式正常。公开售价与成本、专属价格、两条支付渠道、站点设置与私有配置均与部署前备份一致，未修改业务数据或执行付款采购测试。

2026-10-13 已修复不带 `www` 的根域进入家具页面的问题。新增 [根域入口配置](frontend-gateway/nginx-apex.conf)，实际安装于上海 `/etc/nginx/conf.d/qzweb-apex.conf`：`http://qzweb.cloud` 与 `https://qzweb.cloud` 均以 308 跳转到 `https://www.qzweb.cloud`，保留原路径和查询参数。根域使用独立的有效证书，续期配置位于 `/srv/zhixiang-front/shared/letsencrypt/renewal/qzweb.cloud.conf`，由现有 `qzweb-certificates.timer` 管理；HTTP 的 ACME 验证目录仍为 `/var/lib/letsencrypt`。DNS 没有修改。

原因是当前 Nginx 只加载 `/etc/nginx/conf.d/*.conf`，此前已加载的商城配置只声明 `www.qzweb.cloud`，根域请求落入 80 端口的默认家具站 `/etc/nginx/conf.d/oakved.conf`，其页面目录为 `/opt/oakved/frontend/furniture`、标题为 `RH Layout Extraction Preview`。该家具配置仅声明服务器 IP，没有显式绑定 `qzweb.cloud`。旧 `/etc/nginx/sites-enabled/qzweb.cloud` 指向另一套旧站配置，当前没有被加载，不应据此判断正在运行的入口。

追溯证据均按北京时间记录：腾讯云控制台已有记录表显示根域 `@ A 124.220.2.69` 的最后操作时间为 **2026-06-02 10:50:12**，`www` 为 **2026-10-12 16:30:35**；检查时控制台要求重新登录，以上取自已显示记录表，并非新查询的操作审计。服务器家具首页文件创建于 **2026-07-22 09:03:15**，当前 `oakved.conf` 的创建及修改时间均为 **2026-09-21 09:46:32**。这些证据分别说明解析记录和文件时间，不能认定有人在其中某一时刻单独把根域绑定到家具站，也不能由此确定操作者。

配置源码 `2774d53935c098d9a1c86b90a425ca826e91bdc3` 先通过本机 Nginx 1.24 的 6 项 HTTP/HTTPS 跳转、1 项 ACME 入口及日常 `5173/5174/8080` 检查，再推送上线。正式公网 9 项检查覆盖根域两种协议、路径和参数、原 `www` 入口及管理端，实际浏览器也从根域进入 π星卡社。新加坡一致性数据库备份为 `/srv/zhixiang/backups/before-apex-domain-20261013-135130`；上海 Nginx 与证书备份为 `/srv/zhixiang-front/backups/before-apex-domain-20261013-135130`。服务器报告为 `/srv/zhixiang-front/shared/work/apex-domain-20261013-135130.json`，本机证据在忽略目录 `work/codex-runtime/domain-diagnostic/` 和 `work/codex-logs/`。仅增加根域配置及证书并平滑重载 Nginx；家具站与原商城配置哈希保持一致，程序仍为 `20261013-114011`，未替换应用、修改业务数据或进行付款采购测试。

2026-10-13 00:34 按用户要求补齐注册页链接对应的[隐私政策](https://www.qzweb.cloud/privacy)和[服务条款](https://www.qzweb.cloud/terms)。文案快照为 [site-legal.json](../../docs/catalog/site-legal.json)，配置来源 `9b260f889420d8c386c6c2c20d76c27e1630a94d`；通过既有后台 API 仅更新 `site_config` 中的 `legal.privacy.zh-CN`、`legal.terms.zh-CN`。后台 **系统设置 → 站点设置 → 法律条款** 可继续编辑。正文覆盖邮箱注册、订单查询、USDT 核验、卡密交付、具体商品质保与退款、信息收集与存储、个人信息权利及客服微信 `Cqz0924ttxk`；兑换入口遵循订单说明。

先在日常本地入口保存正文、核对实际页面，再推送文案并更新正式配置；本机与正式站各通过 1920、390、320px 共 9 项只读页面检查，两页各 9 节完整显示，注册页两个链接实际点击、刷新保留、未自动勾选协议、无横向溢出和脚本错误均通过。没有注入测试配置。线上备份 `/srv/zhixiang/backups/before-site-legal-20261013-003435`，报告 `/srv/zhixiang/shared/reports/site-legal-20261013-003435.json`；本机报告与截图位于忽略目录 `work/codex-runtime/site-legal-qa/`，配置更新报告为工作区私有文件 `deployment/local/site-legal-deployment.json`。前后端发布、其他站点设置、商品/SKU 售价、两条支付渠道和 storefront 服务状态保持一致；未替换程序、重启服务、上传本机库或提交付款及采购测试。

当前上海静态发布 `/srv/zhixiang-front/releases/20261013-153730`，本次只替换后台资源并补齐商品图片映射，后台源码为 `471ded02e1b265520a53485288bdcc4f47dbece9`；商城文件从原正式版本逐项校验后保留，源码仍为 `aaff34ff2af8d62f9a2cc4061007c6e41b54ea80`。新加坡程序仍为 `/srv/zhixiang/releases/20261013-114011`，程序源码同为 `aaff34ff2af8d62f9a2cc4061007c6e41b54ea80`。两地 `current` 分别指向上述实际版本，不以之后的文档提交替代组件构建来源。

2026-10-13 已完成 [后台品牌与运营入口统一](../../docs/admin-branding.md)：按批准预览采用 π星卡社与派大星 Logo，清理原项目名称、旧域名示例、推广位和上游升级入口。后台的商城入口固定使用 `https://www.qzweb.cloud/`，避免正式站已有的本机预览配置覆盖链接；本次未改写该旧配置值。55 项后台测试、类型构建，以及本机、正式站各 19 项真实浏览器只读检查通过，覆盖 1920、390、320px、三种语言与实际导航；核验 119 个后台资源和 1468 个保留商城文件。

两地备份分别为 `/srv/zhixiang/backups/before-admin-brand-20261013-143008`、`/srv/zhixiang-front/backups/before-admin-brand-20261013-143008`；报告为新加坡 `shared/reports/admin-brand-backup-20261013-143008.json` 与上海 `shared/work/admin-brand-release-20261013-143008.json`，本机证据在 `work/codex-runtime/admin-brand-qa/`、`work/codex-runtime/admin-brand-release/`。准备阶段已有业务记录更新，本次保留了这些更新，并以切换前数据库一致性备份核对发布区间。程序、服务进程、支付与私有配置、业务数据、公开售价和成本、Nginx 配置核对一致；未上传本机库、提交真实付款或采购测试。

此前会员价发布 `20261013-114011` 将统一折扣改为商品/规格固定优惠金额，保留会员等级和升级规则、个人专属价、钱包充值以及现有支付采购流程。

会员价发布说明见 [会员商品优惠价](../../docs/member-pricing.md)。日常本地入口验证了价格保存、清空、跨等级隔离及实际商品接口与订单预览金额，临时会员已清理。正式站 437 个前端文件哈希、商城与后台登录页、服务和业务数据对比核验通过；未在生产创建验收会员或改价。新备份为 `/srv/zhixiang/backups/before-member-prices-20261013-114011`、`/srv/zhixiang-front/backups/before-member-prices-20261013-114011`，核验资料在 `work/codex-runtime/member-prices-release/` 和 `work/codex-runtime/member-prices-qa/`。历史钱包发布记录如下。

2026-10-13 11:13 已为现有 BEP20、Plasma 渠道启用钱包充值，补齐充值支付期限、确认入账和收银页链上信息；后台 **用户详情 → 钱包明细** 正常提示超级管理员本人完成声明，确认后可主动加减余额。未代签正式声明。使用方法见 [钱包充值](../../docs/wallet-recharge.md)。日常本地入口实际验证加减余额和两条充值渠道，后台 52 项、商城 97 项测试及相关 Go 回归、类型构建通过；上线后 5 项只读浏览器检查、437 个静态文件哈希核验通过。

本次备份为 `/srv/zhixiang/backups/before-wallet-20261013-110949-retry`、`/srv/zhixiang-front/backups/before-wallet-20261013-110949`，部署报告分别为 `/srv/zhixiang/shared/reports/wallet-release-20261013-110949.json`、`/srv/zhixiang-front/shared/work/wallet-release-20261013-110949.json`。仅调整两条渠道的付款类型为 `order`、`wallet`；真实用户、余额和流水、订单、专属价、公开售价与成本、其他渠道字段及私有配置保持一致。Redis 未重启，自动充值仍关闭；未上传本机库、提交真实付款或采购验收。私有核验资料在 `work/codex-runtime/wallet-release/`、`work/codex-runtime/wallet-repair-qa/`。

2026-10-13 已上线 **用户管理 → 用户列表 → 用户详情 → 设置商品单价**。填写实际金额，清空恢复现有定价；只对对应登录用户生效，商品展示、购物车与下单预览一致。使用方法见 [用户商品单价](../../docs/user-product-prices.md)。本地 13 项、正式站 9 项浏览器检查及 437 项静态文件核验通过，后台 48 项、商城 95 项测试与相关 Go 回归通过。两地发布前备份为 `backups/before-user-prices-20261013-012037`；历史记录、公开售价、采购成本、支付渠道与私有配置摘要一致，未上传本机库或进行真实付款采购测试。

2026-10-13 00:27 已将公告、11 款 ChatGPT 商品购买说明及新订单发货指引的兑换入口更新为 **https://cdk.bar/**，配置来源 `0448e64db4058e52f6b16fe5ac1a23c70975578a`，公告版本 `bf3a1445`。商品同步工具同时迁移已有指引、iOS 原有链接及其他语言中的旧地址；原站资料快照保留，导入后应用当前兑换配置。9 项临时数据库测试和本机、正式站各三项实际桌面手机检查通过。历史订单、交付记录、售价及其他商品说明摘要一致；前后端程序版本、微信号 `Cqz0924ttxk` 和支付配置保持原状态。线上商品更新前备份 `/srv/zhixiang/backups/before-redemption-url-20261013-002736`，公告更新前备份 `/srv/zhixiang/backups/before-redemption-announcement-20261013-002738`，报告 `/srv/zhixiang/shared/reports/redemption-url-20261013-002736.json`。本机核验记录在忽略目录 `work/codex-runtime/redemption-url-qa/`。

2026-10-12 23:52 按用户要求将客服微信统一更换为 **Cqz0924ttxk**，不展示电话号码。已更新首页公告、首页客服区、全站页脚及对应复制按钮；公告版本 `e97dfd7a`。本地和正式站各通过 1920、390、320px 三项实际页面检查，覆盖公告与客服展示、剪贴板复制、商品详情及公告列表页脚，无旧号码、横向溢出或脚本错误；94 项前台测试、类型检查及生产构建通过。源码 `02c1a082c3b989adee398f16073e7ef8d89a4806`，仅部署上海商城静态资源并更新后台公告。新加坡配置、storefront 模式、支付与库存服务保持一致；修改公告前备份 `/srv/zhixiang/backups/before-wechat-contact-20261012-235139`，静态发布前备份 `/srv/zhixiang/backups/before-frontend-wechat-contact-20261012-235241`。发布报告 `shared/work/wechat-contact-release-20261012-235241.json`，浏览器记录在忽略目录 `work/codex-runtime/wechat-unified-qa/`。

2026-10-12 23:17 发布首页公告弹窗接入。后台 **系统设置 → 站点设置 → 首页公告** 支持标题、图文内容、开关及起止时间；首页复用既有弹窗与版本关闭记录。关闭后本次会话不重复弹出，支持今日不再提示和不再提示；标题或内容更新后重新提示。正式公告尚未填写，配置保持关闭，等待管理员自行设置。本地通过真实设置 API 临时保存公告并完成 12 项检查，随后恢复设置；正式商城通过 12 项只读浏览器检查，其中弹窗使用仅测试浏览器内生效的配置，未修改线上公告。94 项前台测试、类型检查及生产构建通过，六种宽度无溢出或脚本错误。发布报告 `shared/work/announcement-release-20261012-231734.json`，截图与报告在忽略目录 `work/codex-runtime/announcement-qa/`；新加坡一致性备份 `/srv/zhixiang/backups/before-frontend-announcement-20261012-231734`。未进行真实付款或采购测试。使用方法见 [首页公告](../../docs/home-announcement.md)。

2026-10-12 23:35 已按用户提供的参考图片整理并启用正式弹窗 **“购买须知与代理合作”**：包含购买与发货说明、ChatGPT 兑换入口、诚招代理与代理价咨询，以及本店客服微信 **Cqz0924ttxk**。兑换使用 `https://cdkai.kdns.fr/`，代理价及拿货方式联系客服；未沿用参考图片中的第三方联系方式。此次仅通过后台 API 更新 `home_announcement`，文案配置提交 `6c088c7a20725b5ebcfec10286e6346b2c8870d2`，公开版本 `c13a6e2c`；前后端程序发布保持上文版本。实际本机和正式商城各通过 1920、390、320px 三项浏览器检查，未注入测试内容，正文、联系方式、兑换链接及关闭后刷新均通过。线上修改前备份 `/srv/zhixiang/backups/before-announcement-content-20261012-233551`，报告 `/srv/zhixiang/shared/reports/announcement-content-20261012-233551.json`；本机截图在忽略目录 `work/codex-runtime/announcement-content-qa/`。服务、售价和收款渠道保持一致，未重启程序或进行真实付款采购测试。

2026-10-12 20:51 发布商品详情封面修复：主图采用 5:3 比例及 `object-contain`，缩略图也完整显示，避免 `object-cover` 裁掉 Go、Plus、Pro 图中文字。94 项现有前台测试、类型检查和生产构建通过；本机和公网各 30 项页面检查覆盖 15 款商品封面，以及 1920、1280、1024、768、390、320px 六种宽度，无横向溢出或页面脚本错误。DEMO 的既有图标不计入商品封面检查。后台文件、Nginx 配置、16 款售价和两条 USDT 渠道核对正常；后端配置、storefront 模式和库存定时器保持原状态。发布报告为 `shared/work/detail-image-release-20261012-205131.json`，浏览器截图及记录在本机忽略目录 `work/codex-runtime/product-detail-image-qa/`。新加坡一致性备份位于 `/srv/zhixiang/backups/before-frontend-detail-20261012-205131`；没有执行真实付款或采购测试。

`zhixiang-api-tunnel.service` 使用上海本机私钥，将 `127.0.0.1:18880` 转发到新加坡 `127.0.0.1:8080`。新加坡账号 `zhixiang-api-proxy` 只接受上海来源和这一转发目标，禁止终端、其他转发、代理转发与数据库访问；上海私钥不复制至开发机或提交 Git。Nginx 配置和服务模板见本目录 `frontend-gateway/`。后端反代禁止自动重试付款请求；管理 API 未登录返回业务状态码 401。

根域、WWW 与管理 IP 的三张证书由独立 Certbot 5.8.0 环境管理，配置目录为 `/srv/zhixiang-front/shared/letsencrypt`。`qzweb-certificates.timer` 每日 00、06、12、18 时 15 分触发，续期成功后校验并重载 Nginx；管理端使用短期 IP 证书，必须保持该定时器运行。原有 WWW 和管理 IP 两张证书的实际续期、Webroot 配置、定时服务和 HTTPS 访问均已核验；根域证书于 2026-10-13 签发并核验 HTTPS，续期配置已纳入同一定时器。根域及 WWW Webroot 为 `/var/lib/letsencrypt`，管理 IP Webroot 为 `/var/lib/enterprise-exam-acme`，不能互相覆盖；未改动服务器其他站点的证书环境。

上海证据保存在 `shared/work/frontend-final-report.json` 与 `shared/work/renew-final-20261012-1644/status.json`；新加坡最终只读核验报告为 `/srv/zhixiang/shared/reports/shanghai-backend-final-read.json`。本机截图与记录在忽略的 `work/codex-runtime/alipay-domain-qa/`、`work/codex-logs/`。腾讯云上海实例为 `lhins-kb6tfs2w`；尚未另行提供上海 SSH 凭据，维护可使用账号本人已登录的腾讯云自动化助手，不能借用其他账号或项目凭据。

入口 `index.html` 禁用 ETag 和 If-Modified-Since 条件缓存，刷新返回当前发布内容；带哈希的静态资源保持长期缓存。17:13 验证旧条件请求仍返回最新 HTML，并在浏览器确认新导航已加载。证据为 `shared/work/entry-cache-report.json`，修改前两份 Nginx 配置备份在 `backups/before-entry-cache-20261012-1710`。

## 已核验的运行基准

2026-10-12 16:11（北京时间）已完成发布 **`20261012-153531`**，应用已确认的第 2 版导航 Logo：派大星图标使用浅绿圆角方块，名称与副标题放在图标外，增加间距和上下留白。品牌图片、商品内容和购买行为保持一致。

| 项目 | 核验结果 |
| --- | --- |
| 当前发布目录 | `/srv/zhixiang/current` → `/srv/zhixiang/releases/20261012-153531` |
| 正在运行的程序 | 上述目录的 `zhixiang-shop`，90366114 字节 |
| 程序 SHA-256 | `5c3707bff99f2e16a66b4f9924150d2778ca7bcdc19b89e35dd7e2e737e6452f` |
| 发布清单 | `manifest.json` 的程序哈希与运行中程序一致 |
| 编译信息 | Go 1.26.5，Linux amd64，CGO=0，`release,fullstack` |
| 运行模式 | `storefront`，`UPSTREAM_PURCHASE_ENABLED=true` |
| 服务状态 | 商城、专用 Redis、库存定时器、Nginx 均 active |
| 构建源码 | `main` 的 `948c37f2d33420631eb77a75d3c1debe40d967b6`，`vcs.modified=false` |
| 上一个实际发布 | `20261012-141843`，源码 `06620f0e61a32fa17760b6f8b94312a29c03fb4e`，旧程序和升级前备份均保留 |
| 收款渠道 | BEP20 与 Plasma 均启用，公网 IP 支付配置已显示两项 |

完整证据见 [production-baseline.json](production-baseline.json)。发布清单、Go 编译信息及实际运行程序哈希相互匹配。公网 IP 的首页、后台、健康、配置和 20 项资源共 24 项核验通过；桌面 1920px、手机 390px 与 320px 的实际页面符合本地确认方案，Logo 与按钮没有重叠或横向溢出，浏览器无错误。后续文档提交不改变线上程序构建来源，库存仍为动态业务数据。

本次验证包括前台 94 项、后台 28 项现有测试、两套前端 TypeScript 检查及完整发布构建。部署前备份线上数据库、配置、私有凭据、旧发布、Nginx 配置和 Redis 快照；部署保留现场配置、商品说明、价格、付款渠道、权限与声明，Redis 没有重启。自动采购保持既有授权设置，自动充值仍关闭，未提交真实付款、验收订单或采购。截图和核验记录保存在本机忽略目录 `work/codex-runtime/logo-spacing-preview-20261012/`。

## ChatGPT 发货兑换指引

2026-10-12 13:57（北京时间）已应用商品说明配置，14:01 完成应用接口复核。11 款 ChatGPT 卡密（含两款 iOS 渠道）发货使用 **https://cdkai.kdns.fr/**，交付使用说明附完整链接和自助充值步骤；新订单保存该说明，交付页及已启用的发货邮件读取订单快照。历史订单不回写。

配置来源为 `main` 的 `2ef4790a64f351a85e91d59bebe593beb0f31dce`，映射和文案见 [qzweb-redemption-guide.json](../../docs/catalog/qzweb-redemption-guide.json)。该次更新仅修改商品数据库，没有替换程序；当时运行发布为 `20261012-134141`，构建源码为 `b478c600dc6cda365289d4bbb76501987d16a377`。当前程序发布见上表，本次 Logo 发布已对照核验商品交付说明内容保持一致。

数据库、私有配置和当前发布已在服务器备份。原始更新前备份为 `/srv/zhixiang/backups/before-redemption-guide-20261012T055249Z`，最终配置更新前补充备份为 `/srv/zhixiang/backups/before-redemption-guide-20261012T055749Z`；一致性检查均通过。报告为 `/srv/zhixiang/shared/reports/redemption-guide-20261012T055749Z.json`。

已核验 11 款后台商品详情实际返回兑换链接和步骤，两款 iOS 公开详情显示统一兑换入口并保留 30 天权益、剩余时长不叠加的规则。SQLite JSON 按现有 Go 存储契约写为 BLOB。8 项临时数据库测试覆盖备份、回滚、重复应用和存储兼容性；售价、支付渠道及配置核对通过，商城、Redis 和库存定时器保持 active。未创建真实验收订单或主动采购。

## 持久化数据与服务

| 用途 | 服务器位置 |
| --- | --- |
| 配置 | `/srv/zhixiang/shared/config.yml` |
| 生产数据库 | `/srv/zhixiang/shared/db/shop.db` |
| 上传与日志 | `/srv/zhixiang/shared/uploads`、`logs` |
| 持久化 Redis | `shared/redis/`，回环地址端口 16379 |
| 库存助手私有凭据 | `shared/stock-sync.credentials.json` |
| 库存报告 | `shared/reports/merchant1039-stock-sync.json` |
| 当前版本升级前备份 | `/srv/zhixiang/backups/before-20261012-153531`，数据库一致性检查通过，包含配置、凭据、旧程序、Nginx 与 Redis 快照 |
| 本次发布核验报告 | `shared/reports/logo-release-20261012-153531.json` |

`zhixiang-shop.service` 以 `zhixiang` 用户运行；`40-storefront.conf` 指定当前模式。支付监听、采购/交付 worker 与恢复任务由应用管理。`zhixiang-redis.service` 保存订单队列。`zhixiang-stock-sync.timer` 在每次任务完成后等待 300 秒，仅查询和刷新库存；不要另起重复助手或完整 worker 来同步库存。服务单元模板保存在本目录。

商户 1039 使用 `acg-faka`，已有 12 款供货映射，`auto_sync_price=false`。全部 16 款商品售价独立维护，`qzweb-55` 保持 1160.00 元。自动充值关闭；余额和库存需实时查询，不把历史报告的数值当作当前数值。客户确认付款后的采购已经授权，但没有做真实扣款验收，不主动提交有效采购或修改测试订单为已付款。

客户付到 BEP20 / Plasma 地址的 USDT 与供货商采购余额相互独立；自动采购需要供货商人民币余额充足、映射有效且对应 SKU 有库存。余额不足的采购记录进入 `rejected`，客户订单保留已付款状态，补足余额后需对这些失败记录手动重试；扣款结果未知时进入 `manual_review`，核对上游后再处理，不能直接重新采购。两款本地卡密商品和两款人工交付商品不走供货商采购。充值后无需重启，新采购每次都会读取最新余额。2026-10-12 只读核验了实际商户报价、库存、单件购买数量限制与本地采购成本上限，12 款供货商品中 11 款满足余额充足后的采购条件，`qzweb-59` 当时缺货；商品状态与价格会变化，正式采购前仍会重新检查。

链路回归测试位于 `internal/bootstrap/bscusdt/procurement_flow_test.go`：BEP20、Plasma 分别覆盖即时发卡、延迟取卡、队列恢复、余额不足和采购结果未知、充值后手动重试，共 12 个场景，同时核验最终客户订单响应包含交付内容以及重复任务不重复采购。2026-10-12 已通过；测试采用临时数据库和模拟链、队列传输及供应商，不构成真实资金验收。本机私有检查记录保存在忽略的 `work/codex-runtime/finance-release/verification-summary.json`。

## 状态检查

### 本机固定登录入口

用户已授权后续会话复用已提供的服务器信息，并于 2026-10-12 确认上海与新加坡均使用已提供的 `ubuntu` 登录信息。当前 C 盘工作区私有目录 `C:\Codex program\智享A社\deployment\local\` 已保存新加坡 `tencent-cloud-server.credentials.json`、`tencent-cloud-known-hosts`，以及上海 `shanghai-front-server.credentials.json`、`shanghai-front-known-hosts`。上海登录已验证，密码文件仅当前用户和 SYSTEM 可访问，全部由 Git 忽略。字段为 `host`、`port`、`username`、`password`；连接时核对目标 IP，加载对应已保存指纹并拒绝未知或变化的主机密钥，不输出凭据。D 盘工作区若存在其原 `work/codex-runtime/tencent-cloud-server.credentials.json` 与 `tencent-known-hosts`，仍可用于同一新加坡目标；不要因旧路径缺失而忽略当前已保存的私有文件或重复询问。

新会话先复用这一入口，不再要求用户重复提供服务器信息。只有文件缺失、认证失败或目标不匹配时才询问。凭据值不写入本文、Git、日志或发布包；备份仍按生产发布流程保存在私有位置。

### 只读命令

以下命令只查询状态，不重启或部署：

```sh
sudo systemctl status zhixiang-shop.service zhixiang-redis.service zhixiang-stock-sync.timer --no-pager
sudo systemctl list-timers zhixiang-stock-sync.timer --no-pager
sudo readlink -f /srv/zhixiang/current
sudo sha256sum /srv/zhixiang/current/zhixiang-shop
```

日志读取使用有界参数，例如 `journalctl -n 100 --no-pager`，先在服务器本地检查是否含客户或凭据数据。不要输出整个配置、环境变量、商户密钥、JWT、密码或卡密。

## 后续发布

### Plasma 收款配置

线上已启用独立的 `plasma_usdt` 渠道；已有 BEP-20 继续使用 `bsc_usdt`。两张截图与线上配置的收款地址相同：`0x66a8bab15067eab1efc948ac03684f0397c8416f`，两条链的监听、金额标识和交易去重分别保存。新环境的 `config.yml.example` 默认关闭 Plasma，不能据此判断现有生产状态。

Plasma 主网编号为 `9745`，USDT0 合约为 `0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb`，精度为 6；BSC USDT 精度仍为 18。合约依据：[USDT0 官方部署表](https://docs.usdt0.to/technical-documentation/deployments)。2026-10-12 已对默认 Plasma RPC 只读核验链编号、代币精度、`finalized` 和日志接口；模拟链付款及交付测试不等于真实资金验收。

当前生产配置如下，汇率沿用 BEP20 的 6.66。生产 `config.yml` 实际使用 JSON 格式（兼容 YAML），后续修改需保持文件格式并核对其余配置，不能直接在 JSON 文档末尾追加 YAML 段。以下仅以 YAML 展示相同字段：

```yaml
plasma_usdt:
  enabled: true
  recipient: "0x66a8bab15067eab1efc948ac03684f0397c8416f"
  cny_per_usdt: "6.66"
  confirmations: 15
```

已从生产服务器核验 RPC 可达、chain ID 为 9745、合约 `decimals()` 为 6、最终确认区块、日志及交易收据接口可用。默认节点是 `https://rpc.plasma.to`；自定义节点只写服务端 `PLASMA_RPC_URL`，需要分开查询收据时用 `PLASMA_RECEIPT_RPC_URL`。无需钱包私钥或助记词。

首次启用自动创建 `USDT · Plasma` 订单支付渠道；重启不会重新启用已禁用渠道或覆盖商户名称。沿用 storefront 模式即可启动独立 Plasma 监听。后台只读查询入口为 `/api/v1/admin/chain-payments/plasma-usdt/status` 和 `/transfers`，使用已有后台鉴权。核验支付页显示 Plasma、USDT0、正确地址及完整 6 位小数金额；两条链都要求完整金额和最终确认，异常金额或过期付款进入人工核查。

2026-10-12 链路检查发现，维护账号访问两条链的后台监控接口均返回 `compliance_required`，需超级管理员在后台自行确认现有声明，检查脚本不代为确认；此限制不阻断客户支付或链上监听。Plasma 的财务权限遗漏已在发布 `20261012-114319` 修复：内置 `finance` 角色拥有两条链 `status` 和 `transfers` 的 GET 权限。启动时自动补齐已有角色，已从运行中后台权限接口核验；仅增加上述两项策略，未代为确认声明，也未放宽其他角色或写入权限。

### 正式升级规则

GitHub `main` 为当前绿色商城源码主线，推送不会自动更新此服务器。升级遵循 [发布规则](../../docs/release-process.md)：先记录源码提交及构建哈希，备份线上库、配置和当前发布，再执行已授权的程序升级；保留 `shared/`、storefront 模式、支付采购队列和持久化 Redis。不得上传本地预览库覆盖生产订单。

历次部署细节已移入 [历史部署记录](../../docs/history/tencent-deployments-20261012.md)，其中旧 `api`/`checkout` 命令和本机私有包路径仅为历史资料。不要将通用上游安装器或旧初始化脚本用于覆盖正式环境。

### 公告列表（2026-10-13）

已补发“购买须知与代理合作”公告文章，ID 1、slug `purchase-and-agent-guide`，列表 `/notice`、详情 `/blog/purchase-and-agent-guide`。正文来源 `docs/catalog/purchase-notice.json`（内容提交 `1f07520e00941bf78442c86cefb89c92a7db07c8`），与首页弹窗一致。两处内容独立维护；后台文章入口为 **内容管理 → 公告列表**，弹窗入口为 **系统设置 → 站点设置 → 首页公告**。

先通过本机实际页面核验，再推送内容并发布。正式站通过 1920、390、320px 的导航、列表、详情、刷新和返回检查，未注入浏览器内容。程序仍为 `20261013-012037`，弹窗版本 `bf3a1445`。备份位于 `/srv/zhixiang/backups/before-notice-post-20261013-104352`，含一致性 SQLite 快照；服务器报告 `shared/reports/notice-post-20261013-104352.json`。没有替换程序、重启服务、上传本机数据库或执行付款采购测试，售价、成本、支付渠道、个人固定价格和站点设置均未修改。

当时两份已保存的后台口令被登录 API 拒绝，SSH 维护连接有效；通过该通道精确新增公告，保留账号设置。维护脚本先在本机临时库核对与普通 API 写入的字段和 JSON BLOB 格式、防重复行为，再在服务器备份副本演练，最后核验实际公开接口。后续后台操作须使用当前有效的登录凭据，不自动重置账号或反复提交旧口令。
