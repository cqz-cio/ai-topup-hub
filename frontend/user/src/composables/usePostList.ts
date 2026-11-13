import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAppStore } from '../stores/app'
import { postAPI } from '../api'
import { debounceAsync } from '../utils/debounce'
import { usePageSeo } from './usePageSeo'
import { useHead } from '@unhead/vue'
import { postListState, postListPath } from '../utils/postListUrl'
import { schemaJSON } from '../utils/siteIdentity'

/**
 * 文章/公告列表共享逻辑（Blog/Notice，classic + vault 双模板共用）。
 * 完整保留原 views/Blog.vue 与 Notice.vue 的行为，仅抽离为 composable。
 * Blog 启用搜索；Notice 无搜索框（searchKeyword 恒为空，watch 永不触发，行为一致）。
 */
export function usePostList(
  type: 'blog' | 'notice',
  seo: { title: () => string; canonicalPath: string },
) {
  const router = useRouter()
  const route = useRoute()
  const appStore = useAppStore()



  const loading = ref(true)
  const posts = ref<any[]>([])
  const currentPage = ref(postListState(route.query).page)
  const pageSize = ref(12)
  const total = ref(0)
  const totalPages = ref(0)
  const searchKeyword = ref(postListState(route.query).search)
  let request = 0

  usePageSeo({
    title: seo.title,
    description: () => type === 'blog' ? '最新文章和动态' : '购买、交付与售后服务公告',
    canonicalPath: () => postListPath(seo.canonicalPath, searchKeyword.value.trim() ? 1 : currentPage.value),
    noindex: () => !!searchKeyword.value.trim(),
    followWhenNoindex: true,
  })

  const getLocalizedText = (jsonData: any) => {
    if (!jsonData) return ''
    const locale = appStore.locale
    return jsonData[locale] || jsonData['zh-CN'] || jsonData['en-US'] || ''
  }

  const formatDate = (dateString: string) => {
    if (!dateString) return ''
    const date = new Date(dateString)
    return date.toLocaleDateString(appStore.locale, {
      year: 'numeric',
      month: 'long',
      day: 'numeric',
    })
  }

  const loadPosts = async () => {
    const id = ++request
    loading.value = true
    try {
      const params: Record<string, any> = {
        type,
        page: currentPage.value,
        page_size: pageSize.value,
      }
      const keyword = searchKeyword.value.trim()
      if (keyword) {
        params.search = keyword
      }
      const response = await postAPI.list(params)
      if (id !== request) return
      posts.value = response.data.data || []
      if (response.data.pagination) {
        total.value = response.data.pagination.total || 0
        totalPages.value = response.data.pagination.total_page || 0
      }
    } catch (error) {
      if (id !== request) return
      posts.value = []
      console.error('Failed to load posts:', error)
    } finally {
      if (id === request) loading.value = false
    }
  }

  const debouncedSearch = debounceAsync(async () => {
    await router.replace(postListPath(seo.canonicalPath, 1, searchKeyword.value))
  }, 300)
  const pageHref = (page: number) => postListPath(seo.canonicalPath, page, searchKeyword.value)

  const goToPost = (slug: string) => {
    router.push(`/blog/${slug}`)
  }

  const changePage = (page: number) => {
    if (page < 1 || page > totalPages.value) return
    debouncedSearch.cancel()
    void router.push(pageHref(page))
    window.scrollTo({ top: 0, behavior: 'smooth' })
  }

  watch(searchKeyword, value => {
    if (value.trim() !== postListState(route.query).search) debouncedSearch()
  })
  watch(() => [route.query.page, route.query.search], () => {
    debouncedSearch.cancel()
    const state = postListState(route.query)
    currentPage.value = state.page
    searchKeyword.value = state.search
    void loadPosts()
  })

  useHead(() => {
    if (loading.value) return { script: [] }
    const origin = String(appStore.config?.brand?.site_url || window.location.origin).replace(/\/+$/, '')
    const schema = {
      '@context': 'https://schema.org', '@type': 'CollectionPage', name: seo.title(), description: type === 'blog' ? '最新文章和动态' : '购买、交付与售后服务公告',
      url: origin + postListPath(seo.canonicalPath, searchKeyword.value.trim() ? 1 : currentPage.value),
      inLanguage: appStore.locale, isPartOf: { '@id': origin + '/#website' }, about: { '@id': origin + '/#organization' },
      mainEntity: { '@type': 'ItemList', itemListElement: posts.value.map((post, i) => ({
        '@type': 'ListItem', position: i + 1, name: getLocalizedText(post.title), url: `${origin}/blog/${encodeURIComponent(post.slug)}`,
      })) },
    }
    return { script: [{ key: `${type}-list`, type: 'application/ld+json', innerHTML: schemaJSON(schema) }] }
  })

  onMounted(() => {
    loadPosts()
  })

  onUnmounted(() => {
    request++
    debouncedSearch.cancel()
  })

  return {
    loading,
    posts,
    currentPage,
    totalPages,
    total,
    searchKeyword,
    getLocalizedText,
    formatDate,
    goToPost,
    changePage,
    pageHref,
  }
}
