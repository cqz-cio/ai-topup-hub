import {readFile} from 'node:fs/promises'
import {dirname,resolve} from 'node:path'
import {productContentSimilarity,normalizeProductContent} from '../frontend/admin/src/utils/productContentSimilarity.ts'

const file=resolve(process.argv[2] || 'docs/catalog/seo-v4/grok-copy.json')
const catalog=JSON.parse(await readFile(file,'utf8'))
const rows=await Promise.all(catalog.products.map(async p=>({...p,content:p.file?await readFile(resolve(dirname(file),p.file),'utf8'):p.content})))
const pairs=[]
for(let a=0;a<rows.length;a++)for(let b=a+1;b<rows.length;b++)pairs.push({a:rows[a].slug,b:rows[b].slug,similarity:productContentSimilarity(rows[a].content,rows[b].content)})
const duplicates=pairs.filter(p=>p.similarity>0.85)
console.log(JSON.stringify({status:catalog.status,products:rows.length,visibleLengths:rows.map(p=>({slug:p.slug,normalized:normalizeProductContent(p.content).length})),pairs,duplicates,passed:duplicates.length===0},null,2))
process.exitCode=duplicates.length?1:0
