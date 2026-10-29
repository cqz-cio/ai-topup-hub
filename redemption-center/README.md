# PAIPLUS 兑换中心

独立的兑换网站：海绵宝宝街景动画、侧栏四功能布局、自己的 PAIPLUS 卡密、真实持久化后端、管理后台，以及可选择启用的商城交付适配器。

正式入口为 `https://cdk.qzweb.cloud/`，后台为 `/admin`；2026-10-29 已部署并对下文列出的 9 款商品新订单启用商城交付转换。本地验收入口为 `http://127.0.0.1:5191/`，使用 **mock 源头**。本轮验收没有真实付款、采购或充值。

商城后台现已统一管理交付卡密：`https://admin.qzweb.cloud/card-secrets` 的「PAIPLUS 交付卡密」支持状态筛选、显式查看本站码与上游码、退款前实时检查并永久销毁本站码。销毁保留双码核账历史，不执行退款或上游销毁。配置、权限、验证和回退要求见 [卡密管理维护说明](../docs/paiplus-card-management.md)。

## 功能与边界

- 单卡：验证卡密 → 核对账号与套餐 → 用户确认 → 提交 → 查询完成状态。
- 批量：本地读取 Excel/CSV，保留原行号；拒绝缺项、空行、重复卡密与公式；最多 100 行，先核对再提交。可以暂停，已提交行不会再次充值。
- 卡密查询、ChatGPT 账单查询：返回实际字段；缺失值与 `false`、金额 `0` 分开处理。
- 后台：导入源头卡密、生成及下载交付内容、查看本站码、停用/启用、解除尚未提交的账号核验、查看最近兑换。
- 卡密格式：`PAIPLUS-XXXXXXXXXX-XXXXXXXXXX`；每组 10 位且同时包含字母和数字，密码学随机，去掉 `I/O/0/1`。本站码通过私有映射关联源头码，不是替换原码前缀。
- 接收成功、排队和充值完成是不同状态。提交前先保存请求编号；超时保留原记录，服务端后台只查结果，不自动重发充值。
- GPT、Claude、X 使用预检票据；Grok 按源头协议在提交时携带 Session。账号凭证不写浏览器存储；Grok 临时凭证加密，提交或 15 分钟核验过期后清除。
- 库内卡密、源头码、设备标识及账号结果使用 AES-256-GCM 加密。管理员显示卡密需要独立登录并记录审计。API 禁止缓存。

真实源头来自用户提供的 `cdk-partner-kit-v1.zip`。服务端固定访问 `https://cdk.bar/api/partner/v1`，使用 Basic `partner` 和 `CDK_PARTNER_PASSWORD`。浏览器仅访问本站 `/api`，不接触源头密码或源头卡密。上游没有发码接口：须有已采购源头卡，或由商城付款采购后调用交付映射。

## 本地启动

需要 Node.js 24.19 或兼容版本（支持 `node:sqlite`），构建使用锁文件中的依赖。

```powershell
npm ci
# 终端 1
$env:REDEMPTION_PUBLIC_ORIGIN='http://127.0.0.1:5191'
npm run dev:api
# 终端 2
npm run dev -- --port 5191
```

`dev:api` 强制 mock 模式，只允许监听回环地址。第一次启动自动生成 `runtime/local-secrets.json` 和本地数据库；不需要真实源头密码。页面可以生成新一组测试卡，旧记录保留。后台点击“进入本地联调管理页”。mock 绕过登录的入口在 live 模式不可用。

## 构建与测试

```sh
npm test
npm run build
node scripts/prepare-release.mjs
```

`dist/release/` 是实际部署目录，包含 Node 服务端和 `dist/client`，没有数据库、`.env`、本地密钥或开发依赖。启动 `node server/main.mjs`，生产运行无需安装 npm 依赖。`dist/server/index.js` 是保留的静态预览适配器，**单独部署它不能提供兑换功能**。

商城集成回归在仓库根目录执行：

```sh
go test ./internal/modules/procurement/... ./internal/app/container ./internal/architecture -count=1
```

## 正式部署

