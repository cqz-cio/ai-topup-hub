# 换设备开发：客服密钥与本机文件

客服功能位于 `codex/customer-support-agent`，尚未合并主线或部署。2026-10-16 已将现有 DeepSeek API Key 加密保存到仓库 `cqz-cio/ai-topup-hub` 的 **Actions repository secret**，名称为 **`SUPPORT_DEEPSEEK_API_KEY`**。密钥值没有提交到 Git。管理入口：<https://github.com/cqz-cio/ai-topup-hub/settings/secrets/actions>。

## 能拉取什么

Git 拉取的是源码、[机器可读密钥清单](development-secrets.json)、[空值配置模板](examples/support-agent.config.example.json)和初始化脚本。清单记录每个密钥的用途、GitHub 名称、本机路径及配置字段，供新设备和开发助手查阅；没有密钥值。

| 项目 | 对应位置 |
| --- | --- |
| GitHub Secret | Actions → `SUPPORT_DEEPSEEK_API_KEY` |
| 本机私密文件 | `work/codex-runtime/support-agent/private/config.json` |
| 文件中的字段 | `api_key` |
| 可选安全导入环境变量 | `SUPPORT_DEEPSEEK_API_KEY` |
| 模板 | `docs/examples/support-agent.config.example.json` |
| 配置检查 / 初始化 | `node scripts/local/support-secrets.mjs check` / `init` |

**GitHub Secrets 不能读回原文，也不会随 clone/pull 自动下载。** `gh secret list` 和 API 只能查看名称与时间等元数据；密钥值只能由获准的 GitHub Actions 工作流使用。当前没有工作流引用这个 Secret，也没有新增导出密钥的工作流。依据：[GitHub Secrets API](https://docs.github.com/en/rest/actions/secrets#get-a-repository-secret)、[GitHub CLI 加密上传说明](https://cli.github.com/manual/gh_secret_set)。

## 新设备步骤

1. 克隆并切换到客服分支，或在现有克隆中 fetch 后切换；不要把客服草稿合并到正式主线：

   ```sh
   git clone --branch codex/customer-support-agent --single-branch https://github.com/cqz-cio/ai-topup-hub.git
   cd ai-topup-hub
   node scripts/local/support-secrets.mjs check
   ```

2. 从旧设备的私密文件或自己的密码管理器安全取得 Key；如果没有可用副本，在 DeepSeek 控制台创建新 Key。GitHub Actions Secret 不充当可下载的备份。运行初始化并在隐藏输入提示中粘贴 Key：

   ```sh
   node scripts/local/support-secrets.mjs init
   node scripts/local/support-secrets.mjs check
   ```

   已安全注入本进程环境的 `SUPPORT_DEEPSEEK_API_KEY` 也可用于初始化。不把 Key 写进命令参数、源码、截图或日志。脚本最多等待输入 120 秒，不联网；Windows 限制为当前用户与 SYSTEM，其他系统目录为 0700、文件为 0600。现有私密配置不会被覆盖，也不改用量账本。

3. 按 [本地开发](local-development.md) 配好该设备的商城 API、依赖与本机数据，再按 [客服调试说明](customer-support-deepseek.md) 启动客服和预览。当前 PowerShell 启停脚本按 Windows 维护；密钥初始化脚本可跨平台使用，但其他系统的商城运行环境需另行配置。

`check` 只报告文件是否存在及 Key 格式，不调用供应商验证有效性。换设备需要自己的本机商城配置；这里仅同步客服 API Key 的清单，不迁移服务器、支付商户、数据库或其他凭据。公开仓库中的文件清单不代表这些私密文件可以上传。

## 更新与额度

更新 GitHub Secret 和更换本机 Key 是两件事，彼此不会自动同步。可在 GitHub 设置页面更新，或运行 `gh secret set SUPPORT_DEEPSEEK_API_KEY --repo cqz-cio/ai-topup-hub --app actions`，按终端隐藏输入提示录入；不要用带明文 Key 的 `--body` 命令行。实际修改本机私密配置后重启客服服务。

`usage.db`、`usage.db-wal` 和 `usage.db-shm` 是本机额度账本，不是密钥。每台设备独立计量；多设备共用 Key 时本地额度不会自动汇总为供应商账号的总额度。保留原设备账本，不为重置额度清理它，也不在运行中直接搬动 SQLite 文件。
