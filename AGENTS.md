# 智享A社项目背景

> 当前授权（2026-10-12）：用户已明确要求“客户付款后自动从商户1039采购并发卡，先不要做实际扣款测试”。真实客户确认付款后的自动采购已授权并启用；下文先前采购禁用的记录属于历史阶段。仍不得主动提交有效采购、转账或将验收订单改为已付款进行扣款测试，除非用户另行授权。

## 部署服务器

- 用户于 2026-10-11 指定以下腾讯云服务器为项目部署目标，2026-10-12 已按用户要求部署完整商城、商品价格、商户 1039 对接和库存同步；后续本项目会话以此服务器作为默认部署环境。
- 云服务：腾讯云轻量应用服务器（Lighthouse）。
- 地域：新加坡。
- 实例名称：Ubuntu-AiDV。
- 操作系统：Ubuntu 24.04.4 LTS，已于 2026-10-11 通过 SSH 核实。
- 公网 IPv4：`43.134.141.10`。
- 登录账号：`ubuntu`。
- 配置：2 核 CPU、4 GB 内存、60 GB 系统盘。
- 信息来源：用户确认、腾讯云控制台截图及 SSH 实测。当前正式商城 `https://www.qzweb.cloud/` 和后台 `https://124.220.2.69:18888/login` 为上海前端；新加坡保留订单、支付与采购后端，旧 `http://43.134.141.10/` 入口用于兼容。后续使用正式 HTTPS 商城入口验证页面，具体维护说明见 `remote-latest/ops/tencent/README.md`。
- 新加坡部署目录 `/srv/zhixiang`，当前后端 `current` 指向 `releases/20261012-153531`，持久化目录 `shared`。服务 `zhixiang-shop.service` 使用系统账号 `zhixiang`，以 `-mode storefront` 运行，开机自启。上海前端 `/srv/zhixiang-front/current` 指向 `releases/20261012-235241`；商城源码 `02c1a082c3b989adee398f16073e7ef8d89a4806`，后台仍沿用 `948c37f`。
- 当前运行商城 HTTP、BSC 收款监听、订单维护及自动采购发卡队列，服务环境 `UPSTREAM_PURCHASE_ENABLED=true`。只在确认客户付款后创建采购单；不启动自动充值、不注册库存轮询或采购对账任务。库存仍由独立的 `zhixiang-stock-sync.timer` 每 300 秒只查询/同步，不能为库存同步启动额外完整 worker。实际扣款测试仍未获授权，当前商户 1039 余额 0.00 元，正常自动采购需先补足商户余额。

## 后续会话与登录凭据

- 新会话在本项目中工作时，应沿用上述部署背景。用户更新服务器资料时，同步修改本文件。
- 登录密码已按用户要求保存在本地文件 `deployment/local/tencent-cloud-server.credentials.json`，该文件由 `deployment/local/.gitignore` 排除，不提交到 Git。
- 2026-10-12 用户进一步确认上海服务器 `124.220.2.69` 同样使用此前提供的 `ubuntu` 登录信息，明确要求保存并在后续直接复用。上海已验证的私有凭据为 `deployment/local/shanghai-front-server.credentials.json`，主机指纹为 `deployment/local/shanghai-front-known-hosts`；密码文件仅当前用户和 SYSTEM 可访问，全部由 Git 忽略。新加坡继续使用上述文件及 `deployment/local/tencent-cloud-known-hosts`。连接前核对目标，并使用对应保存的主机指纹拒绝未知或变化的主机密钥；不要重复询问已有且有效的登录信息。
- 需要登录服务器时读取对应凭据文件；不要将密码写入项目说明、源码、构建产物或 Git 提交，也不要在对话、命令输出和日志中回显密码。

## ChatGPT 兑换入口更换（2026-10-13）

