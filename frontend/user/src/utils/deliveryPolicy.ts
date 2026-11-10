import policy from '../../../../internal/platform/publiccontent/delivery-policy.json' with { type: 'json' }

export function appendDeliveryPolicy(content: string, isReseller: boolean): string {
  if (isReseller || content.includes(policy.path)) return content
  return content + `<p>${policy.prefix}<a href="${policy.path}">${policy.title}</a>${policy.suffix}</p>`
}

export function merchantReturnPolicy(origin: string) {
  const url = origin.replace(/\/+$/, '') + policy.path
  // Google's link-only option represents the actual policy page without
  // inventing a common return period for differently fulfilled digital goods.
  return { '@type': 'MerchantReturnPolicy', '@id': url + '#merchant-return-policy', merchantReturnLink: url }
}

export function offerMerchantIdentity(origin: string, isReseller: boolean) {
  const seller = { '@id': origin.replace(/\/+$/, '') + '/#organization' }
  return isReseller ? { seller } : { seller, hasMerchantReturnPolicy: { '@id': merchantReturnPolicy(origin)['@id'] } }
}
