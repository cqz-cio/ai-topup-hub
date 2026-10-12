# 智享A社管理后台

本目录是 `ai-topup-hub` 的 Vue 3 管理后台，负责商品、订单、供货、支付及系统配置。使用 TypeScript、Vite、Tailwind CSS、reka-ui。

从仓库根目录使用 `scripts/local/start.ps1` 启动本机前后台及 API。后台地址为 `http://127.0.0.1:5174/`，代理 `/api` 和 `/uploads` 到本机 8080 端口。配置与停止命令见 [本地开发](../../docs/local-development.md)。

```sh
pnpm install --frozen-lockfile
pnpm run test
pnpm run build
```

`pnpm run build` 生成独立根路径资源。Linux/CI 使用 `pnpm run build:fullstack` 生成嵌入式资源；Windows PowerShell 使用：

```powershell
$env:VITE_FULLSTACK = '1'
try { pnpm run build } finally { Remove-Item Env:VITE_FULLSTACK -ErrorAction SilentlyContinue }
```

嵌入式构建注入 `<base href="__DJ_ADMIN_BASE__/">`，后端按 `web.admin_path` 替换；现有正式后台路径为 `/zxa-management/`。Dockerfile 与发布工作流负责构建和嵌入。测试、构建采用有限超时的一次性命令，开发服务使用受管理后台进程。

正式环境见 [生产维护说明](../../ops/tencent/README.md)。上游来源与许可证见 [SOURCE_IMPORT.md](../../SOURCE_IMPORT.md) 和 [LICENSE](../../LICENSE)。
