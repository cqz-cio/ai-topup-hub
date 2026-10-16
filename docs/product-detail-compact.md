# 商品详情页第 1 版

经典商城商品详情页采用独立双栏：左侧完整 5:3 商品图、描述和兑换说明；右侧完整标题、价格、规格、购买条件、数量和操作。手机依次展示图片、购买区、描述/兑换、完整详情。

## 实现边界

- `ProductDetail.vue` 调整展示结构和响应式样式；`ProductImageGallery.vue` 保持完整图及原缩略图交互。
- `ProductRedemptionGuide.vue` 从原 `ProductPurchaseGuide.vue` 原样提取兑换链接与交付说明。经典页将它置于图片下，Vault 默认仍显示在原购买条件区。
- 所有商品数据、后台 HTML、三语规则、价格/库存/数量校验、购买处理和支付逻辑保持原样，继续使用原 composable。
- 不通过改写商品内容缩短页面。新开通/续费/Codex/Gemini 的购买前条件保持在购买按钮前。

## 验证与发布

日常本机和干净构建检查覆盖 1920/768/390/320px、暗色及三语言；已有 138 项测试、类型与构建通过。详情见根目录 `design-qa.md`，证据在忽略目录 `work/codex-runtime/product-detail-preview-20261016/`。

仅部署商城静态资源，从已验证提交生成不可变清单。保留新加坡后端、Redis、库存 timer、上海后台、商品图片、历史散列资源及 Nginx 配置。两地备份、实际发布号及验证结果保存在同目录 `verified-report.json`。

## 商品详情页第 1 版上线（2026-10-16）

- 上海商城 `20261016-100816`，干净源码 `149b049921245d18cebd05d8b5d7f8a2617166ad`。按已批准第 1 版实现独立双栏、完整商品图、图片下方描述/兑换说明和紧凑购买区；手机依次显示图片、购买区、描述/兑换及完整详情。保留已发布商品中心同步效果。
- 商品内容、原图、售价、规则及购买/支付逻辑不变。138 项测试、类型构建、日常本机 17 项、最终构建 4 项及正式站 7 项浏览器检查通过，覆盖 1920/768/390/320px、三语和暗色，页面无脚本错误或横向溢出。维护说明 `docs/product-detail-compact.md`，视觉对照 `design-qa.md`。
- 256 个商城构建文件、6085 个保留商城文件、1165 个后台文件及 57 个 Nginx 文件核验通过；新加坡后端和上海后台仍为 `20261015-160016`，商品内容/售价/配置与后端、Redis 进程保持一致。没有创建验收订单、上传本机库或执行真实付款采购。
- 两地备份分别为 `/srv/zhixiang-front/backups/before-product-detail-compact-20261016-100816` 和 `/srv/zhixiang/backups/before-product-detail-compact-20261016-100816`；私有验收记录 `work/codex-runtime/product-detail-preview-20261016/verified-report.json`。文档提交不代表程序重建。

