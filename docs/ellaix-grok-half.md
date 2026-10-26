# Grok 月付半价商品

2026-10-26 用户确认三款售价后正式新增到原 Grok 分类 #23；原有八款正价商品、价格和历史订单保留。

| 源头 ID | 商品 | 固定售价 | 本次签名成本 | 正式商品/SKU | 映射 |
| --- | --- | --- | --- | --- | --- |
| 116 | Grok Premium 月付半价 | 138.00 | 108.80 | 81 | 47 |
| 117 | Grok Super 月付半价 | 398.00 | 345.00 | 82 | 48 |
| 118 | Grok Ultra 月付半价 | 818.00 | 700.00 | 83 | 49 |

源头为 ELLAIXSHOP 分类 #35 `grok半价`，不是分类 #32 `正价Grok，半价可退费`。三款均为自动发码、单规格、无额外表单，本次没有标价 9999 的占位商品。付款后按已有商户 1039 / 连接 #1 自动采购交付，`auto_sync_price=false` 保持。报价和库存只代表核验快照；后续涨价须复核成本上限，不得开启自动改价覆盖固定售价。

标题、摘要、标签和详情明确标注半价及优惠资格要求：客户账号必须具备对应优惠资格，半价兑换码不能用于普通全价套餐，适用条件、实际扣款和生效结果以兑换页核验为准。半价商品不沿用其他分类的退款或质保承诺。本站售价以页面为准。兑换入口按订单实际交付内容提供，公开页面不添加未经核验的入口。

沿用现有同档月付封面 `/uploads/catalog/grok-20261023/grok-<tier>-monthly.svg`，没有新增媒体、改样式、发布前端或修改 Nginx。原商品记录未修改，Grok 分类现在共 11 款商品，商城公开商品 35 款。

维护：`scripts/add-ellaix-grok-half.py`，配置与 HTML 为 `docs/catalog/ellaix-grok-half/`。脚本复用 `scripts/replace-ellaix-claude-x.py` 的 `require/encode/snapshot/insert` 函数，分发维护包时两者必须同时保存。默认事务演练并回滚；应用需 `--apply --backup-root` 和最近一小时内的签名只读快照。固定商品身份及批准售价，拒绝占位价格、人工交付、额外表单、重复映射和超售价成本；所有既有行必须逐表一致，重复运行只核验不覆盖运营修改。

```text
python -B scripts/tests/test_add_ellaix_grok_half.py
python -B scripts/add-ellaix-grok-half.py --db <目标库> --catalog docs/catalog/ellaix-grok-half/catalog.json --source <私有签名快照>
```

7 项离线测试及正式库副本演练、应用、重复执行通过。正式事务核验 69 张表，只新增 3 商品、3 SKU、3 商品映射和 3 SKU 映射，所有原有行保持。公网三款价格、详情、封面和原八款正价商品验证通过；正式浏览器核验 Premium 半价详情、资格提示、库存及 138 元售价。原库存 service 成功覆盖三款，售价和上游余额保持，未进行真实付款采购或创建验收订单。

后端仍 `20261020-144132`，后端/Redis PID、配置和库存 timer 保持。本机离线业务库未覆盖，维护源码与 Claude、X 商品变更统一提交至 main。

备份：`/srv/zhixiang/backups/before-grok-half-20261026T024457742937Z/shop.db`；服务器维护包：`/srv/zhixiang/shared/maintenance/ellaix-grok-half-20261026`。私有证据：`work/codex-runtime/ellaix-grok-half-20261026/` 中 `validation.json`、`applied.json`、`public-report.json`、`sync-verified.json` 和 `production-premium-half.png`。回退应审阅后定向下架新商品，不回灌整库、删除历史订单或重放采购。
