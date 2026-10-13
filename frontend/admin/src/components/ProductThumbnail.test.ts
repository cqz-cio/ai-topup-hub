import { afterEach, describe, expect, it, vi } from 'vitest'
import { createApp, h, nextTick, reactive, type App } from 'vue'
import ProductThumbnail from './ProductThumbnail.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: () => '暂无图片' }) }))

let app: App | undefined
afterEach(() => { app?.unmount(); document.body.replaceChildren() })

function mount(images: unknown) {
  const props = reactive({ images, alt: 'ChatGPT Plus' })
  const host = document.createElement('div')
  document.body.append(host)
  app = createApp({ render: () => h(ProductThumbnail, props) })
  app.mount(host)
  return { host, props }
}

describe('product thumbnail loading', () => {
  it('replaces a broken image with an accessible placeholder', async () => {
    const { host } = mount(['/catalog-products/missing.svg'])
    host.querySelector('img')!.dispatchEvent(new Event('error'))
    await nextTick()
    expect(host.querySelector('img')).toBeNull()
    expect(host.querySelector('[role="img"]')?.getAttribute('aria-label')).toBe('暂无图片')
  })

  it('retries when a product receives a different image after a failure', async () => {
    const { host, props } = mount(['/catalog-products/missing.svg'])
    host.querySelector('img')!.dispatchEvent(new Event('error'))
    await nextTick()
    props.images = { images: ['/catalog-products/qzweb/plus-20261001.svg'] }
    await nextTick()
    expect(host.querySelector('img')?.getAttribute('src')).toBe('/catalog-products/qzweb/plus-20261001.svg')
    expect(host.querySelector('img')?.alt).toBe('ChatGPT Plus')
    expect(host.querySelector('[role="img"]')).toBeNull()
  })

  it('shows a placeholder for an empty image list without making an image request', () => {
    const { host } = mount([])
    expect(host.querySelector('img')).toBeNull()
    expect(host.textContent).toContain('暂无图片')
  })
})
