# π星卡社 · ai-topup-hub

2026-10-19：订单交付按需读取已发布 `20261019-160954`，商城、后台及后端成套更新；历史交付兼容与权限验证见 [交付隐私说明](docs/delivery-privacy.md)。

2026-10-19：清透薄玻璃商品卡片已发布 `20261019-133929`。降低白雾、细亮边缘与轻微反光，见 [维护与验收](docs/glass-product-cards.md)。

2026-10-19：商品卡片清晰度与浅色背景已发布 `20261019-131341`。文字加深加重，背景调浅，保留透明玻璃感，见 [维护与验收](docs/glass-product-cards.md)。

2026-10-19：透明玻璃商品卡片已发布 `20261019-115828`，背景原图完整显示。维护与验收见 [玻璃卡片说明](docs/glass-product-cards.md)。

本仓库维护 π星卡社商城，技术栈为 **Go + Vue 3**。`main` 是唯一正式源码主线，包含绿色界面、派大星 Logo 和水母背景。新加坡现有部署是生产运行基准，具体发布号与源码同步状态分别记录。

本机日常开发固定使用 `D:\卡网` 的 `main`。新会话从 [AGENTS.md](AGENTS.md) 和本页开始；独立开发草稿在各自工作区维护，历史资料只用于明确请求的追溯或恢复。

当前线上程序包含已确认的品牌、Logo 清晰度和导航留白修正，以及 BEP20、Plasma、TRON USDT 和 XorPay 微信扫码支付。推送源码不等于部署，实际构建来源以 [生产发布记录](ops/tencent/README.md) 为准。

| 入口 | 地址 / 位置 |
| --- | --- |
| 正式商城 | https://www.qzweb.cloud/ （上海前端） |
| 正式后台 | https://admin.qzweb.cloud/login （上海 HTTPS；原 IP:18888 入口保留备用） |
| 订单、支付与采购后端 | 新加坡 `43.134.141.10`，API 仅监听回环地址 |
| 本地商城 | http://127.0.0.1:5173/ |
| 本地后台 | http://127.0.0.1:5174/ |
| 绿色源码基线 | `baseline/green-20261012`，提交 `419141e` |
| 后端源码基准 | `20261019-160954`，源码 `297f11deef4d7193081270c55f60c6ba5cdd7295`；见 [交付按需读取](docs/delivery-privacy.md) |
| 商城与后台静态发布 | 均为 `20261019-160954`，与后端使用同一干净构建源码 |
| 微信支付 | 已启用 Native 扫码，客户手续费 1.6% 向上取分；真实资金验收尚未完成 |

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

目录快照现为 **6 个分类、17 款商品、13 条供货映射**（实际在售状态以后台为准）。`qzweb-55` 售价固定为 **1160.00 元**。

- Git 保存源码、公开商品资料、图片、价格覆盖和工具。
- 数据库保存实际商品、上架状态、库存、订单和用户；配置与凭据私有保存。
- `git pull` 只更新源码，目录文件需要显式导入，真实库存来自供货渠道。
- 本地预览使用库存快照，不表示拥有真实卡密，不连接生产采购。
- 11 款 ChatGPT 卡密（含 iOS）发货附 `https://cdk.bar/` 兑换链接和自助充值指引，线上已应用；文案配置见 `docs/catalog/qzweb-redemption-guide.json`。

详见 [商品与购买流程](docs/self-service-storefront.md)、[供货接入](docs/ellaixshop-distribution.md) 和 [发布规则](docs/release-process.md)。生产库不得被本地预览库覆盖。

后台 **用户管理 → 用户列表 → 用户详情 → 设置商品单价** 可直接为每位用户设置商品或规格的固定金额，登录后展示与下单计价一致；留空沿用现有价格。使用方法见 [按用户设置商品单价](docs/user-product-prices.md)。

