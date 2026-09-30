# 智享A社网页前端

本目录是智享A社商品目录网页，使用 React 19、Vinext、Vite 和 Tailwind CSS。

自助下单商城已将确认后的首页接入 `frontend/user`。正式部署继续构建原用户端；本目录保留为展示页与商品资料参考，具体链路和部署说明见 [自助购买联调说明](../../docs/self-service-storefront.md)。下文说明仅适用于单独运行这一参考展示页。

## 本地运行

需要 Node.js >= 22.13.0 和 npm。在本目录执行：

```sh
npm ci
npm run dev
```

生产构建：

```sh
npm run build
```

静态页面及资源生成在 `dist/client/`。`npm run start` 使用 Wrangler 本地预览构建结果。

## 商品数据

`data/public-catalog.json` 保存网页展示的商品名称、售价、图片和说明，可直接编辑后重新构建。本目录不需要供应商原始资料、成本价或本地后台数据库；也不运行原工作目录中依赖这些资料的商品生成和后台初始化脚本。

## 与卡网集成

默认模式每次向同源 `/api/v1/public/products?page_size=100` 请求公开商品数据，并通过 `zxa-<商品编号>` 匹配商品，更新价格和可售状态。商品详情跳转 `/products/zxa-<商品编号>`；登录、用户订单和游客查单使用 `/auth/login`、`/me/orders`、`/guest/orders`。

部署时可将本目录的 `dist/client/` 用作站点首页，静态资源由同一站点提供；`/api/` 转发至 Go 后端，其余交易页面由现有 `frontend/user` 提供。需要在反向代理配置中明确这些路由，不能将此目录构建结果整体覆盖到原用户端目录。

仅展示商品目录、通过客服咨询购买时，在开发或构建前设置 `NEXT_PUBLIC_CATALOG_ONLY=1`，然后启动开发服务或执行构建。例如 PowerShell：

```powershell
$env:NEXT_PUBLIC_CATALOG_ONLY = '1'
npm run build
```

环境变量在构建时生效。当前提交仅添加网页源码，不修改现有后台、用户端、管理端或线上部署配置。
