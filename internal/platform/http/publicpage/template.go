package publicpage

import "html/template"

type document struct {
	Page
	Site                   Site
	Canonical, Description string
	MetadataTitle          string
	Images                 []string
	Body, Summary          template.HTML
	Schema                 template.JS
	Identity               template.JS
}

// The visible HTML is identical for crawlers and visitors. Vue progressively
// replaces #app with the existing storefront; no bot-only or hidden body copy.
var documentTemplate = template.Must(template.New("public-detail").Parse(`<!doctype html>
<html lang="zh-CN"><head>
<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title data-public-page-seo>{{.MetadataTitle}}</title>
<meta data-public-page-seo name="description" content="{{.Description}}">
<link data-public-page-seo rel="canonical" href="{{.Canonical}}">
{{if .NoIndex}}<meta data-public-page-seo name="robots" content="noindex, follow">{{end}}
<meta data-public-page-seo property="og:title" content="{{.MetadataTitle}}">
<meta data-public-page-seo property="og:description" content="{{.Description}}">
<meta data-public-page-seo property="og:url" content="{{.Canonical}}">
<meta data-public-page-seo property="og:site_name" content="{{.Site.Name}}">
<meta data-public-page-seo property="og:type" content="{{if eq .Kind "product"}}product{{else if or (eq .Kind "article") (eq .Kind "notice")}}article{{else}}website{{end}}">
{{range .Images}}<meta data-public-page-seo property="og:image" content="{{.}}">{{end}}
<script data-public-page-seo type="application/ld+json">{{.Schema}}</script>
<script data-public-page-seo type="application/ld+json">{{.Identity}}</script>
<link rel="stylesheet" href="/storefront-entry.css">
<style data-public-page-seo>
#public-document{max-width:960px;margin:32px auto;padding:24px;color:#173b36;background:#fff;font:16px/1.8 system-ui,sans-serif;overflow-wrap:anywhere}#public-document h1{font-size:28px;font-weight:700}#public-document h2{font-size:22px;font-weight:600;margin:1em 0 .5em}#public-document h3{font-size:18px;font-weight:600}#public-document a{color:#17675a;text-decoration:underline}#public-document p{margin:.8em 0}#public-document ul{list-style:disc;padding-left:24px}#public-document ol{list-style:decimal;padding-left:24px}#public-document img{max-width:100%;height:auto}#public-document .cover{max-height:240px;object-fit:contain}#public-document table{display:block;max-width:100%;overflow:auto;border-collapse:collapse}#public-document td,#public-document th{border:1px solid #bbb;padding:8px}#public-document dt{font-weight:600}#public-document dd{margin-bottom:8px}
</style>
<script data-cfasync="false" type="module" src="/storefront-entry.js"></script>
</head><body><div id="app"><main id="public-document">
<nav aria-label="网站导航"><a href="/">{{.Site.Name}}</a> · <a href="/products">商品中心</a> · <a href="/blog">文章</a> · <a href="/about">关于我们</a></nav>
<article><h1>{{.Title}}</h1>
{{if .PublishedAt}}<time datetime="{{.PublishedAt.Format "2006-01-02T15:04:05Z07:00"}}">{{.PublishedAt.Format "2006-01-02"}}</time>{{end}}
{{range .Images}}<img class="cover" src="{{.}}" alt="{{$.Title}}">{{end}}
{{if .Summary}}<div class="summary">{{.Summary}}</div>{{end}}
{{if .Items}}<section aria-label="在售商品"><h2>商品与当前售价</h2><ul>{{range .Items}}<li><a href="{{.Path}}">{{.Title}}</a> · {{.Price}} {{$.Site.Currency}} · {{if eq .Availability "https://schema.org/OutOfStock"}}暂时缺货{{else}}有货{{end}}{{if .Category}} · {{.Category}}{{end}}{{if .Summary}}<p>{{.Summary}}</p>{{end}}</li>{{end}}</ul></section>{{end}}
{{if eq .Kind "product"}}<p>售价：{{.Price}} {{.Site.Currency}} · {{if eq .Availability "https://schema.org/OutOfStock"}}暂时缺货{{else}}有货{{end}}</p>{{end}}
{{if .Category}}<p>分类：{{.Category}}</p>{{end}}
{{if .Facts}}<dl>{{range .Facts}}<dt>{{.Name}}</dt><dd>{{.Value}}</dd>{{end}}</dl>{{end}}
<section aria-label="完整正文">{{.Body}}</section>
{{if eq .Kind "blog"}}<section aria-label="已发布文章">{{range .Entries}}<article><h2><a href="{{.Path}}">{{.Title}}</a></h2>{{if .PublishedAt}}<time datetime="{{.PublishedAt.Format "2006-01-02T15:04:05Z07:00"}}">{{.PublishedAt.Format "2006-01-02"}}</time>{{end}}<p>{{.Summary}}</p></article>{{else}}<p>暂无匹配文章</p>{{end}}</section>{{end}}
{{if .Pagination}}<nav aria-label="文章分页">{{range .Pagination}}<a href="{{.Path}}">{{.Title}}</a> {{end}}</nav>{{end}}
{{if .Links}}<aside><h2>相关内容</h2><ul>{{range .Links}}<li><a href="{{.Path}}">{{.Title}}</a></li>{{end}}</ul></aside>{{end}}
</article></main></div></body></html>`))
