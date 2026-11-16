import DOMPurify from 'dompurify'

export function visibleFAQs(html: string): {question: string; answer: string}[] {
  const doc = new DOMParser().parseFromString(DOMPurify.sanitize(html), 'text/html')
  const result: {question: string; answer: string}[] = []
  const seen = new Set<string>()
  let active = false
  for (const element of doc.body.querySelectorAll('h1,h2,h3,h4,h5,h6,p')) {
    if (/^H[1-6]$/.test(element.tagName)) {
      const label = element.textContent?.trim() || ''
      active = label.includes('常见问题') || label.toUpperCase() === 'FAQ'
    } else if (active) {
      const first = Array.from(element.childNodes).find(node => node.nodeType !== Node.TEXT_NODE || !!node.textContent?.trim())
      if (!(first instanceof Element) || first.tagName !== 'STRONG') continue
      const question = first.textContent?.trim() || ''
      const answer = Array.from(element.childNodes).slice(Array.from(element.childNodes).indexOf(first) + 1).map(node => node.textContent?.trim() || '').join(' ').replace(/\s+/g, ' ').trim()
      if (question && answer && !seen.has(question)) { result.push({question,answer}); seen.add(question) }
    }
  }
  return result
}
export function faqSchema(html: string, url: string) {
  const faqs = visibleFAQs(html)
  return faqs.length ? {'@context':'https://schema.org','@type':'FAQPage','@id':url+'#faq',mainEntity:faqs.map(faq=>({'@type':'Question',name:faq.question,acceptedAnswer:{'@type':'Answer',text:faq.answer}}))} : undefined
}
export function breadcrumbSchema(origin: string, title: string, path: string, parents: {title:string;path:string}[] = []) {
  if (path === '/' || !title) return undefined
  return {'@context':'https://schema.org','@type':'BreadcrumbList',itemListElement:[{title:'首页',path:'/'},...parents,{title,path}].map((item,index)=>({'@type':'ListItem',position:index+1,name:item.title,item:origin+item.path}))}
}
