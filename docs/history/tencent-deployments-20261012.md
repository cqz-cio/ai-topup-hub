> 历史归档（2026-10-12）：本文保留当时状态、命令与验收记录，不作为当前操作指南。现行说明见 [项目入口](../../README.md)、[本地开发](../local-development.md) 和 [生产环境](../../ops/tencent/README.md)。余额、库存和私有文件路径可能已经变化。

# 腾讯云商城运行说明

2026-10-12 已将当前 Go + Vue 全栈商城部署到腾讯云轻量应用服务器 `43.134.141.10`。前台 [http://43.134.141.10/](http://43.134.141.10/)；后台路径 `/zxa-management/`。当前通过 IP 的 HTTP 入口访问，尚未绑定域名或配置 HTTPS。

发布 `20261012-001724` 使用 Linux amd64、`CGO_ENABLED=0`、`release,fullstack` 标签，内嵌用户端和后台构建资源。后台构建使用 `VITE_FULLSTACK=1`；所有 18 张商品/分类图片一起内嵌。运行时无需 Docker、nginx 或 Node。

| 用途 | 位置 |
| --- | --- |
| 当前发布 | `/srv/zhixiang/current` → `releases/20261012-013649` |
| 配置 | `/srv/zhixiang/shared/config.yml` |
| 数据库 | `/srv/zhixiang/shared/db/shop.db` |
| 上传和日志 | `/srv/zhixiang/shared/uploads`、`logs` |
| 库存报告 | `/srv/zhixiang/shared/reports/merchant1039-stock-sync.json` |
| 初始备份 | `/srv/zhixiang/backups/initial-20261012-001724` |

`zhixiang-shop.service` 以系统用户 `zhixiang` 运行，使用 `-mode storefront`，监听 80 端口并开机自启。该模式运行商城 HTTP、BEP20 收款监听、订单维护、客户付款后的采购及卡密交付 worker，以及每分钟采购恢复任务。`zhixiang-redis.service` 提供绑定 `127.0.0.1:16379` 的专用持久化订单队列；Redis 配置在 `shared/redis/redis.conf`，权限 0600，启用 AOF everysec、noeviction、256MB 上限。`zhixiang-stock-sync.timer` 每次同步结束后 300 秒启动 `zhixiang-stock-sync.service`；相关单元源文件保存在本目录。

用户于 2026-10-12 在启用 BEP20 支付后进一步授权客户付款后从商户 1039 自动采购并发卡，且不要实际扣款测试。当前环境 `UPSTREAM_PURCHASE_ENABLED=true`，采购按已绑定商品及 SKU 和订单成本上限执行；卡密保存后完成订单并展示在订单详情。余额不足会保留已付款待发货订单并记录采购异常，商户当前余额 0.00 元，正常采购前需补足余额。自动充值仍关闭，不注册库存轮询或采购对账。定时器使用 [库存脚本](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/scripts/sync-ellaixshop-stock.py)，只访问后台登录、连接/余额查询、映射同步和公开商品接口。不得为验收主动提交真实采购或将测试订单改为已付款。

商户连接 ID 1，协议 `acg-faka`，商户 ID `1039`，`auto_sync_price=false`。数据库中保存加密商户密钥，配置保留原应用加密密钥。库存助手的后台凭据在 `shared/stock-sync.credentials.json`，仅通过服务器回环地址调用后台；配置、凭据和数据库权限为 0600。不要输出密码、商户密钥、JWT 或整个配置文件。

全部 6 个分类、16 款商品售价已保留，`qzweb-55` 为 1160.00 元；12 款有供货映射，其中 11 款有货，`qzweb-59` 库存为 0。`qzweb-52/53/62/1` 未在该商户接口中提供，仍显示缺货。实际库存取商户接口，库存同步不会覆盖本站售价。

服务状态及只查询库存的维护命令：

```sh
sudo systemctl status zhixiang-shop.service zhixiang-stock-sync.timer --no-pager
sudo systemctl status zhixiang-redis.service --no-pager
sudo systemctl list-timers zhixiang-stock-sync.timer --no-pager
sudo systemctl start zhixiang-stock-sync.service
sudo systemctl restart zhixiang-shop.service
```

应用日志位于 `shared/logs`，同步任务日志可用 `journalctl -u zhixiang-stock-sync.service` 查看。查询日志时仍须避免将凭据或客户卡密输出到聊天。

本次公网首页、健康接口、前后台入口资源、6 个分类/16 款商品、12 个商品封面/6 个分类图标均通过验证，产品和 SKU 售价一致；服务重启后库存同步成功，商户余额前后均为 0.00 元。部署快照和上线后的历史订单、支付、用户、卡密及采购表摘要一致，没有实际采购或付款。

本地发布文件及一致数据库备份在项目根目录被 Git 忽略的 `deployment/local/tencent-release/20261012-001724/`，验证报告为 `deployment/local/tencent-deployment-verification.json` 和 `tencent-restart-verification.json`。后续升级须先使用 SQLite backup 备份线上库、配置和现有发布，再切换版本；不要用本机预览库覆盖线上新订单。初始部署脚本会拒绝覆盖已有服务器数据库。

2026-10-12 已升级程序到 `20261012-003834`，前台首次访问默认简体中文，不再随浏览器语言切换；用户手动语言选择继续保留。此次升级未上传本机数据库，切换前的线上数据库、配置和库存助手凭据备份在 `/srv/zhixiang/backups/before-20261012-003834`，原发布仍保留用于回退。前后台程序、价格及历史业务记录核验通过，库存同步和采购禁用设置保持有效。

英文及繁体浏览器首次访问、手机显示、手动切换和刷新记忆均通过实际浏览器验证。本次私有发布包及不可变资源快照位于项目根目录 `deployment/local/tencent-release/20261012-003834/`，升级报告为 `deployment/local/tencent-application-upgrade.json`，浏览器报告为 `deployment/local/chinese-default-browser-verification.json`。可复用程序升级脚本为 `deployment/local/upgrade-tencent-application.py`，仅打包程序与发布清单，保留服务器持久化库与配置；执行前须准备新发布并核验现有版本。

2026-10-12 已将商品界面显示修复发布为 `20261012-004733`：封面按原图比例完整显示，标题与库存文字可换行，平板导航改为展开菜单，手机窄屏商品单列显示，页脚避让底部导航。Vue 类型检查和生产构建通过，公网在 1920、1280、1024、768、390、320px 六种宽度验证商品、搜索、分类、导航及页脚通过，无文字截断或脚本错误；英文布局也已在本机核验。

本次保留后台资源和线上数据库，只切换程序，旧版本及切换前备份 `/srv/zhixiang/backups/before-20261012-004733` 保留。16 款商品及 SKU 售价和历史业务表摘要一致，1160.00 元调价保留；商户 1039 库存同步成功，余额前后均为 0.00 元，采购接口未调用，采购禁用开关有效。当前不可变资源快照位于 `deployment/local/tencent-release/20261012-004733/`，公网浏览器报告位于 `remote-latest/work/codex-runtime/storefront-layout-qa/live-layout-report.json`，均为被 Git 忽略的本地运行资料。

2026-10-12 已发布 USDT/BEP20 支付版本 `20261012-005645`，沿用原收款地址 `0x66a8bab15067eab1efc948ac03684f0397c8416f`、报价 `1 USDT = 6.66 CNY`、15 个区块确认及 finalized 校验。支付单固定六位金额，收银台显示二维码、完整收款地址、金额复制和到账轮询；详见 [支付说明](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/docs/usdt-bep20-payments.md)。只创建未付款验收订单，没有转账或采购，不能将节点连通和模拟回归当作真实资金验收。

该支付版本通过 drop-in `zhixiang-shop.service.d/30-checkout.conf` 运行；原版本与升级前备份 `/srv/zhixiang/backups/before-20261012-005645` 保留。原有历史记录与商品/SKU 售价未改变，新增的未付款验收订单按正常超时流程保留。程序重启后金额不变、监听游标继续推进、Redis 和库存定时器 active/enabled 已验证。服务器报告在 `shared/reports/bep20-payment-upgrade.json` 和 `bep20-runtime-verification.json`；私有发布包和浏览器报告在被 Git 忽略的 `deployment/local/`。

自动采购发卡版本 `20261012-013649` 使用 `40-storefront.conf` 覆盖上述 checkout 模式和采购禁用环境；备份在 `/srv/zhixiang/backups/before-20261012-013649`。每分钟恢复漏采购/漏入队/已接受但未交付的订单，唯一采购索引和原子提交保护重复扣款，未知采购结果必须人工核查，空卡密保持未交付。付款、采购、发卡、父子订单和恢复模拟回归通过；线上 worker、恢复调度和监听游标均已验证，历史业务记录、价格、支付参数及 Redis 密码未改变，无实际扣款测试。报告在 `shared/reports/auto-procurement-upgrade.json` 和 `auto-procurement-runtime-verification.json`。后续升级须保留 storefront 模式、支付与采购队列以及持久化 Redis；私有脚本 `deployment/local/deploy-auto-procurement.py` 支持从已核对的 checkout 或 storefront 版本升级，后续升级仍应先核对新状态，不能直接套用旧 api 或 checkout 初始化脚本。
