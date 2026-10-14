# TRON USDT 收款

新增独立的 `tronusdt` / `usdt-trc20` 原生收款渠道，支持商品订单、游客订单和钱包充值。收银页沿用现有组件，显示 TRON（TRC20）、USDT、收款地址二维码、可复制的完整应付数量及付款期限。

## 地址与代币

- 收款地址为运营者提供的 TRON 主网 Base58Check 地址，区分大小写，程序启动时校验校验和；不配置钱包私钥或助记词。
- 只识别 Tether 官方 USDT 合约 `TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t`，精度固定为 6。其他同名代币、stUSDT 和 TRX 不计为 USDT 付款。
- 内部按 TRON 主网身份、官方合约、收款地址隔离记录，既有 BEP20 和 Plasma 记录保持独立。
- 金额沿用[两位 USDT 数量规则](usdt-payment-amounts.md)：按锁定汇率向上取两位，以 0.01 为步长避开历史已分配金额，最多增加 0.99。历史金额不回收；到账核对仍精确到六位原始整数数量。

## 后端配置

`config.yml` 中的 `tron_usdt` 使用 `enabled`、`recipient`、`cny_per_usdt`、`confirmations`。示例默认关闭且不填写实际收款地址。启用后自动创建一次订单及钱包渠道，重启不覆盖后台已修改的名称或启停状态。

默认 HTTPS 节点为 `https://tron-rpc.publicnode.com`。`TRON_RPC_URL` 可指定其他支持标准 FullNode / SolidityNode HTTP API 的 HTTPS 节点；TronGrid 密钥仅通过服务端 `TRON_API_KEY` 环境变量设置。接口错误不返回节点 URL 或密钥。选用的节点必须能返回从持久化游标开始的完整区块和回执；节点历史数据不足时停止推进游标，需要换用具备该段历史数据的节点，不能手动跳过区块。

监听固定检查主网创世块及 USDT 精度，按顺序读取已固化区块和该块全部交易回执。回执交易集合必须与区块交易集合相等；失败、缺失、重复、错误区块、错误时间或不完整响应均不能跳过。只接收成功执行的官方合约 Transfer 事件，再独立读取回执核对收款人、数量及区块归属。每次最多处理 10 个区块，进度保存在数据库，重启可恢复。

达到配置确认数并且区块已固化后，按准确数量和支付期限匹配订单，调用已有支付成功处理。以主网身份、交易哈希和事件序号去重；同一交易的多个转账事件分别处理。少付、多付、迟到和重复付款保留人工核查记录。支付成功后进入现有发货或钱包入账流程，不持有钱包签名权限。

后台只读入口为 `/api/v1/admin/chain-payments/tron-usdt/status` 与 `/transfers`，沿用已有后台鉴权、财务权限和声明要求。

## 验证边界

自动测试覆盖主网和合约身份、地址校验、完整回执、错误币种、确认等待、精确金额、去重、商品发货、钱包入账与模拟供应商采购。可通过 `TRON_READONLY_PROBE=1` 定向运行 `TestReadOnlyMainnetProtocol`，只读验证公共主网的历史交易回执；不下单、不广播交易、不付款或采购。

模拟联调与公共回执查询不代表经营者收款地址已完成真实资金验收。发布信息与本地、线上核验结果以发布记录为准。

依据：[Tether 官方合约](https://tether.to/en/supported-protocols/)、[TRON 钱包接入](https://developers.tron.network/docs/exchangewallet-integrate-with-the-tron-network)、[PublicNode TRON](https://tron.publicnode.com/)。
