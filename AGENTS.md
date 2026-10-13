# π星卡社开发约束

- 客服微信统一使用 `Cqz0924ttxk`，不能用电话号码作为微信号展示或回退值。
- 2026-10-13 已将“购买须知与代理合作”发布为 `notice` 文章（ID 1，slug `purchase-and-agent-guide`），公开列表 `/notice`、详情 `/blog/purchase-and-agent-guide`，文案快照 `docs/catalog/purchase-notice.json`。后台 **内容管理 → 公告列表** 维护文章；首页弹窗仍在站点设置维护，两处不会自动同步。正文与弹窗一致，客服微信和 `https://cdk.bar/` 正确。本机及正式站各通过 1920、390、320px 的实际导航、列表、详情、刷新和返回检查。
- 本次公告仅增加内容数据，程序仍为发布 `20261013-012037`，首页弹窗版本仍为 `bf3a1445`；售价、成本、专属价格、支付渠道和站点设置核对一致，没有重启或上传本机库。备份 `/srv/zhixiang/backups/before-notice-post-20261013-104352`，私有报告为外层 `deployment/local/notice-post-production-deployment.json`。发布时服务器 `stock-sync.credentials.json` 和本机 `deployment/local/credentials.json` 的后台密码均被正式 API 拒绝；SSH 指纹核验及服务器维护登录有效。本次通过已授权的 SSH 维护通道精确新增这条公告，未重置账号。后续不要反复使用这两份无效口令登录正式后台，应核对新的有效凭据；该状态不表示 SSH 凭据失效。
- 2026-10-13 已补齐并上线中文隐私政策和服务条款，文案快照为 `docs/catalog/site-legal.json`，配置来源 `9b260f889420d8c386c6c2c20d76c27e1630a94d`。后台 **系统设置 → 站点设置 → 法律条款** 可继续编辑；公开入口 `/privacy`、`/terms`，注册页实际链接已核验。仅修改 `site_config.legal.privacy.zh-CN` 和 `site_config.legal.terms.zh-CN`，保留其他配置及语言内容。条款中的兑换入口遵循各笔订单交付说明，质保和退款按具体商品约定及适用法律处理；不写统一永久质保或“概不退款”。本机、正式站各通过 1920、390、320px 共 9 项只读页面检查，记录在忽略目录 `work/codex-runtime/site-legal-qa/`。线上备份 `/srv/zhixiang/backups/before-site-legal-20261013-003435`，未更换程序、重启后端、上传本机库或执行付款采购测试。

