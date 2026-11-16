import identities from '../../../../internal/platform/publiccontent/grok-products.json' with {type:'json'}
import clusters from '../../../../internal/platform/publiccontent/product-clusters.json' with {type:'json'}

export type GrokProductIdentity = { id: number; slug: string; slug_aliases?: string[] }
export function isGrokProduct(product: GrokProductIdentity | null | undefined): boolean {
  return !!product && identities.some(key => key === product.slug || product.slug_aliases?.includes(key))
}
export function otherGrokProducts<T extends GrokProductIdentity>(current: GrokProductIdentity, rows: T[]): T[] {
  if (!isGrokProduct(current)) return []
  return rows.filter(row => row.id !== current.id && row.slug !== current.slug && isGrokProduct(row))
}

export function relatedProductCluster(product: GrokProductIdentity | null | undefined) {
  if (!product) return undefined
  if (isGrokProduct(product)) return {key:'grok', title:'其他 Grok 档位', slugs:identities, extra:[]}
  return clusters.find(cluster => [...cluster.slugs, ...cluster.extra].some(key => key === product.slug || product.slug_aliases?.includes(key)))
}
export function otherRelatedProducts<T extends GrokProductIdentity>(current: GrokProductIdentity, rows: T[]): T[] {
  const cluster=relatedProductCluster(current)
  if (!cluster) return []
  return rows.filter(row => row.id !== current.id && row.slug !== current.slug && relatedProductCluster(row)?.key === cluster.key)
}