- 用户指定后续统一使用 `https://cdk.bar/`。公告及 11 款 ChatGPT 商品购买说明、新订单发货指引已同步；源码配置 `0448e64db4058e52f6b16fe5ac1a23c70975578a`，公告版本 `bf3a1445`。客服微信保持 `Cqz0924ttxk`，不展示电话号码。
- 商品同步工具会将源快照和已有商品中的旧兑换地址迁移为当前配置，保留套餐、质保及 iOS 不叠加时长的原规则；原站资料快照不修改。只更新目标商品内容和交付说明，历史订单及已保存交付快照不回写，Gemini、DEMO 不套用此地址。
- 9 项数据库范围、备份、回滚及迁移测试通过；日常本机和正式站的公告、11款公开商品及桌面手机实际页面核验通过。未替换程序、重启后端、上传本机库或进行付款采购测试。线上备份 `/srv/zhixiang/backups/before-redemption-url-20261013-002736`、`/srv/zhixiang/backups/before-redemption-announcement-20261013-002738`；私有报告 `deployment/local/redemption-url-product-deployment.json`、`redemption-announcement-deployment.json`，浏览器记录 `remote-latest/work/codex-runtime/redemption-url-qa/`。

## 客服微信统一设置（2026-10-12）

- 用户明确指定客服微信为 `Cqz0924ttxk`，要求统一使用微信号且不要展示电话号码。首页公告、首页客服区及全站页脚已全部更换；对应复制按钮内容已核验，后续不能回退到电话号码。
- 正式上海前端 `20261012-235241`，源码 `02c1a082c3b989adee398f16073e7ef8d89a4806`；公告版本 `e97dfd7a`。94 项前台测试、类型检查和生产构建通过；本机、正式站各检查桌面 1920px 和手机 390/320px，覆盖客服显示、实际复制、商品详情与公告页页脚，无旧号码或页面错误。
- 新加坡后端和支付采购配置保持原状态；仅修改公告设置并发布商城静态资源，没有上传本机库、重启后端或执行付款采购测试。上海备份 `/srv/zhixiang-front/backups/before-wechat-contact-20261012-235241`，公告修改前备份 `/srv/zhixiang/backups/before-wechat-contact-20261012-235139`。私有报告 `deployment/local/wechat-front-deployment.json`、`wechat-announcement-deployment.json`、`wechat-front-live-verification.json`；浏览器记录 `remote-latest/work/codex-runtime/wechat-unified-qa/`。

## 可配置首页公告弹窗（2026-10-12）

- 用户要求首次进入商城时显示可自行设置的弹窗公告，已将既有 `home_announcement` 配置接入当前 `Storefront.vue` 首页并发布上海前端 `20261012-231734`，商城源码 `a2cda8e818712a3a23d6b7534f8f795dbca26604`。后台路径为 **系统设置 → 站点设置 → 首页公告**，支持标题、富文本、图片、链接、开关及起止时间。2026-10-12 23:35 已按用户参考图片整理并启用正式公告“购买须知与代理合作”：购买与发货说明、ChatGPT 卡密兑换须知、诚招代理及代理价咨询、客服微信 Cqz0924ttxk；兑换入口 https://cdk.bar/。文案快照 `remote-latest/docs/catalog/home-announcement.json`，配置来源提交 `6c088c7a20725b5ebcfec10286e6346b2c8870d2`，公开版本 `c13a6e2c`。后续可在后台修改。
- 关闭后本次会话不重复弹出，支持今日不再提示和不再提示；标题或内容变化后再次提醒。日常本机入口通过实际公告保存、公开接口与 12 项浏览器检查，临时内容已恢复；公网通过 12 项只读检查，弹窗仅在测试浏览器内注入验证，覆盖六种宽度，无页面错误。94 项前台测试、类型检查和生产构建通过。
- 仅部署商城静态资源，后台、新加坡后端及支付采购配置保持原状态；售价和两条 USDT 渠道核对正常，没有上传本机库或进行付款采购测试。上海备份 `/srv/zhixiang-front/backups/before-announcement-20261012-231734`，新加坡备份 `/srv/zhixiang/backups/before-frontend-announcement-20261012-231734`。私有报告为 `deployment/local/announcement-deployment.json`、`announcement-live-verification.json`；截图与浏览器记录为 `remote-latest/work/codex-runtime/announcement-qa/`，均由 Git 忽略。

- 本次正式内容仅更新后台 `home_announcement`，前后端程序版本保持不变。本机与正式站各通过桌面 1920px、手机 390px 和 320px 的实际公开配置检查，未使用测试注入；联系人、兑换链接、代理价咨询和关闭后刷新均正确，窄屏标题完整。线上备份 `/srv/zhixiang/backups/before-announcement-content-20261012-233551`，私有报告 `deployment/local/announcement-content-deployment.json`、`announcement-content-live-verification.json`；截图与报告在 `remote-latest/work/codex-runtime/announcement-content-qa/`。没有重启服务、上传本机库、修改售价及支付渠道或执行付款采购测试。

