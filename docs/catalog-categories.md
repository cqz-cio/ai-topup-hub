# GPT、Codex 与 Gemini 商品分类

正式目录按服务分成 GPT、Codex、Gemini。GPT 合并原渠道一、渠道二及 iOS 的订阅商品，Codex 单独收纳六档点数，Gemini 保持原四款。渠道、开通/续费、质保等说明仍由每款商品名称和详情表达；商品售价、供货映射、库存、上架状态及历史订单不随分类迁移改变。

规则源为 `docs/catalog/catalog-categories.json`。GPT 沿用 `qzweb-category-4`，Gemini 沿用 `qzweb-category-7`，Codex 使用新 slug `codex`；旧渠道二和 iOS 分类在迁移完商品后停用，记录不删除。停用商品也按同一规则归档，但不重新上架。DEMO、空分类及其他历史类别保留各环境原状态。

使用 `scripts/reorganize-catalog-categories.py --db <目标库> --catalog docs/catalog/catalog-categories.json --backup-root <私有备份目录>` 定向迁移。先对副本和日常本机验证，再推送并在正式服务器执行同一提交；脚本先备份，再在事务内只改分类字段与 `products.category_id`，校验其他表及商品字段一致，并检查公开商品集合未变。可重复执行；遇到缺商品、分类冲突或未迁走商品时回滚，不自行猜测归属。

`sync-qzweb-catalog.py` 对旧来源分类应用同一覆盖规则；`codex-credits.json` 指向独立 Codex 分类，旧原站快照保持原样。库存同步只查询/同步现有映射，不改分类。不得为本次维护覆盖线上数据库、重启支付采购服务或提交订单。
