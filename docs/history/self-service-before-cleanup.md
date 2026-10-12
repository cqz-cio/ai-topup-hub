> 历史归档（2026-10-12）：本文保留当时状态、命令与验收记录，不作为当前操作指南。现行说明见 [项目入口](../../README.md)、[本地开发](../local-development.md) 和 [生产环境](../../ops/tencent/README.md)。余额、库存和私有文件路径可能已经变化。

# 自助下单与账号／卡密交付

生产商城首页位于 `frontend/user/src/views/Storefront.vue`，已经接入现有 Vue 用户端。`frontend/catalog` 保留原 React 展示页与商品资料作为参考，不需要部署第二套首页，也不要用它的构建产物覆盖用户端。

2026-10-12 已在服务器发布界面完整显示修复 `20261012-004733`。商品封面采用原图 5:3 比例与完整适配，长标题、购买说明和库存可换行；1280px 以下启用展开导航与底部导航，手机窄屏使用单列商品，分类名称不再横向截断，页脚留出底栏和安全区空间。Vue 类型检查、生产构建以及公网六种宽度的商品、搜索、分类、菜单和页脚验证通过。此次只升级程序，保留全部商品及 SKU 售价、默认中文、真实库存同步和禁止实际采购的设置，线上数据库未被本机库覆盖。

## 购买链路

首页从 `/api/v1/public/products` 分页读取后台上架商品，使用后台提供的 slug、标题、价格、促销价和库存状态。分类与搜索包含后续新上架商品；接口失败、价格无效或无可售规格时不保留可购买入口。回到页面或每隔 60 秒重新获取库存，最终成交价格与库存仍由商品详情、下单预览及后端交易校验决定。

购买按钮进入 `/products/:slug`，继续复用选规格、数量、购物车或立即购买、`/checkout`、`/pay`。登录买家通过“我的订单”查看交付内容；游客使用下单邮箱与查询密码通过 `/guest/orders` 查单、复制和下载。

首版交付的是账号凭据或充值卡密。代充值商品由买家按交付说明兑换，不在本次购买流程中收集 Session 或执行账号充值。

## 商品资料

