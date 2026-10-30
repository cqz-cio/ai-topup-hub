# ChatGPT Session 获取指引

核对账号页在 ChatGPT（`product=gpt`）商品的 Session 输入框上方提供三步指引：

1. 打开 `https://chatgpt.com/`，登录并核对目标充值账号邮箱。
2. 在同一浏览器打开 `https://chatgpt.com/api/auth/session`，复制完整 JSON。
3. 返回粘贴，核验后再次确认邮箱与套餐，再由用户确认充值。

两个链接均在新标签页打开，使用 `noopener noreferrer` 和 `no-referrer`，不会携带本站卡密或 Session。Claude、Grok、X 保持各自原有凭证指引。

## 2026-10-30 验证记录

- 基线为 `codex/paiplus-binding` 的 `4f177b81bda499ae79d155f299e6027618b06163`。修改前本机构建的 `index-521HNEB6.js` 和 `index-CDH66LoU.css` 与本次取得的正式站公开资源 SHA256 完全一致。
- 在已登录的 Chrome 中打开上述 Session 地址，只检查字段名与存在性，实际返回含 `accessToken`、`sessionToken`、`user`、`account` 等字段。没有输出、保存、复制到剪贴板或向第三方提交真实凭证；检查后关闭页面。
- `RedemptionService.preflight` 将完整字符串放入 `{credential: {mode: 'session', session}}`，由 `Partner` 发送至固定上游 `/preflight`。本站不删减 JSON 字段。现有页面提示要求 `sessionToken` 或 `session_token`，本次实际返回符合这一提示。
- 新增三个集成测试，使用与观察到的字段结构相同、值完全虚构的 JSON，通过真实 `Partner` 适配器和替身 HTTP 传输检查：完整 JSON 原样传递，成功进入 `checked`，受限进入 `blocked`，拒绝恢复 `preview`；均不调用 `redeem`、不标记提交、不保存 GPT Session。
- 60 项自动测试通过，生产构建通过。浏览器模拟核验进入邮箱/套餐确认页，返回更换账号后 Session 输入框为空。桌面、390px 手机、320px 深色模式无横向溢出，Claude 不显示 ChatGPT 指引。

边界：以上确认当前浏览器会话能获取所需字段，以及本地核验转发链路兼容该 JSON 格式。未将真实 Session 与正式卡密发送给上游做预检，不能将模拟核验说成真实上游核验成功；账号资格、凭证有效性及上游可用性仍由实际预检结果决定。

私有测试日志和截图保存在 `D:/卡网/work/codex-logs/` 与 `D:/卡网/work/codex-runtime/session-guide-20261030/`。发布采用独立前端静态更新：保留旧散列资源，只新增本次 JS/CSS 并原子替换入口；后端、生产配置和业务数据不替换。

## 正式发布：2026-10-30

- 已按用户授权上线 `https://cdk.qzweb.cloud`，静态更新号 `session-guide-20261030-154100`，干净构建源码 `c6f05e2172f3cdc68fea3f6880b73eab183990dc`，GitHub 分支 `codex/session-guide`；`main` 与原 `codex/paiplus-binding` 未修改。
- 正式资源为 `index-DIi8bby2.js`、`index-B0Y20b2C.css`，与完成桌面和手机验收的本机构建逐文件一致。31 项公网检查通过，包括全部 30 个构建文件的哈希和 live 模式接口；正式浏览器加载新资源，首页显示正常、无控制台错误。正式第二步需要有效卡密，本次未提交正式卡密或真实 Session；该页视觉与交互沿用同一构建的本地模拟验收证据。
- 备份 `/srv/paiplus/backups/before-session-guide-20261030-154100` 包含完整旧发布、配置及一致性 SQLite 备份，数据库完整性检查通过；上线窗口内 7 张业务表逐行摘要一致。保留 49 个旧发布文件和 60 个 Nginx 文件，环境配置及服务 PID 未变。
- 当前程序目录仍为 `/srv/paiplus/releases/20261029-204310`，仅静态入口更新；没有重启服务、上传业务数据库或发起充值。回退只需将备份中 `release/dist/client/index.html` 原子还原到当前 `dist/client/index.html`，不要恢复整库或回退隐私修复程序。
- 服务器证据 `/srv/paiplus/static-updates/session-guide-20261030-154100/verified-report.json`；本地发布证据位于当前工作区 `work/codex-runtime/session-guide-release-20261030/`，日志位于 `work/codex-logs/`。
