# 卡密兑换业务说明

## 当前实现

商城收款已增加 [USDT / BEP-20 链上监听](usdt-bep20-payments.md)，通过原支付服务确认后进入下述发卡流程；上线启用和真实资金验收仍需完成。

客户付款 → worker 生成并交付固定格式卡密及兑换地址 → 客户核验卡密 → 提交授权 Session 并查询账号 → 客户确认 → 锁定卡密、账号和套餐 → 卡台供卡 → AICDK 绑卡并付款 → 查询结果并确认到账 → 核销卡密。

卡网在发出卡密后完成订单交付，兑换进度单独记录。付款、发码、账号查询均不会触发充值；只有客户确认后才允许执行。重复通知、重复点击及刷新均沿用原任务。

后端和 A 方案客户兑换页面均已实现，页面路由为 `/redeem`，可用于登录客户及游客。功能默认关闭，尚未部署；真实卡台和商家银行验证页面仍未接入。浏览器使用模拟接口验证流程，没有真实密钥或银行卡参与测试。

## 客户页面

`frontend/user/src/views/Redeem.vue` 使用已确认的 A 方案：顶栏、四步引导、主操作区及右侧兑换凭证，手机端自动改为单栏。输入卡密后可开始新兑换或查询历史进度。核验账号需要勾选授权；确认页单独要求勾选目标账号确认；已有订阅、凭据缺失、核验过期均不能提交。

Session 输入框在核验请求结束后清空，卡密与确认令牌只保存在当前页面内存，不写入 URL、localStorage 或 sessionStorage。刷新页面后重新输入原卡密查询；刷新不会自动再提交充值。提交响应丢失时只查询原记录。处理中通常每 10 秒查询一次，网络异常降为 30 秒，页面隐藏时暂停，离开页面取消请求和计时器。

兑换页使用独立页面外壳，禁用站点自定义脚本注入；从商城进入及返回商城使用完整页面导航，防止之前运行的自定义脚本继续监听授权输入。不会向此公开兑换 API 发送商城登录令牌或 Cookie。

配置 `redeem_url` 为部署后的实际地址，例如 `https://你的商城域名/redeem`。如果部署在独立域名，仍需把该站的 `/api/v1` 反向代理至本后端，或在构建时设置 `VITE_API_BASE_URL` 并配置后端允许的跨域来源。静态服务器需要将 `/redeem` 回退到用户前端的 `index.html`。

前端验证：在 `frontend/user` 运行 `pnpm test`、`pnpm build`。已使用浏览器模拟核验、独立确认、进度恢复、到账、已有订阅拒绝、核验过期、断网响应核实和手机布局；模拟通过不等同于真实充值通过。

## 商品与卡密

卡密是一份已付款的套餐权益凭据。后台在付款成功后按订单 SKU 的商家配置生成卡密，并保存订单、套餐、地区、充值通道和供卡规则快照。卡密字符本身不编码套餐；客户端不能选择或改写套餐。已发卡密不随之后的 SKU 配置修改而变更。

Plus、Pro 5x、Pro 20x 分别使用 `chatgptplusplan`、`chatgptprolite`、`chatgptpro`。当前适配器使用同一充值接口，通过服务端保存的 `plan` 参数区分套餐，供卡规则也取自该卡密记录。

客户验码后先看到绑定套餐；核验账号后同时显示服务方检测到的当前订阅及本次充值套餐。最终确认勾选明确包含账号和所购套餐。已有订阅时显示检测结果并阻止新开通，不自动升级或切换套餐。

- 主站 `manual` 交付商品，每个叶子订单一个 SKU、数量 1；支持登录和游客订单。混合商品转人工处理。
- 支持 Plus / Pro 5x / Pro 20x 新订阅。仅放行服务方明确返回 `free` 的账号；不实现已有订阅升级、续费、GPT Credits 或自动退款。
- 卡密固定为 `XXXXX-XXXXX-XXXXX-XXXXX-XXXXX`，25 位安全随机字母数字，排除 `I/O/0/1`，有数据库唯一约束。
- 输入兼容大小写、首尾空白及完全省略分隔符；格式有效不代表有兑换权益。
- 卡密目前没有使用期限；原订单退款、关闭或删除后不能继续兑换。失败或待核实卡密不会自动恢复可用。
- 卡密是持有者凭据，不要求登录原购买者账号；登录和游客订单都通过原有订单查询权限领取卡密。

