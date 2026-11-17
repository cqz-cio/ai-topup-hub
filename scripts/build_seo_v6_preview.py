#!/usr/bin/env python3
"""Build a standalone review artifact, never a storefront publication."""
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
DOCS = ROOT / "docs/catalog/seo-engineering-v6-20261117"
DRAFTS = [
    ("category", "GPT 分类页 · 选择对照补充", "draft-category.html"),
    ("plus", "Plus 商品页 · 操作说明补充", "draft-plus.html"),
    ("ph", "菲区教程 · 路线对照补充", "draft-ph-guide.html"),
    ("account", "成品号说明页 · 尚未注册 URL", "draft-ready-account.html"),
]


def build():
    sections = []
    for anchor, title, filename in DRAFTS:
        content = (DOCS / filename).read_text(encoding="utf-8")
        sections.append(f'<section class="draft" id="{anchor}"><p class="eyebrow">审阅草稿 · {title}</p>{content}</section>')
    html = """<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex, nofollow"><title>SEO v6 第一阶段 · 审阅预览</title>
<style>
:root{font-family:system-ui,"Microsoft YaHei",sans-serif;color:#183b36;background:#f1f5f3;line-height:1.75}
*{box-sizing:border-box}body{margin:0}header,main{max-width:1040px;margin:auto;padding:24px}
header h1{font-size:clamp(23px,4vw,36px);line-height:1.3;margin-bottom:12px}nav{display:flex;flex-wrap:wrap;gap:8px 22px}
a{color:#146653;text-underline-offset:3px;overflow-wrap:anywhere}a:focus-visible,summary:focus-visible{outline:3px solid #207d66;outline-offset:4px}
.warning{background:#fff1cd;border-left:4px solid #a06500;padding:14px 18px;border-radius:8px}
.draft{padding:24px;background:white;border:1px solid #d5e2db;border-radius:14px;margin-bottom:26px;scroll-margin-top:16px}
.eyebrow{font-size:14px;color:#5a6d64;border-bottom:1px solid #d5e2db;padding-bottom:14px;font-weight:700}
h2{font-size:22px;line-height:1.45}h1{font-size:28px}td,th{text-align:left;vertical-align:top;padding:12px;border:1px solid #d5e2db}
table{width:100%;border-collapse:collapse;font-size:14px}th{background:#edf4ef}.table-scroll{overflow-x:auto}td{min-width:130px}
.pending{background:#fff1cd}li{margin-block:8px}details{padding:12px;border:1px solid #c8dbcf;border-radius:10px}summary{cursor:pointer;font-weight:650}
.notice{border:1px solid #c8dbcf;border-radius:10px;background:#f5faf7;padding:14px;margin-bottom:12px}
.proposed-main{min-height:190px;padding:24px;border:1px dashed #9cb9ab;border-radius:10px;background:#f7faf8}
small{color:#52645c}@media(max-width:600px){header,main{padding:16px}.draft{padding:16px}h2{font-size:20px}ul,ol{padding-left:24px}}
</style></head><body>
<header><p>π星卡社 / SEO v6 / 第一阶段</p><h1>让规则可核对，让文案有依据</h1>
<div class="warning">仅供店主与 Grok 审阅。已按店主回复整理：去掉 Plus 的菲区标签，保留商品原有资格；成品号为 Plus 一个月订阅、订阅质保一个月、115 元/个。此处是更新后的审阅稿，尚未发布。</div>
<p>这里展示增量草稿，不替换已发布 v5 正文、退款条款或六条菲区 FAQ。商品链接打开现网已发布内容，不代表本稿已上线。</p>
<nav><a href="#category">分类对照</a><a href="#plus">Plus 操作</a><a href="#ph">菲区路线</a><a href="#account">成品号</a><a href="#notice-proposal">轻提示提案</a><a href="owner-questions.md">书面确认清单</a></nav></header><main>
""" + "\n".join(sections) + """
<section class="draft" id="notice-proposal"><p class="eyebrow">交互提案 · 未改商城组件或公告配置</p>
<h2>首页购买提醒：不遮住商品</h2>
<div class="notice">先核对套餐和账号条件；自动发码后仍要按订单指引完成兑换。
<a href="https://www.qzweb.cloud/blog/purchase-and-agent-guide">购买与交付说明</a></div>
<details><summary>查看购买提醒</summary><p>套餐及账号条件以商品详情为准。收到卡密后，还需进入订单交付说明的兑换入口，核对目标账号与套餐并完成兑换。</p>
<p>完整风险与购买条件继续在<a href="https://www.qzweb.cloud/blog/purchase-and-agent-guide">既有说明</a>保留。代理合作及疑问请通过<a href="https://www.qzweb.cloud/about">原客服入口</a>咨询。</p></details>
<div class="proposed-main"><h3>页面主要内容仍在正常文档流中</h3><p>提示默认收起，无全屏遮罩、无强制焦点跳转；本区域只是评审布局示例，不是商品或库存数据。</p></div>
</section><small>本地预览 / noindex / 无写库与发布功能。截图需求见 owner-questions.md；没有伪造订单或兑换截图。</small>
</main></body></html>
"""
    (DOCS / "preview.html").write_text(html, encoding="utf-8")
    print(DOCS / "preview.html")


if __name__ == "__main__":
    build()