- `main` 是唯一正式源码主线，维护当前绿色 Go + Vue 商城，包含已确认的 π星卡社名称、派大星 Logo 和水母背景。用户端为 `frontend/user`，后台为 `frontend/admin`；这些品牌改动属于同一项目主版本。
- 支付、订单、数据库和采购后端部署在腾讯云新加坡 `43.134.141.10`，账号 `ubuntu`，目录 `/srv/zhixiang`。两套前端部署在上海 `124.220.2.69` 的 `/srv/zhixiang-front`：商城 `https://www.qzweb.cloud/`，管理端 `https://124.220.2.69:18888/login`。上海通过受限 SSH 通道连接新加坡回环 API；维护说明见 `ops/tencent/README.md`。
- 当前新加坡后端发布为 `20261013-114011`，程序及其内嵌旧商城资源源码为 `aaff34ff2af8d62f9a2cc4061007c6e41b54ea80`；上海静态发布为 `20261013-160811`，商城源码为 `c0d5e48c1407261e5d800969c228f03a644732d7`，后台资源保留 `471ded02e1b265520a53485288bdcc4f47dbece9`。后端使用 `-mode storefront`、`UPSTREAM_PURCHASE_ENABLED=true`。BEP20 与 Plasma USDT 收款均已启用，支持商品订单与钱包充值；精确核验范围以 `ops/tencent/production-baseline.json` 为准，不把后续文档提交自动当作线上程序的 Git 版本。
- 本地使用 `scripts/local/start.ps1`、`status.ps1`、`stop.ps1`，运行库为 `work/codex-runtime/live/shop.db`，本地采购关闭。旧说明位于 `docs/history/`，不能按历史文档覆盖当前环境。
- 用户要求固定执行“先本地验证，再推送和上线”：先在日常本地入口验证实际运行程序、配置、相关数据和页面，保存验证记录；通过后才推送已验证提交并部署所需改动。后端改动须重建并重启本地后端，支付渠道及商品说明不能只在临时测试入口核验。上线后对照核验本地与生产；环境私有配置、订单和实时库存独立维护，不互相覆盖。
- 用户已授权真实客户确认付款后的自动采购发卡，仍禁止主动提交有效采购、转账，或将验收订单改为已付款进行扣款测试。测试使用模拟供应商和临时数据库。
- 未完成真实资金验收，不得把模拟测试通过描述成真实收款或采购验收。
- 库存由独立的 `zhixiang-stock-sync.timer` 每 300 秒只查询和同步。不要重复启动助手或为同步启动完整 worker。自动充值保持关闭。
- 后续升级保留 storefront 模式、支付及采购队列、持久化 Redis。先备份线上数据，不用本机数据库覆盖线上订单，也不套用旧 api 或 checkout 初始化脚本。
- 用户已授权跨会话读取、保存并复用服务器信息，2026-10-12 再次确认上海和新加坡使用此前提供的同一 `ubuntu` 登录信息。当前 C 盘工作区 `C:\Codex program\智享A社\deployment\local\` 中，新加坡凭据和指纹为 `tencent-cloud-server.credentials.json`、`tencent-cloud-known-hosts`，上海凭据和指纹为 `shanghai-front-server.credentials.json`、`shanghai-front-known-hosts`；登录均已验证，文件被 Git 忽略，上海密码文件仅当前用户和 SYSTEM 可访问。D 盘工作区原 `work/codex-runtime/tencent-cloud-server.credentials.json` 与 `tencent-known-hosts` 若存在仍可用于新加坡。新会话先检查对应目标的已保存文件，不因旧路径缺失而重复索要信息；仅在所有已保存入口缺失、凭据失效或目标不一致时询问。禁止输出密码。
- 当前后台品牌与运营入口已统一为 π星卡社，实际发布和组件来源分别按上文记录。首页公告、客服微信 `Cqz0924ttxk`、`https://cdk.bar/` 兑换指引及法律条款保持原配置。2026-10-13 已新增后台用户详情的“设置商品单价”入口；独立表 `user_product_prices` 保存个人固定金额，规格价优先于商品价，留空回退现有定价。个人单价不叠加等级、活动或数量折扣，不改变公开售价、采购成本和历史订单。详见 `docs/user-product-prices.md`。
- 凭据只保存在忽略的 `work/` 或 `deployment/local/`，限制本机文件访问权限。不得在源码、提交、对话、日志或构建产物中写入或回显密码、商户密钥、JWT、私钥或助记词；项目文档只记录私有文件路径和连接方式。使用已保存的 SSH 主机指纹核验服务器，指纹变化时停止连接并核查。
- 商户 1039 使用 `acg-faka`；密钥仅用于本地签名，不随请求发送。余额采购须持久化原子提交状态，未知结果进入人工核查，不能自动重新采购。
- 商品目录为 6 个分类、16 款商品，供货映射 12 款。`qzweb-55` 售价固定为 1160.00 元；库存及商户进价同步不能覆盖本站售价。
- 11 款 ChatGPT 卡密（渠道一、渠道二及 iOS）统一使用 `https://cdk.bar/` 自助充值，发货须附兑换链接和操作指引；配置见 `docs/catalog/qzweb-redemption-guide.json`。Gemini、DEMO 不套用此地址；iOS 原有覆盖充值与不叠加剩余时长的规则保留。
- 原始目录和用户价格覆盖保存在 `docs/catalog/`；同步脚本优先遵循价格覆盖，不能拿参考站库存替代商户库存。
- 表格和 Word 默认白底黑字、首行仅加粗；不使用彩色底色。其他交付物按任务需求选择样式，用户明确指定时优先遵循用户要求。

## 新会话入口与工作区边界

