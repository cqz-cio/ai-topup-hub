import assets from '../../../../internal/platform/publiccontent/seo-assets.json' with { type: 'json' }
import generated from '../../../../internal/platform/publiccontent/seo-share-images.json' with { type: 'json' }

export function identityLogo(raw: string, mainSite: boolean): string {
  return mainSite && raw === assets.legacyLogo ? assets.logo : raw
}

export function rasterShareUrl(origin: string, raw: string): string {
    if (!raw.trim()) return ''
    try {
      const url = new URL(raw.trim(), origin + '/')
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return ''
      return /\.(png|jpe?g|webp|gif)$/i.test(url.pathname) ? url.href : ''
    } catch { return '' }
}

export function defaultShareImage(raw: string, origin: string, mainSite: boolean): string {
  if (mainSite && (!rasterShareUrl(origin, raw) || [assets.legacyLogo, assets.logo].includes(raw))) return assets.shareImage
  return raw
}

export function shareMetadata(origin: string, images: string[], fallback = '', alt = '') {
  const result = { image: '', card: 'summary_large_image', width: 0, height: 0, alt }
  let siteOrigin = ''
  try { siteOrigin = new URL(origin).origin } catch { /* Absolute images remain valid without a site URL. */ }
  for (const raw of [...images, fallback]) {
    let image = rasterShareUrl(origin, raw)
    if (!image) continue
    const url = new URL(image)
    if (url.origin === siteOrigin) {
      const exported = (generated as Record<string, string>)[url.pathname]
      if (exported) {
        image = new URL(exported, origin).href
        result.width = 1200
        result.height = 630
      } else if (url.pathname === assets.shareImage) {
        result.width = assets.shareImageWidth
        result.height = assets.shareImageHeight
      }
    }
    return { ...result, image }
  }
  return result
}
