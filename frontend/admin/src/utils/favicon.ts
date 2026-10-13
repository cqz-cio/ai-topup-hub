import { getImageUrl } from './image'
import { ADMIN_BRAND_LOGO } from './branding'

const SITE_ICON_LINK_ID = 'site-favicon'
const DEFAULT_SITE_ICON = ADMIN_BRAND_LOGO

export function resolveSiteIconHref(value: unknown): string {
  const icon = String(value || '').trim()
  return icon ? getImageUrl(icon) : DEFAULT_SITE_ICON
}

export function applySiteIcon(value: unknown) {
  const link = document.getElementById(SITE_ICON_LINK_ID) as HTMLLinkElement | null
  if (link) {
    link.href = resolveSiteIconHref(value)
  }
}
