// Reuse the deployed visible-copy normalization; do not invent a second score.
import { readFileSync, writeFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { productContentSimilarity } from '../frontend/admin/src/utils/productContentSimilarity.ts'

const folder = new URL('../docs/catalog/seo-engineering-v6-20261117/', import.meta.url)
const names = ['draft-category.html', 'draft-plus.html', 'draft-ph-guide.html', 'draft-ready-account.html']
const rows = []
for (let i = 0; i < names.length; i++) {
  for (let j = i + 1; j < names.length; j++) {
    const similarity = productContentSimilarity(readFileSync(new URL(names[i], folder), 'utf8'), readFileSync(new URL(names[j], folder), 'utf8'))
    rows.push({ left: names[i], right: names[j], similarity, nearDuplicate: similarity > 0.85 })
  }
}
const report = { algorithm: 'existing normalizeProductContent + 3-gram Jaccard', threshold: 0.85,
  scope: 'incremental drafts only; approved full content must be rechecked before any future publication', rows }
const output = new URL('evidence/draft-similarity.json', folder)
writeFileSync(output, JSON.stringify(report, null, 2) + '\n')
console.log(fileURLToPath(output))
if (rows.some(row => row.nearDuplicate)) process.exitCode = 1
