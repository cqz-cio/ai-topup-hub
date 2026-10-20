// Transparent variants for these three existing product icons only.
// Original files and product records remain available for rollback.
const transparentProductIcons: Record<string, string> = {
    '/uploads/86gamestore/f0bb72279d34b2b14d08.jpg': '/media/storefront-20261020/gpt.webp',
    '/uploads/86gamestore/c779fc65589ae19c7ff2.jpg': '/catalog-products/transparent-icons-20261020/telegram.svg',
    '/catalog-products/qzweb/aa16fb0f3bb8f6933985a4c8df660946.jpg': '/media/storefront-20261020/gemini.webp',
    '/catalog-products/transparent-icons-20261020/gpt.png': '/media/storefront-20261020/gpt.webp',
    '/catalog-products/transparent-icons-20261020/gemini.png': '/media/storefront-20261020/gemini.webp',
    '/brand/patrick-logo.png': '/media/storefront-20261020/patrick-logo.webp',
}

/**
 * 获取完整的图片 URL
 * 如果传入的是相对路径（如 /uploads/xxx.png），则拼接 API 基础 URL
 * 如果传入的是完整 URL（如 http://...），则直接返回
 */
export function getImageUrl(path: string | undefined | null): string {
    if (!path) return ''

    // 如果已经是完整 URL，直接返回
    if (path.startsWith('http://') || path.startsWith('https://')) {
        return path
    }

    // 获取 API 基础 URL
    const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || ''

    // 确保路径以 / 开头
    const normalizedPath = path.startsWith('/') ? path : `/${path}`

    const transparentIcon = transparentProductIcons[normalizedPath.split(/[?#]/, 1)[0] ?? '']
    if (transparentIcon) return transparentIcon

    return `${apiBaseUrl}${normalizedPath}`
}

/**
 * 从 images 数组中获取第一张图片的完整 URL
 */
export function getFirstImageUrl(images: any): string {
    if (!images) return ''

    let imageUrl = ''

    if (Array.isArray(images)) {
        imageUrl = images[0] || ''
    } else if (images.images && Array.isArray(images.images)) {
        imageUrl = images.images[0] || ''
    }

    return getImageUrl(imageUrl)
}
