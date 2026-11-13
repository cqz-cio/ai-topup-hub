import type { Plugin } from 'vite'
import { publicDocumentBootCss, publicDocumentBootScript } from './publicDocumentBoot.ts'

// Backend HTML and frontend assets may be released independently. These stable,
// uncached entry points always load the current frontend's content-hashed files.
export function publicDocumentSources(html: string) {
  const paths = (tag: 'script' | 'link', attribute: string, marker: string) =>
    [...html.matchAll(new RegExp(`<${tag}\\b[^>]*>`, 'g'))]
      .map(match => match[0])
      .filter(value => value.includes(marker))
      .map(value => value.match(new RegExp(`\\b${attribute}="([^"]+)"`))?.[1])
      .filter((value): value is string => !!value)
  const scripts = paths('script', 'src', 'type="module"')
  const styles = paths('link', 'href', 'rel="stylesheet"')
  if (!scripts.length || !styles.length) throw new Error('Public document boot assets missing from index.html')
  for (const path of [...scripts, ...styles]) {
    if (!/^\/assets\/[\w./-]+$/.test(path) || path.includes('..')) {
      throw new Error(`Unexpected public document asset: ${path}`)
    }
  }
  return {
    js: publicDocumentBootScript(scripts),
    css: styles.map(path => `@import ${JSON.stringify(path)};`).join('\n') + '\n' + publicDocumentBootCss,
  }
}

export function publicDocumentEntry(): Plugin {
  return {
    name: 'public-document-entry',
    enforce: 'post',
    generateBundle(_options, bundle) {
      const index = bundle['index.html']
      if (!index || index.type !== 'asset') throw new Error('Public document index.html missing')
      const source = typeof index.source === 'string' ? index.source : new TextDecoder().decode(index.source)
      const entry = publicDocumentSources(source)
      this.emitFile({ type: 'asset', fileName: 'storefront-entry.js', source: entry.js })
      this.emitFile({ type: 'asset', fileName: 'storefront-entry.css', source: entry.css })
    },
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const path = req.url?.split('?')[0]
        if (path !== '/storefront-entry.js' && path !== '/storefront-entry.css') return next()
        res.setHeader('Cache-Control', 'no-cache')
        res.setHeader('Content-Type', path.endsWith('.js') ? 'text/javascript' : 'text/css')
        res.end(path.endsWith('.js') ? publicDocumentBootScript(['/@vite/client', '/src/main.ts']) : publicDocumentBootCss)
      })
    },
  }
}
