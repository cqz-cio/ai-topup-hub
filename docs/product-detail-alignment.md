# 商品详情排版与兑换入口交付范围

用户 2026-10-16 的标注明确要求：规格移到价格右侧，商品描述与购买须知重新排版、两栏齐边；具体兑换网址只在付款后的订单交付中展示。该要求取代此前售前展示兑换入口的记录。

经典详情页的图片、描述与资格摘要集中在左栏，标题、价格/规格、数量和操作在右栏，两张卡片共用一行高度。原商品正文按既有标题分区，不改套餐条件、质保承诺、价格或购买代码。390px 保留价格/规格并排，320px 改为上下排列；手机购买须知仍在购买按钮前。

`ProductRedemptionGuide` 只说明付款后查看订单交付。经典和 Vault 均不再补入外部兑换链接；商城、关于页默认文案与三语言文案不含该域名。公开数据通过 `scripts/public_redemption.py` 精确更新商品公开字段、公告正文、首页公告和关于页配置。每次应用先做一致性备份，事务内比对全部表，拒绝任何范围外字段变化；售后 `instructions_json`、历史订单、价格、支付配置及采购信息保持原值。

```powershell
python -B scripts/public_redemption.py --database work/codex-runtime/live/shop.db
python -B scripts/public_redemption.py --database work/codex-runtime/live/shop.db --apply --backup-dir work/codex-runtime/product-detail-alignment-20261016/local-backups
```

商品导入和 Codex 维护脚本同步使用公开文案处理函数，交付指引继续保留具体地址。历史商城中含兑换网址的 45 个资源加入 Nginx 精确撤下清单，与原有 6 个地址共同返回 410/no-store。回滚旧程序时也要保留此规则；已被浏览器或第三方下载的副本无法撤回。

验证证据位于忽略目录 `work/codex-runtime/product-detail-alignment-20261016/`：143 项商城回归、13 项迁移/交付保留测试、日常 5173 的桌面和窄屏检查、公开 API 扫描、最终静态构建扫描与页面截图。测试没有提交订单或进行真实付款、采购。

## 发布结果

正式版本 `20261016-104524`，构建源码 `6aa97dc1d31ea8bd0878554fcbb1b5a03e825d72`。7 项线上页面检查、21 项公开 API、7 项入口和资源哈希、51 项撤下地址检查通过。原交付说明、售价、支付渠道与后端/Redis 进程保持一致，精确文案迁移修改 14 个公开字段。两地备份及私有报告见生产基准。
