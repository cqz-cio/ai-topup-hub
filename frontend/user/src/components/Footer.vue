<template>
  <footer class="brand-footer">
    <div class="container mx-auto">
      <div class="brand-footer-top">
        <div>
          <RouterLink to="/" class="brand-footer-name"><img v-if="brandLogo" :src="brandLogo" :alt="brandSiteName" class="inline-block h-8 max-w-[120px] object-contain mr-3" />{{ brandSiteName }}</RouterLink>
          <p class="brand-footer-description">{{ brandDescription || t('shopUi.brandDescription') }}</p>
        </div>
        <div class="brand-footer-links">
          <div><h4>{{ t('shopUi.catalog') }}</h4><router-link v-for="item in quickLinks" :key="item.path" :to="item.path">{{ t(item.label) }}</router-link></div>
          <div><h4>{{ t('shopUi.lookupAction') }}</h4><router-link to="/me/orders">{{ t('shopUi.memberOrders') }}</router-link><router-link to="/guest/orders">{{ t('shopUi.guestOrders') }}</router-link><router-link to="/cart">{{ t('navbar.cart') }}</router-link></div>
          <div v-if="supportWechat || config?.contact?.telegram || config?.contact?.whatsapp"><h4>{{ t('shopUi.support') }}</h4><button v-if="supportWechat" type="button" class="flex items-center gap-2 text-left" @click="copyWechat">{{ t('shopUi.wechat') }}<Copy :size="12" /></button><span v-if="supportWechat" class="font-semibold">{{ supportWechat }}</span><a v-if="config?.contact?.telegram" :href="config.contact.telegram" target="_blank" rel="noopener noreferrer">Telegram</a><a v-if="config?.contact?.whatsapp" :href="config.contact.whatsapp" target="_blank" rel="noopener noreferrer">WhatsApp</a></div>
        </div>
      </div>
      <div class="brand-footer-bottom">
        <span>© {{ currentYear }} {{ brandSiteName }}. {{ t('footer.rights') }}</span>
        <div><router-link to="/privacy">{{ t('footer.privacy') }}</router-link><router-link to="/terms">{{ t('footer.terms') }}</router-link><a v-for="link in footerLinks" :key="link.name" :href="link.url || undefined" :target="link.url ? '_blank' : undefined" rel="noopener noreferrer">{{ link.name }}</a></div>
      </div>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Copy, Home, LayoutGrid, Newspaper, Info } from 'lucide-vue-next'
import { copyText } from '../utils/clipboard'
import { toast } from '../composables/useToast'
import { useAppStore } from '../stores/app'
import { getImageUrl } from '../utils/image'
import { getLocalizedText } from '../utils/resellerSiteConfig'

const { t } = useI18n()
const appStore = useAppStore()

const config = computed(() => appStore.config)

const brandSiteName = computed(() => {
  const siteName = config.value?.brand?.site_name
  return typeof siteName === 'string' && siteName.trim() ? siteName.trim() : '智享A社'
})

const brandDescription = computed(() => {
  const desc = config.value?.brand?.site_description
  if (desc && typeof desc === 'object') {
    const val = desc[appStore.locale] || desc['zh-CN'] || ''
    return typeof val === 'string' ? val.trim() : ''
  }
  return ''
})


const brandLogo = computed(() => {
  const raw = String(config.value?.brand?.site_logo || '').trim()
  return raw ? getImageUrl(raw) : (appStore.isResellerTenant ? '' : '/brand-icon.png')
})

const isListMode = computed(() => config.value?.template_mode === 'list')
const navConfig = computed(() => config.value?.nav_config as { builtin?: Record<string, boolean> } | undefined)

const quickLinks = computed(() => {
  const items = [
    { path: '/', label: 'nav.home', icon: Home },
  ]
  if (!isListMode.value) {
    items.push({ path: '/products', label: 'nav.products', icon: LayoutGrid })
  }
  const builtin = navConfig.value?.builtin
  if (!builtin || builtin.blog !== false) {
    items.push({ path: '/blog', label: 'nav.blog', icon: Newspaper })
  }
  if (!builtin || builtin.about !== false) {
    items.push({ path: '/about', label: 'nav.about', icon: Info })
  }
  return items
})

const footerLinks = computed(() => {
  const links = config.value?.footer_links
  if (!Array.isArray(links)) return []
  return links
    .map((item: any) => ({
      name: typeof item?.name === 'string' ? item.name.trim() : getLocalizedText(item?.name, appStore.locale),
      url: String(item?.url || '').trim(),
    }))
    .filter((item) => item.name)
})

const currentYear = new Date().getFullYear()

const supportWechat = computed(() => String(config.value?.contact?.wechat || '').trim() || (appStore.isResellerTenant ? '' : '15824485317'))
const copyWechat = async () => {
  try { await copyText(supportWechat.value); toast.success(t('shopUi.wechatCopied')) }
  catch { toast.error(t('shopUi.wechatCopyFailed')) }
}
</script>
