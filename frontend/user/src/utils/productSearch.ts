import { loadStorefrontProducts, type StorefrontProduct, type StorefrontProductPage } from './storefront.ts'

const localizedValues = (value: unknown): string[] => {
  if (typeof value === 'string') return [value]
  if (!value || typeof value !== 'object' || Array.isArray(value)) return []
  return Object.values(value).filter((text): text is string => typeof text === 'string')
}

function normalizeSearchText(text: string): string {
  return text.normalize('NFKC').toLowerCase()
    .replace(/chat\s+gpt/g, 'chatgpt')
    .replace(/(^|[^\d])(5|20|50)\s*(?:x|×|倍)(?![a-z\d])/g, '$1$2x')
    // The catalog also names Pro tiers without the trailing x (e.g. Pro 20新老号).
    .replace(/\bpro\s*(5|20|50)(?!\d)(?:x)?/g, 'pro $1x')
    .replace(/\s+/g, ' ').trim()
}

export function matchesProductSearch(
  product: Pick<StorefrontProduct, 'title' | 'category'>,
  query: string,
  extraText: string[] = [],
): boolean {
  const tokens = normalizeSearchText(query).split(' ').filter(Boolean)
  if (!tokens.length) return true
  const text = normalizeSearchText([
    ...localizedValues(product.title), ...localizedValues(product.category?.name), ...extraText,
  ].join(' '))
  return tokens.every((token) => {
    // Numeric searches must not confuse 5x with 50x or credit quantities like 25000.
    if (/^\d+x?$/.test(token)) return new RegExp(`(?<!\\d)${token}(?!\\d${token.endsWith('x') ? '|[a-z]' : ''})`).test(text)
    return text.includes(token)
  })
}

// Search the complete category before paginating, so aliases and the count are
// consistent even when a matching product is beyond the first API page.
export async function loadProductSearchPage(
  readPage: (page: number) => Promise<StorefrontProductPage>,
  query: string,
  page: number,
  pageSize: number,
) {
  const matches = (await loadStorefrontProducts(readPage)).filter((product) => matchesProductSearch(product, query))
  return {
    products: matches.slice((page - 1) * pageSize, page * pageSize),
    totalItems: matches.length,
    totalPages: Math.ceil(matches.length / pageSize),
  }
}