任务表保存卡密摘要和加密副本；现有交付记录保存明文卡密供购买者查看。Session 和账号快照加密存储，未确认的核验 10 分钟过期，worker 清理凭据，终态清理 Session。结果未知时保留恢复所需资料。卡号/CVC 不落库。开发与发布模式的 SQL 日志均隐藏参数。

## 配置与运行

创建人工交付商品并取得 SKU ID；配置至少 32 字符的 `app.secret_key`，已有环境不要随意换密钥。密钥通过服务端环境变量 `AICDK_API_KEY` 注入。前端实际上线后填写 HTTPS 兑换页地址。

```yaml
auto_recharge:
  enabled: false
  redeem_url: "https://redeem.example.com/redeem"
  bindings:
    - sku_id: 123
      plan: chatgptplusplan
      region: US
      region_version: 12 # 仅示例；从 /api/config 获取当前版本
      channel: "3"
      card_rule: plus-us
      cancel_after_success: true
```

`card_rule` 是未来卡台规则名。`cancel_after_success` 表示充值后关闭自动续费，不表示解绑或销卡。API 服务积分与订阅银行卡付款分开结算。

使用 `all` 模式，或同时运行 `api` 与 `worker`。worker 每 10 秒扫描，执行中任务通常间隔 30 秒推进；漏付款通知可从数据库恢复。数据库租约协调多个 worker；同一账号的未结束充值用唯一约束互斥。

公开接口每 IP 每分钟 30 次，Redis 不可用时沿用进程内限流。多实例需共享 Redis 保证全局计数。

## 客户兑换 API

前缀 `/api/v1/recharge/redemptions`，全部为 POST JSON，以卡密授权、支持游客。响应 `Cache-Control: no-store`；卡密、Session、确认令牌不能出现在 URL、埋点、访问日志或浏览器持久存储中。

| 路径 | 请求字段 | 行为 |
| --- | --- | --- |
| `/lookup` | `code` | 验证卡密及查询套餐、状态，也用于进度轮询 |
| `/account` | `code`, `session`, `authorized: true` | 核验账号及订阅，返回短期确认令牌 |
| `/confirm` | `code`, `confirmation_token`, `confirmed: true` | 客户确认，排队充值；不会在 HTTP 请求内扣款 |

`session` 必须是包含 `accessToken`、`sessionToken` 的完整 Session JSON 再序列化为字符串。只接受客户已授权的账号。

```json
{"code":"<REDEMPTION_CODE>","session":"{\"accessToken\":\"<AUTHORIZED_TOKEN>\",\"sessionToken\":\"<AUTHORIZED_SESSION>\"}","authorized":true}
```

允许充值的核验响应：

```json
{"data":{"state":"awaiting_confirmation","plan":"chatgptplusplan","account":{"account_id":"<PROVIDER_ACCOUNT_ID>","current_plan":"free"},"confirmation_token":"<SHORT_LIVED_TOKEN>","checked_until":"<UTC_DATETIME>"}}
```

AICDK 必须返回 `account_id` 和 `subscription.plan_type`；文档未保证每次返回邮箱，因此当前展示服务端确认的账号标识，不把客户 Session 自述的邮箱当作已验证身份。缺字段、查询未完成、请求失败均不放行；`subscription: null` 不解释为免费账号。真实联调必须确认返回结构。

已有订阅返回 `existing_subscription_not_supported`，无确认令牌。重新提交 Session 会先使旧令牌失效，即使新核验失败。令牌有效期 10 分钟，过期必须重查。

```json
{"code":"<REDEMPTION_CODE>","confirmation_token":"<SHORT_LIVED_TOKEN>","confirmed":true}
```

