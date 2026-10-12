# 新加坡正式环境

服务器 `43.134.141.10` 是 π星卡社的正式环境。[商城](http://43.134.141.10/)与[后台](http://43.134.141.10/zxa-management/)由同一 Go 程序提供。

服务器已有 `www.qzweb.cloud` 的 HTTPS 反向代理，本次保留其配置并验证源站 TLS、页面和资源。2026-10-12 16:12（北京时间）公网 Cloudflare 入口仍返回旧版“智享A社”页面，且健康、配置接口与源站不同；这不是新版域名上线成功，当前正式新版入口以上述 IP 地址为准。后续切换域名需核对 DNS、Cloudflare 回源和缓存，不能仅凭源站证书或本地 SNI 核验宣告公网域名已切换。

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

用户已授权后续会话复用已提供的服务器信息。当前 Windows 主目录的凭据文件为 `D:\卡网\work\codex-runtime\tencent-cloud-server.credentials.json`，字段为 `host`、`port`、`username`、`password`；文件已存在且由 Git 忽略。SSH 主机指纹保存在 `work/codex-runtime/tencent-known-hosts`。连接时从文件读取并核对 `43.134.141.10` / `ubuntu`，加载已保存指纹并拒绝未知或变化的主机密钥。不要输出整个凭据文件。

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
