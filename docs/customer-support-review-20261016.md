# 客服 Agent 分支检查记录（2026-10-16）

本次按用户要求从 `codex/customer-support-agent` 的 `7ea326b3a92920248bedcc2ecc0d090c88bf5c8a` 新建 `codex/customer-support-review-20261016`，保留原分支的完整代码与五个开发提交。本次新增检查记录及 README 入口，没有修改业务源码、界面或私密配置。

原客服工作区在检查开始时没有未提交改动，原分支与 GitHub 同名分支的 SHA 一致。日常 `D:\卡网` 当时位于 `codex/product-center-display`（`44afe45851006df9c80958d637b54b326a7d6bef`），保留了其他任务的未提交改动；这些草稿未带入本分支。

## 检查范围

- 检查聊天前端到独立 Go 服务的请求流程、公开商品目录读取、上下文维护、输出来源与链接校验。
- 检查本机 Host / Origin 限制、会话与并发限制、每日次数和费用预占、重复请求及未知结果处理。
- 检查密钥模板、初始化脚本、文件权限及 Git 忽略规则。扫描原 Agent 分支新增或修改的 36 个文件，未发现真实凭据模式；测试中的显式假密钥单独识别。该检查不是完整的历史秘密审计。
- 确认 `work/`、私密配置、用量数据库、依赖目录及前端构建目录未进入 Git 跟踪；本次不读取或导出真实 API Key。

## 本机验证

| 检查 | 结果 |
| --- | --- |
| `go test -v -count=1 -timeout=90s ./internal/modules/supportagent ./cmd/support-agent` | 通过：9 个顶层测试，含子测试共 18 项；入口包没有测试文件 |
| `frontend/user` 的 `node --experimental-strip-types --test tests/*.test.ts` | 177 项通过，0 失败、0 跳过 |
| `node --test scripts/tests/support-secrets.test.mjs` | 3 项通过，包括 Windows ACL 和已有文件防覆盖 |
| `frontend/user` 的 `pnpm run build` | Vue / TypeScript 类型检查和 Vite 生产构建通过 |
| `go build -o <忽略目录中的输出文件> ./cmd/support-agent` | 独立客服程序构建通过 |
| `git diff --check origin/main...HEAD` | 通过 |
| 现有日常 `8080` 公开商品接口、`8085` 客服健康接口及 `5175` 预览 | 均返回 HTTP 200 |
| 现有 `5175` 实际浏览器 | Classic 与 Vault 均可展开客服；Classic 的本地问候回复、人工微信 / QQ / Telegram 展示通过 |

测试与构建均为一次性执行，使用显式超时并保存日志。Go 模型测试使用模拟供应商和临时账本。浏览器仅发送本地规则处理的问候，本次未调用真实付费模型、创建商城订单或执行付款采购。

自动化检查在本分支的独立工作区执行，依赖复用原客服工作区且锁文件逐字节一致。浏览器复查使用已运行的原客服分支 `5175` 入口；本次源码没有功能变化，未替换或重启已有服务。本次未重复完整移动端视觉验收，既有视觉记录见根目录 `design-qa.md`。

日志保存在本机忽略目录 `D:\卡网\work\codex-logs\agent-*-20261016-*.log`，截图与命令执行元数据汇总位于 `D:\卡网\work\codex-runtime\agent-review-20261016\`。它们不随 Git 提交。

## 后续开发边界

检查时远端 `main` 为 `b9459c9e493588ce943289f4349a3b2a27762512`；原 Agent 分支与它已分叉，主线有 14 个本分支未包含的提交，本分支有 5 个独有开发提交。本次保留这条开发线，没有合并主线。

当前分支仍是本地试用版本。主线后续的售前兑换网址移除、客服、商品地区及倍率等修正尚未完整纳入；例如旧商城展示代码仍含需要在合并时处理的历史文案。因此测试和构建通过仅说明本次本地检查结果，不代表可直接覆盖正式商城，也不代表真实模型或生产验收。

继续开发和换设备时，沿用 [DeepSeek 本地调试](customer-support-deepseek.md) 与 [开发密钥清单](development-secrets.md)。本次仅推送独立检查分支，不合并 `main`、不部署，也不迁移数据库、私密配置或运行账本。
