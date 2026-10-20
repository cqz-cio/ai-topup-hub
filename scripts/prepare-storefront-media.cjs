// Run with Node + sharp (or NODE_PATH pointing to the bundled sharp installation).
// Keep originals intact. Bump the output directory when changing published bytes.
const sharp = require('sharp')
const fs = require('node:fs/promises')
const path = require('node:path')

const root = path.resolve(__dirname, '../frontend/user/public')
const destination = path.join(root, 'media/storefront-20261020')
const files = [
  ['brand/jellyfishing-background.png', 'jellyfishing.webp'],
  ['brand/jellyfishing/clean-plate.png', 'clean-plate.webp'],
  ['brand/jellyfishing/foreground.png', 'foreground.webp', 0, true],
  ['brand/patrick-logo.png', 'patrick-logo.webp', 256],
  ['brand/patrick-logo.png', 'patrick-icon.png', 48],
  ['catalog-products/transparent-icons-20261020/gemini.png', 'gemini.webp', 960],
  ['catalog-products/transparent-icons-20261020/gpt.png', 'gpt.webp', 960],
]

async function main() {
  await fs.mkdir(destination, { recursive: true })
  const report = []
  for (const [source, output, width, lossless] of files) {
    let image = sharp(path.join(root, source))
    if (output === 'foreground.webp') {
      // The renderer uses this layer only as a source-in alpha mask.
      // Preserve every alpha value; omit RGB detail that is immediately discarded.
      const { data, info } = await image.ensureAlpha().extractChannel('alpha').raw().toBuffer({ resolveWithObject: true })
      image = sharp({ create: { width: info.width, height: info.height, channels: 3, background: '#fff' } })
        .joinChannel(data, { raw: { width: info.width, height: info.height, channels: 1 } })
    }
    if (width) image = image.resize({ width, withoutEnlargement: true })
    image = output.endsWith('.png')
      ? image.png({ compressionLevel: 9 })
      : image.webp({ quality: 90, alphaQuality: 100, effort: 6, lossless: !!lossless })
    const info = await image.toFile(path.join(destination, output))
    report.push({ source, output, originalBytes: (await fs.stat(path.join(root, source))).size, bytes: info.size, width: info.width, height: info.height })
  }
  console.log(JSON.stringify(report, null, 2))
}
main().catch(error => { console.error(error); process.exitCode = 1 })
