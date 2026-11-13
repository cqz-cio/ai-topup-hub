import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { usePageSeo } from './usePageSeo'
import { useHead } from '@unhead/vue'
import { schemaJSON } from '../utils/siteIdentity'

/**
 * 条款/隐私页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/Legal.vue 的行为，仅抽离为 composable。
 * type 以 getter 传入以保持响应性（来源于组件 props）。
 */
export function useLegal(type: () => 'terms' | 'privacy') {
  const { t } = useI18n()
  const appStore = useAppStore()

  usePageSeo({
    title: () => type() === 'terms' ? t('footer.terms') : t('footer.privacy'),
    description: () => type() === 'terms' ? t('footer.terms') : t('footer.privacy'),
    canonicalPath: () => type() === 'terms' ? '/terms' : '/privacy',
  })

  const loading = computed(() => appStore.loading)
  const locale = computed(() => appStore.locale)

  const title = computed(() => {
    return type() === 'terms' ? t('footer.terms') : t('footer.privacy')
  })

  const content = computed(() => {
    const config = appStore.config
    if (!config?.legal) return ''

    const legal = config.legal
    const lang = locale.value

    if (type() === 'terms' && legal.terms) {
      return legal.terms[lang] || legal.terms['zh-CN'] || legal.terms['en-US'] || ''
    } else if (type() === 'privacy' && legal.privacy) {
      return legal.privacy[lang] || legal.privacy['zh-CN'] || legal.privacy['en-US'] || ''
    }
    return ''
  })

  useHead(() => {
    if (!appStore.config || !content.value) return { script: [] }
    const origin = String(appStore.config.brand?.site_url || window.location.origin).replace(/\/+$/, '')
    return { script: [{ key: 'legal-page', type: 'application/ld+json', innerHTML: schemaJSON({
      '@context': 'https://schema.org', '@type': 'WebPage', name: title.value, description: title.value,
      url: origin + (type() === 'terms' ? '/terms' : '/privacy'), inLanguage: appStore.locale,
      isPartOf: { '@id': origin + '/#website' }, about: { '@id': origin + '/#organization' },
    }) }] }
  })

  return {
    loading,
    title,
    content,
  }
}
