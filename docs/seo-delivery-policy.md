# 交付与售后说明、商品结构化数据

用户在本地验收后授权上线。2026-11-10 新加坡商城与后端发布 `20261110-163649`，实际构建源码 `466e48a57e85b04aa229ad1e27d5227327c80f18`，基于 `55b5137facf549fd1d99b440c43a2fac02cbf689`。正式入口：[购买、交付与售后说明](https://www.qzweb.cloud/blog/purchase-and-agent-guide)。

## 内容范围

- 沿用 `/blog/purchase-and-agent-guide`，标题调整为“购买、交付与售后说明”，保留公告类型、原发布日期、付款教程、开票和代理合作信息。
- 区分系统确认付款、自动／人工交付、领取卡密、兑换和核对账号权益；补充游客及会员查单入口。
- 退款说明忠实概括 `qzweb-5` 与 `86gamestore-88` 的现有规则：人工 Plus 未发货核查、未使用卡密核查并销毁、异常充值核查、掉订阅按实际剩余时长退款、补差价重开、错误账号处理、退款方式与客服时间。明确这些细则不能推广到其他商品。
- 全部在售商品提供统一说明入口；GPT 商品与六篇指南通过原有“继续了解 GPT 充值”区域链接。GPT 主推页增加说明链接。
- 关于我们只更新原有两张服务卡片文案和原有购买须知链接的文字，卡片数量、布局、样式与交互保持。
- 不更改商品数据库的价格、库存、交付内容、原售后条款，也不新增评价或评分。

## 结构化数据

主站 `Organization.hasMerchantReturnPolicy` 使用 Google 支持的仅链接形式：

```json
{
  "@type": "MerchantReturnPolicy",
  "@id": "https://www.qzweb.cloud/blog/purchase-and-agent-guide#merchant-return-policy",
  "merchantReturnLink": "https://www.qzweb.cloud/blog/purchase-and-agent-guide"
}
```

`Product` 和 `Offer` 增加稳定标识；`Offer.seller` 引用本站 Organization，`Offer.hasMerchantReturnPolicy` 仅通过 `@id` 引用上述政策。这保留各商品实际售后差异，不虚构全站统一天数、适用国家、配送、退货费用、商品编码或品牌授权。

初始公开 HTML 与浏览器挂载后的数据使用同一政策路径。分销站不继承主站政策或说明链接，商家身份仍跟随当前租户。原价格、库存可售状态、币种、图片和摘要的数据来源保持。

Google 的可选字段提示可能继续存在。该改动不保证特定富媒体样式、评分星级、收录或排名；现已上线；Google 重新抓取后，可在 Search Console／富媒体搜索结果测试中检查正式效果。本次未验证新增收录、搜索排名或 AI 引用。

依据：[Google 商家退货政策](https://developers.google.com/search/docs/appearance/structured-data/return-policy?hl=zh-cn)的必需属性选项 B，以及[商家商品信息](https://developers.google.com/search/docs/appearance/structured-data/merchant-listing?hl=zh-cn)的全局政策 `@id` 引用规则。

## 搜索结果参考

2026-11-10 在 Chrome 的 Google“GPT充值”结果中参考 [GETGPT Pro](https://getgpt.pro/) 与 [V3 小店商品页](https://shop.gpt.ge/buy/18)：借鉴套餐适用条件、交付步骤、开通核对和售后入口的组织方式。该次结果带有个性化及日本东京地区信号，不代表固定排名。未照搬对方的价格、时效、评级、退款保证或品牌表述。截图和页面记录位于本机证据目录。

## 本地内容同步

`scripts/seo_delivery_content.py` 默认只预览；`--apply-local` 只允许本工作区 `work/` 下现有 SQLite 副本。它通过已核对线上原文的 SHA256 与明确的字段白名单更新已有公告和关于页；遇到较新文案、链接变化或文章状态变化即拒绝覆盖。

写入前备份，事务内比较更新，逐表审计全部字段并验证幂等性。保留其他语言、文章类型、发布日期与所有未列入的字段。后续上线必须重新核对线上当前文案，并按正式发布流程定向同步，不能上传本机数据库。`docs/catalog/purchase-notice.json` 同步维护相同的新文案。

## 验证与预览

- 260 项前端测试、26 项相关 Go 测试（含子案例）、13 项 SEO 内容迁移测试通过。
- 前端类型检查、生产构建及后端构建通过。
- 85 项本地 HTTP 检查通过，覆盖 25 款在售商品的真实价格、政策链接、结构化数据、完整正文和真实 404。
- 普通访客、Googlebot、GPTBot、OAI-SearchBot 的原始公开 HTML 一致；禁用 JavaScript 仍可读取完整售后说明。
- 内容迁移审计本地 70 张表，白名单外字段保持。
- 浏览器复查桌面 1920、手机 390／320 像素；检查售后正文、关于页、商品数量 1→2→1、购买按钮及关于页→说明页→GPT 主推页跳转。手机无横向溢出，原有 33 个 CSS 文件的内容与第三轮构建一致。
- 浏览器曾有一次本地 ProductDetail 模块加载失败，重载后商品页面恢复，后续未观察到新增错误；该记录保留在 `browser-verification.json`，不将其描述为全程零报错。未进行支付或订单创建。

本地入口：`http://127.0.0.1:5184/blog/purchase-and-agent-guide`；无 JavaScript 检查使用端口 `5185`。预览后端仅监听回环 `8184`，网关拒绝写请求；未进行真实下单、付款、采购或退款。该本地验证在本次上线前完成。

本机证据：`work/codex-logs/seo-delivery-20261110/`。初始 pnpm 自动联网失败、Go 检查超时的日志保留；随后使用已缓存 Go 1.26.5 和直接 Node 工具命令完成上述验证。

## 正式发布与验收（2026-11-10）

- 源码 CI 四项工作全部成功；商城、后台类型检查和生产构建通过，551 个构建文件公开资源扫描通过。生产包与已批准的本地产物对应，原有样式保持。
- 238 项公网检查通过，覆盖 25 款在售商品、完整售后正文、政策和商家引用、真实价格、canonical、sitemap、robots、真实 404 及公开入口。Googlebot、GPTBot、OAI-SearchBot、bingbot 的模拟 UA 与普通访客原始 HTML 一致；这不是 Google 已重新抓取的证据。
- 15 条正式浏览器页面／交互记录通过，覆盖桌面 1920 和手机 390／320 像素、商品数量、结算四种支付渠道及手续费、人工商品登录保护、游客查单表单、购物车、搜索、排序、关于页和客服窗口。观察到的浏览器错误为 0，所测页面无破图及横向溢出。未提交订单、付款、采购、退款或测试邮件；未验证真实交易全链路。
- 428 个当前商城文件、11,533 个保留商城文件和 1,559 个后台文件校验通过；26 个 Nginx 文件全部保持。Redis、PAIPLUS、客服进程保持，付费模型关闭。
- 生产副本验证、事务审计和正式核验覆盖 73 张表。只修改原公告的标题／摘要／正文，以及关于配置里的两段服务文案和一个链接标题。62 张表完全一致；其余为上述两张内容表，以及已逐字段核对的登录、库存／同步、目录心跳和链游标变化。售价、支付、其他设置、商品正文、映射和私有配置保持，三条 U 汇率仍为 6.65。
- 两套库存 service 均执行成功，报告时间晚于切换。临时传输文件已清理，备份保留。

备份：`/srv/zhixiang/backups/before-seo-delivery-20261110-163649`。回退须协调恢复后端和商城 `20261110-133230`，使用备份中 `incoming/content/content_release.py` 的 `rollback` 对比日志和当前值，仅回退本次四个字段；发生并发内容修改时先人工核对。不得恢复整个业务数据库。Nginx 未改，无需回退配置。

源码与发布记录归档 main，生产标签 `production/seo-delivery-20261110-163649` 指向实际构建源码。公开记录：`docs/history/seo-delivery-release-20261110.json`；私有证据：`work/codex-runtime/seo-delivery-release-20261110/verified-report.json`；截图与 HTTP／浏览器明细：`work/codex-logs/seo-delivery-release-20261110/`。