商城 **个人中心 → 我的钱包** 已支持 BEP20、Plasma、TRON 和微信充值；后台 **用户详情 → 钱包明细** 可增加或扣减余额，首次使用需超级管理员本人完成现有声明。渠道配置、操作与验证范围见 [钱包充值与后台余额调整](docs/wallet-recharge.md)。

后台品牌、系统信息和运营入口的维护说明见 [后台品牌与运营入口](docs/admin-branding.md)。

## 构建与来源

前台与后台由 Vite 构建，`release,fullstack` 将两者嵌入同一个 Go 程序。Dockerfile 和 GitHub 工作流仅构建 `frontend/user`、`frontend/admin`。

本项目基于 Dujiao-Next v1.4.8 独立导入，沿用 Go 模块路径及底层程序名称以保持兼容；它们不表示存在另一套正式商城。来源及许可证见 [SOURCE_IMPORT.md](SOURCE_IMPORT.md) 和 [LICENSE](LICENSE)。[历史上游说明](docs/history/README.md)中的通用安装、升级命令不作为智享A社生产部署入口。

经营者[成交与上游余额邮件](docs/sale-email.md)已启用：订单、采购金额、当前上游余额和查询时间合并通知。该功能首次发布于 `20261013-162050`；当前后端与上海两套前端为 `20261015-160016`，并已补齐[验证码邮件网址与有效期](docs/verification-email.md)、邮箱验证码发送成功提醒及付款页返回对应商品详情。

USDT 新支付单统一向上取到两位小数，同额时按 0.01 USDT 选择最小可用金额，最大增加 0.99；旧单保留原金额。规则与容量限制见 [USDT 付款数量](docs/usdt-payment-amounts.md)。

[微信支付](docs/xorpay-wechat.md)已接入 XorPay，支持商品付款与钱包充值。客户手续费为 1.6%，不足一分向上取整，例如 99.00 元实付 100.59 元。回调验签、金额核对与重复通知处理已接入既有支付流程；已验证实际接口出码和正式页面，尚未进行真实资金验收。

2026-10-13 已补充[渠道一 ChatGPT Pro 5x](docs/channel-one-5x.md)，售价 700.00 元，复用既有供货及库存同步；渠道二 5x 保持独立。

2026-10-14 已上线[应用层防刷与资源保护](docs/abuse-protection.md)：注册和登录分层限流、邮件额度原子预占、验证码单次消费、请求体与并发限制及支付回调独立保护。首次后端发布为 `20261014-085600`，当前版本继续保留。本机及正式入口验证通过，数据和支付配置保留；应用防护不代表带宽型 DDoS 承载能力。

2026-10-14 已统一并开启[分销申请](docs/reseller-applications.md)：后台分销概览显示实际启用与申请状态，商城按同一服务端规则展示申请、待审核、已开通等状态。申请提交后仍需管理员审核；域名与店铺配置另行维护。

2026-10-14 已按确认预览上线[关于页面与充值说明](docs/about-page.md)，保留原有商品详情和商品例外。

[TRON（TRC20）USDT](docs/tron-usdt-payment.md) 已启用，支持商品付款及钱包充值。按官方合约、收款地址和支付单准确数量识别，达到确认要求后复用现有订单发货和钱包入账流程；真实资金验收尚未完成。

关于页面所有文案、充值续费说明、客服提示及相关入口现在都可在后台「系统设置 → 站点设置 → 关于我们 (About)」直接修改，保存后刷新商城生效；原有商品说明保持独立。

2026-10-15 已上线[付款前卡台余额检查](docs/payment-balance-guard.md)：U 和微信商品付款前汇总采购成本并查询余额，不足或无法查询时暂停付款；弹窗明确联系客服，邮件提醒按实际发送结果显示。日常本地验收通过后推送、备份并发布，原商品、收款配置和业务数据保留。

2026-10-16 已按批准稿修正商城 Pro 10x／25x 封面、统一字体及地区标注，移除公开渠道编号，见 [商城倍率与地区展示](docs/catalog-presentation.md)。