- 本机唯一日常主目录是 `D:\卡网`，默认主线为 `main`。新会话先读本文件和根目录 `README.md`，再核对分支、HEAD 与 `git status --short`；不能凭旧会话的目录名、旧截图或历史构建推断当前版本。
- 当前操作依据仅为根目录说明、`docs/local-development.md`、`docs/release-process.md` 和 `ops/tencent/`。`docs/history/`、`work/archive/` 与历史标签仅供明确请求的追溯或恢复，常规开发不要展开、搜索或作为启动和部署依据。
- 并行设计或功能草稿在独立 `codex/*` 分支及工作区中维护。未经用户要求合入，不把草稿文件复制到主目录，不将草稿称为主版本或线上版本；不得擅自清理其他会话正在使用的工作区。
- 不在主目录中新增第二套完整仓库、重复前台或名称相近的“最新版”目录。本机数据库、日志、构建与恢复资料只放在已有的忽略目录，不能提交 Git。
- 交付时分别说明主目录是否有未提交改动、是否与远端同步、是否实际部署。只有核验 `git status --short` 为空时才能称工作区干净；保留的运行数据和恢复备份不算另一套活动项目。

## 用户商品单价发布（2026-10-13）

- 先通过日常本机入口的 13 项实际浏览器检查，再推送和发布；后台 48 项、商城 95 项测试及两套类型构建通过，价格、订单、支付、模拟采购、架构和 RBAC 回归通过。正式站 9 项只读页面检查及 437 项静态文件哈希核验通过，覆盖 1920、390、320px，无脚本错误或横向溢出。
- 新加坡及上海备份分别为 `/srv/zhixiang/backups/before-user-prices-20261013-012037`、`/srv/zhixiang-front/backups/before-user-prices-20261013-012037`。线上历史记录、全部公开售价和 SKU 成本、供货连接、两条支付渠道、配置和凭据摘要保持一致。未上传本机库、为真实用户填测试价格、提交订单、付款或有效采购；正式专属价格表目前为空。
- 本次在 C 盘可用源码工作区通过 `scripts/local/` 重建日常 `8080/5173/5174` 入口，保留原 `work/codex-runtime/brand-preview/shop.db`；运行信息保存于 `work/codex-logs/`，旧 `local-preview-logs/runtime.json` 也已同步。本机采购明确关闭，库存助手和 Redis 未重复启动。D 盘主目录在本次环境中不存在。私有发布包和核验报告在外层忽略的 `deployment/local/user-price-*`，截图与复用工具在 `work/user-price-qa/`。

## 钱包充值与后台调账修复（2026-10-13）

- 发布 `20261013-110949` 已为现有 BEP20、Plasma 渠道开启 `order` 与 `wallet`，补齐充值单支付期限、链上入账与充值收银页信息。后台用户详情的“钱包明细”接入已有声明弹窗，首次使用仍须超级管理员本人确认；本次未代签正式声明或变更真实用户余额。操作见 `docs/wallet-recharge.md`。
- 在 `D:\卡网` 日常 `8080/5173/5174` 入口重建验证；后台 52 项、商城 97 项测试，两套类型构建及支付、钱包、权限、架构 Go 回归通过。本地验证两条未付款充值单及余额增加 100、扣减 25、流水记录，测试用户和记录已清理；本地采购保持关闭。
- 正式站 5 项只读浏览器检查与 437 个前端文件哈希核验通过。两地备份为 `/srv/zhixiang/backups/before-wallet-20261013-110949-retry`、`/srv/zhixiang-front/backups/before-wallet-20261013-110949`；报告、截图在忽略目录 `work/codex-runtime/wallet-release/`、`work/codex-runtime/wallet-repair-qa/`。用户、余额、订单、专属价、公开售价和成本、私有配置保持一致，Redis 未重启。未上传本机库或进行真实付款、采购验收。
- 本次通过忽略文件 `work/codex-runtime/private-admin-access/production-admin.credentials.json` 完成一次正式后台只读登录；账号没有重置。此前公告记录中的两份失效口令仍不应重试。

- 会员等级保留默认等级、充值/消费升级和用户归属，统一百分比折扣已停用。通过「会员等级 → 设置商品价」分别设置商品和规格的固定优惠金额；个人专属价优先，未设置会员价沿用商城当前价，更低的活动价和数量优惠保留。不得重新叠加历史 `discount_rate`。操作见 `docs/member-pricing.md`。

## 根域入口修复（2026-10-13）

