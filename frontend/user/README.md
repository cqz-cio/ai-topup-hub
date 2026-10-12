# 智享A社商城前台

本目录是 `ai-topup-hub` 唯一正式的绿色商城前台，使用 Vue 3、TypeScript、Vite、Tailwind CSS、Pinia 和 vue-i18n。

从仓库根目录使用 `scripts/local/start.ps1` 启动整个本地项目，前台地址为 `http://127.0.0.1:5173/`。配置、数据和停止命令见 [本地开发](../../docs/local-development.md)。

单独开发时，先启动本机后端，再在此目录运行：

```sh
pnpm install --frozen-lockfile
pnpm run dev --host 127.0.0.1 --strictPort
```

开发服务将 `/api`、`/uploads`、`/sitemap.xml`、`/robots.txt` 代理到本机 8080 端口，保留原始 Host 以支持租户解析。长期服务使用受管理的后台进程；测试和构建采用有限超时的一次性命令。

```sh
pnpm run test
pnpm run build
```

正式构建与后台资源一起嵌入 Go 二进制；由 `internal/web/handler.go` 提供根路径 `/`。Dockerfile 与发布工作流负责全栈打包。正式服务器状态见 [运行说明](../../ops/tencent/README.md)，推送源码不会自动部署。

上游来源与许可证保留在根目录 [SOURCE_IMPORT.md](../../SOURCE_IMPORT.md) 和 [LICENSE](../../LICENSE)。
