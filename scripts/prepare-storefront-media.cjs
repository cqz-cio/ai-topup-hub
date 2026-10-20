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
  ['brand/jellyfishing/foreground.png', 'foreground.webp'],
  ['brand/patrick-logo.png', 'patrick-logo.webp'],
  ['brand/patrick-logo.png', 'patrick-icon.png', 48],
  ['catalog-products/transparent-icons-20261020/gemini.png', 'gemini.webp'],
  ['catalog-products/transparent-icons-20261020/gpt.png', 'gpt.webp'],
]

async function verifyPixels(source, output, maskOnly) {
  const decode = file => sharp(file).ensureAlpha().raw().toBuffer({ resolveWithObject: true })
  const [before, after] = await Promise.all([decode(source), decode(output)])
  if (before.info.width !== after.info.width || before.info.height !== after.info.height ||
      before.data.length !== after.data.length) throw new Error(`Image dimensions changed: ${output}`)
  for (let i = 0; i < before.data.length; i += 4) {
    if (before.data[i + 3] !== after.data[i + 3]) throw new Error(`Alpha changed: ${output}`)
    // RGB beneath fully transparent pixels is not visible. The foreground is
    // used only as an alpha mask; all other visible RGB must be bit-for-bit equal.
    if (!maskOnly && before.data[i + 3] > 0) {
      for (let channel = 0; channel < 3; channel++) {
        if (before.data[i + channel] !== after.data[i + channel]) {
          throw new Error(`Visible pixel changed: ${output}`)
        }
      }
    }
  }
}

async function main() {
  await fs.mkdir(destination, { recursive: true })
  const report = []
  for (const [source, output, width] of files) {
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
      : image.webp({ quality: 100, effort: 6, lossless: true })
    const info = await image.toFile(path.join(destination, output))
    if (!width) await verifyPixels(path.join(root, source), path.join(destination, output), output === 'foreground.webp')
    report.push({
      source, output, originalBytes: (await fs.stat(path.join(root, source))).size,
      bytes: info.size, width: info.width, height: info.height,
      verification: width ? 'favicon-resized' : output === 'foreground.webp' ? 'alpha-identical' : 'visible-pixels-identical',
    })
  }
  console.log(JSON.stringify(report, null, 2))
}
main().catch(error => { console.error(error); process.exitCode = 1 })