1. 准备兑换域名、HTTPS、独立运行用户和 Node 24。参考 `ops/nginx.conf.example` 与 `ops/paiplus.service`，模板需要替换域名、证书和安装路径。
2. 将 release 目录放到 `/srv/paiplus/current`；数据使用独立 `/srv/paiplus/data`。从 `.env.example` 建立服务器私有配置 `/etc/paiplus.env`（0600）；所有占位项必须填写。
3. `REDEMPTION_MASTER_KEY` 是随机 32 字节的 Base64；管理密钥和商城集成密钥各自使用独立随机值，至少 32 字符。仅在服务器生成并保存，不能放入前端、Git 或发布包。
4. `REDEMPTION_MODE=live`，`REDEMPTION_PUBLIC_ORIGIN=https://实际兑换域名`，填写 `CDK_PARTNER_PASSWORD`。正式数据库必须是新库，不能复用 mock 库；不允许随意更换已有数据库的加密密钥。
5. Node 监听 127.0.0.1:8190，Nginx 代理。使用模板的 `X-Real-IP` 时设置 `REDEMPTION_TRUST_PROXY=loopback`，不信任直接公网请求的转发头。首次验证首页、管理登录、静态资源、API 来源限制，再用获准的真实测试卡验收充值。

充值结果需要持续查询，运行一个 Node 实例并保留本机持久卷。不要用临时文件系统或同时启动多个实例；单实例负责四路并发查询，长时间未确认记录会逐步降低查询频率。外部故障不会触发重新充值。

## 备份与恢复

```sh
node scripts/backup.mjs /srv/paiplus/data/redemption.db /安全备份目录/redemption-时间戳.db
```

备份使用 SQLite 一致性快照，不直接复制活跃 WAL 数据库。加密主密钥必须另行保管；丢失主密钥将无法还原已发卡映射。恢复时停止服务，使用匹配的数据库和原密钥；不要通过重置数据库、清除已提交标记或新建请求编号重试充值。定期对隔离副本做恢复检查。

## 商城自动交付接入

仓库的 Go 采购交付新增可选 `DeliveryRewriter`。默认关闭，只有明确选中的供货连接、商品、订单编号下限及付款时间范围启用。参数模板见 `ops/shop-integration.env.example`。

商城采购成功后，先保存私有原始结果与待绑定标记，再调用兑换站。兑换站按 `shop:<连接>:order:<订单>:<序号>` 永久绑定，一张源头码只能对应一张 PAIPLUS。商城仅从经过校验的结构化本站码生成交付正文；账号组合中的邮箱、密码和 2FA 内容保留，未知文本或格式暂停发货等待核对。原始结果继续仅存采购私有记录。

客户交付正文与订单使用说明在同一事务保存。订单读取、会员/游客下载、邮件正文及大批量邮件附件、下游通知均读取这一份本站内容。转换服务故障或响应丢失时不发原码；自动恢复直接使用已保存原件和同一编号，不重新采购。即使暂时停用集成或改变范围，已标记待绑定订单也会继续阻止原码交付。

启用前必须选择实际由 CDK API 支持的商品 ID，记录当时最大订单号并将下限设为下一单，同时设置未来生效的付款时间。Gemini 私有链接、短信链接和非 CDK 商品不应选择。只接受单商品订单（1–100 份）、自动交付和可识别的纯卡密/带标签卡密；其他格式保留待核查，不擅自猜测内容。

正式域名为 **https://cdk.qzweb.cloud**，使用现有上海服务器 `124.220.2.69` 的独立服务。所选新订单交付时会自动用本站说明覆盖其订单快照，不依赖批量修改商品数据库。已有历史交付和原始采购结果不迁移；售前页面不公开兑换入口。

回退前先暂停所选商品新下单，核对并处理 `paiplus_binding_pending` / `paiplus_format_review` 订单。不得直接切回不了解这些标记的旧商城程序，否则旧程序可能发出原码。已发出的 PAIPLUS 仍依赖兑换站，须保留站点、数据库与主密钥；不回灌商城整库。

## 自动发货整链路验收

`PAIPLUS_TEST_NODE` 指向 Node 24 可执行文件后，在商城根目录运行：

