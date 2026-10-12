# 新加坡正式环境

现有服务器 `43.134.141.10` 是智享A社卡网的正式环境。[商城](http://43.134.141.10/)与[后台](http://43.134.141.10/zxa-management/)由同一 Go 程序提供。当前入口为 HTTP/IP，未配置域名与 HTTPS。

## 已核验的运行基准

2026-10-12 已备份并发布 **`20261012-111003`**，新增独立 Plasma USDT 收款，保留 BEP20 和此前的 Logo 清晰度修正。发布后已通过 SSH 与公网 GET 核验。

| 项目 | 核验结果 |
| --- | --- |
| 当前发布目录 | `/srv/zhixiang/current` → `/srv/zhixiang/releases/20261012-111003` |
| 正在运行的程序 | 上述目录的 `zhixiang-shop`，90349730 字节 |
| 程序 SHA-256 | `a3d18cf153fda70210c3c54a4be8890088209565d7b8222e9d41b2dfe38a1e5a` |
| 发布清单 | `manifest.json` 的程序哈希与运行中程序一致 |
| 编译信息 | Go 1.26.5，Linux amd64，CGO=0，`release,fullstack` |
| 运行模式 | `storefront`，`UPSTREAM_PURCHASE_ENABLED=true` |
| 服务状态 | 商城、专用 Redis、库存定时器均 active |
| 构建源码 | `main` 的 `800bf983e6eaaf10855eb9b79e707f2132176f9a`，`vcs.modified=false` |
| 上一个发布 | `20261012-105801`，保留于服务器及升级前备份 |
| 收款渠道 | BEP20 与 Plasma 均启用，公网支付配置已显示两项 |

完整证据见 [production-baseline.json](production-baseline.json)。本次发布清单与 Go 编译信息均记录完整 Git SHA，运行中程序哈希与本地发布包一致，前后台入口资源及品牌图片哈希核验通过。公网首页、后台、健康接口均为 200；6 个公开分类、原有售价和支付渠道保持不变。后续仅更新文档的提交不表示再次部署程序。库存为动态数据，本地快照不是实时生产库存。

## 持久化数据与服务

| 用途 | 服务器位置 |
| --- | --- |
| 配置 | `/srv/zhixiang/shared/config.yml` |
| 生产数据库 | `/srv/zhixiang/shared/db/shop.db` |
| 上传与日志 | `/srv/zhixiang/shared/uploads`、`logs` |
| 持久化 Redis | `shared/redis/`，回环地址端口 16379 |
| 库存助手私有凭据 | `shared/stock-sync.credentials.json` |
| 库存报告 | `shared/reports/merchant1039-stock-sync.json` |
| 当前版本升级前备份 | `/srv/zhixiang/backups/before-20261012-111003`，数据库一致性检查通过，包含原配置与旧程序 |
| 本次发布核验报告 | `shared/reports/plasma-release-20261012-111003.json` |

`zhixiang-shop.service` 以 `zhixiang` 用户运行；`40-storefront.conf` 指定当前模式。支付监听、采购/交付 worker 与恢复任务由应用管理。`zhixiang-redis.service` 保存订单队列。`zhixiang-stock-sync.timer` 在每次任务完成后等待 300 秒，仅查询和刷新库存；不要另起重复助手或完整 worker 来同步库存。服务单元模板保存在本目录。

商户 1039 使用 `acg-faka`，已有 12 款供货映射，`auto_sync_price=false`。全部 16 款商品售价独立维护，`qzweb-55` 保持 1160.00 元。自动充值关闭；余额和库存需实时查询，不把历史报告的数值当作当前数值。客户确认付款后的采购已经授权，但没有做真实扣款验收，不主动提交有效采购或修改测试订单为已付款。

客户付到 BEP20 / Plasma 地址的 USDT 与供货商采购余额相互独立；自动采购需要供货商人民币余额充足、映射有效且对应 SKU 有库存。余额不足的采购记录进入 `rejected`，客户订单保留已付款状态，补足余额后需对这些失败记录手动重试；扣款结果未知时进入 `manual_review`，核对上游后再处理，不能直接重新采购。两款本地卡密商品和两款人工交付商品不走供货商采购。

链路回归测试位于 `internal/bootstrap/bscusdt/procurement_flow_test.go`：BEP20、Plasma 分别覆盖即时发卡、延迟取卡、队列恢复、余额不足和采购结果未知，共 10 个场景，同时核验最终客户订单响应包含交付内容以及重复任务不重复采购。2026-10-12 已通过；测试采用临时数据库和模拟链、队列传输及供应商，不构成真实资金验收。本机私有检查记录保存在忽略的 `work/codex-runtime/payment-chain-audit-20261012.json`。

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

2026-10-12 链路检查发现，维护账号访问两条链的后台监控接口均返回 `compliance_required`，需超级管理员在后台自行确认现有声明，检查脚本不代为确认；此限制不阻断客户支付或链上监听。另有待修复的监控权限遗漏：内置 `finance` 角色目前只有 BEP20 的状态和转账查询权限，尚未加入 Plasma 对应权限；超级管理员不受这一角色策略遗漏影响。本轮检查没有修改线上权限或程序。

### 正式升级规则

GitHub `main` 为当前绿色商城源码主线，推送不会自动更新此服务器。升级遵循 [发布规则](../../docs/release-process.md)：先记录源码提交及构建哈希，备份线上库、配置和当前发布，再执行已授权的程序升级；保留 `shared/`、storefront 模式、支付采购队列和持久化 Redis。不得上传本地预览库覆盖生产订单。

历次部署细节已移入 [历史部署记录](../../docs/history/tencent-deployments-20261012.md)，其中旧 `api`/`checkout` 命令和本机私有包路径仅为历史资料。不要将通用上游安装器或旧初始化脚本用于覆盖正式环境。