确认后冻结原 Session、账号及套餐。重复确认返回原任务状态；刷新后可以凭卡密查询进度。执行付款前再核实账号未变且仍适用，失败转人工，不扣款。

## 商家操作 API

前缀 `/api/v1/admin`，受 JWT、RBAC 和现有支付声明中间件保护。`recharge_operator` 角色可操作：

| 方法 / 路径 | 用途 |
| --- | --- |
| `GET /orders/:id/auto-recharge` | 状态及对账所需 operation_id、幂等标识、积分状态 |
| `POST /orders/:id/auto-recharge/advance` | 推进原任务，不能跳过客户确认 |
| `GET /orders/:id/auto-recharge/verification` | 商家取得银行验证公钥和 client_secret |
| `POST /orders/:id/auto-recharge/finalize` | 完成银行验证后续接原任务 |

登录客户仍能通过 `GET /api/v1/orders/:order_no/auto-recharge` 查询本人订单状态。旧用户及管理员 `/session` 写入接口已撤下。

银行验证由持卡商家完成。`verification_stage=bind` 使用 SetupIntent，其余使用支付验证。浏览器只提交 `{"authentication_failed":false}`，服务器使用已保存的原账号、Session、Intent、Checkout 和套餐。仍以服务方结果为准，不能凭浏览器“成功”直接核销。

## 状态与恢复

| 状态 | 行为 |
| --- | --- |
| `issuing` | 先持久化同一码，再交付；崩溃只补原交付 |
| `issued` | 等待客户核验，不付款 |
| `awaiting_confirmation` | 核验通过，等客户确认；10 分钟过期 |
| `waiting_card` | 已确认，等待卡台；目前缺省停在此处 |
| `waiting_configuration` | 检查密钥、积分、地区配置 |
| `submitting` | 保存了支付提交标记；崩溃后不能自动再付 |
| `queued / running / stopping` | 查询原服务方任务 |
| `action_required` | 等待持卡商家银行验证 |
| `unknown` | 有编号查原任务，无编号人工核实；不能换 UUID 或卡重付 |
| `confirming_subscription` | 付款成功，继续确认订阅同步 |
| `succeeded` | 确认成功，补记本地核销 |
| `completed` | 卡密已核销，保留原订单发货内容 |
| `failed / canceled / manual_review` | 停止自动执行，不自动重发、退款或重付 |

对方无 Webhook。HTTP 200、`pending=false`、积分预占均不单独代表充值成功。订阅同步不明确时继续查订阅；当前确认器需要匹配套餐和有效期，缺字段保持待核实。

本地退款无法撤销对方已受理的付款。提交前发现退款则停止；提交后继续对账、禁止银行验证续接和核销，取得终态后转人工。操作员核实原任务扣款及积分后决定如何处理。当前不提供强制重付或删除重建接口。

先前开发版缺少兑换码或客户确认记录的任务转人工核对，不自动推断已经获授权。当前尚未上线，本次不删除历史数据、不执行真实付款。

## 卡台与验证

实现 `contract.Cards` 并在 `internal/app/container/autorecharge.go` 注入 `Options.Cards`：`Reserve(ctx, taskKey, rule)` 必须对同一任务返回同一不透明卡引用；`Credentials(ctx, reference)` 仅在发送前临时取得卡资料，遵守 context 超时，不记录原始卡资料。缺省 `NoCards` 不供卡。

```sh
go test -count=1 -timeout=90s ./internal/modules/autorecharge/... ./internal/bootstrap/autorecharge ./internal/platform/database/gormdb
```

测试覆盖真实订单持久化、游客发卡兑换、漏通知及发卡崩溃恢复、确认前不扣款、过期及变更核验、并发确认与账号互斥、退款阻断、结果不明不重付、银行验证续接、订阅确认及敏感字段保护。

依据：[AICDK 文档](https://www.aicdkshop.com/api-docs)、[OpenAPI](https://www.aicdkshop.com/api/partner-openapi.json)。卡台额度、限次、冻结和解绑规则需取得卡台协议后实现；实际接口字段及真实充值仍需受控联调。
