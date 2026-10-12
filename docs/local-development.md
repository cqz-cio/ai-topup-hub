# 本地开发与预览

在仓库根目录操作，使用 `main`。正式页面只有 `frontend/user` 和 `frontend/admin`，后端入口为 `cmd/server`。

## 已配置的 Windows 工作区

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/local/start.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/local/status.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/local/stop.ps1
```

`start.ps1 -Build` 显式重新构建后端；服务已经运行时先停止再构建。前端日常预览使用 Vite。脚本仅管理这个工作区记录的进程，遇到其他程序占用端口时退出。

| 内容 | 当前路径 / 端口 |
| --- | --- |
| 商城前台 | `frontend/user`，127.0.0.1:5173 |
| 管理后台 | `frontend/admin`，127.0.0.1:5174 |
| Go API | 127.0.0.1:8080 |
| 本机配置 | 根目录 `config.yml`，不提交 |
| 本机数据库 | `work/codex-runtime/live/shop.db` |
| 进程状态与日志 | `work/codex-logs/` |
| 当前持久化 Redis | Ubuntu WSL，127.0.0.1:6390 |
| 旧版归档与 Git 备份 | `work/archive/` |

脚本不会安装、重置或清空 Redis；它检查配置中启用的 Redis 是否可连接。现有 WSL Redis 配置和数据在 `work/codex-runtime/live/redis*`，包文件在 `work/codex-runtime/real-redis/`。停止商城不会停止或清空 Redis。

本地后端使用 `-mode storefront`，强制 `UPSTREAM_PURCHASE_ENABLED=false`。不要使用本机预览进行真实付款；本地与生产的交易配置和数据分别维护。

## 新工作区准备

需要 Go（版本按 `go.mod`）、Node.js 24、pnpm 10.34.3，以及配置启用时所需的 Redis。

1. 从 `config.yml.example` 创建私有 `config.yml`，分别设置不同的随机应用/JWT 密钥，不复制生产密钥。
2. 设置本机监听 `127.0.0.1:8080`，SQLite 路径为 `./work/codex-runtime/live/shop.db`，配置本机独立 Redis。仅看目录时可关闭 Redis、队列及 BSC 收款监听；这份预览配置不能用于生产。
3. 分别在 `frontend/user`、`frontend/admin` 安装依赖：`pnpm install --frozen-lockfile`。一次性命令设置有限超时并保存日志。
4. 执行 `scripts/local/start.ps1 -Build`。首次管理员初始化按本机配置处理，不把凭据写入 Git。
5. 新库没有生产商品。使用 [目录导入工具](../scripts/sync-qzweb-catalog.py) 显式导入公开资料；需要库存预览时从生产公开接口取得快照，仅写本机库。目录导入本身不产生真实库存。

缺少配置或依赖时脚本明确报告，不使用演示数据覆盖已有库。

## 验证与构建

前台和后台各自执行 `pnpm run test`、`pnpm run build`。Windows 构建嵌入式后台时先设置 `$env:VITE_FULLSTACK='1'` 再构建，完成后清除该变量；Linux/CI 使用 `pnpm run build:fullstack`。

后端按变更运行相关测试，单次测试设置 `-timeout`。完整发布先构建两套 Vue 资源并复制到 `internal/web/dist/{user,admin}`，再构建 Go `-tags release,fullstack`。

旧的 2026-09-30 主网支付验收记录已归档，不作为当前启动指南。