## 商品详情封面完整显示（2026-10-12）

- 用户截图中的详情主图仍使用 4:3 的 `object-cover`，导致 5:3 原图左右被裁切；`remote-latest/frontend/user/src/components/product/ProductImageGallery.vue` 已改为 5:3 的 `object-contain`，缩略图也改为完整显示。
- 修复源码 `6d8c1e94105d483e8a55b2318808c6c24a12877a` 已合入并推送 `main`，先通过日常本地入口验证，再发布上海前端 `20261012-205131`。94 项前台测试、类型检查和生产构建通过，本机和公网各 30 项只读页面检查通过；覆盖 15 款实际商品封面和 1920、1280、1024、768、390、320px 六种宽度，无横向溢出或页面脚本错误。DEMO 既有图标不计入商品封面检查。
- 此次仅更新商城静态资源，后台及旧哈希资源保留；新加坡后端、Redis、storefront 模式、支付与采购配置、库存定时器保持原状态。全部 16 款商品及 SKU 售价核对一致，`qzweb-55` 仍为 1160.00 元，两条 USDT 渠道正常；未上传本机数据库、重启后端或执行真实付款及采购测试。
- 上海备份 `/srv/zhixiang-front/backups/before-detail-image-20261012-205131`；新加坡一致性备份 `/srv/zhixiang/backups/before-frontend-detail-20261012-205131`。私有发布包及报告位于 `deployment/local/detail-image-releases/20261012-205131/`、`detail-image-deployment.json`、`detail-image-live-verification.json`；截图与浏览器报告在 `remote-latest/work/codex-runtime/product-detail-image-qa/`，均由 Git 忽略。

## 访问检查（2026-10-11）

- 已实测通过 SSH 登录 `43.134.141.10`，登录账号为 `ubuntu`。
- 从服务器查询的实际公网出口 IP 为 `43.134.141.10`，与用户在 ELLAIXSHOP 添加的白名单地址一致。
- 从服务器访问 `https://ellaixshop.com/` 返回 HTTP 200，网络连通检查通过。
- 商户认证和余额下单接口白名单已完成下文记录的无扣款验证；真实扣款及卡密交付流程尚未验收。

## ELLAIXSHOP 商户接口验证（2026-10-11）

- 用户明确授权使用此前商户 1039 的密钥进行不扣款验证；本次密钥仅在检查进程内存中使用，未写入本地或远程文件。
- 22:43（Asia/Shanghai）从服务器实际出口 `43.134.141.10` 验证 `https://ellaixshop.com`；商户认证接口 `/shared/authentication/connect` 返回 HTTP 200、业务 code 200。
- 向余额下单接口 `/shared/commodity/trade` 发送空 `shared_code`、`num=0`，返回 HTTP 200、业务 code 0、消息“商品不存在”。结合已保存的上游代码先检查 IP 白名单、再检查商品的顺序，确认此服务器已通过下单接口白名单校验。
- 测试前后余额均为 0 元，未提交有效采购。实际扣款、取得卡密及客户订单交付仍未验收。
- 不含密钥的检查报告保存在 `deployment/local/ellaixshop-server-access-report.json`；可复用检查脚本为 `deployment/local/check_ellaixshop_whitelist.py`，运行时需要隐藏输入商户密钥。二者均被 Git 忽略。

## 官方接口文档核对与上线状态（2026-10-11）

