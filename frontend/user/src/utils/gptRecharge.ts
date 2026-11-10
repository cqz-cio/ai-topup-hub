import guide from '../../../../internal/platform/publiccontent/gpt-recharge.json' with { type: 'json' }

export const gptRecharge = guide
export const gptRechargeHtml = guide.sections.map(section => `<h2>${section.title}</h2>${section.html}`).join('')

export function appendRechargeLinks(content: string, kind: 'product' | 'article', slug: string, category: string, isReseller: boolean): string {
  if (isReseller || !(kind === 'product'
    ? guide.productCategories.includes(category)
    : guide.blogSlugs.includes(slug))) return content
  const links = guide.relatedLinks.filter(link => link.path !== `/blog/${slug}` && !content.includes(link.path))
  if (!links.length) return content
  return content + `<h2>${guide.relatedHeading}</h2><ul>` + links.map(link => `<li><a href="${link.path}">${link.title}</a></li>`).join('') + '</ul>'
}
