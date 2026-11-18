# SEO v6 限定上线记录（2026-11-18）

店主明确授权在复核后只上线 cart 抓取例外和两处已列明内容修改。北京时间 **00:18:18**，后端切换至 **20261118-000600**；33 次正式公开请求和八项验收通过。其它 v6 草稿没有发布。

## 11 月 18 日后续只读调查

**调查结论。** 00:18 上线时的 sitemap 有 47 条。随后 3 个分类停用，既有公开目录规则使这 3 个分类页和 9 个商品页返回 404，sitemap 减至 35 条。11:50 单独上线成品号说明页后增至 36 条。本轮匿名 GET 实测，当前 sitemap 的 **36/36** 个地址均返回 200。

### 分类数据与后台操作记录

以下均为 2026-11-18 北京时间。对照 v6 切换前备份与调查时正式库，三个分类的 `is_active` 均从 1 变为 0；后台访问日志在下列时刻记录了对应状态接口的成功请求。分类及其 9 个商品的 `deleted_at` 在两次查询中均为空，9 个商品自身的 `is_active` 均为 1。

| 分类 | 状态操作时间 | 后台请求 | 影响范围 |
| --- | --- | --- | --- |
| #20 GPT 人工及覆盖充值 | 09:56:05 | `PATCH /api/v1/admin/categories/20/active`，HTTP 200 | 1 个分类页、4 个商品页 |
| #19 Telegram Premium | 09:56:37 | `PATCH /api/v1/admin/categories/19/active`，HTTP 200 | 1 个分类页、3 个商品页 |
| #11 Gemini | 09:56:44 | `PATCH /api/v1/admin/categories/11/active`，HTTP 200 | 1 个分类页、2 个商品页 |

分类开关更新分类的 `is_active`；公开商品查询同时要求商品上架、所属分类启用，sitemap 也按公开商品和启用分类生成。这些条件在 v6 之前已经存在。重新启用任一分类，可能同时公开其下仍在上架状态的商品。[分类状态写入](../internal/modules/catalog/category/infrastructure/gormstore/store.go) · [商品可见性规则](../internal/modules/catalog/product/store/gormstore/product_store.go) · [sitemap 生成规则](../internal/modules/sitemap/application/service.go)

### 12 个旧地址及入链检查

逐一请求下表地址，并扫描当前 sitemap 全部 36 页的服务端 HTML 和桌面浏览器渲染后的链接，同时匹配已登记的旧别名。下表“未发现”只覆盖这 36 个匿名公开页面，不涵盖登录后页面、历史订单或站外来源。

| 完整 URL | 原页面类型 | 实测 HTTP | 扫描范围内的入链 |
| --- | --- | --- | --- |
| <https://www.qzweb.cloud/categories/gpt-overwrite-topup> | GPT 人工及覆盖充值分类 #20 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/chatgpt-go-manual> | Go 人工充值商品 #59 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/chatgpt-plus-manual> | Plus 人工充值商品 #57 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/chatgpt-pro-100-manual> | Pro 100 美金人工充值商品 #55 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/chatgpt-pro-200-manual> | Pro 200 美金人工充值商品 #54 | 404 | 未发现 |
| <https://www.qzweb.cloud/categories/telegram-premium-links> | Telegram Premium 分类 #19 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/telegram-premium-3m> | Telegram 三个月商品 #60 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/telegram-premium-6m> | Telegram 六个月商品 #61 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/telegram-premium-12m> | Telegram 十二个月商品 #62 | 404 | 未发现 |
| <https://www.qzweb.cloud/categories/gemini> | Gemini 分类 #11 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/gemini-pro-18m-warranty> | Gemini 18 个月有质保商品 #40 | 404 | 未发现 |
| <https://www.qzweb.cloud/products/gemini-pro-18m-no-warranty> | Gemini 18 个月无质保商品 #41 | 404 | 未发现 |

### 已上线项目复核