- 用户提供的协议文档：`https://faka.wiki/zh-cn/dev/api.html`。余额下单使用 `/shared/commodity/trade`，强制从上游商户账户余额采购；客户向本站付款由本站支付系统确认。
- `remote-latest` 中已实现 Acg-Faka 签名、商品/价格/库存查询、余额采购、按字符串订单号查卡密，以及本站确认付款后创建采购任务并交付客户订单的链路。当前适配自动发卡、单规格、无需额外填写字段的商品。
- 22:56 从服务器使用仅含商户 ID 和签名的真实请求验证成功：商户 1039 认证、4 个分类/13 个商品、商品 4 详情、实时库存和 33.00 CNY 询价；用不存在订单号验证查询接口返回“订单不存在”。未调用有效采购，前后余额均为 0 元。
- 已按文档建议修正 `remote-latest/internal/upstream/acg_faka.go`：商户密钥仅用于本地生成签名，不随请求发送。相关上游、采购集成及支付测试均通过。修正已随 2026-10-12 商城版本部署；源码尚未提交推送。
- 2026-10-11 22:54 检查时服务器仅 DNS 和 SSH 监听。2026-10-12 已部署后端、供货连接、12 款映射和独立库存定时器；真实客户收款、采购扣款和卡密交付仍未验收，须遵循用户不实际扣款的限制。
- 不含密钥的检查报告：`deployment/local/faka-document-contract-report.json` 与 `deployment/local/project-deployment-inspection.json`，均位于 Git 忽略的本地目录。

## 商品资料来源与本机同步（2026-10-11）

- 用户要求商品信息以 `https://www.qzweb.cloud/cat/4` 所在网站为准，并明确选择“同步网站全部分类”。当前已同步全部六个公开分类及 16 款公开可见商品：ChatGpt 渠道一（5）、ChatGPT渠道二（4）、Gemini（4）、【荐】ins/真人老号[全网最低价]（0）、Chatgpt ios渠道（2）及 DEMO（1）。空分类也在首页展示。
- 商品 slug 为 `qzweb-{原站商品ID}`，分类 slug 为 `qzweb-category-{原站分类ID}`。标题、售价、封面、兑换说明、质保条件和原交付类型均已同步；不同渠道、开通、续费和预售分别保留。iOS 两款商品按原站售价 9999.00 元保留，DEMO 按演示商品保留。价格均为公开零售价，不能用作商户采购成本；原站库存数量只保留为来源记录，不写成本地库存。
- 当前商城使用 `remote-latest/frontend/user` 的 Vue 用户端及后台商品 API，旧 React 目录和原静态商品数据只作历史参考。全站资料保存在 `remote-latest/docs/catalog/qzweb-all.json`；复用同步工具为 `remote-latest/scripts/sync-qzweb-catalog.py`，默认使用全站快照。原单分类快照 `qzweb-cat-4.json` 保留，不能把它当作当前全站目录。
- 本机商城入口 `http://127.0.0.1:5173/`，后台 `http://127.0.0.1:8080/`。运行库 `remote-latest/work/codex-runtime/brand-preview/shop.db` 和基础库 `deployment/local/db/dujiao.db` 均已同步；修改前备份在 `deployment/local/backups/`，不提交 Git。
- 原 `zxa-*` 目录已全部下架，八个旧分类停用，商品记录及历史订单保留。最初同步商品资料时，16 款商品尚未绑定供货映射、未导入本站卡密或设置人工库存；随后已完成下文记录的 12 款上游映射与库存同步，并于 2026-10-12 一起部署到腾讯云服务器。不得用参考站页面库存冒充供货接口库存。
- 全部商品的名称、售价、详情、数量限制、分类与交付类型，以及 18 个图片文件均已通过真实公开接口核对；首页五项商品显示测试、Vue/TypeScript 检查及生产构建通过。核对报告为 `deployment/local/qzweb-all-verification.json`，被 Git 忽略。
- 用户随后要求渠道一「ChatGPT Pro 20新老号(到期)开通 质保一个月」（`qzweb-55`）由 1150.00 调整为 1160.00 元，本站商品和 SKU 售价均以 1160.00 为准。来源快照保留原价；`remote-latest/docs/catalog/qzweb-price-overrides.json` 保存此定价，商品同步工具优先采用该文件，不能以来源快照覆盖用户调价。

## 商户 1039 商品映射和库存（2026-10-12）

