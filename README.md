# π星卡社 · ai-topup-hub

本仓库维护 π星卡社商城，技术栈为 **Go + Vue 3**。`main` 是唯一正式源码主线，包含绿色界面、派大星 Logo 和水母背景。新加坡现有部署是生产运行基准，具体发布号与源码同步状态分别记录。

本机日常开发固定使用 `D:\卡网` 的 `main`。新会话从 [AGENTS.md](AGENTS.md) 和本页开始；独立开发草稿在各自工作区维护，历史资料只用于明确请求的追溯或恢复。

当前线上程序包含已确认的品牌、Logo 清晰度和导航留白修正，以及 BEP20、Plasma 两种 USDT 支付渠道。推送源码不等于部署，实际构建来源以 [生产发布记录](ops/tencent/README.md) 为准。

| 入口 | 地址 / 位置 |
| --- | --- |
| 正式商城 | https://www.qzweb.cloud/ （上海前端） |
| 正式后台 | https://124.220.2.69:18888/login （上海独立 HTTPS 端口） |
| 订单、支付与采购后端 | 新加坡 `43.134.141.10`，API 仅监听回环地址 |
| 本地商城 | http://127.0.0.1:5173/ |
| 本地后台 | http://127.0.0.1:5174/ |
| 绿色源码基线 | `baseline/green-20261012`，提交 `419141e` |
| 生产发布记录 | `20261013-110949`，构建源码 `4623e55c6caae3356880dc81d5db98c64c34c60c`，见 [生产维护说明](ops/tencent/README.md) |

源码标签、服务器发布号和数据库是三个独立对象。线上比对证据与核验范围在 [production-baseline.json](ops/tencent/production-baseline.json)。推送 GitHub 不会自动部署服务器，也不会同步数据库。2026-10-12 已将 `www` 解析切到上海；两套前端发布 `20261012-164700` 与新加坡后端的构建源码 `948c37f` 对齐，域名 HTTPS、管理端登录鉴权和证书自动续期已核验。旧新加坡 HTTP 入口保留兼容，日常使用上表 HTTPS 地址。正式支付宝仍未启用，代码和沙箱结果见 [支付宝接入记录](docs/alipay-web-payment.md)。

## 目录

```text
cmd/                 Go 程序入口
internal/            后端业务模块、数据库和队列
frontend/user/       唯一正式商城前台（绿色 Vue 版本）
frontend/admin/      Vue 管理后台
scripts/local/       本地启动、停止和状态检查
scripts/             商品目录导入、供货库存维护等工具
docs/catalog/        公开商品目录、价格覆盖与映射
docs/                当前开发及业务说明
docs/history/        历史资料，不作为当前启动说明
ops/tencent/         新加坡生产维护、服务单元与基准记录
work/                本机数据、日志、备份和临时工具（不提交）
```

早期 React 展示页 `frontend/catalog` 已从主线移除。源码保存在绿色基线标签的 Git 历史中，本地另有归档；不要另外运行或部署旧展示页。

## 本地使用

在项目根目录运行：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/local/start.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/local/status.ps1
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/local/stop.ps1
```

首次配置、依赖和构建见 [本地开发](docs/local-development.md)。脚本复用已有本机配置和数据库，不覆盖数据。本地采购关闭，生产交易配置独立管理。

## 商品与库存

当前目录为 **6 个分类、16 款商品、12 款供货映射**。`qzweb-55` 售价固定为 **1160.00 元**。

- Git 保存源码、公开商品资料、图片、价格覆盖和工具。
- 数据库保存实际商品、上架状态、库存、订单和用户；配置与凭据私有保存。
- `git pull` 只更新源码，目录文件需要显式导入，真实库存来自供货渠道。
- 本地预览使用库存快照，不表示拥有真实卡密，不连接生产采购。
- 11 款 ChatGPT 卡密（含 iOS）发货附 `https://cdk.bar/` 兑换链接和自助充值指引，线上已应用；文案配置见 `docs/catalog/qzweb-redemption-guide.json`。

详见 [商品与购买流程](docs/self-service-storefront.md)、[供货接入](docs/ellaixshop-distribution.md) 和 [发布规则](docs/release-process.md)。生产库不得被本地预览库覆盖。

后台 **用户管理 → 用户列表 → 用户详情 → 设置商品单价** 可直接为每位用户设置商品或规格的固定金额，登录后展示与下单计价一致；留空沿用现有价格。使用方法见 [按用户设置商品单价](docs/user-product-prices.md)。

商城 **个人中心 → 我的钱包** 已支持 BEP20、Plasma 充值；后台 **用户详情 → 钱包明细** 可增加或扣减余额，首次使用需超级管理员本人完成现有声明。渠道配置、操作与验证范围见 [钱包充值与后台余额调整](docs/wallet-recharge.md)。

## 构建与来源

前台与后台由 Vite 构建，`release,fullstack` 将两者嵌入同一个 Go 程序。Dockerfile 和 GitHub 工作流仅构建 `frontend/user`、`frontend/admin`。

本项目基于 Dujiao-Next v1.4.8 独立导入，沿用 Go 模块路径及底层程序名称以保持兼容；它们不表示存在另一套正式商城。来源及许可证见 [SOURCE_IMPORT.md](SOURCE_IMPORT.md) 和 [LICENSE](LICENSE)。[历史上游说明](docs/history/README.md)中的通用安装、升级命令不作为智享A社生产部署入口。
