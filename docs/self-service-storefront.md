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

商品详情 → 选规格和数量 → 结算 → 选择页面提供的支付渠道 → 后端确认到账 → 采购及交付 → 订单中领取。当前 U 支付支持 TRON（TRC20）、BEP20 和 Plasma（USDT0），另有微信扫码；实际可选渠道受商品与渠道配置约束。各网络不可混用，按该笔付款页的网络、地址及完整应付数量付款。

会员在“我的订单”领取；游客使用下单邮箱与查询密码查单。生产以 `-mode storefront` 运行付款监听、采购、交付和恢复任务，Redis 队列持久化；自动充值关闭。

### 卡密兑换指引

2026-10-12 用户确认：11 款 ChatGPT 卡密（渠道一、渠道二以及 iOS 渠道）统一使用 **https://cdk.bar/** 自助充值。发货时须同时提供完整卡密、兑换链接和操作步骤：验证卡密 → 核对套餐 → 按兑换页提示验证账号 → 确认充值 → 查询处理结果。商城发卡完成与客户充值完成是两个不同状态。

固定映射及文案保存在 [qzweb-redemption-guide.json](catalog/qzweb-redemption-guide.json)。Gemini 和 DEMO 不套用此兑换地址。iOS 渠道统一入口，但原有覆盖充值、30 天权益及剩余时长不叠加的说明继续保留。

现有订单会保存下单时的商品使用说明，新订单的交付页和启用后的发货邮件都会附带该说明。指引链接用完整网址作为可见文字，邮件转换为纯文本时仍会保留地址。此配置不改动卡密本身、付款与采购流程，也不回写历史订单。

目录导入脚本会自动应用这份配置，避免后续同步丢失指引。已有 SQLite 环境使用下列脚本处理商品使用说明，并修正两款 iOS 商品的过时兑换说明，无需重新导入整个目录或重新构建程序。JSON 按当前 Go 读写契约保存为 BLOB，不能直接以 SQLite TEXT 写入，否则应用会读不到说明：

```powershell
# 只读检查，不写入数据库
python scripts/redemption_guide.py --database work/codex-runtime/live/shop.db

# 正式应用前先确认目标环境；脚本先做一致性备份，再原子更新商品说明
python scripts/redemption_guide.py --database work/codex-runtime/live/shop.db --apply --backup-dir work/codex-runtime/redemption-guide-backups
```

命令应在有有限超时的执行器中运行。生产应用需使用服务器现有数据库及服务器私有备份目录，不能上传本地库覆盖生产；仅修改 Git 配置不代表线上已生效。脚本默认只读，缺少任一预期商品时停止，重复应用不会重复追加指引。

首页定期读取公开库存，最终价格、库存和权限由后端校验。真实采购只允许发生于已确认的客户付款流程，不主动创建有效采购或把验收订单改成已付款。

仓库整理只核对页面、资料、图片和构建，不进行付款或采购，不将模拟测试当作真实资金验收。旧阶段记录见 [历史资料](history/README.md)。