- 用户明确要求将现有商品对接商户 1039 并显示真实库存，且先不要实际扣款。该限制继续有效；此次仅保存连接、商品/规格映射和查询库存，没有创建订单或调用有效采购接口。
- 两个本机数据库均已保存站点连接 ID 1：`https://ellaixshop.com`、商户 ID `1039`、协议 `acg-faka`、状态 active。密钥通过后台按应用密钥 AES 加密保存于 `site_connections.api_secret`，不在对话、源码、公开资料或日志中回显。运行库和基础库的应用加密密钥已核对一致。
- 按来源商品共享编码逐项比对商户接口，12 款现有商品和单规格已绑定；关系记录在 `remote-latest/docs/catalog/qzweb-ellaixshop-mappings.json`。`qzweb-4/5/55/7/56/63/58/64/57/50/51` 有货；`qzweb-59` 已绑定但真实库存为 0。
- `qzweb-52`、`qzweb-53`（两款 Gemini 人工预售）、`qzweb-62`（iOS Pro 20x）和 `qzweb-1`（DEMO）未出现在商户 1039 接口中，仍未绑定且缺货。商户额外提供的商品 6 不属于当前参考站公开目录，未新增商品。不能按类似名称替代渠道或交付类型。
- 已绑定商品内部交付类型为 upstream，公开展示为 auto，库存显示模式 exact，显示真实供货数量。商品和 SKU 成本保存为实际 CNY 商户报价，售价保留原设置，包括 `qzweb-55` 的 1160.00 元。连接 `auto_sync_price=false`，不能用商户进价覆盖本站售价。
- 本机 API 已重建并切换为 `remote-latest/work/codex-runtime/local-preview-supplier-server.exe`（包含密钥不随签名请求发送的修正），仍以 api 模式运行，运行配置的 Redis 和队列关闭。
- `remote-latest/scripts/sync-ellaixshop-stock.py` 仅允许后台登录、连接查询/测试、商品映射查询/同步和公开商品查询，不调用订单、付款或采购接口。本机隐藏后台助手已启动，每 300 秒刷新一次，首页每 60 秒及重新获得焦点时获取更新。助手运行信息在 `remote-latest/work/codex-runtime/local-preview-logs/runtime.json` 的 `stock_sync` 字段；此助手不是开机启动服务。
- 助手将本机运行库的已映射商品、规格、映射和加密连接同步到基础库。手动刷新用上述脚本，加 `--credentials-file deployment/local/credentials.json --report-file deployment/local/merchant1039-stock-sync.json --live-db remote-latest/work/codex-runtime/brand-preview/shop.db --mirror-db deployment/local/db/dujiao.db`；持续运行再加 `--watch --interval 300`。不要重复启动助手，先检查 runtime 中的 PID。
- 只读检查前后商户余额均为 0.00 元；真实采购和客户付款后的发货仍未验收。两个数据库的历史订单、支付、用户、卡密和采购记录与映射前备份逐行一致。库存、价格、规格映射和外键检查通过，首页七项测试、Vue 类型检查及生产构建通过，相关上游、映射、库存展示和采购模拟测试通过。
- 本地脱敏报告：`deployment/local/merchant1039-binding-report.json`、`merchant1039-stock-sync.json`、`merchant1039-integration-verification.json`；数据库备份位于 `deployment/local/backups/before-merchant1039-mappings-*`，均由 Git 忽略。该连接、映射和库存展示已于 2026-10-12 正式部署；采购扣款仍禁用。

## 腾讯云当前部署和维护（2026-10-12）

