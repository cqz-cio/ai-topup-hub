# 本机真实主网支付验收

2026-09-30 已将本机商城切换为真实 BNB Smart Chain 主网收款。代码与节点能力验证通过，真实转账到账后自动发货的资金验收尚待买家实际转账。

## 当前入口

- 商城：http://127.0.0.1:5173/
- 真实支付验收商品：http://127.0.0.1:5173/products/mainnet-payment-verification
- 后台库存管理：http://127.0.0.1:5174/card-secrets

验收商品价格为人民币 0.10 元，使用实际 USDT 收款，交付一份唯一支付验收凭证。商品页面明确说明不包含 ChatGPT、邮箱账号或外部充值权益。原八款商品保留目录，自动库存为零；按对应商品和规格导入真实账号／卡密后即可销售。

这是本机运行环境，地址仅用于本机访问。对外营业还需按现有部署方式发布用户端和后台，并配置公网 HTTPS 域名与反向代理。

## 真实支付链路

1. 买家打开验收商品，自助下单，保存游客查单邮箱和订单密码。
2. 选择 `USDT · BNB Smart Chain (BEP-20)`，使用支付页的金额复制和地址复制按钮。按该笔订单完整的六位小数金额到账。
3. 正式后端持续扫描真实主网的指定 USDT 合约、收款地址和最终确认区块；至少 15 个区块确认后核验事件、成功收据及区块 hash。
4. 原支付服务更新订单，真实 Redis 队列驱动原发货 worker，扣减对应 SKU 库存并交付唯一凭证。
5. 支付页面自动转到订单详情，并显示交付内容。重复事件沿用原入账与交付记录。

收款地址：`0x66a8bab15067eab1efc948ac03684f0397c8416f`。使用当前支付页金额；早期模拟订单属于保留的测试数据库，不能使用它们的金额验收正式订单。二维码承载收款地址，转账金额按页面完整数字填写。

## 当前运行与数据

正式后端为 `work/codex-runtime/mainnet-server.exe`，由标准 `cmd/server` 构建，以 `-mode all` 运行。根目录 `config.yml` 已启用 `bsc_usdt`，主网 chain ID 56、USDT 精度 18、汇率 6.66、确认数 15。

区块及事件 RPC 为 `https://bsc-rpc.publicnode.com`，收据 RPC 为 `https://bsc-dataseed.bnbchain.org`。两端网络与 USDT 精度均核验，最终收据仍与事件及区块 hash 比对。公共节点有方法和限流限制；可通过服务器环境变量配置稳定的专用 RPC，详见 [支付说明](usdt-bep20-payments.md)。

- 正式 SQLite：`work/codex-runtime/live/shop.db`。
- Redis 8 在本机 Ubuntu WSL 中运行，Windows 通过 `127.0.0.1:6390` 访问；开启 AOF，每秒持久化，并禁止内存淘汰队列数据。
- Redis 文件：`work/codex-runtime/live/redis/`，包括 PID、日志、AOF 和 RDB。
- 后端运行 PID、日志及数据库位置：`work/codex-logs/mainnet-runtime.json`。
- 后台及用户端进程记录：`work/codex-logs/admin-preview.json`、`work/codex-logs/storefront-preview.json`。
- 后台登录信息保存在本机私有文件 `work/codex-runtime/live/credentials.json`。此文件同时含 Redis 密码，不提交或转发。

正式配置、数据库、凭据、软件包与运行日志均由 Git 和 Docker 构建上下文排除。早期模拟数据保留，模拟后端与内存 Redis 已停止。

本机启动辅助脚本位于忽略的 `work/codex-tools/`：先运行 `start-real-redis.sh` 对应的 WSL 启动，再运行 `start-mainnet-backend.ps1`；用户端与后台的启动脚本为 `start-preview.ps1`、`start-admin-preview.ps1`。这些辅助文件记录本次本机环境，线上部署使用现有项目构建与 `cmd/server -mode all`。

## 库存导入

后台登录后进入“卡密管理”，选择对应商品和 SKU，通过批量添加或 CSV 导入真实账号／卡密。商品交付方式须为自动发货；交付说明填写实际使用方式。库存为空时商城拒绝购买，不能用无限人工库存冒充自动库存。

后台支付与财务管理沿用仓库原有首次运营声明，需要商家本人阅读并确认。库存管理本身可访问；本次没有代填该声明。来源是 `frontend/admin/src/components/ComplianceAckDialog.vue` 以及后台 `PaymentComplianceRequired` 中间件。

## 验收证据和未完成项

- 真实主网、USDT 精度、最新／最终区块、目标地址日志查询和公开交易收据验证通过：`work/codex-logs/mainnet-probe.json`。
- 持久化监听游标在 13 秒内从 124853593 推进到 124853615，最新扫描时间有效；正式库无演示卡密：`work/codex-logs/mainnet-scanner-status.json`。
- Redis PONG、AOF 开启及写入正常，后台登录、商品管理通过：`work/codex-logs/live-runtime-status.json`。
- Chrome 桌面与手机实际剪贴板复制、真实支付单创建、刷新恢复、未付款不交付及零控制台错误通过：`work/codex-logs/storefront-qa/mainnet-payment-results.json`。
- 嵌入浏览器拒绝 Clipboard API 时，浏览器原生复制回退仍复制正确的完整金额；按钮特写为 `work/codex-logs/storefront-qa/mainnet-copy-buttons.png`，结果为 `mainnet-copy-final-results.json`。
- 现有后台 Chrome 登录、卡密管理页面及批量导入入口通过：`work/codex-logs/storefront-qa/mainnet-admin-results.json`。前端完整 80 项测试、TypeScript 与生产构建通过；依赖版本与锁文件未改动。
- 双 RPC 路由、错误收据网络、HTTPS、重定向、到账确认和付款发卡回归通过。

尚未观察到验收订单的真实 USDT 转账，也未导入真实账号／卡密库存。实际转账后需要核对交易 hash、入账金额、订单完成、对应库存扣减和交付内容；未取得这些证据前不声称资金验收完成。