- `http://qzweb.cloud` 和 `https://qzweb.cloud` 已统一 308 跳转到 `https://www.qzweb.cloud`，保留路径与查询参数。配置源码 `2774d53935c098d9a1c86b90a425ca826e91bdc3`，模板 `ops/tencent/frontend-gateway/nginx-apex.conf`，上海实际文件 `/etc/nginx/conf.d/qzweb-apex.conf`。根域独立证书已纳入现有 `qzweb-certificates.timer`；维护时须保留根域与 WWW 两套入口及 ACME 目录。
- 此前根域没有被已加载的商城配置接管，请求落到上海默认家具站 `oakved.conf`，页面在 `/opt/oakved/frontend/furniture`。家具配置没有显式声明根域；旧 `sites-enabled/qzweb.cloud` 当前不被 Nginx 加载。时间证据与限制详见 `ops/tencent/README.md`，不得把文件时间说成确定的域名绑定时间。
- 本机 10 项检查、正式公网 9 项检查及实际浏览器导航通过。两地备份均为各自 `backups/before-apex-domain-20261013-135130`，私有证据在 `work/codex-runtime/domain-diagnostic/`。本次只增加根域配置与证书、平滑重载 Nginx；未改 DNS、家具站及原商城配置、应用程序或业务数据，生产程序仍为 `20261013-114011`。

## 后台品牌与运营入口统一（2026-10-13）

- 已按批准的预览统一登录页、三语标题、桌面/手机侧栏、折叠 Logo、页脚及默认图标，移除原项目广告、赞助和 Telegram Bot 购买推广。顶部为只读「系统信息」，不再提供上游一键升级。设置示例和恢复码文件名使用本站名称；真实支付/供货协议名称、许可证说明与财务声明确认流程保留。见 `docs/admin-branding.md`。
- 后台商城入口固定指向 `https://www.qzweb.cloud/`；正式 `site_config.brand.site_url` 中既有的本机预览地址没有被本次发布改写，也不再覆盖这两个后台入口。客服微信仍为 `Cqz0924ttxk`，ChatGPT 兑换入口仍为 `https://cdk.bar/`。
- 55 项后台测试、类型构建、本机与正式站各 19 项浏览器检查通过，覆盖 1920、390、320px 及三种语言；无脚本错误、横向溢出、后台广告/升级请求或验收业务写入。上海 `20261013-143008` 仅替换后台静态资源，核验 119 个后台文件，保留商城 1468 个文件；后台源码 `ebee216118e6ee8d7d3c53c4ab36449b371850b3`，新加坡程序与商城资源仍为原基准。
- 两地备份均为各自 `backups/before-admin-brand-20261013-143008`；核验记录在忽略目录 `work/codex-runtime/admin-brand-qa/`、`work/codex-runtime/admin-brand-release/`。发布前准备期间的订单、支付及专属价记录更新已保留；发布区间以切换前一致性备份比对，业务数据一致。后端与 Redis 进程、收款配置、售价与成本、Nginx 配置保持原状态，未上传本机库或进行真实付款采购测试。

## 错误页图标与商品列表路由修复（2026-10-13）

- 配置源码 `9fd00866eb655b0da9e95c8fdaca20152c73911e` 已上线。商城 Nginx 必须使用 `try_files $uri /index.html`，不能恢复 `$uri/`：`/products` 与真实图片目录同名，优先匹配目录会使商品列表直接访问或刷新报 403。
- 商城和后台共同加载 `/etc/nginx/snippets/qzweb-errors.conf`，来源 `ops/tencent/frontend-gateway/nginx-errors.conf`；默认 `/favicon.ico` 返回派大星 PNG，403/404/500/502/503/504 错误页明确引用带版本的图标地址。更新两套 Nginx 入口模板时须先安装共享片段，保留状态码和 API 错误透传；更换 Logo 时同步更新图标 URL 版本。
- 本机 35 项、正式服务器 15 项、公网 16 项检查及实际浏览器商品页、403/404 图标核验通过。两地备份均为各自 `backups/before-favicon-route-20261013-144241`，私有记录在 `work/codex-runtime/favicon-repair/`。仅平滑重载 Nginx，上海发布仍为 `20261013-143008`，新加坡程序仍为 `20261013-114011`；现有前端资源、价格与成本、支付渠道、专属价格、站点及私有配置保持一致，未重启应用或执行付款采购测试。

## 后台报错标签图标补充修复（2026-10-13）

