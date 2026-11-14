import { computed } from 'vue'
import { useHead } from '@unhead/vue'
import { useAppStore } from '../stores/app'
import { seoText, seoTitle } from '../utils/seo'
import { defaultShareImage, rasterShareUrl, shareMetadata } from '../utils/shareSeo'

export interface PageSeoOptions {
  /** 页面专属标题；若与站点名相同，仅展示站点名 */
  title?: () => string | undefined | null
  /** 页面描述；缺省时回落到站点全局描述 */
  description?: () => string | undefined | null
  /** 原始分享图 URL；不应用商品展示图的透明替换规则。 */
  image?: () => string | undefined | null
  images?: () => string[] | undefined | null
  imageWidth?: () => number | undefined
  imageHeight?: () => number | undefined
  imageAlt?: () => string | undefined
  /** 规范化路径（如 /products/foo），缺省时取当前 location.pathname */
  canonicalPath?: () => string | undefined | null
  /** og:type，默认 website */
  type?: () => string | undefined | null
  /** 标识页面不该被索引（私密页/动态查询页） */
  noindex?: () => boolean
  followWhenNoindex?: boolean
}

const trimSlashEnd = (s: string) => s.replace(/\/+$/, '')

export function usePageSeo(options: PageSeoOptions = {}) {
  const appStore = useAppStore()

  const baseUrl = computed(() => {
    const fromConfig = trimSlashEnd(String(appStore.config?.brand?.site_url || '').trim())
    if (fromConfig) return fromConfig
    if (typeof window !== 'undefined') return trimSlashEnd(window.location.origin)
    return ''
  })

  const siteName = computed(() => {
    return String(appStore.config?.brand?.site_name || '').trim()
      || seoText(appStore.config?.seo?.title, appStore.locale)
  })

  const siteTitle = computed(() => seoText(appStore.config?.seo?.title, appStore.locale) || siteName.value)

  const siteDescription = computed(() => {
    return seoText(appStore.config?.seo?.description, appStore.locale)
  })

  const defaultImage = computed(() => {
    const seo = appStore.config?.seo as { default_og_image?: string } | undefined
    const configured = String(seo?.default_og_image || '').trim()
    return defaultShareImage(configured, baseUrl.value, !appStore.isResellerTenant)
  })

  const pageTitle = computed(() => {
    const raw = options.title?.()
    return raw ? String(raw).trim() : ''
  })

  const fullTitle = computed(() => {
    return (pageTitle.value ? seoTitle(pageTitle.value, siteName.value) : siteTitle.value) || undefined
  })

  const fullDescription = computed(() => {
    const pageDesc = options.description?.()
    if (pageDesc && String(pageDesc).trim()) return String(pageDesc).trim()
    return siteDescription.value || undefined
  })

  const fullCanonical = computed(() => {
    const explicit = options.canonicalPath?.()
    let path = explicit && String(explicit).trim() ? String(explicit).trim() : ''
    if (!path && typeof window !== 'undefined') path = window.location.pathname
    if (!path) path = '/'
    if (!path.startsWith('/')) path = '/' + path
    if (!baseUrl.value) return path
    return baseUrl.value + path
  })

  const sharing = computed(() => {
    const raw = options.image?.()
    const candidates = [...(raw ? [String(raw)] : []), ...(options.images?.() || [])]
    const sharing = shareMetadata(baseUrl.value, candidates, defaultImage.value,
      options.imageAlt?.() || pageTitle.value || siteName.value)
    const width = options.imageWidth?.()
    const height = options.imageHeight?.()
    if (raw && sharing.image === rasterShareUrl(baseUrl.value, String(raw)) && width && height && width > 0 && height > 0 && width <= 10000 && height <= 10000) {
      sharing.width = width
      sharing.height = height
    }
    return sharing
  })
  const fullImage = computed(() => sharing.value.image)

  useHead({
    title: () => fullTitle.value,
    link: () => {
      if (!fullCanonical.value) return []
      return [{ key: 'canonical', rel: 'canonical', href: fullCanonical.value }]
    },
    meta: () => {
      const tags: Array<{ name?: string; property?: string; content: string }> = []

      if (options.noindex?.()) {
        tags.push({ name: 'robots', content: options.followWhenNoindex ? 'noindex, follow' : 'noindex, nofollow' })
      }

      if (fullDescription.value) {
        tags.push({ name: 'description', content: fullDescription.value })
      }

      const ogType = (options.type?.() && String(options.type()).trim()) || 'website'
      tags.push({ property: 'og:type', content: ogType })
      if (fullTitle.value) tags.push({ property: 'og:title', content: fullTitle.value })
      if (fullDescription.value) tags.push({ property: 'og:description', content: fullDescription.value })
      if (fullCanonical.value) tags.push({ property: 'og:url', content: fullCanonical.value })
      if (fullImage.value) {
        tags.push({ property: 'og:image', content: fullImage.value }, { property: 'og:image:alt', content: sharing.value.alt })
        if (sharing.value.width) tags.push({ property: 'og:image:width', content: String(sharing.value.width) })
        if (sharing.value.height) tags.push({ property: 'og:image:height', content: String(sharing.value.height) })
      }
      if (siteName.value) tags.push({ property: 'og:site_name', content: siteName.value })

      tags.push({ name: 'twitter:card', content: sharing.value.card })
      if (fullTitle.value) tags.push({ name: 'twitter:title', content: fullTitle.value })
      if (fullDescription.value) tags.push({ name: 'twitter:description', content: fullDescription.value })
      if (fullImage.value) tags.push({ name: 'twitter:image', content: fullImage.value }, { name: 'twitter:image:alt', content: sharing.value.alt })

      return tags
    },
  })
}