- 首次完整部署发布 `20261012-001724` 使用 `remote-latest` Go + Vue 前台/后台的 fullstack Linux amd64 二进制，内嵌封面和分类图标；不是项目根目录的旧 React 页面。当前已升级到下文记录的 `20261012-004733`，无需 Docker 或单独 nginx。
- 线上数据库 `/srv/zhixiang/shared/db/shop.db`，配置 `/srv/zhixiang/shared/config.yml`，上传目录 `/srv/zhixiang/shared/uploads`，应用日志 `/srv/zhixiang/shared/logs`。从当前本机库使用 SQLite backup API 生成一致快照；上线后本机库与线上库独立，更新线上环境前应先备份线上数据，不能直接覆盖历史订单。
- 应用加密密钥沿用原运行配置，商户密钥仍在数据库中加密保存；后台登录凭据文件 `/srv/zhixiang/shared/stock-sync.credentials.json` 仅供本机回环地址的库存助手使用。配置、凭据和数据库权限为 0600，不向公网响应暴露。
- `zhixiang-shop.service` 与 `zhixiang-stock-sync.timer` 均 enabled/active。定时器启动 `zhixiang-stock-sync.service`，只调用余额、商品映射同步和公开库存查询；报告在 `/srv/zhixiang/shared/reports/merchant1039-stock-sync.json`。systemd 单元源文件和维护说明位于 `remote-latest/ops/tencent/`。
- 新增适配器保护：`UPSTREAM_PURCHASE_ENABLED` 一旦设置，仅值 `true` 允许采购；当前服务固定设置 `false`，在任何采购网络请求前拒绝采购。保持该限制，直到用户明确授权实际扣款。库存读取、余额查询不受此开关影响。
- 公网首页、健康检查、前台/后台入口资源、6 个分类/16 款商品、12 个商品封面/6 个分类图标均已验证；全部商品和 SKU 售价与快照一致，`qzweb-55` 为 1160.00 元。12 款映射已同步，11 款有货，1 款库存为 0，4 款未映射仍缺货。
- 已验证服务重启及重启后的库存同步；同步前后商户余额均为 0.00 元，没有创建采购或支付，没有调用真实采购接口。历史订单、支付、用户、卡密及采购表完整摘要与部署快照一致。
- 初始服务器备份 `/srv/zhixiang/backups/initial-20261012-001724`；本地发布包和数据库备份在 `deployment/local/tencent-release/20261012-001724/`，部署/核对报告为 `tencent-deployment.json`、`tencent-deployment-verification.json`、`tencent-restart-verification.json`，均被 Git 忽略。可复用只读核对脚本 `deployment/local/verify-tencent-release.py`；首次安装脚本不能直接用于覆盖已有部署。

## 默认中文和服务器升级（2026-10-12）

- 用户要求卡网默认显示中文并发布服务器，已完成程序升级 `20261012-003834`。前台 `frontend/user/src/i18n/index.ts` 在没有有效手动语言选择时固定使用简体中文 `zh-CN`，不再跟随浏览器语言；有效的已保存语言选择继续保留。后台原本默认中文。
- 此次只更新程序，未上传或覆盖本机数据库；在切换前备份线上数据库、配置及库存助手凭据到 `/srv/zhixiang/backups/before-20261012-003834`，旧发布仍保留。服务与库存定时器恢复正常；商品及 SKU 售价、订单、支付、用户、卡密和采购记录的完整摘要保持一致。
- 英文浏览器、繁体浏览器和手机端首次访问均实测为简体中文；手动切英文、刷新保留选择、切回中文均通过，Vue/TypeScript 检查及生产构建通过。公网资源、16 款商品、6 个分类及库存同步核验通过，余额前后均为 0.00 元，采购接口未调用，`UPSTREAM_PURCHASE_ENABLED=false` 保持有效。
- 本地发布目录 `deployment/local/tencent-release/20261012-003834/` 保存实际嵌入的不可变前后台资源快照；核验应使用发布快照，避免被其他本机构建覆盖。私有升级报告 `deployment/local/tencent-application-upgrade.json`、浏览器报告 `chinese-default-browser-verification.json` 及升级脚本 `upgrade-tencent-application.py`、`upgrade-tencent-application-server.py` 均位于 Git 忽略的本地目录。

## 商品界面完整显示修复（2026-10-12）

- 用户反馈商品界面未完整显示，已发布程序版本 `20261012-004733`。现有商品封面采用原图 5:3 比例及 `object-fit: contain`，Go、Plus、Pro 图中文字完整显示；商品长标题、购买说明、价格和真实库存可换行，操作按钮保留可点击尺寸。
- 1280px 以下使用可展开导航和底部导航，手机窄屏商品改为单列，分类名称换行；页脚预留底栏及安全区空间。默认中文与手动语言记忆保持有效。
- Vue 类型检查、生产构建和浏览器交互验证通过。公网实测 1920、1280、1024、768、390、320px 六种宽度，无横向溢出、文字截断或浏览器脚本错误；分类、搜索、菜单与页脚可正常使用，浏览器验证只允许读取请求。英文布局也已完成本机各尺寸检查。
- 升级仅上传程序与发布清单，沿用原后台资源，没有上传本机数据库。线上切换前备份在 `/srv/zhixiang/backups/before-20261012-004733`，前一版本保留。全部商品及 SKU 售价、历史订单、支付、用户、卡密和采购表摘要均保持一致；`qzweb-55` 仍为 1160.00 元。
- 商户 1039 的 12 款映射库存同步通过，服务与库存定时器 active/enabled，余额前后均为 0.00 元，采购接口未调用，`UPSTREAM_PURCHASE_ENABLED=false` 保持有效。
- 私有发布快照在 `deployment/local/tencent-release/20261012-004733/`；升级与公网核验报告为 `tencent-application-upgrade.json`、`tencent-deployment-verification.json`。浏览器截图及报告在 `remote-latest/work/codex-runtime/storefront-layout-qa/`，公网报告 `live-layout-report.json`；以上运行文件均由 Git 忽略。

