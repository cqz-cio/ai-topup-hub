# π星卡社后台品牌与运营入口

管理后台使用 π星卡社名称及现有派大星 Logo。登录页、浏览器标题和默认图标、桌面与手机导航、折叠侧栏、页脚，以及简体中文、繁体中文、英文界面均使用本站品牌。

运营仪表盘上方提供购买须知、会员商品价和客服兑换入口。客服微信为 `Cqz0924ttxk`，ChatGPT 卡密自助充值入口为 `https://cdk.bar/`，具体兑换方式仍以各笔订单交付说明为准。会员价入口遵循现有权限；商品价和订单计价规则不变。

原项目的后台广告、赞助位和 Telegram Bot 授权购买推广已移除。Telegram Bot 的实际连接、配置和授权状态功能保留。顶部「系统信息」只展示本站信息与程序标识，不查询或下载上游发行版，也不执行程序替换、重启或回滚；维护仍遵循 [发布规则](release-process.md)。程序标识与服务器发布号分别记录。

设置页面的站名、介绍、邮件署名、支付显示名称、回调域名和机器人用户名示例已改为本站内容。示例文字不自动写入现有配置；已保存的支付渠道、邮件账号和密钥仍独立维护。用户自定义站点图标优先于默认品牌图标。

第三方支付产品名称、供货协议标识、Go 模块路径、原项目来源与许可证说明保留准确名称。财务声明确认流程与已有确认记录未调整。

维护入口：

- 品牌常量与公开入口：`frontend/admin/src/utils/branding.ts`。
- 品牌组件与运营说明：`frontend/admin/src/components/AdminBrand.vue`、`components/admin/DashboardOperations.vue`、`components/SystemInfoDialog.vue`。
- 三语文案：`frontend/admin/src/i18n/adminBrand.ts` 与 `i18n/index.ts`。
- 默认品牌素材：`frontend/admin/public/brand/patrick-logo.png`，复用商城已确认的 Logo。