2026-10-11 用户确认同步 [QZWEB 网站](https://www.qzweb.cloud/cat/4) 的全部公开分类。六个分类及公开列表中实际可见的 16 款商品已全部同步，标题、公开售价、封面、兑换步骤和质保条件与该站一致；不同渠道、开通、续费、预售及不同质保分别保留。完整资料保存在 [qzweb-all.json](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/docs/catalog/qzweb-all.json)，其中价格为公开零售价，不是商户采购成本。分类统计中的商品数可能包含非公开商品，此次只同步实际公开列表中的商品。

用户随后指定渠道一「ChatGPT Pro 20新老号(到期)开通 质保一个月」（`qzweb-55`）售价由 1150.00 调整为 1160.00 元。本站商品及 SKU 使用此价格；来源快照保留原价，单独的 [qzweb-price-overrides.json](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/docs/catalog/qzweb-price-overrides.json) 保存用户调价。同步工具会优先使用该文件，重新同步时也保留此定价。

| 分类 | 公开商品数 | 展示情况 |
| --- | ---: | --- |
| ChatGpt 渠道一 | 5 | Go、Plus、Pro 开通与续费 |
| ChatGPT渠道二 | 4 | 独立保留渠道二售价与说明 |
| Gemini | 4 | 两款自动交付、两款人工预售 |
| 【荐】ins/真人老号[全网最低价] | 0 | 保留空分类 |
| Chatgpt ios渠道 | 2 | 原站售价均为 9999.00 元，原站已售罄 |
| DEMO | 1 | 保留“演示商品”说明，原站已售罄 |

本站 slug 为 `qzweb-{原站商品ID}`，分类 slug 为 `qzweb-category-{原站分类ID}`。原 `zxa-*` 商品目录及八个旧分类均已下架或停用，商品记录和历史订单保留。14 款自动交付商品和两款人工预售商品保留原交付类型；未导入本站卡密或设置人工可售库存，不把原站页面库存冒充真实供货库存。

2026-10-12 已按商品共享编码将 12 款现有商品及 SKU 绑定到 ELLAIXSHOP 商户 1039，其中 11 款有库存，iOS Pro 5x（`qzweb-59`）库存为 0。两款 Gemini 人工预售（52、53）、iOS Pro 20x（62）和 DEMO（1）未出现在该商户接口里，保留缺货。对应关系记录在 [qzweb-ellaixshop-mappings.json](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/docs/catalog/qzweb-ellaixshop-mappings.json)。内部交付类型为 upstream，公开页面仍显示自动交付；首页及详情页显示真实库存数量。

连接关闭自动同步售价，库存刷新保留全部售价及用户指定的 Pro 20 开通款 1160.00 元。本站保存的是加密商户密钥；浏览器不接触该密钥。本机库存助手每 5 分钟调用现有映射同步接口，首页每 60 秒和重新获得焦点时获取新的公开库存。刷新工具 [sync-ellaixshop-stock.py](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/scripts/sync-ellaixshop-stock.py) 不调用订单、付款或采购接口。2026-10-12 已将这些配置部署到腾讯云，服务器每 5 分钟由 systemd 定时器同步库存。当前 API 模式关闭 Redis/采购队列，采购开关 `UPSTREAM_PURCHASE_ENABLED=false`，连接测试前后余额均为 0.00 元，未做真实采购或发货；部署维护见 [腾讯云运行说明](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/ops/tencent/README.md)。

本机 `http://127.0.0.1:5173/` 展示数据已通过后台 API 更新；运行库 `work/codex-runtime/brand-preview/shop.db` 和项目根目录下的 `deployment/local/db/dujiao.db` 均已同步，避免重建预览时恢复旧商品。首页分类按公开分类接口排序展示，包含没有商品的分类；分类接口暂时不可用时回退到当前商品中的分类。修改前数据库与商品记录备份位于根目录下被 Git 忽略的 `deployment/local/backups/`。2026-10-12 已将当前全栈版本、16 款商品和 12 款映射部署至 [线上商城](http://43.134.141.10/)，线上持久化库为 `/srv/zhixiang/shared/db/shop.db`。

可用 `scripts/sync-qzweb-catalog.py` 将已核对资料应用到指定后台，默认使用全站资料 `docs/catalog/qzweb-all.json`；通过 `--manifest` 也可明确指定单分类快照。凭据从调用者指定的本地文件读取，不在工具、资料文件或输出中保存；参数 `--credentials-file` 和 `--backup-dir` 为必填。工具通过后台 API 保存商品，先备份旧商品资料，并逐项核对公开接口的标题、售价、SKU 价格、分类、交付类型、封面和详情；不创建供货连接、库存、支付或采购订单。封面及分类图标随用户端放在 `/catalog-products/qzweb/` 下，部署时应一起发布。重新同步原站新变化前，应先刷新并核对资料快照与图片。

本次公开接口逐项验证全部 16 款商品、六个分类和 18 个图片文件通过；分类与商品展示顺序符合原站。两个数据库的历史订单、支付、用户、卡密、供货连接、映射及采购记录均与备份一致。首页商品逻辑的五项测试、Vue/TypeScript 检查和用户端生产构建通过。只读核对报告位于根目录下被 Git 忽略的 `deployment/local/qzweb-all-verification.json`。

`frontend/user/src/data/storefrontCatalog.ts` 只补充旧目录商品的使用期限、质保和展示图，按 `zxa-*` slug 匹配，不包含售价与库存。新商品从后台读取实际资料，不套用这些旧目录说明。调整仍在售旧商品的使用规则时，应同时更新补充资料和后台商品详情。

图片放在用户端 `/catalog-products/` 下，避免和 `/products/:slug` 的交易路由冲突。原目录的 Gmail 商品错误引用了 Gemini 图片，首页对这一旧图使用修正后的 Gmail 展示图；后台配置其他新图片时仍优先展示后台图片。

原资料中 Pro 5X 图片写“不支持提前续费”，说明文本却写“可提前续费”。正式上架前应确认供应商实际规则并统一详情与图片，不能把两种说法同时作为购买承诺。

## 运行与部署

继续使用现有 `frontend/user` 构建命令、Dockerfile 和 fullstack 发布流程。新首页及其静态素材随用户端构建一起发布，现有 Go 用户端 SPA 路由和 API 转发保持兼容；没有引入 React/Vinext 生产构建依赖。

正式自动交付需完成：

1. 后台商品选择自动交付，并按对应商品规格导入真实账号／卡密库存；在交付说明中写清使用或兑换方法。不要用人工无限库存替代自动发货库存。
2. 启用仓库已有的 **USDT · BNB Smart Chain（BEP-20）** 收款。在服务器配置 `bsc_usdt.enabled: true`，沿用已确定的收款地址、汇率及确认数，并在后台启用对应渠道。前端会从订单可用渠道接口读取 `bscusdt / usdt-bep20`，在结算页供游客和会员选择；链上监听核验到账后通过原支付服务入账。具体配置见 [USDT BEP-20 收款说明](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/docs/usdt-bep20-payments.md)。
3. 在 `config.yml` 启用并配置 Redis 队列；运行 `-mode all`，或同时运行 API 与 worker。支付成功会入队，由 worker 消耗对应规格卡密并将订单完成。只有 API 进程无法完成这条普通卡密发货链路。

早期回归验证使用独立 SQLite、内存 Redis 协议测试服务及本地 TLS JSON-RPC；这些记录保留在忽略的 `work/codex-runtime/`。2026-09-30 本机商城已切换到正式后端入口、真实主网 RPC、独立正式数据库和 Redis 8 的 AOF 持久化队列，模拟后端及内存 Redis 已停止。八款原商品没有导入演示卡密，真实库存待后台导入；单独的 0.10 元支付验收商品交付唯一凭证，并明确不含账号或充值权益。当前环境和验收入口见 [本机主网支付验收](https://github.com/cqz-cio/ai-topup-hub/blob/baseline/green-20261012/docs/local-mainnet-acceptance.md)。真实订单的资金到账及自动发货仍待实际转账验收。

## USDT 收银台

当前已有配置为 BNB Smart Chain 主网（chain ID 56），收款地址 `0x66a8bab15067eab1efc948ac03684f0397c8416f`，汇率 `1 USDT = 6.66 CNY`，至少 15 个区块确认并满足最终确认。正式服务器可使用以下配置段：

```yaml
bsc_usdt:
  enabled: true
  recipient: "0x66a8bab15067eab1efc948ac03684f0397c8416f"
  cny_per_usdt: "6.66"
  confirmations: 15
```

默认主网区块与日志节点使用 PublicNode，收据节点使用 BNB 官方 RPC；两端分别核验网络与代币，并核对收据、事件和区块 hash。服务端可通过 `BSC_RPC_URL` 设置单一稳定的 HTTPS RPC，必要时再用 `BSC_RECEIPT_RPC_URL` 单独配置收据节点。运行 `-mode all`，或同时运行 API 和 worker，使到账监听及卡密发货持续工作。已有渠道若被后台停用，配置启用不会替管理员重新打开，需要在渠道管理中明确启用。

收银台展示网络、收款地址、地址二维码及后台分配的六位小数 USDT 金额，金额和地址旁都有清晰的复制按钮。每笔付款保留汇率快照与独立金额尾数；订单账本保留人民币金额。客户按完整金额到账，平台转账手续费另计。重新打开待付款订单会恢复原支付单及金额，链上确认后页面自动转到订单详情并继续刷新交付内容。

## 验证范围

用户端完整测试包含价格与库存、游客凭证保护、订单刷新等；生产构建经过 TypeScript 检查。后端支付、订单及发货相关测试通过。

Chrome 本地联调验证首页到选规格、游客结算、模拟到账验签、金额匹配、队列 worker 自动交付、查单和下载。无效签名、错误金额、错误游客密码被拒绝，重复支付回调没有重复交付。页面同时检查桌面与手机布局、分类搜索、无结果状态和浏览器错误。

会员余额支付也经过实际 API 与 worker 验证：余额只扣除一次，不产生线上支付金额，交付对应商品规格的测试卡密。测试服务和凭证所在的 `work/` 同时被 Git 与 Docker 构建上下文排除。

USDT BEP-20 完整联调包含游客下单与会员线上付款、网络／地址／二维码及六位金额展示、刷新页面恢复原支付单、未确认转账保持待付款、最终确认后队列发货与自动跳转、重复转账不重复交付、查单下载和会员余额未扣款。收银台手机布局无水平溢出，结算页的渠道名完整换行，保留金额限制提示。对应链上金额分配、RPC 校验、确认规则、异常到账及套餐卡密发放测试通过；最终用户端生产构建通过。
