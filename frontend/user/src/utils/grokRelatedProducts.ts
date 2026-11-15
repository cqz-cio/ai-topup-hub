import identities from '../../../../internal/platform/publiccontent/grok-products.json' with {type:'json'}

export type GrokProductIdentity = { id: number; slug: string; slug_aliases?: string[] }
export function isGrokProduct(product: GrokProductIdentity | null | undefined): boolean {
  return !!product && identities.some(key => key === product.slug || product.slug_aliases?.includes(key))
}
export function otherGrokProducts<T extends GrokProductIdentity>(current: GrokProductIdentity, rows: T[]): T[] {
  if (!isGrokProduct(current)) return []
  return rows.filter(row => row.id !== current.id && row.slug !== current.slug && isGrokProduct(row))
}
