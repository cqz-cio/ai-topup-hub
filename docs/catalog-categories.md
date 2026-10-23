# GPT、Codex 与 Gemini 商品分类

正式目录按服务分成 GPT、Codex、Gemini。GPT 合并原渠道一、渠道二及 iOS 的订阅商品，Codex 单独收纳六档点数，Gemini 保持原四款。渠道、开通/续费、质保等说明仍由每款商品名称和详情表达；商品售价、供货映射、库存、上架状态及历史订单不随分类迁移改变。

首页“全部商品”与左侧目录共用分类顺序，依次展示 GPT、Codex、Gemini；分类内保留既有商品顺序，Codex 点数按面额递增。搜索和分类筛选也遵循同一顺序，空分类不显示商品区块。

规则源为 `docs/catalog/catalog-categories.json`。GPT 沿用 `qzweb-category-4`，Gemini 沿用 `qzweb-category-7`，Codex 使用新 slug `codex`；旧渠道二和 iOS 分类在迁移完商品后停用，记录不删除。停用商品也按同一规则归档，但不重新上架。DEMO、空分类及其他历史类别保留各环境原状态。

使用 `scripts/reorganize-catalog-categories.py --db <目标库> --catalog docs/catalog/catalog-categories.json --backup-root <私有备份目录>` 定向迁移。先对副本和日常本机验证，再推送并在正式服务器执行同一提交；脚本先备份，再在事务内只改分类字段与 `products.category_id`，校验其他表及商品字段一致，并检查公开商品集合未变。可重复执行；遇到缺商品、分类冲突或未迁走商品时回滚，不自行猜测归属。

`sync-qzweb-catalog.py` 对旧来源分类应用同一覆盖规则；`codex-credits.json` 指向独立 Codex 分类，旧原站快照保持原样。库存同步只查询/同步现有映射，不改分类。不得为本次维护覆盖线上数据库、重启支付采购服务或提交订单。

## 商品分类整理（2026-10-15）

- 正式目录改为 **GPT 8 款、Codex 6 款、Gemini 4 款**，共 18 款公开商品。GPT 合并原渠道一、渠道二及 iOS 订阅商品，Codex 六档点数独立分类；渠道、开通/续费及质保信息保留在商品名称和详情。已下架商品保持下架，旧分类记录保留并停用。
- 分类配置与维护脚本来源 `6f60401fd1b956ab2f05d28a075987517bc41b63`，说明见 `docs/catalog-categories.md`。GPT 沿用分类 ID 9，Gemini ID 11，新增 Codex ID 15；后续同步使用 `catalog-categories.json`，避免还原旧渠道分类。
- 数据库副本的重复执行、冲突回滚及缺商品回滚通过；日常本机 11 项、正式站 12 项页面检查通过，覆盖首页、商品中心、搜索数量、刷新、详情及 1920/390/320px，无页面错误或横向溢出。本机保留独立的 iOS/DEMO 上架状态，公开商品共 21 款，不拿本机目录覆盖正式站。
- 本次只改分类及商品归属。事务内 67 张其他表与全部商品其他字段一致，公开商品集合、商品/SKU 售价及详情核对一致。上海商城仍为 `20261015-230538`，新加坡后端及后台仍为 `20261015-160016`；后端/Redis/库存 timer 进程与配置未变，没有上传本机库、重启服务、创建订单或执行实际付款采购。
- 线上数据库备份 `/srv/zhixiang/backups/before-product-categories-20261015T154259351422Z`，配置备份 `/srv/zhixiang/backups/before-product-categories-config-20261015T154259350695Z`。外层私有报告在 `deployment/local/product-categories/`，浏览器证据在 `work/codex-runtime/product-categories-qa/`。

## 首页商品展示顺序（2026-10-16）

- 上海商城静态发布 `20261016-092713`，干净源码 `1a5d4dcfddbfe5c2e8da2fc6fefdfe190edc09c9`。首页“全部商品”按目录导航动态排序为 **GPT 8 款 → Codex 6 款 → Gemini 4 款**；分类内原顺序和 Codex 点数说明保留，搜索及分类筛选遵循同一顺序，空分类不显示商品区块。
- 138 项商城测试、类型及生产构建通过；日常本机、最终构建及正式站均检查 1920/390/320px，分类筛选和搜索通过，没有脚本错误或横向溢出。6 项公网入口/资源哈希与发布包一致，258 个商城构建文件及 1165 个保留后台文件通过核验。
- 新加坡后端及上海后台仍为 `20261015-160016`；商品售价、支付渠道、公告、配置、后端和 Redis 进程保持一致，未修改正式数据库、上传本机库或执行真实付款采购。备份为两地各自的 `backups/before-catalog-order-20261016-092713`，私有证据 `work/codex-runtime/catalog-order/verified-report.json`。