| 项目 | 只读验收结果 |
| --- | --- |
| 成品号说明页 | 正式正文与 [main 归档](catalog/ready-account-guide/content.html) 一致：115 元/个、一个月 Plus 订阅、付款后自动交付卡密和账号信息、一个月订阅质保，质保期内掉订阅可按剩余天数退款或按优惠价补购。正文仅链接客服入口，该目标返回 200；无 Plus 商品链接或图片。页面 200、自身 canonical、SSR 正文、BlogPosting 与 BreadcrumbList、博客列表入口及 sitemap 均已核验。390px、320px 实测可见正文，无横向溢出。发布前备份与调查时的 SKU 主键集合一致，各 97 个；文章 #11 单独新增。 |
| 购物车 | 5 种公开入口各测 GET、HEAD，共 10/10 通过。入口及其查询串允许抓取，返回 200、`X-Robots-Tag: noindex, nofollow`、`Cache-Control: no-cache`。购物车未进 sitemap，结账、支付、订单、后台、鉴权及购物车子路径等 12 个对照路径仍受 robots 限制。 |
| Plus 标签与教程 | 正式目录中 #30 标签为空，四款菲区 Pro 保留原标签。区分 Plus 与四款 Pro 的批准句在教程中出现一次，旧句已移除。去标签有[店主书面确认](catalog/seo-engineering-v6-20261117/owner-confirmation.json)，未据此推定地区资格。 |
| 归档脚本 | 只读计算服务器备份内 `content_update.py` 与 [main 原件](history/seo-v6-20261118/content_update.py) 的 SHA-256，二者一致，均为 `2d2b6fb0a123d406e66a6cbec721f3186e0e7630ae3bdae561ed24a47ada90e2`。核验限于此文件，未导入或执行脚本。 |
| 搜索效果 | Google 实际收录和排名效果仍为**未验证**。本轮未登录 GSC 或请求编入索引。 |

### 仍待店主决定

**三个分类维持停用，去留待店主确认。** 访问日志能证实状态接口的请求时间和结果。已核对的提交、上线记录和书面确认中，未找到店主明确批准停用这三个分类的指令；操作者身份、停用意图与期限均待确认。本轮不恢复分类或商品、不删除页面、不新增跳转，不为补足 sitemap 数量调整可见性。

GPT 充值分类页、Plus 商品页及菲区教程的增强文案继续暂缓。首页保留现有弹窗，S1–S5 截图待取得真实、获准公开的脱敏素材后再议。此次只补充调查记录。

## 程序、备份与范围

- PR #12 的审阅提交 `a5c30b63b803f23ddea5048c631dae5c32cd5982`，Linux CI [44114800099](https://github.com/cqz-cio/ai-topup-hub/actions/runs/44114800099) 四项通过。执行时最新 main 为 `781fcd1062bbccfa4a25fe5d0f7dad051a5b8885`，robots 生成代码未漂移。
- 合并及实际构建源码：`a39a7348dfa2d81db8141a99a4dbee40b451c2c6`；合并树与审阅提交一致。Go 1.26.5，Linux amd64，`release,fullstack`，原生构建信息 `vcs.modified=false`。
- 程序 SHA256：`decd021ba2e8617633ca4af8ac73d05495e6cf284c6d80e5d422fb402dae88ac`，143,245,474 字节。733 个内嵌前端文件复用已发布原字节，不重建或上传商城、后台资源。
- 唯一运行代码修改是 `GenerateRobots` 的四条精确 Allow，保留原 `Disallow: /cart`。未改支付、结账、订单、售价算法、卡密、采购或后台鉴权。
- 备份：`/srv/zhixiang/backups/before-seo-v6-20261118-000600`。含原后端、版本引用、私有配置、Nginx/systemd 备份、`before-stage.db`、`before-cutover.db`、审批、事务与演练回执。
- 前端仍为 `/srv/zhixiang-front/releases/20261117-162000`；没有切换前端目录，没有使用旧 v4 暂存包。

## 两处内容事务

1. 商品 #30 `chatgpt-plus-code` 的 tags 从唯一的 `ChatGpt 菲区` 改为空数组，并更新该商品的 `updated_at`。
2. 文章 #10 `chatgpt-ph-recharge-guide` 的中文正文只把原“本站 Plus 兑换码没有标注菲区……”一句替换为：“Plus 与本节四款菲区 Pro 是不同套餐，请分别按各自商品说明选择，不能用 Pro 档位或资格规则替代 Plus。”

