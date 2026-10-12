# 新加坡正式环境

现有服务器 `43.134.141.10` 是智享A社卡网的正式环境。[商城](http://43.134.141.10/)与[后台](http://43.134.141.10/zxa-management/)由同一 Go 程序提供。当前入口为 HTTP/IP，未配置域名与 HTTPS。

## 已核验的运行基准

2026-10-12 通过 SSH 只读核验，当前发布为 **`20261012-013649`**。本次仓库整理没有部署程序、切换服务或写入生产数据库。

| 项目 | 核验结果 |
| --- | --- |
| 当前发布目录 | `/srv/zhixiang/current` → `/srv/zhixiang/releases/20261012-013649` |
| 正在运行的程序 | 上述目录的 `zhixiang-shop`，108794100 字节 |
| 程序 SHA-256 | `a2d0549bdaf0b9ffff4b25e1f4868063c2cd1cf6ce80692f668b3990dea3884b` |
| 发布清单 | `manifest.json` 的程序哈希与运行中程序一致 |
| 编译信息 | Go 1.26.5，Linux amd64，CGO=0，`release,fullstack` |
| 运行模式 | `storefront`，`UPSTREAM_PURCHASE_ENABLED=true` |
| 服务状态 | 商城、专用 Redis、库存定时器均 active |
| 源码基线 | Git 标签 `baseline/green-20261012`，提交 `419141e`；不是已证实的生产构建提交号 |

完整证据见 [production-baseline.json](production-baseline.json)。线上程序没有内嵌 `vcs.revision`，发布清单也没有源码提交号，因此不能从现有记录确定精确 Git 版本。前后台部分编译入口资源与本地构建不同；16 款商品公开资料、6 个分类、18 张商品/分类图片一致。库存为动态数据，本地快照不是实时生产库存。

## 持久化数据与服务

| 用途 | 服务器位置 |
| --- | --- |
| 配置 | `/srv/zhixiang/shared/config.yml` |
| 生产数据库 | `/srv/zhixiang/shared/db/shop.db` |
| 上传与日志 | `/srv/zhixiang/shared/uploads`、`logs` |
| 持久化 Redis | `shared/redis/`，回环地址端口 16379 |
| 库存助手私有凭据 | `shared/stock-sync.credentials.json` |
| 库存报告 | `shared/reports/merchant1039-stock-sync.json` |
| 当前版本升级前备份 | `/srv/zhixiang/backups/before-20261012-013649` |

`zhixiang-shop.service` 以 `zhixiang` 用户运行；`40-storefront.conf` 指定当前模式。支付监听、采购/交付 worker 与恢复任务由应用管理。`zhixiang-redis.service` 保存订单队列。`zhixiang-stock-sync.timer` 在每次任务完成后等待 300 秒，仅查询和刷新库存；不要另起重复助手或完整 worker 来同步库存。服务单元模板保存在本目录。

商户 1039 使用 `acg-faka`，已有 12 款供货映射，`auto_sync_price=false`。全部 16 款商品售价独立维护，`qzweb-55` 保持 1160.00 元。自动充值关闭；余额和库存需实时查询，不把历史报告的数值当作当前数值。客户确认付款后的采购已经授权，但没有做真实扣款验收，不主动提交有效采购或修改测试订单为已付款。

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

源码支持独立的 `plasma_usdt` 渠道；已有 BEP-20 继续使用 `bsc_usdt`。两张收款截图提供的地址相同：`0x66a8bab15067eab1efc948ac03684f0397c8416f`，但两条链的监听、金额标识和交易去重分别保存。配置示例默认关闭 Plasma，源码支持不表示线上已经启用。

Plasma 主网编号为 `9745`，USDT0 合约为 `0xb8ce59fc3717ada4c02eadf9682a9e934f625ebb`，精度为 6；BSC USDT 精度仍为 18。合约依据：[USDT0 官方部署表](https://docs.usdt0.to/technical-documentation/deployments)。2026-10-12 已对默认 Plasma RPC 只读核验链编号、代币精度、`finalized` 和日志接口；模拟链付款及交付测试不等于真实资金验收。

发布程序前先按下方发布规则备份。私有配置添加：

```yaml
plasma_usdt:
  enabled: false
  recipient: "0x66a8bab15067eab1efc948ac03684f0397c8416f"
  cny_per_usdt: "6.66"
  confirmations: 15
```

从生产服务器核验 RPC 可达、chain ID 为 9745、合约 `decimals()` 为 6、最终确认区块及交易收据接口可用后，再开启 `enabled` 并重启商城。默认节点是 `https://rpc.plasma.to`；自定义节点只写服务端 `PLASMA_RPC_URL`，需要分开查询收据时用 `PLASMA_RECEIPT_RPC_URL`。无需钱包私钥或助记词。

首次启用自动创建 `USDT · Plasma` 订单支付渠道；重启不会重新启用已禁用渠道或覆盖商户名称。沿用 storefront 模式即可启动独立 Plasma 监听。后台只读查询入口为 `/api/v1/admin/chain-payments/plasma-usdt/status` 和 `/transfers`，使用已有后台鉴权。核验支付页显示 Plasma、USDT0、正确地址及完整 6 位小数金额；两条链都要求完整金额和最终确认，异常金额或过期付款进入人工核查。

### 正式升级规则

GitHub `main` 为当前绿色商城源码主线，推送不会自动更新此服务器。升级遵循 [发布规则](../../docs/release-process.md)：先记录源码提交及构建哈希，备份线上库、配置和当前发布，再执行已授权的程序升级；保留 `shared/`、storefront 模式、支付采购队列和持久化 Redis。不得上传本地预览库覆盖生产订单。

历次部署细节已移入 [历史部署记录](../../docs/history/tencent-deployments-20261012.md)，其中旧 `api`/`checkout` 命令和本机私有包路径仅为历史资料。不要将通用上游安装器或旧初始化脚本用于覆盖正式环境。
