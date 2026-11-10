export function postListState(query: Record<string, unknown>) {
  const raw = typeof query.page === 'string' ? query.page : '1'
  const page = /^\d+$/.test(raw) && Number(raw) >= 1 && Number(raw) <= 100000 ? Number(raw) : 1
  return { page, search: typeof query.search === 'string' ? query.search.trim().slice(0, 200) : '' }
}
export function postListPath(base: string, page: number, search = '') {
  const query = new URLSearchParams()
  if (page > 1) query.set('page', String(page))
  if (search.trim()) query.set('search', search.trim())
  return base + (query.size ? '?' + query.toString() : '')
}
