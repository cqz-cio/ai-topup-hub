# 支付宝 AI 网页应用收款接入

## 当前结果（2026-10-12）

开发草稿位于独立分支 `codex/alipay-sandbox`。本次没有合入主线、修改线上渠道或部署。

- 已通过官方 npm 包安装 `alipay-aipay` Skill：安装包版本 1.7.3，实际 Skill 版本 1.7.0。
- Skill 的 `resolve-project` 不识别本项目的 `go.mod`，因此其完整自动流程停在项目校验。没有用前端目录冒充服务端项目，也没有将后端迁移到其他语言。
- 按官方文档复用 Go 中现有的 `alipay.trade.page.pay` 和异步通知验签，新增 `alipay.trade.query` 补偿查询。
- 查询使用支付记录的 `GatewayOrderNo`，校验支付宝对原始响应 JSON 的签名和返回订单号，再由原有支付事务校验金额、币种并保持幂等。网页轮询可查询官方支付宝付款状态。
- 本地模拟网关、签名校验、支付服务临时数据库和沙箱工具测试已通过。1160 元案例仅证明本地代码没有 1000 元硬限制，不证明商户额度或真实资金收款能力。
- 已在官方沙箱完成 0.01 元虚拟付款：官方收银台显示付款成功，服务端查询返回 `TRADE_SUCCESS`，响应签名、商户订单号和金额均核验通过。独立工具未加载商城数据库或采购服务。
- **公网异步通知联调、商户签约和正式支付验收尚未完成。** 不应启用线上支付宝自动采购或称为生产就绪。

## 官方沙箱测试

独立工具 `cmd/alipay-sandbox` 不加载商城数据库、队列或采购服务，只允许当前官方沙箱网关。其测试金额限制为 0.01 至 1.00 元，这是工具的测试保护，不是支付宝收款限额。

1. 最简单的方式：在支付宝“技术集成 → 代码接入”中点击“配置沙箱密钥”区域的复制按钮，将整段原文保存为忽略目录中的 `work/alipay-sandbox.private.env`；工具可直接读取，无需整理 JSON。也支持将 `docs/alipay-sandbox.example.json` 复制为私有 JSON 后填写。空字段必须填入当前真实沙箱配置，不能用生产密钥。
2. 在支付宝技术集成页填入对应值：`AIPAY_APP_ID` → `app_id`；非 Java 沙箱私钥 `AIPAY_PRIVATE_PKCS_KEY` → `private_key`；`AIPAY_ALIPAY_PUBLIC_KEY` → `alipay_public_key`。最后一项是**支付宝公钥**，不是应用公钥。私有文件不得提交、发送到聊天或写入日志。Windows 下限制文件 ACL 为当前用户，Linux 下使用 0600。
3. 在项目根目录构建 `go build -o work/alipay-sandbox.exe ./cmd/alipay-sandbox`，构建进程由调用者设置不超过 120 秒的超时。
4. 运行 `work/alipay-sandbox.exe -config work/alipay-sandbox.private.env -action create`。命令只准备请求，返回订单号及本机 HTML 路径，不代表支付宝已创建订单或已经付款。打开 HTML 的链接进入官方沙箱收银台，用官方提供的沙箱买家账号及支付密码完成虚拟支付，也可使用沙箱 APP。
5. 运行 `work/alipay-sandbox.exe -config work/alipay-sandbox.private.env -action query -order ALIPAY-SANDBOX-这里替换为上一步订单号`。只有验签、订单号、金额和成功状态全部通过，才返回 `sandbox_payment_verified: true`。查询请求有 20 秒超时，未支付会返回非零退出码。

沙箱允许暂缺 `notify_url` 和 `return_url`，正式网关仍保持原有必填校验。上述独立工具没有验证公网通知，输出始终保留 `public_notify_tested: false`。正式接入前必须验证真实 HTTPS 通知入口、验签、金额及归属校验、重复通知只履约一次和 `success` 应答。浏览器查询只在付款页打开时触发，不能替代异步通知。

## 正式渠道与入驻

后台现有支付渠道选择“官方 / 支付宝”，交互方式选择 `page`。AI 网页应用收款的 PC 和 H5 都使用 `alipay.trade.page.pay`；现有 `qr` 对应当面付、`wap` 对应另一支付接口，不能用它们替代本次产品签约。

正式应用与沙箱的 app_id、密钥和网关必须隔离。正式密钥应由账户持有人在本机生成及配置，不在聊天提供。正式回调为实际域名下的 `/api/v1/payments/callback`，返回地址为商城支付页。不要将沙箱配置写入生产数据库。

当前账户申请表的已上线网站分支要求可访问且已完成 ICP 备案的链接。海外服务器的托管备案条件与支付宝产品准入条件是两件事。页面另有未上线分支，可提供首页、商品/服务页及支付页截图，并明确限制额度和有效期；现有已上线商城应如实申报。

当前默认“杂货店”类目不是本项目已确认的经营类目，不应直接提交。实际业务为第三方 AI 会员卡密/充值，能否以个人无营业执照主体入驻、需要哪些授权或资质，以及最终额度，仍须支付宝按真实业务审核。页面展示默认类目费率 0.6%，有效期至 2027-03-31，不能作为真实类目已签约的报价或批准凭据。

没有核实“自动支付统一限额 1000 元”的依据。具体限额应以签约产品和账户审批结果为准；API 金额字段的取值范围不能当作账户实际额度。

## 依据及本地验证

- [支付宝 AI 网页应用收款接入指南](https://aipay.alipay.com/docs/vibe-pay/ai-web-app-payment-qianyi/ai-web-app-payment-integration-guide.html)
- [交易查询接口](https://aipay.alipay.com/docs/vibe-pay/ai-web-app-payment-qianyi/api-list/alipay-trade-query.html)
- [异步通知说明](https://aipay.alipay.com/docs/vibe-pay/ai-web-app-payment-qianyi/api-list/async-notify-verify.html)
- [支付宝产品开通页](https://aipay.alipay.com/open-flow/product?accessMode=agent&product=AI_WEB_APP_PAY)
- [支付产品额度规则](https://help.alipay.com/enterprise/knowledgeDetail.htm?knowledgeId=201602191944)
- [腾讯云境外服务器备案说明](https://cloud.tencent.com.cn/document/faq/243/43878)

本次通过的命令：

```text
go test -count=1 -timeout=90s ./internal/modules/payment/infrastructure/gateway/alipay ./internal/modules/payment/infrastructure/gateway/adapters/alipay ./internal/modules/payment/application ./cmd/alipay-sandbox
```

自动测试使用本地 HTTP 模拟网关和内存数据库，覆盖未付款、签名被篡改、订单号错误、金额错误、已付款重复查询及支付时间时区。

2026-10-12 另完成官方沙箱网络联调：先查询到已验签的 `WAIT_BUYER_PAY`，在官方沙箱收银台使用测试买家支付 0.01 元后，再查询到已验签的 `TRADE_SUCCESS`。脱敏结果保存在本机忽略目录 `work/alipay-sandbox/verified-result.json`。本次没有验证公网通知，也没有真实收款或真实采购。
