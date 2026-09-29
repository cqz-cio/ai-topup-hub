# AICDK 自动充值后端接入

> 此文保留早期直接充值方案作为开发记录。当前的发卡、兑换确认、接口和启用步骤，以 [卡密兑换业务说明](redemption-flow.md) 为准；下文旧 `/session` 写入入口已撤下，旧的付款直接充值流程不再启用。

此模块连接本地已支付订单和 AICDK 的订阅充值任务。当前已实现后端流程，尚未接真实卡台，也没有新增前端页面。默认关闭；未供卡时不会发起充值。

## 已确认的兑换业务流程

最终业务流程为：客户购买并收到兑换卡密 → 在兑换站核验卡密 → 提交授权 Session → 查询订阅并核验套餐适用条件 → 客户确认账号与充值 → 锁定卡密及账号 → 取卡并调用 AICDK 充值 → 确认到账并核销卡密。

卡网发出兑换卡密即完成发货，兑换进度独立管理。客户明确确认前不绑卡、不支付订阅费用；查询成功不代表保证可充值成功。确认后同一卡密只能关联一个执行中的充值任务，结果不明时不得释放卡密或换任务重复扣款。

以上为已确认的目标流程。当前付款通知直接创建充值任务的代码还未替换；兑换权益存储、发卡、核验与客户确认入口尚待实现。保持功能关闭，不能把现有接口直接作为完整兑换业务上线。

### 固定兑换卡密格式

- 固定 25 位随机字符，5 组、每组 5 位，组间使用 ASCII `-`，展示长度 29 位。
- 字符集为大写英文字母和数字，排除 `I`、`O`、`0`、`1`；每张卡密整体必须同时包含字母和数字。
- 示例：`K7M2P-R8W4X-6NQ9T-H3V5C-Y2D8F`（仅展示，不是可用卡密）。
- 输入兼容小写、首尾空白及完全省略分隔符；混用或错位的分隔符、内部空白及其他字符不接受。
- 使用密码学安全随机源，不拼接订单号、时间或流水号，随机源失败时停止生成。
- 格式生成与标准化已实现于 `internal/modules/autorecharge/domain/redemption_code.go`，尚未接入发卡流程。格式正确仅代表语法有效，不代表卡密存在或具备兑换权益。
- 后续发卡持久化必须保证唯一性；查询按标准化后的值处理，最终以数据库中的套餐、有效期和核销状态决定能否兑换。

## 当前充值执行模块范围

- 支持 Plus / Pro 5x / Pro 20x 新开通，对应 `/api/partner/v1/pay`。
- 沿用本地商品、收款、订单和交付记录；不新增一种支付收款渠道。
- 第一阶段使用主站 `manual` 交付商品，一个叶子订单仅含一个 SKU、数量为 1。不要用普通商品表单收集 Session 或卡信息。
- 已支付订单通知创建任务；数据库定时扫描补偿漏通知和服务重启，不依赖 Redis 执行充值任务。
- 可用积分和地区版本预检查、固定幂等 UUID、字符串 operation_id、任务轮询、银行验证续接、订阅确认和本地幂等交付。
- API Key 只通过服务端环境变量读取。Session 和银行验证快照使用 `app.secret_key` 加密，终态清除；卡号/CVC 不落数据库、不返回客户端、不写日志。
- 不包含升级、GPT Credits、自动退款、卡台实装或银行验证 UI。充值费用与本地收款分别结算，API 积分不等于订阅费用。

文档依据：https://www.aicdkshop.com/api-docs ，机器定义：https://www.aicdkshop.com/api/partner-openapi.json 。

## 配置

1. 创建一个人工交付商品，记录 SKU ID，设置单次购买数量为 1。先在测试环境配置。
2. 设置至少 32 字符的随机 `app.secret_key`。在已有环境中不要随意更换此密钥。
3. 在用户中心申请 AICDK API 权限，取得密钥，通过部署环境变量 `AICDK_API_KEY` 提供。不要提交真实密钥到 Git。
4. 读取 `/api/config` 当前地区、套餐、通道和版本；以下版本数字只是示例，不能作为永久配置。

```yaml
auto_recharge:
  enabled: true
  bindings:
    - sku_id: 123
      plan: chatgptplusplan
      region: US
      region_version: 12
      channel: "3"
      card_rule: plus-us
      cancel_after_success: true
```

配置启用时会检查 SKU、套餐、地区、通道、供卡规则名称和加密密钥。`card_rule` 是留给将来卡台的规则标识，当前不代表已有卡台规则引擎。`cancel_after_success` 为充值后关闭自动续费的显式业务选项，由商家配置；不要默认解释为解绑或销卡。

运行现有服务的 `all` 模式，或同时运行 `api` 和 `worker` 模式。独立的 auto-recharge worker 每 10 秒扫描一次，任务通常间隔 30 秒查询。可以有多个 worker，数据库租约确保同一任务同一时刻只有一个执行者。应用启动时自动迁移 `auto_recharge_tasks` 表。

