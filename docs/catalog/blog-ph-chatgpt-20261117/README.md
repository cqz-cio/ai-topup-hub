# 菲区 ChatGPT 博客发布资料

标题：**菲区 ChatGPT 充值怎么选、怎么兑**。规范地址：`https://www.qzweb.cloud/blog/chatgpt-ph-recharge-guide`。

分支为 `codex/blog-ph-chatgpt-20261117`，本次基于已复检修订 `3bd8a265a695cdb15e80051c75950c0dcad6551b`。店主已明确批准合并和发布，`publication_approved: true`。这里的发布前验证不是上线回执，生产结果以 `publication-result.json` 及最终 Git 记录为准。

## 店主四项确认

1. 保留上一轮合格修改，批准本文合并到 main 后发布。
2. 新增第六条成品号 FAQ，使用店主提供的完整表述：本站提供成品号，规格和价格通过关于页客服核实；公开目录没有独立商品页，不编造链接、规格或价格，与自有账号充值区分。
3. 文章关联商品 id **45、31、33、32**，写入前核对均为对应菲区 Pro；不一致立即停止。
4. 给 `chatgpt-code-region-check`、`chatgpt-payment-methods` 文末各加一次回链，锚文字为本文标题。

完整确认记录见 [catalog.json](catalog.json)。标题、slug、摘要和已收窄的渠道说明保持；Plus 仍未标为菲区，不增加官方地区比价或开通保证。

## 阅读与预览

- [完整 Markdown](chatgpt-ph-recharge-guide.md)、[正文 HTML](chatgpt-ph-recharge-guide.html)、[本地预览](preview.html)。下载整个目录后，直接用浏览器打开预览；本地预览保持 noindex，不作为生产入口上传。
- [事实来源](sources.json)、[上一轮事实收窄证据](review-fact-checks.json)、[发布前商品与文章核对](publication-preflight.json)。成品号 FAQ 的新增事实由店主直接确认。
- [验证记录](validation.json)、[内链检查](link-checks.json)、[旧文章定向追加清单](backlinks.json)。

HTML 为单一正文来源，Markdown、预览与 6 条 FAQPage 问答同步。预览复用现有服务端正文渲染器，仅供内容检查，不改变商城风格和布局。

## 发布与回退

发布工具为 `scripts/publish_chatgpt_ph_blog.py`，默认只读计划，执行必须提供准确内容 SHA256 和备份位置。先做一致性备份、事务演练、重复执行与定向回退检查，再执行正式事务。只新增一篇博客、四条商品关联，修改两篇旧文的中文正文；旧文章原发布日期和其他语言保持。

新文加入 `gpt-recharge.json` 的既有 `blogSlugs`，使服务端和浏览器继续使用同一作者与关联指南规则。程序发布保留现有后台、兑换服务、所有旧静态资源及生产 Nginx。

回退使用发布回执中的精确前后值：先核对没有后续编辑，再仅删除本次新文和四条关联、撤回两段回链。不得回灌整库。支付、结账、订单、商品售价、卡密、后台鉴权和 Nginx 不在修改范围。

发布后检查新文 200、自指 canonical、真实发布日期、6 条 FAQPage、四款关联商品、博客列表、sitemap 与两条回链，再请求 Google 收录；配额不足时按店主授权排到翌日。申请成功不代表已经收录或获得排名。