```sh
go test ./internal/modules/procurement/integrationtest -run 'TestPAIPLUSShipping|TestRedemptionIssuerFailure' -count=1 -v
```

测试会启动独立 Node 兑换服务、真实 Go 采购适配器、临时 SQLite 及本机 SMTP 接收端。三种场景为单卡、24 张批量、账号加卡密，共 26 张卡；覆盖绑定后丢失响应、重启后恢复、重复回调、私有说明替换、会员/游客鉴权与完整下载、邮件正文及附件、下游交付投影、确认后兑换完成且重复确认仅提交一次。源头充值由模拟接口代替，不调用真实采购或充值，不发送客户邮件。

2026-10-29 本地整链路及 33 项 Node 测试通过，包含发布目录链接启动回归。应用代码按平台合并核验 1,800 个顶层 Go 测试，9 个外部依赖检查按条件跳过；日志文件与系统升级锁的两组检查在 Linux 临时目录复验通过。真实充值仍需要获准的测试卡与账号，不能用本地模拟结果代替。

## 2026-10-29 正式启用记录

- 上海兑换站、新加坡后端发布 `20261029-153938`；程序源码 `c8c3bcd7593d9cceb47ca6cf8c5d58ff4301f1bb`，干净构建，任务分支 `codex/paiplus-binding` 已推送。用户要求保留远程 `main`，本次未更新它。
- `cdk.qzweb.cloud` 的 A 记录指向上海 `124.220.2.69`。HTTPS 证书已签发，Node 24.19.0 安装在 `/srv/paiplus/runtime`，现有全局 Node 20 保持。`paiplus.service` 监听 `127.0.0.1:8190`；现有 Nginx 使用 `conf.d/*.conf`，本站链接为 `/etc/nginx/conf.d/cdk.qzweb.cloud.conf`。
- 当前供货连接 #1、商品 #29、30、31、32、33、35、37、45、65 启用。订单下限固定为 **75**，付款时间下限为 **2026-10-29T07:54:51.676959Z**（北京时间 15:54:51）。后续重启或发布不能重算这些下限；新增商品须先核验 CDK API 支持与发货格式，再审阅加入范围。未选择 Gemini 私有链接、接码链接、人工商品或已下架且未核验的新商品。
- 商城配置 `/srv/zhixiang/shared/paiplus-integration.env`，仅 root 可读；新增 systemd `97-paiplus.conf`。原采购下限 44、付款时间 2026-10-16T13:31:15.812954Z、Redis、4 个私有配置文件和 6 个既有 drop-in 保持。上海商城与后台静态发布未切换。
- 切换时 72 张业务表一致。复核订单、条目、交付、采购、支付、支付渠道、钱包记录与发布前备份一致；售价保持。两套库存 service 均 `Result=success`，timer active，并核对新报告；第一家现用 `merchant1039-catalog-sync.json`，不能误用旧 stock 报告时间。
- 21 个兑换站发布文件及 7 个公开资源哈希通过；HTTPS、Secure Cookie、API 不缓存、来源检查、未授权管理/集成请求、私有文件不可读、模拟入口关闭均通过。正式浏览器检查四个工具入口及无效卡密提示。只用不存在的测试码检查上游鉴权（无凭据 401，有凭据按无效卡密返回 400），未提交真实充值或正式验收订单。
- 商城备份 `/srv/zhixiang/backups/before-paiplus-20261029-153938`；兑换站配置与初始一致性备份 `/srv/paiplus/backups/before-first-release`。首次启动路径问题已修复，已有主密钥未更换。增量还原后的完整二进制 SHA256 为 `b546ee81804b2181d81f061386da2a988dded5c2a7fb8e0043b8061955d3991b`。临时传输入口已撤销，原 57 个 Nginx 文件保持。
- 私有验收证据保存在本机 `redemption-center/runtime/`；店主管理入口及口令在该目录的 `production-admin-access.txt`，不提交 Git。服务器的数据库与加密主密钥必须一起保留；后续备份使用上面的 SQLite 一致性备份脚本。

## 2026-10-29 商城介绍与发货模板补齐

