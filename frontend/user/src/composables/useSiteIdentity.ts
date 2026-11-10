import { useHead } from '@unhead/vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '../stores/app'
import { siteIdentitySchema, schemaJSON } from '../utils/siteIdentity'
import { seoText } from '../utils/seo'

export function useSiteIdentity() {
  const app = useAppStore(), route = useRoute()
  useHead(() => {
    if (!app.config || !/^\/(?:about|blog(?:\/[^/]+)?|products(?:\/[^/]+)?|categories\/[^/]+|notice)?\/?$/.test(route.path)) return { script: [] }
    return { script: [{ key: 'site-identity', type: 'application/ld+json', innerHTML: schemaJSON(siteIdentitySchema(app.config.brand, window.location.origin, seoText(app.config.seo?.description, app.locale), !app.isResellerTenant)) }] }
  })
}
