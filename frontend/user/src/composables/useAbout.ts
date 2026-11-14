import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '../stores/app'
import { usePageSeo } from './usePageSeo'
import { useHead } from '@unhead/vue'
import { resolveAboutPage } from '../../../shared/aboutPage'
import { schemaJSON } from '../utils/siteIdentity'

/**
 * 关于页共享逻辑（classic + vault 双模板共用）。
 * 完整保留原 views/About.vue 的行为，仅抽离为 composable。
 */
export function useAbout() {
  const { t, locale } = useI18n()
  const appStore = useAppStore()

  usePageSeo({
    title: () => appStore.isResellerTenant ? t('nav.about') : t('shopUi.aboutTitle'),
    imageAlt: () => appStore.isResellerTenant ? heroTitle.value : resolveAboutPage(appStore.config?.about_page, appStore.config?.about, locale.value, appStore.config?.contact).title,
    description: () => appStore.isResellerTenant ? '' : resolveAboutPage(appStore.config?.about_page, appStore.config?.about, locale.value, appStore.config?.contact).subtitle,
    canonicalPath: () => '/about',
  })


  const aboutConfig = computed(() => appStore.config?.about || null)
  const contactConfig = computed(() => appStore.config?.contact || null)

  const resolveLocalizedText = (raw: unknown): string => {
    if (!raw || typeof raw !== 'object') {
      return ''
    }

    const record = raw as Record<string, unknown>
    const lang = String(locale.value || appStore.locale || 'zh-CN')
    const candidates = [record[lang], record['zh-CN'], record['zh-TW'], record['en-US']]

    for (const candidate of candidates) {
      if (typeof candidate === 'string' && candidate.trim() !== '') {
        return candidate.trim()
      }
    }

    return ''
  }

  const heroTitle = computed(() => resolveLocalizedText(aboutConfig.value?.hero?.title))
  const heroSubtitle = computed(() => resolveLocalizedText(aboutConfig.value?.hero?.subtitle))
  const introductionText = computed(() => resolveLocalizedText(aboutConfig.value?.introduction))
  const servicesTitle = computed(() => resolveLocalizedText(aboutConfig.value?.services?.title))
  const contactTitle = computed(() => resolveLocalizedText(aboutConfig.value?.contact?.title))
  const contactText = computed(() => resolveLocalizedText(aboutConfig.value?.contact?.text))

  const serviceItems = computed(() => {
    const raw = aboutConfig.value?.services?.items
    if (!Array.isArray(raw)) {
      return []
    }

    return raw
      .map((item) => resolveLocalizedText(item))
      .filter((item) => item !== '')
  })

  const hasIntroduction = computed(() => introductionText.value !== '')
  const hasServices = computed(() => servicesTitle.value !== '' || serviceItems.value.length > 0)
  const hasContactLinks = computed(() => !!(contactConfig.value?.telegram || contactConfig.value?.whatsapp))
  const hasContact = computed(() => contactTitle.value !== '' || contactText.value !== '' || hasContactLinks.value)

  useHead(() => {
    if (!appStore.config) return { script: [] }
    const copy = resolveAboutPage(appStore.config.about_page, appStore.config.about, locale.value, appStore.config.contact)
    const origin = String(appStore.config.brand?.site_url || window.location.origin).replace(/\/+$/, '')
    const schema = {
      '@context': 'https://schema.org', '@type': 'AboutPage',
      name: appStore.isResellerTenant ? heroTitle.value : copy.title,
      description: appStore.isResellerTenant ? heroSubtitle.value : copy.subtitle,
      url: origin + '/about', inLanguage: appStore.locale,
      isPartOf: { '@id': origin + '/#website' }, about: { '@id': origin + '/#organization' },
    }
    return { script: [{ key: 'about-page', type: 'application/ld+json', innerHTML: schemaJSON(schema) }] }
  })

  onMounted(async () => {
    if (!appStore.config) {
      await appStore.loadConfig()
    }
  })

  return {
    contactConfig,
    heroTitle,
    heroSubtitle,
    introductionText,
    servicesTitle,
    contactTitle,
    contactText,
    serviceItems,
    hasIntroduction,
    hasServices,
    hasContactLinks,
    hasContact,
  }
}
