import type { Metadata } from 'next';
import './globals.css';
export const metadata: Metadata = {
  title: '智享A社 · 商品目录',
  icons: {
    icon: [
      { url: '/favicon-32-v2.png', sizes: '32x32', type: 'image/png' },
      { url: '/brand-icon-v2.svg', sizes: 'any', type: 'image/svg+xml' },
    ],
    shortcut: '/favicon.ico?v=2',
    apple: '/apple-touch-icon-v2.png',
  },
  description:
    process.env.NEXT_PUBLIC_CATALOG_ONLY === '1' ? '智享A社 AI 服务与邮箱账号商品目录，查看价格、规格及质保说明，联系微信客服咨询购买。' : 'AI 服务商品目录、在线订单与交付进度。扫码付款，商家人工核实，订单内查看交付信息。',
};
export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="zh-CN">
      <body>
        {children}
        <footer className="site-footer" aria-label="网站备案信息">
          <a
            href="https://beian.miit.gov.cn/"
            target="_blank"
            rel="noopener noreferrer"
          >
            浙ICP备2026050415号-2
          </a>
        </footer>
      </body>
    </html>
  );
}
