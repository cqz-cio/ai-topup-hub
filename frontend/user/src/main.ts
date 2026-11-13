import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { createHead } from '@unhead/vue/client'
import './style.css'
import App from './App.vue'
import router, { warmupCommonRoutes } from './router'
import i18n, { detectLocale, setI18nLocale } from './i18n'
import { useTelegramMiniAppStore } from './stores/telegramMiniApp'
import { initTemplateOverride } from './templates/registry'
import { isRedemptionPath } from './utils/redemption'
import { initializeLightTheme } from './utils/theme'

initializeLightTheme()

// 预览用：?template=vault 持久化激活模板（站长正式切换走站点配置）
initTemplateOverride()

const brandLog = (globalThis as any).console?.log?.bind(console)
brandLog?.(
  '%c Dujiao-Next %c Digital Commerce Platform ',
  'background:#0071e3;color:#fff;padding:4px 8px;border-radius:4px 0 0 4px;font-weight:bold;',
  'background:#1d1d1f;color:#f5f5f7;padding:4px 8px;border-radius:0 4px 4px 0;',
)
brandLog?.('%cGitHub → https://github.com/dujiao-next', 'color:#6e6e73;')

const app = createApp(App)
const head = createHead()
const pinia = createPinia()

app.use(pinia)
app.use(head)
app.use(router)
app.use(i18n)

const publicDocument = document.querySelector('#public-document')

// Keep the server document until the first route, configuration and locale are
// ready, so replacing it never exposes an empty router view.
Promise.all([
  publicDocument ? router.isReady() : Promise.resolve(),
  isRedemptionPath(window.location.pathname) ? Promise.resolve() : useTelegramMiniAppStore(pinia).init(),
  setI18nLocale(detectLocale()),
]).then(() => {
  // The server document is visible until the interactive app is ready. Remove
  // only its head entries so Unhead owns subsequent navigation without duplicates.
  document.querySelectorAll('[data-public-page-seo]').forEach(node => node.remove())
  app.mount('#app')
}).catch(() => {
  // Preserve readable content if initialization fails after a partial mount.
  if (publicDocument && !publicDocument.isConnected) {
    document.querySelector('#app')?.replaceChildren(publicDocument)
  }
  document.documentElement.setAttribute('data-public-document-boot-failed', '')
})

void router.isReady().then(() => {
    warmupCommonRoutes()
})