上线后的新订单交付程序已使用本站域名；本次继续清理同一范围内 9 款商品的原始模板。中文摘要和详情明确发放本站 **PAIPLUS 卡密**，具体兑换入口继续仅随订单交付；商品 `instructions_json` 中的旧兑换地址（含成品号的 `/recharge`）统一为 `https://cdk.qzweb.cloud/`。只改动 27 个文案字段，保留套餐、售后规则、其他语言、售价、支付配置、历史订单与原交付快照。短信、人工、其他未接入商品及归档商品没有套用此域名。

维护函数为 `scripts/paiplus_catalog_copy.py` 的 `apply_review`：固定核对商品 ID/slug、上架与交付方式；使用逐字段审阅的 before/after 拒绝覆盖运营新修改。先 SQLite 一致性备份，在备份副本演练及重复应用，再在单个事务内核对全部 72 张表，仅允许指定文案字段变化。正式 9 个公开商品接口逐项匹配，私有交付模板不出现在公开响应；公开配置未加入兑换地址。程序、服务和原始绑定范围未改变。

备份 `/srv/zhixiang/backups/before-paiplus-copy-20261029T082143817556Z/shop.db`，同目录保存审阅文案和报告。本机证据为 `redemption-center/runtime/paiplus-content-update-backend.json`。回退仅审阅恢复指定文案字段，不回灌整库。历史 `qzweb-redemption-guide.json` 涵盖其他与归档商品，不能把它的所有旧域名全局替换或重新导入覆盖本次范围；新增商品仍须单独确认其卡密可由本站兑换。

## 兑换站图标

当前为清晰度优化版 **v4**：保留完整人物，采用紧凑的屈肘托举构图、较大的正面脸部和较粗轮廓；去掉透明外边距，并显式提供原生 16px 图标。当前文件为 `public/icons/spongebob-lifting-gary-v4*`，生成提示和尺寸说明见 `design/favicon-v4.md`。下面 v3 为首版上线记录，回退或后续发布时不要误当成当前图标。

v4 已发布为静态补丁 `favicon-20261029T091427Z`，干净构建来源 `a0d8f702277998a86c38bdf80e2b755905626fd2`；原程序仍为 `20261029-153938`。新增 8 个版本化图标，替换入口 HTML 与默认 `favicon.ico`。原 29 个其他发布文件、60 个 Nginx 文件、配置与服务进程保持；10 个公网 HTML/图标请求的状态、MIME 和 SHA256 通过，会话接口正常。已检查本地 16/32/48px 原生尺寸对比，以及正式页面的五条图标引用和无破图、无错误日志。正式站源码位于任务分支 `codex/paiplus-binding`。

v4 备份为 `/srv/paiplus/backups/before-favicon-20261029T091427Z`，补丁清单及报告在 `/srv/paiplus/static-updates/favicon-20261029T091427Z/`。回退本次优化只恢复备份的 `release/dist/client/index.html` 和 `release/dist/client/favicon.ico`，保留历史图标，不恢复业务数据库。私有验收为 `runtime/paiplus-deploy-favicon-v4-frontend.json`，尺寸对比截图为 `runtime/favicon-v4-comparison.png`。本地对比 HTML 仅在忽略目录，未发布到正式站。

Chrome 标签页已刷新。受浏览器工具控制时观察到扩展会将 `rel=icon` 临时替换为带 `data-codex-favicon-badge` 的 SVG，包含 0.3 透明度原图和指针标记；此操作状态图标不作为真实客户显示的验收依据，也不删除或覆盖该标记。正式源文件和无此标记的内置浏览器引用已独立核验。

用户于 2026-10-29 确认全身海绵宝宝双手举起小蜗的透明图标，采用现有街景壁纸中的动画画风。批准原图为 `public/icons/spongebob-lifting-gary-v3.png`，由内置 imagegen 参考 `public/assets/bikini-bottom-afternoon.png` 编辑生成；保留原始 PNG 字节，格式转换只做缩放，未重绘或裁切人物。早期手绘 v1 和 v2 预览未获采用，不应再次用于站点。

同名 16、32、48、180、192、512px PNG 保留真实 alpha；版本化 ICO 与 `public/favicon.ico` 包含 16/32/48px。`index.html` 引用版本化 ICO、32/48px PNG 与 180px Apple Touch 图标，避免复用旧图标缓存。网站页首品牌图保持原样。生成提示与来源记录见 `design/favicon-v3.md`。