单事务核对 75 张表，其它字段及记录保持。文章表没有 `updated_at` 列，原发布日期保留。其它语言字节、六条 FAQ、URL、四款 Pro 的标签及价格库存没有被此事务改写。没有将 115 元写入任何商品或 SKU。

9 项本地事务测试通过，覆盖身份/旧值/Pro 标签漂移拒绝、重复执行不更新时间、中途失败回滚及回退冲突。正式库一致性副本的 75 表演练、重复执行拒绝及精确字段回退通过后，才执行正式事务。

## 正式公开验收

| 项目 | 结果 |
| --- | --- |
| 1. cart 抓取 | 从正式 `/robots.txt` 实际读到四条 Allow；/cart、/cart/ 及查询串可抓取。不是本地规则推断；响应仍为 public, max-age=3600，但已取得新正文。 |
| 2. 敏感路径 | /cart/checkout、/checkout、/pay、/orders/、/admin/、/auth/ 等继续禁止。 |
| 3. cart 响应 | 五种入口 GET/HEAD 共 10 次均 200、noindex/nofollow、no-cache。 |
| 4. sitemap | 47 个 URL，无 cart；未发布的成品号说明页仍 404、未加入 sitemap。 |
| 5. Plus 标签 | #30 公开列表和详情 API 的 tags 都为空。 |
| 6. Pro 标签 | #45、31、33、32 公开详情均保留原菲区标签。 |
| 7. 教程 | 旧句消失、新句恰好出现一次；6 条可见/结构化 FAQ 内容未变，自身 canonical 保留。 |
| 8. 售价 | 25 款公开商品及其 SKU 售价保持；库内全部 97 个商品、97 个 SKU 的售价前后相同。 |

首轮公开验收通过，没有把缓存中的旧 robots 当作生效，也未操作 Search Console 或声称重新收录。

## 服务与受保护内容

- 15,733 个前端文件、26 个 Nginx 文件、35 个已发布上传媒体、私有配置摘要、systemd 文件及两套共享库存脚本均保持。没有重载 Nginx。
- 后端保持 storefront；采购边界保持 `44 / 2026-10-16T13:31:15.812954Z`，自动采购开关保持原值。Redis、Nginx、兑换/Wudi、客服的进程均未重启。
- 第一家按原 timer 于北京时间 **00:20:35** 成功同步；第二家于 **00:20:53** 成功同步。两套 timer active，service Result=success、退出码 0，报告均产生于本次切换之后，均记录未调用采购端点。未另启同步助手。
- 只检查 sitemap 专用缓存前缀，本次没有待删除键；未清空 Redis。
- 成品号新页面、首页弹窗、分类对照表和 S1–S5 截图未发布。没有创建验收订单、付款、采购或操作 GSC。

## 回退

- 程序：只把 `/srv/zhixiang/current` 切回 `/srv/zhixiang/releases/20261117-162000` 并重启 `zhixiang-shop.service`，再核对健康与采购边界。前端链接、Nginx、Redis、库存任务和共享目录不回退。
- 内容：使用备份内 `content_update.py` 的 `rollback` 与 `content-receipt.json`，先对照当前值仍等于本次新值，再恢复商品 #30 的 tags/本次更新时间及文章 #10 的精确原正文。若目标字段已被后续编辑则拒绝，不能覆盖新编辑。
- **禁止整库回灌。** 一致性数据库备份只供核验、取证及定向恢复，不是恢复整个业务库的授权。

## 实际执行脚本归档

[content_update.py 原件](history/seo-v6-20261118/content_update.py) 取自上述正式备份目录下的同名文件，共 6,542 字节，SHA-256 为 `2d2b6fb0a123d406e66a6cbec721f3186e0e7630ae3bdae561ed24a47ada90e2`。静态核对了源码中的常量、SQL、导入和回执处理逻辑，未发现硬编码凭据或内嵌的生产数据快照。归档只包含脚本原件；运行时回执和数据库备份留在服务器。本次归档未导入或执行脚本。

完整私有证据在独立工作区 `work/v6-live/`，公开摘要见 [验收 JSON](history/seo-v6-release-20261118.json)。只读预检列名错误、归档共享索引状态误判等切换前问题已保留；修正后才继续执行。记录提交不触发重复部署，原主目录分支及既有改动未写入。
