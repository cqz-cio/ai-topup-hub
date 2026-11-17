# 首页弹窗复现与轻提示提案

2026-11-17 在 Orca 实际浏览器访问公开首页。只读访问和浏览器本地关闭设置，无登录、无订单、无采购。截图来自真实页面，没有生成或改绘界面。测试最初两个公告存储键均为空；结束恢复为最初状态。

## 复现记录

| 场景 | 实测 |
| --- | --- |
| 首次访客，1289 × 931 | 弹窗“购买须知与代理合作”出现；对话框 640 × 819.27。全屏背景遮罩模糊主内容。 |
| 同次首次访问，390 × 844 | 实际 innerWidth=390；对话框 366 × 759.59，高度占 90%；页面 scrollWidth=390。 |
| 同次首次访问，320 × 740 | 实际 innerWidth=320；对话框 296 × 666，高度占 90%；页面 scrollWidth=320。 |
| 点击顶部关闭 | 对话框消失，sessionStorage `announcement_closed=63bd14ea`。 |
| 同一标签刷新 | 页面已完成渲染且弹窗不再出现，session 值保留。 |
| 清除该 session 键再刷新 | 弹窗重新出现，符合首次访客条件。 |
| 点击“今日不再提示”后刷新 | localStorage `announcement_dismiss={"version":"63bd14ea","mode":"today","date":"2026-11-17"}`，完成渲染后弹窗不再出现。 |
| “不再提示” / 错误存储值 / 旧版本 | 补充实测见 evidence/browser.json；实现还会在公告 version 变化时使旧关闭记录失效。 |

截图：[桌面](evidence/home-desktop.png)、[390](evidence/home-390.png)、[320](evidence/home-320.png)。刷新会使 Orca 视口恢复桌面，因此尺寸结果均以当次 innerWidth / innerHeight 记录为准，未把后续刷新冒充手机测试。

触发链：`views/Storefront.vue` 非 catalogOnly → 配置 announcement → `useAnnouncement.shouldShow` → `AnnouncementModal.vue`。有 version 且没有本版本永久/今日/会话关闭记录就展示。两个键是 `announcement_dismiss`（localStorage）与 `announcement_closed`（sessionStorage）。今日按浏览器本地日期。读取/写入失败会被捕获，因此不能依赖存储始终可用。

## 建议方案（只做预览，不改运行组件）

首页主标题下放一条占正常文档流的短提示：“先核对套餐和账号条件；自动发码后仍要按订单指引完成兑换。”旁边链接“购买与交付说明”，指向既有 `/blog/purchase-and-agent-guide`。可展开“查看购买提醒”，不自动展开、不加全屏遮罩、不锁焦点。代理合作入口放在展开内容或关于页原客服链接，避免与购物选择争夺首屏。

完整风险、适用条件、质保与支付注意事项继续在商品页、既有公告和教程保留；不删除或缩写它们的业务含义。公告配置与内容文章并非自动同步，后续若获批须单独核对两者。本轮不改公告正文或版本、不改支付/结账。

已有关闭偏好应被尊重；轻提示即使浏览器不能保存偏好也不能阻挡主内容。键盘可操作、关闭/展开有清楚名称，390/320 下可读；不新增自动任务。

预览的建议提示位于 [preview.html](preview.html#notice-proposal)。这是体验改善；没有本站被排名处罚的证据，不承诺搜索表现。依据：[Google 插页说明](https://developers.google.com/search/docs/appearance/avoid-intrusive-interstitials?hl=zh-cn)。
