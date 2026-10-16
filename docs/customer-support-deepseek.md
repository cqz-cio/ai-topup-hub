# 小π客服：DeepSeek 本地调试

本功能仅在 `codex/customer-support-agent` 独立分支开发。没有合并、推送或部署到正式网站。入口仍为 `http://127.0.0.1:5175/?template=classic`，派大星外观、连续动作和鼠标拖动保留。

## 运行与密钥

独立客服 Go 进程只监听 `127.0.0.1:8085`。前端通过 Vite 的 `/api/v1/support` 代理访问它，商城其他 API 继续使用已有的本机 `8080`。客服程序只读公开商品接口，不打开商城数据库，不携带商城登录令牌，不连接生产服务器、供应商或支付接口。

在客服工作区根目录执行：

```powershell
pwsh -NoProfile -File scripts/local/support-agent.ps1 start
pwsh -NoProfile -File scripts/local/support-preview.ps1 start
```

修改客服 Go 源码后，仅停止并重建该服务：

```powershell
pwsh -NoProfile -File scripts/local/support-agent.ps1 stop
pwsh -NoProfile -File scripts/local/support-agent.ps1 start -Build
pwsh -NoProfile -File scripts/local/support-agent.ps1 status
```

`support-preview.ps1 stop` 仅停止该分支前端；`support-agent.ps1 stop` 停止客服进程。PID、启动时间和独立日志保存在忽略的 `work/codex-logs/`。脚本核验所属进程后再停止，构建有 120 秒超时，服务有健康检查；不管理日常商城、Redis 或线上进程。

密钥只在本机忽略文件 `work/codex-runtime/support-agent/private/config.json` 中，文件夹 ACL 限当前用户与 SYSTEM。使用者在本机私密编辑器维护，不放进命令参数、Vite 环境变量、前端代码、Git、截图或日志。配置未提供或非法时拒绝启动。这个目录同时保存 `usage.db` 及其 SQLite WAL 文件，保留它们才能延续额度记录，不能当临时缓存清理。

配置字段：`api_key`、`model`、`max_output_tokens`、`daily_budget_nanousd`、`daily_calls`、`session_daily_calls`、`session_per_minute`、`global_per_minute`。当前值见下表，密钥不在文档中。改配置后重启客服服务。

## 模型与成本控制

使用官方 `https://api.deepseek.com/chat/completions` 的 `deepseek-flash`，关闭思考，不启用流式输出，没有 tools、搜索、浏览器、代码执行或客户端自选模型。官方接口与参数参考 [DeepSeek 文档](https://api-docs.deepseek.com/)。

| 限制 | 当前本地值 |
| --- | --- |
| 输入 | 500 个 Unicode 字符，HTTP body 最多 4 KiB |
| 输出 | 每次最多 800 token |
| 上下文 | 最近 6 轮、最多 9 KiB；总模型 messages 最多 28 KiB |
| 会话 | HttpOnly / SameSite=Strict cookie；闲置 30 分钟过期；最多 200 个内存会话 |
| 频率 | 每会话每分钟 6 次；本机所有会话合计每分钟 20 次 |
| 并发 | 每会话 1 个；全局 2 个 |
| 每日调用 | 全局 100 次；每会话 50 次；以北京时间日历日计 |
| 每日预算 | 保守估算 0.25 美元；到任一上限即拒绝模型调用 |
| 超时 | 模型 35 秒，完整服务请求 45 秒；不自动重试 |

SQLite 在调用前原子预占最大可能费用及次数，已知 usage 再结算；网络失败、取消或结果不明的预占保留，重启不清零。重复 request ID 不会再次调用模型。账本只记录不可逆摘要 ID、日期、计数、token 和费用，不存客户聊天。每分钟限流和会话历史在内存中，重启会丢失，持久化的每日全局限额仍有效。

费用估算采用 2026-10-16 查到的 Flash 峰时未缓存单价：输入 $0.30 / 百万 token、输出 $1.20 / 百万 token；忽略缓存和谷时优惠，预占输入按 UTF-8 字节数加余量计。此值是本程序的保守估算，不是供应商账单或其控制台硬限额；供应商调价必须同步 `ledger.go`。参考 [官方价格](https://api-docs.deepseek.com/quick_start/pricing/)。

## 对话与资料边界

- 浏览器只提交当前消息、语言、商品 slug、会话与请求 ID；不接受自定义 system prompt、history、tools 或模型参数。会话上下文由客服后端维护，刷新页面开始新对话，语言切换取消旧回复并重置上下文。
- 小范围整句规则处理问候、人工入口、明显凭据，以及“今天天气咋样”等明确站外问题，不调用模型。带商城语义的“你们卖的 GPT 能查天气吗”、省略句“这个怎么用”等仍送模型结合上下文理解。模型范围约束不是零消耗分类器，复杂越界输入可能消耗一次受限回复，也不是百分之百的语义安全保证。
- 一般业务资料在 `knowledge.go` 维护；当前商品名、公开参考价及详情来自本机公开商品接口，60 秒缓存，最多 5 页。仅反序列化允许字段，剥离 HTML、脚本和网址，不读取交付私密字段。目录失败时明确只提供一般指引，不以旧缓存冒充当前商品状态。
- 页面上下文仅使用在公开目录中找到的 slug；不能把客户端传入的地址作为抓取 URL。售前和聊天中都不展示具体兑换域名，只指向付款后的订单交付说明。
- 模型按 JSON 返回纯文本和资料 ID。后端检查结构、长度、来源 ID 与敏感链接；站内链接由服务端白名单生成，前端再校验。文本由 Vue 转义，没有 HTML 执行。来源 ID 校验不能自动证明每句话正确，具体业务细则仍需继续人工打磨。
- 暂不读取私人订单、支付状态、钱包余额、佣金或实际兑换结果；不能退款、补发、付款、充值或采购。未知规则澄清或转人工，联系方式保持微信 `Cqz0924ttxk`、QQ `1669268759`、Telegram `@czq_piestar`。
- 模型看到的是必要聊天上下文和公开资料。邮箱在发送前隐藏；明显 key、token 或长凭据直接提示重述，但这不是完整 DLP 系统。前端明确标识 AI 和 DeepSeek，提示勿提交密码或完整卡密。聊天不写入应用日志或数据库；异常只记录固定错误分类和 HTTP 状态，不记录供应商响应原文。

## 验证与后续上线条件

Go 测试使用模拟模型、公开目录桩和临时预算库，覆盖自然追问、JSON 上下文、幂等、未知结果不退款、重启预算、限流并发、来源与输出校验、Origin/Host/请求体边界及敏感资料隔离。前端测试覆盖请求字段、链接白名单、失败不重试和错误提示。少量真实模型调用与浏览器检查单独记录在忽略目录，不能说成生产验收。

本服务只有本机身份和单进程限制，不具备对公网发布的完整身份验证、代理 IP 信任、多实例额度协调、挑战机制、内容维护后台及业务授权。上线前必须同步主线之后的售前兑换网址移除、分类和客服修复，正式审查限流、隐私及价格，再按本地验证和发布流程执行；当前分支不能直接覆盖正式商城。现在不做这些生产改动。
