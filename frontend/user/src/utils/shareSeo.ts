import assets from '../../../../internal/platform/publiccontent/seo-assets.json' with { type: 'json' }

export function identityLogo(raw: string, mainSite: boolean): string {
  return mainSite && raw === assets.legacyLogo ? assets.logo : raw
}

export function shareMetadata(origin: string, images: string[], fallback = '') {
  const raster = (raw: string) => {
    if (!raw.trim()) return ''
    try {
      const url = new URL(raw.trim(), origin + '/')
      if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) return ''
      return /\.(png|jpe?g|webp|gif)$/i.test(url.pathname) ? url.href : ''
    } catch { return '' }
  }
  for (const raw of images) {
    const image = raster(raw)
    if (image) return { image, card: 'summary_large_image' }
  }
  return { image: raster(fallback), card: 'summary' }
}
