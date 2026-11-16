# SEO v5 商品文案待确认稿

13 篇 HTML 是本地草稿，`copy.json` 状态为 `pending_operator_confirmation`。目前没有生产审批文件。依据 2026-11-16 公开商品接口，保留原文的适用账号、订阅期限、退款公式、责任及客服条款；只替换商品开篇并新增独立选购说明、比较与 FAQ。

需确认事项列在 `copy.json.requires_confirmation`。所有未知权益保持未知，不作官方授权、额度、开通速度或退款到账时限承诺。Telegram 三个选项已知差别仅为时长，不编造额外功能、优惠或适用场景。

`before.json` 只含公开商品文案、标题、分类及图片元数据，用于防止覆盖更新后的运营内容。Plus 两页和 Grok 八页不进入文案更新范围。Gemini 两页只替换主图和分享图，不改质保说明；分类图同步替换。

本地预览：`python scripts/seo_v5_content.py --database work/seo-v5/shop.db --apply-local --backup-root work/seo-v5/backups`。该选项限制数据库必须处于本工作区 `work/` 内。

正式操作默认为只读计划；`--apply` 需要 `--approval-file` 和 `--backup-root`。审批文件格式为 `status: approved`、`copy_sha256: 只读计划实际输出的摘要`、`confirmed_by: 实际确认人`。摘要绑定清单、13 篇 HTML、基线和 Gemini 图片字节。修改任一内容后旧审批失效。不要把本段示例当成已确认，不要在合并或 CI 中自动执行写库。
