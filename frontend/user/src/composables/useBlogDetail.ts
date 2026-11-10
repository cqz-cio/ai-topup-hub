import { ref, onMounted, onUnmounted, computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { postAPI } from '../api'
import { debounceAsync } from '../utils/debounce'
import { useLocalized } from './useProduct'
import { usePageSeo } from './usePageSeo'
import { useHead } from '@unhead/vue'
import { getImageUrl } from '../utils/image'
import { appendRechargeLinks } from '../utils/gptRecharge'

/**
 * 文章/公告详情共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/BlogDetail.vue 的行为，仅抽离为 composable。
 */
export function useBlogDetail() {
  const route = useRoute()
  const { t } = useI18n()
  const appStore = useAppStore()
  const { formatPrice } = useLocalized()

  const loading = ref(true)
  const post = ref<any>(null)
  const relatedProducts = computed<any[]>(() => post.value?.related_products || [])

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

  const readingContent = computed(() => appendRechargeLinks(
    getLocalizedText(post.value?.content), 'article', post.value?.slug || '', '', appStore.isResellerTenant,
  ))

  usePageSeo({
    title: () => post.value ? getLocalizedText(post.value.title) : '',
    description: () => post.value ? getLocalizedText(post.value.summary) : '',
    image: () => post.value?.thumbnail || '',
    canonicalPath: () => `/blog/${(route.params.slug as string) || ''}`,
    type: () => 'article',
  })

  // Keep article structured data after Vue replaces the initial public HTML.
  useHead(() => {
    if (!post.value || loading.value) return { script: [] }
    const origin = String(appStore.config?.brand?.site_url || window.location.origin).replace(/\/+$/, '')
    const url = `${origin}/blog/${encodeURIComponent(post.value.slug)}`
    const schema: Record<string, unknown> = {
      '@context': 'https://schema.org',
      '@type': post.value.type === 'notice' ? 'Article' : 'BlogPosting',
      headline: getLocalizedText(post.value.title),
      description: getLocalizedText(post.value.summary),
      url,
      mainEntityOfPage: url,
      inLanguage: appStore.locale,
    }
    if (post.value.published_at) schema.datePublished = post.value.published_at
    if (post.value.thumbnail) schema.image = new URL(getImageUrl(post.value.thumbnail), origin + '/').href
    return { script: [{ key: 'article-schema', type: 'application/ld+json', innerHTML: JSON.stringify(schema).replace(/</g, '\\u003c') }] }
  })

  const backLink = computed(() => {
    if (!post.value) return '/blog'
    return post.value.type === 'notice' ? '/notice' : '/blog'
  })

  const backText = computed(() => {
    if (!post.value) return t('blogDetail.backToBlog')
    return post.value.type === 'notice' ? t('blogDetail.backToNotice') : t('blogDetail.backToBlog')
  })

  const loadPost = async () => {
    loading.value = true
    try {
      const slug = route.params.slug as string
      const response = await postAPI.detail(slug)
      post.value = response.data.data || null
    } catch (error) {
      console.error('Failed to load post:', error)
      post.value = null
    } finally {
      loading.value = false
    }
  }

  const debouncedLoadPost = debounceAsync(loadPost, 300)

  onMounted(() => {
    loadPost()
  })

  onUnmounted(() => {
    debouncedLoadPost.cancel()
  })

  return {
    readingContent,
    loading,
    post,
    relatedProducts,
    getLocalizedText,
    formatDate,
    formatPrice,
    backLink,
    backText,
  }
}
