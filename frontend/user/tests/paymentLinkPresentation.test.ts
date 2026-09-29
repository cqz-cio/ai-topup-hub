import test from 'node:test'
import assert from 'node:assert/strict'
import { readdirSync, readFileSync } from 'node:fs'
import { basename, join, relative } from 'node:path'
import { fileURLToPath } from 'node:url'

const userSourceRoot = fileURLToPath(new URL('../src', import.meta.url))
const paymentViewNames = new Set(['Payment.vue', 'RechargeOrderDetail.vue'])
const defaultPaymentView = join(userSourceRoot, 'views', 'Payment.vue')

const collectPaymentViews = (directory: string): string[] => {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return collectPaymentViews(path)
    return paymentViewNames.has(entry.name) ? [path] : []
  })
}

const defaultPaymentViews = [
  defaultPaymentView,
  join(userSourceRoot, 'views', 'RechargeOrderDetail.vue'),
]

const paymentViews = [
  ...defaultPaymentViews,
  ...collectPaymentViews(join(userSourceRoot, 'templates')),
]

test('payment views keep pay URLs behind actions instead of rendering the raw link', () => {
  assert.ok(paymentViews.length >= 4, 'default and themed payment views should be covered')

  for (const path of paymentViews) {
    const source = readFileSync(path, 'utf8')
    const template = source.match(/<template>([\s\S]*?)<\/template>/)?.[1]
    const viewName = relative(userSourceRoot, path) || basename(path)

    assert.ok(template, `${viewName} should contain a template`)
    assert.match(template, /payment\.openPayLink/, `${viewName} should retain the open payment link action`)
    assert.doesNotMatch(
      template,
      /\{\{\s*(?:paymentResult\.pay_url|payLink)\s*\}\}/,
      `${viewName} should not render the raw payment URL`,
    )
  }
})

test('default payment views present open-link actions as primary buttons', () => {
  for (const path of defaultPaymentViews) {
    const source = readFileSync(path, 'utf8')
    const template = source.match(/<template>([\s\S]*?)<\/template>/)?.[1]
    const viewName = relative(userSourceRoot, path) || basename(path)

    assert.ok(template, `${viewName} should contain a template`)

    const openLinkButtons = [
      ...template.matchAll(/<Button\b([^>]*@click="handleOpenPayLink"[^>]*)>/g),
    ]

    assert.ok(openLinkButtons.length > 0, `${viewName} should contain an open payment link button`)
    for (const [, attributes] of openLinkButtons) {
      assert.match(
        attributes,
        /variant="default"/,
        `${viewName} should emphasize the open payment link as the primary action`,
      )
    }
  }
})

test('default payment view presents copy-link actions as outlined secondary buttons', () => {
  const source = readFileSync(defaultPaymentView, 'utf8')
  const template = source.match(/<template>([\s\S]*?)<\/template>/)?.[1]

  assert.ok(template, 'views/Payment.vue should contain a template')

  const copyLinkButtons = [
    ...template.matchAll(/<Button\b([^>]*@click="handleCopyPayLink"[^>]*)>/g),
  ]

  assert.ok(copyLinkButtons.length > 0, 'views/Payment.vue should contain copy payment link buttons')
  for (const [, attributes] of copyLinkButtons) {
    assert.match(
      attributes,
      /variant="outline"/,
      'views/Payment.vue should present copy payment links as outlined secondary actions',
    )
  }
})
