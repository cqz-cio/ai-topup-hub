// Public display details from frontend/catalog/data/public-catalog.json.
// Prices, availability and purchase slugs always come from the storefront API.
export interface StorefrontDisplayDetails {
  category: string
  deliveryType: string
  period: string
  periodLabel: string
  warranty: string
  image: string
}

export const storefrontDisplayDetails: Record<
  string,
  StorefrontDisplayDetails
> = {
  'zxa-1': {
    category: 'ChatGPT',
    deliveryType: '成品账号',
    period: '月卡',
    periodLabel: '使用期限',
    warranty: '24小时质保',
    image: '/catalog-products/51b9fa3a993e4df58dc55ee3af246b.jpg',
  },
  'zxa-2': {
    category: 'ChatGPT',
    deliveryType: '成品账号',
    period: '月卡',
    periodLabel: '使用期限',
    warranty: '20天质保',
    image: '/catalog-products/274c9483478cf511bbd14bc65a3926.jpg',
  },
  'zxa-3': {
    category: 'ChatGPT',
    deliveryType: '成品账号',
    period: '无付费订阅',
    periodLabel: '使用期限',
    warranty: '首登质保',
    image: '/catalog-products/9c416262bd799c295d0beca233a53f.jpg',
  },
  'zxa-4': {
    category: 'ChatGPT',
    deliveryType: '自助充值',
    period: '月卡',
    periodLabel: '使用期限',
    warranty: '订阅质保',
    image: '/catalog-products/68ab65a1551230b14e21bf165a14f1.jpg',
  },
  'zxa-5': {
    category: 'ChatGPT',
    deliveryType: '自助充值',
    period: '月卡',
    periodLabel: '使用期限',
    warranty: '订阅质保',
    image: '/catalog-products/ad164ca5e1678795061ce64705be4c.jpg',
  },
  'zxa-6': {
    category: 'ChatGPT',
    deliveryType: '自助充值',
    period: '月卡',
    periodLabel: '使用期限',
    warranty: '订阅质保',
    image: '/catalog-products/b729782a65596b0cf751320e932aca.jpg',
  },
  'zxa-8': {
    category: '验证码服务',
    deliveryType: '验证码服务',
    period: '约25天',
    periodLabel: '使用期限',
    warranty: '仅验证码服务',
    image: '/catalog-products/097b405cdd903e0750a19c6d848b4c.jpg',
  },
  'zxa-23': {
    category: '邮箱账号',
    deliveryType: '成品账号',
    period: '200X—201X年',
    periodLabel: '注册年份',
    warranty: '售后范围咨询确认',
    image: '/catalog-products/gmail-random-account.png',
  },
}
