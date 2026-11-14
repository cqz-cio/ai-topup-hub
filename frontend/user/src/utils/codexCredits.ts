// Explicit catalog identities keep similarly named subscriptions and SMS items unchanged.
const creditPoints: Readonly<Record<string, number>> = {
  'qzweb-73': 250,
  'qzweb-74': 500,
  'qzweb-75': 1000,
  'qzweb-76': 2500,
  'qzweb-77': 5000,
  'qzweb-78': 25000,
}

export function codexCreditPoints(product: { slug?: string; slug_aliases?: string[] }): number | undefined {
  return creditPoints[product.slug || ''] ?? product.slug_aliases?.map(slug => creditPoints[slug]).find(points => points !== undefined)
}

export function splitCodexCredits<T extends { slug?: string; slug_aliases?: string[] }>(products: readonly T[]) {
  return {
    credits: products.filter(product => codexCreditPoints(product) !== undefined)
      .sort((a, b) => codexCreditPoints(a)! - codexCreditPoints(b)!),
    other: products.filter(product => codexCreditPoints(product) === undefined),
  }
}

export function exactCreditStock(product: {
  stock_quantity_hidden?: boolean
  stock_display_mode?: string
  auto_stock_available?: number
}): number | undefined {
  if (product.stock_quantity_hidden || product.stock_display_mode !== 'exact') return undefined
  const quantity = Number(product.auto_stock_available)
  return Number.isSafeInteger(quantity) && quantity >= 0 ? quantity : undefined
}
