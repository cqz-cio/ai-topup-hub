import {ref, watch} from 'vue'
import {api} from '../api/client'
import {useAppStore} from '../stores/app'
import {seoText} from '../utils/seo'

export function useFooterCategories() {
  const app = useAppStore()
  const links = ref<{path:string;title:string}[]>([])
  watch(() => [app.config, app.locale], async (_, __, onCleanup) => {
    let stale = false; onCleanup(() => {stale = true})
    const initial = app.config?.public_footer_categories
    if (Array.isArray(initial) && app.locale === 'zh-CN') {
      links.value = initial.filter(link => typeof link.Path === 'string' && /^\/categories\/[^/?#]+$/.test(link.Path)).map(link => ({path:link.Path,title:String(link.Title || '')}))
      return
    }
    if (!app.config) return
    try {
      const response = await api.get('/public/categories')
      const flatten = (rows:any[]):any[] => rows.flatMap(row => [row, ...flatten(row.children || [])])
      const categories = flatten(response.data.data || [])
      const populated = new Set<number>()
      for (let page = 1; !stale; page++) {
        const result = await api.get('/public/products',{params:{page,page_size:100}})
        const rows = result.data.data || []
        for (const row of rows) populated.add(Number(row.category_id || row.category?.id))
        if (rows.length < 100 || page * 100 >= (result.data.pagination?.total || 0)) break
      }
      const parents = new Set(categories.filter(row=>populated.has(row.id) && row.parent_id).map(row=>row.parent_id))
      if (!stale) links.value = categories.filter(row=>row.slug && (populated.has(row.id) || (!row.parent_id && parents.has(row.id)))).map(row=>({path:'/categories/'+encodeURIComponent(row.slug),title:seoText(row.name,app.locale)}))
    } catch { if (!stale) links.value=[] }
  }, {immediate:true})
  return links
}
