package publicpage

import (
	"bytes"
	"encoding/json"
	"html/template"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/platform/publiccontent"
	htmlnode "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

type questionAnswer struct{ Question, Answer string }

func contentImageDimensions(body string) string {
	nodes, err := htmlnode.ParseFragment(strings.NewReader(body), &htmlnode.Node{Type: htmlnode.ElementNode, Data: "div", DataAtom: atom.Div})
	if err != nil {
		return body
	}
	var visit func(*htmlnode.Node)
	visit = func(node *htmlnode.Node) {
		if node.Type == htmlnode.ElementNode && node.Data == "img" {
			var raw string
			for _, attr := range node.Attr {
				if attr.Key == "src" {
					raw = attr.Val
				}
			}
			if size := publiccontent.PublicImageSize(raw); size.Width > 0 && size.Height > 0 {
				attrs := make([]htmlnode.Attribute, 0, len(node.Attr)+2)
				for _, attr := range node.Attr {
					if attr.Key != "width" && attr.Key != "height" {
						attrs = append(attrs, attr)
					}
				}
				node.Attr = append(attrs, htmlnode.Attribute{Key: "width", Val: strconv.Itoa(size.Width)}, htmlnode.Attribute{Key: "height", Val: strconv.Itoa(size.Height)})
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	var out bytes.Buffer
	for _, node := range nodes {
		visit(node)
		if err := htmlnode.Render(&out, node); err != nil {
			return body
		}
	}
	return out.String()
}

// Read questions from the same sanitized visible body. Only explicitly labelled
// FAQ sections and paragraph-leading strong questions participate; ordinary
// product headings must never become invented Q&A markup.
func visibleFAQs(body string) []questionAnswer {
	root, err := htmlnode.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}
	var result []questionAnswer
	seen := map[string]bool{}
	active := false
	var walk func(*htmlnode.Node)
	walk = func(node *htmlnode.Node) {
		if node.Type == htmlnode.ElementNode {
			if len(node.Data) == 2 && node.Data[0] == 'h' && node.Data[1] >= '1' && node.Data[1] <= '6' {
				label := nodeText(node)
				active = strings.Contains(label, "常见问题") || strings.EqualFold(label, "FAQ")
			}
			if active && node.Data == "p" {
				first := node.FirstChild
				for first != nil && first.Type == htmlnode.TextNode && strings.TrimSpace(first.Data) == "" {
					first = first.NextSibling
				}
				if first != nil && first.Type == htmlnode.ElementNode && first.Data == "strong" {
					question := nodeText(first)
					var answer strings.Builder
					for next := first.NextSibling; next != nil; next = next.NextSibling {
						answer.WriteString(nodeText(next))
						answer.WriteByte(' ')
					}
					text := strings.Join(strings.Fields(answer.String()), " ")
					if question != "" && text != "" && !seen[question] {
						result = append(result, questionAnswer{question, text})
						seen[question] = true
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(root)
	return result
}

func nodeText(node *htmlnode.Node) string {
	if node.Type == htmlnode.TextNode {
		return node.Data
	}
	var result strings.Builder
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		result.WriteString(nodeText(child))
	}
	return strings.TrimSpace(result.String())
}

func documentSemantics(page Page, body, origin string) template.JS {
	links := []Link{{Title: "首页", Path: "/"}}
	switch page.Kind {
	case "product":
		if page.CategoryPath != "" {
			links = append(links, Link{page.Category, page.CategoryPath})
		}
	case "article":
		links = append(links, Link{"博客", "/blog"})
	case "notice":
		links = append(links, Link{"公告", "/notice"})
	}
	var graph []any
	if page.Path != "/" {
		links = append(links, Link{page.Title, page.Path})
		items := make([]map[string]any, 0, len(links))
		for i, link := range links {
			items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "name": link.Title, "item": origin + link.Path})
		}
		graph = append(graph, map[string]any{"@type": "BreadcrumbList", "itemListElement": items})
	}
	if faqs := visibleFAQs(body); len(faqs) > 0 {
		items := make([]map[string]any, 0, len(faqs))
		for _, faq := range faqs {
			items = append(items, map[string]any{"@type": "Question", "name": faq.Question, "acceptedAnswer": map[string]string{"@type": "Answer", "text": faq.Answer}})
		}
		graph = append(graph, map[string]any{"@type": "FAQPage", "@id": origin + page.Path + "#faq", "mainEntity": items})
	}
	if len(graph) == 0 {
		return ""
	}
	raw, _ := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": graph})
	return template.JS(raw) // json.Marshal escapes HTML delimiters.
}

func descriptionFor(page Page) string {
	for _, value := range []string{page.SEODescription, page.Summary, page.Content} {
		text := plainText(value)
		if text == "" || text == plainText(page.Title) {
			continue
		}
		runes := []rune(text)
		if len(runes) > 160 {
			text = string(runes[:160])
		}
		return text
	}
	return "" // Never fill an absent description with a duplicate title.
}
