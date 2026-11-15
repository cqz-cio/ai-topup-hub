/** Advisory comparison of visible product copy, not an indexing prediction. */
export function normalizeProductContent(content: string): string {
  const visible = content
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/<(script|style|template)\b[^>]*>[\s\S]*?<\/\1\s*>/gi, '')
    .replace(/<[^>]*>/g, ' ')
    .replace(/&#(x[\da-f]+|\d+);/gi, (_, code: string) => {
      const value = code[0]?.toLowerCase() === 'x' ? parseInt(code.slice(1), 16) : Number(code)
      return value > 0 && value <= 0x10ffff ? String.fromCodePoint(value) : ''
    })
    .replace(/&(nbsp|amp|lt|gt|quot|apos);/gi, ' ')
    .normalize('NFKC').toLowerCase()
  return visible.replace(/\b(ultra|super|premium|plus|monthly|yearly|cny|rmb)\b|年付|月付|暂时缺货|缺货|有货|库存/gi, '')
    .replace(/[\p{N}\p{P}\p{S}\s]/gu, '')
}

function shingles(text: string): Set<string> {
  return new Set(Array.from({ length: Math.max(0, text.length - 2) }, (_, i) => text.slice(i, i + 3)))
}

export function productContentSimilarity(a: string, b: string): number {
  const left = shingles(normalizeProductContent(a)), right = shingles(normalizeProductContent(b))
  if (!left.size || !right.size) return 0
  let intersection = 0
  for (const value of left) if (right.has(value)) intersection++
  return intersection / (left.size + right.size - intersection)
}

export function nearDuplicateProducts(content: string, peers: { id: number; content: string }[], ownID?: number | null) {
  if (normalizeProductContent(content).length < 60) return []
  return peers.filter(peer => peer.id !== ownID && normalizeProductContent(peer.content).length >= 60)
    .map(peer => ({ id: peer.id, similarity: productContentSimilarity(content, peer.content) }))
    .filter(peer => peer.similarity > 0.85).sort((a, b) => b.similarity - a.similarity)
}
