# 商品详情海洋按钮（本地预览）

2026-10-22 用户选择海浪气泡方案，并将“立即购买”调成与“加入购物车”接近的浅冰蓝色后，授权本地替换。

- 组件：`frontend/user/src/components/product/OceanPurchaseButton.vue`，经典详情购买区及移动端固定购买条共用。
- 装饰：`frontend/user/public/brand/ocean-buttons/` 中的两张 PNG。图中不包含按钮文字或购物车图标；文字沿用现有翻译，图标使用现有 Lucide ShoppingCart。
- 主题范围：只在 `.brand-product-scene` 下启用海洋外观；分销站保留普通按钮，Vault 使用独立组件。
- 交互：父组件原有 `canPurchase`、`buyNow`、`addToCart` 保持不变。禁用、悬停、按下、键盘焦点保留，装饰不接收点击。
- 登录购买和联系客服按钮保持原样。背景、售价、库存、规格、订单和支付逻辑不属于本次变更。
- 本次仅在本地检查，不代表已上线。

视觉参考为本会话批准的 `exec-4486b2d0-9a74-46ce-b663-808ca87c14d8.png`，验证截图及记录见 `design-qa.md` 最新章节。
