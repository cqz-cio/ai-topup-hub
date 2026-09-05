import type { Metadata } from 'next';
import './globals.css';
export const metadata:Metadata={title:'智享A社 · 商品目录',description:'AI 服务商品目录：商品图片、原价及购买须知。静态预览，暂未开放购买。'};
export default function RootLayout({children}:{children:React.ReactNode}){return <html lang="zh-CN"><body>{children}</body></html>}