真实卡台未接入时，启用后已支付订单停在 `waiting_card`。没有 AICDK 密钥也不会扣款。此时可以使用模拟测试核对其余流程。

## HTTP 接口

接口前缀 `/api/v1`，沿用现有 JWT 身份验证。

| 方法 / 路径 | 用途 |
| --- | --- |
| `GET /orders/:order_no/auto-recharge` | 登录客户按订单号查询自己的充值任务 |
| `POST /orders/:order_no/auto-recharge/session` | 登录客户按订单号提交自己订单的授权 Session |
| `GET /admin/orders/:id/auto-recharge` | 管理员查询任务 |
| `POST /admin/orders/:id/auto-recharge/session` | 管理员代录授权资料，适用于游客订单 |
| `POST /admin/orders/:id/auto-recharge/advance` | 推进一次原任务；不会重新创建不确定的支付 |
| `GET /admin/orders/:id/auto-recharge/verification` | 向持卡商家返回银行验证所需数据，禁止公开 |
| `POST /admin/orders/:id/auto-recharge/finalize` | 核实银行验证结果并续接原任务 |

管理员接口同时受 JWT、RBAC 和现有支付声明中间件保护。新增 `recharge_operator` 内置角色可操作这些接口；普通用户不能读取商家银行卡验证凭据。

Session 提交请求（占位示例）：

```json
{"authorized":true,"session":"{\"accessToken\":\"<AUTHORIZED_ACCESS_TOKEN>\",\"sessionToken\":\"<AUTHORIZED_SESSION_TOKEN>\"}"}
```

必须是完整 Session 的 JSON 字符串。请求仅在尚未提交充值时允许更新；发起之后参数被冻结。前端只能在确认用户授权后提交，不得把 Session 写入普通订单表单、URL、浏览器本地存储或统计埋点。

银行验证由持卡商家完成。前端使用对方返回的 Stripe 公钥及 client_secret；`verification_stage=bind` 使用 SetupIntent 绑卡验证，其余使用支付验证。浏览器回传仅接受：

```json
{"authentication_failed":false}
```

后端从加密快照取原账号、Checkout、Intent 和套餐，不信任浏览器上传的替换值。不因为浏览器返回成功就交付，仍以原任务服务端结果为准。验证 UI 尚未实现，后续需先确认界面方案。

## 状态和恢复

| 状态 | 行为 |
| --- | --- |
| `waiting_card` | 等待接入卡台或规则供卡 |
| `waiting_session` | 等待客户授权资料 |
| `waiting_configuration` | 检查 API 密钥、积分及地区配置 |
| `submitting` | 已持久化提交标记；此时发生崩溃将进入待核实，禁止自动重付 |
| `queued / running / stopping` | 查询同一个 operation_id |
| `action_required` | 等待商家完成银行验证，不能按失败重付 |
| `unknown` | 结果不明；有 operation_id 时查原任务，无编号时人工联系服务方核实 |
| `confirming_subscription` | 付款成功，继续核实订阅同步，不再调用充值 |
| `succeeded` | 已确认成功，重试本地交付，不再付款 |
| `completed` | 本地交付成功，清除临时凭据 |
| `failed / canceled / manual_review` | 停止自动执行；不自动换卡、创建新支付或退款 |

对方不发送 Webhook。HTTP 200、`pending=false`、积分预占都不单独代表充值完成。订阅查询若缺失套餐或有效期证明，保持待核实，不推断已到账。

本地退款或关闭订单无法撤销服务方已经受理的充值。此时继续查询原任务，停止本地交付和银行验证续接，取得最终结果后转人工核对；不会自动退款或重新支付。

若修改地区配置导致已有任务快照过期，不自动改变原任务参数。尚未受理的订单需要管理员核实配置后处理；已有任务永远按原任务继续。当前不提供删除重建或强制重付接口。

## 将来接入卡台的位置

实现 `internal/modules/autorecharge/contract.Cards`，在 `internal/app/container/autorecharge.go` 的 `Options.Cards` 注入真实适配器：

- `Reserve(ctx, taskKey, rule)`：按规则占用卡片；同一个任务 UUID 必须始终返回同一个卡引用，不能换卡。
- `Credentials(ctx, reference)`：在实际发送请求前临时提供卡号、有效期和 CVC；失败进入等待供卡。
- 持久化记录只保存不透明卡引用，不保存卡号/CVC。需要卡台保证引用稳定、供卡幂等；确认卡台协议后再增加额度与限次管理。

缺省实现 `NoCards` 只返回“未配置”。测试中的模拟卡台和模拟服务位于测试文件，不会注入生产运行。

## 验证

```sh
go test -count=1 -timeout=90s ./internal/modules/autorecharge/...
```

覆盖无卡暂停、重复付款通知、模糊网络失败不重付、崩溃恢复、账号归属、加密与脱敏、银行验证原任务续接、订阅同步、数据库租约竞争和 HTTP 重定向保护。真实账号、真实卡台及实际扣款仍需上线前单独联调。
