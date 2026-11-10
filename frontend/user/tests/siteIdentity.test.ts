import test from 'node:test'
import assert from 'node:assert/strict'
import { siteIdentitySchema, schemaJSON } from '../src/utils/siteIdentity.ts'
import { postListPath, postListState } from '../src/utils/postListUrl.ts'

test('identity follows the current tenant without invented company/address/official claims', () => {
  const result = siteIdentitySchema({ site_name: 'Tenant </script>', site_url: 'https://tenant.example', site_logo: 'javascript:alert(1)' }, 'https://other.example')
  const serialized = schemaJSON(result)
  assert.ok(serialized.includes('https://tenant.example/#organization'))
  assert.ok(!serialized.includes('other.example'))
  assert.doesNotMatch(serialized, /π星|logo|legalName|address|sameAs|telephone|<\/script>/)
  assert.equal(JSON.parse(serialized)['@graph'][0].name, 'Tenant </script>')
})

test('blog paging and search survive a share or reload while tracking parameters stay out of canonical', () => {
  assert.deepEqual(postListState({ page: '2', search: ' Plus ', utm_source: 'test' }), { page: 2, search: 'Plus' })
  assert.equal(postListPath('/blog', 2), '/blog?page=2')
  assert.equal(postListPath('/blog', 1), '/blog')
  assert.equal(postListPath('/blog', 2, 'a&b'), '/blog?page=2&search=a%26b')
  for (const value of ['-1', '0', '1.5', '100001', ['2']]) assert.equal(postListState({ page: value }).page, 1)
})
