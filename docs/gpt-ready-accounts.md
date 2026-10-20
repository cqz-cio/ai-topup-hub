# GPT成品号

2026-10-20 用户确认新增独立分类“GPT成品号”：

| 商品 | 售价 | 质保 | 使用限制 |
| --- | --- | --- | --- |
| GPT Plus 巴西区独享成品号 | ¥90 | 购买后24小时内，符合规则可免费更换一次 | 支持反代；有网络限制及手机绑定提示 |
| 越南 Plus 个人独享成品号 | ¥115 | 前7天免费补号；第8～30天补差价换号或按剩余价值退款 | 禁止反代；不要修改密码或2FA |

两款均为人工发货，客户先联系客服确认，再购买。微信 `Cqz0924ttxk`、QQ `1669268759`、Telegram `@czq_piestar`。

## 内容来源与运营

配置在 `docs/catalog/gpt-ready-accounts/catalog.json`，两份 HTML 正文与其放在同一目录。沿用现有透明 GPT 图标和商品页面。封禁率来自用户提供的供货说明，未独立验证，不作为零风险承诺。

没有提供可售库存数量，因此初始手动库存为 **0**。按后续批准的预览，两款商品公开展示时隐藏库存/售罄提示，主按钮统一为“联系客服”，打开微信、QQ 和 Telegram 联系方式。客户先联系客服确认库存和发货安排。微信与 QQ 可复制，Telegram 可直达。

此规则由 `frontend/user/src/utils/contactFirstProducts.ts` 精确匹配 `gpt-plus-brazil-account-24h` 和 `gpt-plus-vietnam-account-30d`，覆盖首页、商品中心、两套模板详情及手机购买栏。其他商品的库存与购买行为不变；两款商品即使日后补充库存，前端仍先显示联系入口，若要开放在线购买须另行调整规则。此功能不是后端客服审批锁。不得为展示效果虚增库存，不对接上游采购或自动发卡。

弹窗共用现有客服配置，分销站不回退到主站客服。支持明暗主题、三种语言、键盘关闭和关闭后返回触发按钮。商品售价、质保正文和后台发货方式不变。

成本尚未提供，使用系统默认值 0；这不是实际采购成本，运营应在需要计算利润前补齐。售价分别固定为本次用户确认的 90 和 115 元。交付后说明独立保存，不改历史订单。

## 数据录入

先在数据库副本验证，再备份并写入日常本机库，通过实际页面与公开接口验证后才发布：

```powershell
python -X utf8 scripts/add-gpt-ready-accounts.py --db work/codex-runtime/live/shop.db --catalog docs/catalog/gpt-ready-accounts/catalog.json
python -X utf8 scripts/add-gpt-ready-accounts.py --db work/codex-runtime/live/shop.db --catalog docs/catalog/gpt-ready-accounts/catalog.json --backup-dir work/codex-runtime/gpt-ready-accounts-20261020/local-backups --apply
```

默认仅验证并回滚；`--apply` 先做一致性备份，然后在事务内创建分类、两款商品与各一个默认规格。所有原有记录及其他表逐一核对不变。重复运行不新增、不重置库存；遇到名称、价格、内容等冲突直接回滚。正式站使用正式库，禁止上传本机数据库。

验证证据位于忽略目录 `work/codex-runtime/gpt-ready-accounts-20261020/`。只读验收不创建订单，不执行付款或采购。