- 配置源码 `7a669ccbf6d6165a26daf82e5a4aa3cb06e52ff9` 已于 `20261013-145235` 上线。此前缺失 `.js` 等资源的错误页仍继承资源 MIME 类型，浏览器按原始文本显示，HTML 中的图标声明失效；共享错误片段的六个命名位置均须保留 `types { }`，确保错误页返回 `text/html`。版本化派大星图标、原错误状态码及 API JSON 错误透传保留。
- 本机 37 项 HTTP、22 项浏览器检查，正式站 16 项浏览器、12 项服务器 HTTP 检查通过。覆盖 403/404/500/502/503/504、资源后缀、1920/390/320px、模拟接口失败和失效登录；5xx 仅在本机复现。两地备份均为各自 `backups/before-admin-error-icon-20261013-145235`，私有证据在 `work/codex-runtime/admin-error-icon/`。仅平滑重载 Nginx；上海静态 `20261013-143008` 的 1960 个文件、新加坡程序 `20261013-114011`、后端与 Redis 进程、价格和收款配置均核对一致，未修改业务数据或制造生产服务中断。

## 后台商品缩略图修复（2026-10-13）

- 已按批准的 B 方案上线 100×60 完整横图，商品列表和批发价列表共用图片组件；加载失败或无图片显示统一占位，地址更新后可恢复。商品原图和业务数据未修改。见 `docs/admin-product-thumbnails.md`。
- 后台须保留 `/catalog-products/` 到当前商城资源的映射，以及 `/products/` 下仅匹配图片扩展名的规则；不能用整个 `/products/` 前缀拦截管理页面。日常本地后台通过 5173 读取相同图片，须同时运行商城入口。
- 上海发布 `20261013-153730`，后台源码 `471ded02e1b265520a53485288bdcc4f47dbece9`；119 个后台文件、1468 个保留商城文件核验通过。40 个商品图片地址及全部 44 条后台记录的实际加载正常。本地 58 项测试、类型构建、50 项 HTTP 检查、7 项浏览器检查，以及正式 49 项服务器检查、9 项浏览器检查通过；截图覆盖 1920、390、320px。320px 且有系统滚动条时，未改动的顶部控件仍有 5px 溢出，商品表自身横向滚动正常。
- 两地备份均为各自 `backups/before-product-thumbnails-20261013-153730`；核验记录在忽略目录 `work/codex-runtime/product-thumbnails/`。新加坡程序仍为 `20261013-114011`，后端与 Redis 进程、商城资源、商品图地址、价格与成本、支付及专属价配置均与本次切换前备份一致。未上传本机库或进行付款采购测试。

## 注册图形验证码（2026-10-13）

- 已按批准预览启用注册图形验证码，并区分“图形验证码”与“邮箱验证码”。先通过图形校验才能发送邮箱验证码；注册继续要求有效邮箱验证码。注册发送成功或失败后刷新已消耗的图片，保留邮箱和密码。后台“系统设置 → 验证码设置”维护，最小配置为 `docs/catalog/registration-captcha.json`，操作说明见 `docs/registration-captcha.md`。保持邮箱验证开启；登录、找回密码、游客下单和礼品卡场景本次未启用。
- 上海发布 `20261013-160811`，商城源码 `c0d5e48c1407261e5d800969c228f03a644732d7`。97 项商城测试、类型构建、验证码及注册相关 Go 回归、本机 13 项和正式站 11 项浏览器检查通过，覆盖 1920、390、320px 和三语，无脚本错误或横向溢出。正式 322 个新商城文件及 588 个保留后台文件核验通过；旧散列资源、商品图及 Nginx 配置保留。
- 两地备份为 `/srv/zhixiang/backups/before-registration-captcha-20261013-160811`、`/srv/zhixiang/backups/before-registration-captcha-settings-20261013-160811`、`/srv/zhixiang-front/backups/before-registration-captcha-20261013-160811`；证据在忽略目录 `work/codex-runtime/register-captcha-preview-20261013/`。仅更新商城资源及 `captcha_config.provider=image`、`captcha_config.scenes.register_send_code=true`。新加坡程序与后端、Redis 进程保持原状态；其他站点和私有配置、售价与成本、收款渠道及个人专属价一致。未上传本机库、创建正式测试用户、发送真实验收邮件或进行付款采购测试；成功发信后的前端状态使用浏览器模拟响应验证。
