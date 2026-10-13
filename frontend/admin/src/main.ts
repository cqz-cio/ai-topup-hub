import { createApp, watch } from 'vue'
import { createPinia } from 'pinia'
import './style.css'
import App from './App.vue'
import router from './router'
import i18n from './i18n'

watch(i18n.global.locale, (locale) => {
  document.title = i18n.global.t('admin.layout.workspace')
  document.documentElement.lang = locale
}, { immediate: true })

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.use(i18n)
app.mount('#app')
