# SEO v6 限定上线记录（2026-11-18）

店主明确授权在复核后只上线 cart 抓取例外和两处已列明内容修改。北京时间 **00:18:18**，后端切换至 **20261118-000600**；33 次正式公开请求和八项验收通过。其它 v6 草稿没有发布。

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
