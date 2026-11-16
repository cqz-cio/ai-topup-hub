import sizes from '../../../../internal/platform/publiccontent/image-dimensions.json' with {type: 'json'}
const known: Record<string, {width: number; height: number}> = sizes

/** Intrinsic dimensions only for verified repository assets. Unknowns stay unknown. */
export function imageDimensions(raw: string | undefined | null): {width?: number; height?: number} {
  if (!raw) return {}
  try {
    const url = new URL(raw, 'https://www.qzweb.cloud')
    if (!['www.qzweb.cloud', 'qzweb.cloud'].includes(url.hostname)) return {}
    return known[url.pathname] || {}
  } catch { return {} }
}
