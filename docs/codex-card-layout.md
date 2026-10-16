# Codex 商品卡片统一布局

经典商城的首页与商品中心共用 `StorefrontProductCard.vue`，Codex 六档点数和普通商品采用同一套信息结构、价格库存及选购/详情入口。

- 保留原有横图，5:3 图片区域完整显示；桌面三列，1050px 以下两列，480px 以下单列。
- 商品全名、三行交付/购买/使用与质保信息、价格库存和 44px 选购按钮保持一致。
- Codex 资格提示与预检说明集中在商品组顶部，领取与帮助说明保留在底部。
- 使用现有商品价格、库存、权限、搜索、分类与详情路由；不修改数据或支付/采购逻辑。Vault 保留原组件默认展示。
- 价格、库存和文案从现有数据及多语言字段读取。不得把预览中的库存数字写死为业务值。

实施依据：用户批准的 `work/codex-runtime/point-card-layout-preview-20261016/desktop.jpg` 和 `mobile-full.jpg`。私有验证与发布证据位于 `work/codex-runtime/codex-card-layout/`。
