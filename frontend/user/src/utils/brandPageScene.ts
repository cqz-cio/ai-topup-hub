export type BrandPageScene = 'street' | 'garden' | 'home'

// Only the approved classic storefront routes receive a new scene.
// Tenant, template and standalone exclusions remain in App.vue.
export function getBrandPageScene(routeName: unknown): BrandPageScene | null {
  if (typeof routeName !== 'string') return null
  if (['cart', 'checkout', 'payment'].includes(routeName)) return 'street'
  if (['products', 'category-products', 'blog', 'blog-detail', 'notice', 'about', 'terms', 'privacy'].includes(routeName)) return 'garden'
  if (routeName.startsWith('personal-center') || [
    'guest-orders', 'guest-order-detail', 'order-detail', 'recharge-order-detail',
    'user-login', 'user-register', 'user-forgot',
  ].includes(routeName)) return 'home'
  return null
}