## USDT / BEP20 支付启用（2026-10-12）

- 用户要求沿用之前的 U 支付完善结账方式，已发布 `20261012-005645`。支付渠道 ID 1，provider `bscusdt`、channel `usdt-bep20`，前台显示 `USDT · BEP20` / `BNB Smart Chain`，仅用于商城订单，不开放钱包充值。
- 沿用原收款地址 `0x66a8bab15067eab1efc948ac03684f0397c8416f`，BNB Smart Chain 主网 chain ID 56，限定 USDT 合约 `0x55d398326f99059ff775485246999027b3197955`、18 位精度。沿用原固定报价 `1 USDT = 6.66 CNY`，每笔支付锁定汇率并生成六位小数精确金额；至少 15 个确认且通过 finalized、回执与区块 hash 校验才入账。没有使用或保存钱包私钥/助记词。
- 新增 `-mode checkout`，同时运行 HTTP、收款监听和只允许订单超时、本地卡密交付、交易通知等任务的 worker；采购提交、采购查询、库存轮询及采购对账均不注册，也无周期任务。该模式明确取消支付服务的采购依赖，付款后不创建采购单。`UPSTREAM_PURCHASE_ENABLED=false` 和自动充值关闭继续有效。当前供货商品真实付款后只能确认收款并等待发货；自动采购与卡密交付仍须另行授权和验收，不能声称已经完成真实资金验收。
- 服务器 `zhixiang-redis.service` 为专用 Redis 7，绑定 `127.0.0.1:16379`，密码仅存于私有配置，启用 AOF everysec 和 noeviction，256MB 上限。商城 systemd drop-in `/etc/systemd/system/zhixiang-shop.service.d/30-checkout.conf` 切换运行模式；商城、Redis、库存定时器均 active/enabled。队列 db 1、缓存 db 0；不得打印 Redis 密码或整份配置。
- 切换前备份在线库、配置、库存凭据、原服务单元到 `/srv/zhixiang/backups/before-20261012-005645`。此次未上传本机数据库，原有订单、支付、用户、卡密、采购记录和全部售价核对一致；`qzweb-55` 的 1160.00 元调价保留。库存同步 12 款映射通过，商户余额前后 0.00 元、采购接口未调用。
- 公网创建了一笔未付款支付验收订单（游客邮箱以 `bep20-qa-` 开头、`@example.invalid` 结尾），仅用于真实接口/页面验证，未转账、未确认付款或采购；新增父子订单及支付记录按正常超时流程保留，不能删历史记录来掩盖测试。原有记录逐行摘要未改变。
- 后端支付/监听/采购隔离回归、前端 92 项测试、类型检查和生产构建通过。本机及公网桌面、手机的支付选择、提交按钮、二维码、精确金额与地址的实际剪贴板复制、刷新恢复均通过，无横向溢出和页面脚本错误；线上监听游标推进、重启继续扫描且金额不变已验证。真实转账入账和供货商发卡尚未验收。
- 本机预览后端已切换为 `remote-latest/work/codex-runtime/local-preview-checkout-server.exe`，运行模式 `checkout`；运行信息仍在 `local-preview-logs/runtime.json`，`payment_redis` 字段记录本机隐藏 Redis PID。原库存助手继续独立运行，未重复启动。运行库和基础库配置已启用同一支付参数，基础库若启动会自动初始化支付渠道；不得将本机库覆盖线上订单。
- 私有发布快照 `deployment/local/tencent-release/20261012-005645/`；脱敏报告 `bep20-payment-deployment.json`、`bep20-local-verification.json`、`bep20-live-verification.json` 及相应浏览器报告均在 Git 忽略的 `deployment/local/`。服务器重启/历史摘要报告在 `/srv/zhixiang/shared/reports/bep20-runtime-verification.json`。复用支付部署脚本 `deploy-bep20-payment.py`；旧程序升级脚本只支持原 api/全部关闭的环境，后续升级须保留当前支付配置、订单队列及 checkout 模式。