已于 2026-10-29 发布静态补丁 `favicon-20261029T090203Z`，干净构建来源 `49ad5d3f1131416545143b3a0a6a72a505a441dc`；源码推送至已授权的 `codex/paiplus-binding` 分支。兑换站程序仍为 `20261029-153938`，本次只新增 9 个图标文件并替换 `dist/client/index.html`。6 个原客户端资源与原 21 个非入口发布文件逐字节保持，60 个 Nginx 文件、私有配置和服务进程保持；没有重启服务或调用充值接口。页面及图标共 10 个 HTTPS 请求的状态、MIME 与 SHA256 核验通过，正式会话接口正常，本地与正式浏览器读到新图标引用，正式 180px 图标可完整加载。

备份 `/srv/paiplus/backups/before-favicon-20261029T090203Z` 包含原发布、私有配置和兑换数据库一致性快照；补丁清单与验收报告在 `/srv/paiplus/static-updates/favicon-20261029T090203Z/`，叠加在原发布清单之上。回退只恢复备份中的 `release/dist/client/index.html`，新增的版本化图标可保留；不恢复业务数据库。后续发布须包含本次图标及引用，避免以旧基础发布覆盖。私有本机证据为 `runtime/paiplus-deploy-favicon-frontend.json`，正式图标截图为 `runtime/favicon-v3-production.png`。

2026-10-29 排查浏览器“不安全”提示：服务器与本机独立 TLS 验证均通过，证书 SAN 为 `cdk.qzweb.cloud`、签发者 Let's Encrypt YE1，到期时间 2027-01-27 06:28:35 UTC；两处证书 SHA256 一致，均协商 TLS 1.3。HTTP 返回 301 至 HTTPS，站点启用 HSTS；页面资源为本站 HTTPS，未发现网络 HTTP 资源。用户反馈手机正常、电脑 Chrome 仍提示不安全，尚未取得 Chrome 安全概览的具体原因，不能把服务器核验通过称为已修复浏览器警告。证据位于 `runtime/paiplus-tls-inspect-frontend.json` 和 `runtime/local-tls-check.json`。


## 2026-10-29 兑换站独立导航

按用户要求，兑换站页面不再提供商城跳转：移除首页、商品中心、我的订单、去订单领取卡密及客服弹窗中的商城链接。卡密与使用说明改为直接输入收到的 PAIPLUS；客服提示直接联系提供卡密的客服。页首说明为「卡密兑换 · 自助服务」，保留四个兑换工具、帮助、主题切换、动画、品牌与 v4 图标。后台一对一绑定、状态核验及销毁接口继续保持，不切断商城发货接入。

正式静态补丁为 `standalone-20261029T122008Z`，界面源码 `b5acebb74581bddda7a43805820560a428b2c27d`，任务分支 `codex/paiplus-binding`。仅新增 `assets/index-DdVPQS8k.js` 并原子替换 `dist/client/index.html`；Node 程序仍为 `20261029-181554`，46 个原文件、60 个 Nginx 文件、私有配置和服务进程保持。未写业务数据库、重启服务或调用充值/采购接口；main 未更新。

最终生产构建、本地及正式桌面/390px/320px 浏览器检查通过：四个工具切换、帮助和客服弹窗、手机菜单正常，无页面错误、破图或横向溢出。正式入口、管理页入口、新脚本与 v4 图标的 SHA256 和 MIME 核验通过，正式会话保持 live 且无模拟样例。

备份 `/srv/paiplus/backups/before-standalone-20261029T122008Z/index.html`；补丁清单与报告在 `/srv/paiplus/static-updates/standalone-20261029T122008Z/`。回退仅将此入口文件原子恢复到记录的发布目录，保留新旧版本资源，不恢复服务器代码、配置或数据库。后续完整发布须包含本次独立导航，不能以旧 HTML 覆盖。私有证据为 `runtime/paiplus-standalone-deploy-frontend.json`、`runtime/standalone-browser-checks.json`，正式截图为 `runtime/standalone-production.png`。
