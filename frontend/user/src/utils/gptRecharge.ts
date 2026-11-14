import guide from '../../../../internal/platform/publiccontent/gpt-recharge.json' with { type: 'json' }

export const gptRecharge = guide
export const gptRechargeHtml = guide.sections.map(section => `<h2>${section.title}</h2>${section.html}`).join('')

export type CatalogLinks = Record<string, string>
export function rewriteCatalogLinks(content: string, links: CatalogLinks): string {
  const identities = guide.catalogIdentities as Record<string, string>
  const references = Object.fromEntries(Object.entries(identities).flatMap(([path, alias]) => [[path, path], [alias, path]]))
  return content.replace(/href="([^"]+)"/g, (attribute, path: string) => {
    const reference = references[path]
    if (!reference) return attribute
    const target = links[reference] || ''
    return /^\/(products|categories)\/[a-z0-9]+(?:-[a-z0-9]+)*$/.test(target) ? `href="${target}"` : ''
  })
}

export function resolveRechargeGuide(links: CatalogLinks) {
  const path = links[guide.path] || ''
  return {
    ...guide, path, categorySlug: path.replace(/^\/categories\//, ''),
    productCategories: guide.productCategories.map(slug => links['/categories/' + slug]).filter(Boolean).map(path => path!.replace(/^\/categories\//, '')),
    sections: guide.sections.map(section => ({ ...section, html: rewriteCatalogLinks(section.html, links) })),
    relatedLinks: guide.relatedLinks.map(link => ({ ...link, path: link.path in guide.catalogIdentities ? links[link.path] || '' : link.path })).filter(link => link.path),
  }
}

export function rechargeHtml(resolved: ReturnType<typeof resolveRechargeGuide>): string {
  return resolved.sections.map(section => `<h2>${section.title}</h2>${section.html}`).join('')
}

export function appendRechargeLinks(content: string, kind: 'product' | 'article', slug: string, category: string, isReseller: boolean, links: CatalogLinks = {}): string {
  if (isReseller) return content
  const guide = resolveRechargeGuide(links)
  content = rewriteCatalogLinks(content, links)
  const permanentProduct = Object.values(gptRecharge.catalogIdentities).includes('/products/' + slug)
  if (!(kind === 'product'
    ? guide.productCategories.includes(category) || permanentProduct
    : guide.blogSlugs.includes(slug))) return content
  const related = guide.relatedLinks.filter(link => link.path !== `/blog/${slug}` && !content.includes(link.path))
  if (!related.length) return content
  return content + `<h2>${guide.relatedHeading}</h2><ul>` + related.map(link => `<li><a href="${link.path}">${link.title}</a></li>`).join('') + '</ul>'
}
