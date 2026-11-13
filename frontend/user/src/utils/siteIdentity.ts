import identity from '../../../../internal/platform/publiccontent/site-identity.json' with { type: 'json' }
import { merchantReturnPolicy } from './deliveryPolicy.ts'
import { identityLogo } from './shareSeo.ts'

export const identityStatement = identity.statement
export function siteIdentitySchema(brand: { site_name?: string; site_url?: string; site_logo?: string } | undefined, originFallback: string, description = '', mainSite = false) {
  const origin = String(brand?.site_url || originFallback).replace(/\/+$/, '')
  const name = String(brand?.site_name || '商城')
  const organization: Record<string, unknown> = {
    '@type': 'Organization', '@id': origin + '/#organization', name, url: origin + '/',
    contactPoint: { '@type': 'ContactPoint', contactType: 'customer support', url: origin + '/about' },
  }
  if (description) organization.description = description
  if (mainSite) organization.hasMerchantReturnPolicy = merchantReturnPolicy(origin)
  if (brand?.site_logo) {
    try {
      const logo = new URL(identityLogo(brand.site_logo, mainSite), origin + '/')
      if (['https:', 'http:'].includes(logo.protocol) && !logo.username && !logo.password) organization.logo = logo.href
    } catch { /* Invalid optional logo is omitted. */ }
  }
  return { '@context': 'https://schema.org', '@graph': [organization, {
    '@type': 'WebSite', '@id': origin + '/#website', name, url: origin + '/', publisher: { '@id': origin + '/#organization' },
  }] }
}

export const schemaJSON = (value: unknown) => JSON.stringify(value).replace(/</g, '\\u003c')
