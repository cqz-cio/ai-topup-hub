# 绿色商城：商品与购买流程

正式前台入口是 `frontend/user/src/views/Storefront.vue`。运行方式以 [本地开发](local-development.md) 和 [生产维护](../ops/tencent/README.md) 为准，旧 React 展示页已归档。

## 当前目录

| 分类 | 商品数 |
| --- | ---: |
| ChatGpt 渠道一 | 5 |
| ChatGPT渠道二 | 4 |
| Gemini | 4 |
| 【荐】ins/真人老号[全网最低价] | 0 |
| Chatgpt ios渠道 | 2 |
| DEMO | 1 |

资料在 [qzweb-all.json](catalog/qzweb-all.json)，售价覆盖在 [qzweb-price-overrides.json](catalog/qzweb-price-overrides.json)。`qzweb-55` 的本站售价为 1160.00 元，不能被原站快照的 1150.00 元或商户进价覆盖。

商品使用 `qzweb-*`，分类使用 `qzweb-category-*`。老 `zxa-*` 商品保留历史记录，本地支付验收商品已下架，均不作为当前公开目录。

## 数据来源

首页与详情从公开 API 读取商品和库存，Git 中的 JSON 资料不会在拉取时自动导入数据库。

`scripts/sync-qzweb-catalog.py` 通过指定后台导入目录，先备份商品资料并应用价格覆盖。它不会创建供货凭据、真实卡密、订单或采购。封面和图标位于 `frontend/user/public/catalog-products/qzweb/`，共 18 个现用文件。

12 款映射见 [qzweb-ellaixshop-mappings.json](catalog/qzweb-ellaixshop-mappings.json)。`qzweb-52/53/62/1` 没有对应映射，保留缺货。生产库存由独立定时器同步，不写死历史数量。

当前本地库是 `work/codex-runtime/live/shop.db`。2026-10-12 从生产公开 GET 接口导入 16 款商品、6 个分类和库存快照。备份和报告位于忽略的 `work/codex-runtime/catalog-snapshot-*`；本地库存连接停用、采购关闭，没有复制生产密钥或真实卡密。

## 购买与交付

商品详情 → 选规格和数量 → 结算 → USDT/BEP20 支付 → 后端确认到账 → 采购及交付 → 订单中领取。

会员在“我的订单”领取；游客使用下单邮箱与查询密码查单。生产以 `-mode storefront` 运行付款监听、采购、交付和恢复任务，Redis 队列持久化；自动充值关闭。

首页定期读取公开库存，最终价格、库存和权限由后端校验。真实采购只允许发生于已确认的客户付款流程，不主动创建有效采购或把验收订单改成已付款。

仓库整理只核对页面、资料、图片和构建，不进行付款或采购，不将模拟测试当作真实资金验收。旧阶段记录见 [历史资料](history/README.md)。
