// Catalog-specific summaries of the published purchase rules. Keep these profiles
// in sync when changing eligibility or warranty terms in the catalog.
export type PurchaseRule = 'activation' | 'renewal' | 'go' | 'credits' | 'jio' | 'accountAge' | 'antigravity' | 'subscriptionWarranty' | 'activationOnly' | 'geminiWarranty' | 'presale'

const activationSlugs = new Set(['qzweb-5', 'qzweb-6', 'qzweb-55', 'qzweb-56', 'qzweb-57', 'qzweb-58'])
const creditSlugs = new Set(['qzweb-73', 'qzweb-74', 'qzweb-75', 'qzweb-76', 'qzweb-77', 'qzweb-78'])

export function productPurchaseGuide(slug: string): { rules: PurchaseRule[]; kind: 'code' | 'link' } | null {
  const code = (rules: PurchaseRule[]) => ({ rules, kind: 'code' as const })
  if (creditSlugs.has(slug)) return code(['credits'])
  if (activationSlugs.has(slug)) return code(['activation', 'subscriptionWarranty'])
  if (slug === 'qzweb-7') return code(['renewal', 'subscriptionWarranty'])
  if (slug === 'qzweb-4') return code(['go'])
  if (['qzweb-50', 'qzweb-51', 'qzweb-52', 'qzweb-53'].includes(slug)) {
    const rules: PurchaseRule[] = ['jio']
    if (['qzweb-50', 'qzweb-53'].includes(slug)) rules.push('accountAge', 'activationOnly')
    else rules.push('geminiWarranty')
    rules.push('antigravity')
    if (slug === 'qzweb-52') rules.push('presale')
    return { rules, kind: 'link' }
  }
  return null
}