## 客户付款后自动采购发卡（2026-10-12）

- 用户进一步明确授权真实客户确认付款后自动从商户 1039 采购并发卡，仍要求先不要实际扣款测试。已发布 `20261012-013649`，新模式 `storefront` 保留原收款监听和订单维护，增加采购提交、按字符串上游订单号查询卡密、每分钟订单恢复及交付回调。服务 drop-in `/etc/systemd/system/zhixiang-shop.service.d/40-storefront.conf` 覆盖原 checkout 模式，`UPSTREAM_PURCHASE_ENABLED=true`。自动充值继续关闭，库存仍独立同步。
- 付款事务提交后持久化采购意图，即使 Redis 入队失败也保留采购单。每个本地子订单有唯一采购记录；父订单重复回调跳过已有子订单并补齐剩余子订单。每分钟补齐已付款但缺采购记录的订单，并恢复丢失的 pending/failed/accepted 任务。
- Acg-Faka 发起余额采购前原子持久化 `submitting`；响应未知、重复请求或中途进程退出进入 `manual_review`，不得自动重购或强行后台重试。仅未发送扣款请求的只读预检失败按间隔重试；余额/库存不足、采购成本超过订单成本上限保留已付款待交付订单并记录异常，处理后可在后台重试。没有卡密或交付数据不能标记已发货。卡密先保存交付记录，再完成子订单及父订单状态。
- 内存数据库和进程内模拟供应商验收付款确认、自动采购、卡密保存、父子订单交付、重复回调/并发采购防重复、队列中断恢复、延迟卡密恢复、未知扣款禁止重试、空卡密保护、交付后订单读取异常恢复及只读预检延迟重试；相关支付、采购、监听、应用装配、队列及架构回归通过。未转账、未将线上验收单改成已付款、未调用真实有效采购接口。另有三个已有 HTTP 合规中间件测试依赖 CGO sqlite3，本机无 C 编译器无法运行，与此次改动无关。
- 升级前确认线上没有等待采购的已付款供货订单，采购表为 0；备份 `/srv/zhixiang/backups/before-20261012-013649` 保存线上库、配置、库存凭据及旧单元。仅上传程序，未上传本机库，支付配置和 Redis 凭据未修改，前后台资源沿用上一版不可变快照。全部订单、支付、用户、卡密、采购、交付、链上金额记录及商品/SKU 售价摘要一致；商户库存 12 款映射仍正常，余额前后 0.00 元，无采购扣款测试。
- 线上商城、Redis 和库存定时器 active/enabled，采购 worker 和每分钟恢复调度已通过 Redis 只读元数据验证，支付监听游标正常推进。真实转账到账及上游资金采购仍未经资金验收，不能把模拟通过表述为已完成真实扣款测试。
- 本机预览已同步为 `remote-latest/work/codex-runtime/local-preview-storefront-v2-server.exe -mode storefront`，进程环境启用采购；原 Redis 和库存助手继续运行，未重复启动。切换前备份两份本机库，并确认不存在历史待采购已付款订单。预览运行信息仍在 `local-preview-logs/runtime.json`。
- 发布快照 `deployment/local/tencent-release/20261012-013649/`；私有报告 `auto-procurement-deployment.json`、`auto-procurement-live-verification.json`、`auto-procurement-local-verification.json` 与 `auto-procurement-tests.log` 均被 Git 忽略。复用脚本 `prepare-auto-procurement-release.py`、`deploy-auto-procurement.py`、`deploy-auto-procurement-server.py` 和 `verify-auto-procurement-server.py`；服务器报告在 `shared/reports/auto-procurement-upgrade.json` 与 `auto-procurement-runtime-verification.json`。后续程序升级必须保留当前 storefront 模式和支付/采购队列配置；不要套用旧 api 或首次 checkout 初始化脚本。
