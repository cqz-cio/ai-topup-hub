// Only the shop's existing vector covers get a transparent catalog variant.
// Classic catalog cards and product details share these variants.
// Uploaded/custom images retain their original asset.
const glassCoverNames = new Set([
  'go-20261001.svg',
  'plus-20261001.svg',
  'pro5x-ph-20261016.svg',
  'pro5x-cl-20261016.svg',
  'pro10x-direct-ph-20261016.svg',
  'pro10x-renew-ph-20261016.svg',
  'pro25x-ph-20261016.svg',
  'pro25x-cl-20261016.svg',
  'pro5x-ios-20261007.svg',
  'pro20x-ios-20261007.svg',
  'codex-250-ph-20261016.svg',
  'codex-500-ph-20261016.svg',
  'codex-1000-ph-20261016.svg',
  'codex-2500-ph-20261016.svg',
  'codex-5000-ph-20261016.svg',
  'codex-25000-ph-20261016.svg',
])

// Approved USD labels retain the same transparent-cover treatment in both themes.
const proUsdCovers = new Set([
  '/catalog-products/pro-usd-20261111/pro-100-ph.svg',
  '/catalog-products/pro-usd-20261111/pro-100-eg.svg',
  '/catalog-products/pro-usd-20261111/pro-200-ph.svg',
  '/catalog-products/pro-usd-20261111/pro-200-renew-ph.svg',
  '/catalog-products/pro-usd-20261111/pro-500-ph.svg',
  '/catalog-products/pro-usd-20261111/pro-500-eg.svg',
])

export function getCatalogGlassCover(image: string): string {
  const prefix = '/catalog-products/qzweb/'
  const path = image.split(/[?#]/, 1)[0] ?? ''
  if (proUsdCovers.has(path)) return path
  if (!path.startsWith(prefix)) return ''
  const name = path.slice(prefix.length)
  return glassCoverNames.has(name) ? `/catalog-products/glass-20261019/${name}` : ''
}
