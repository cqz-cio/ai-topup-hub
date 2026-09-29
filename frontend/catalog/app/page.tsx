'use client';
// Product JPGs are already optimized locally; this export has no image server.
/* eslint-disable next/no-img-element */
import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { Search, Headphones, MessageCircle, X, Info } from 'lucide-react';
import products from '../data/public-catalog.json';
import {
  Dialog,
  DialogTrigger,
  DialogContent,
  DialogTitle,
  DialogDescription,
  DialogClose,
} from '@/components/ui/dialog';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { Input } from '@/components/ui/input';
type Product = (typeof products)[number] & { available?: boolean };
const categories = ['全部', ...new Set(products.map((p) => p.category))];
const wechat = '15824485317';
const catalogOnly = process.env.NEXT_PUBLIC_CATALOG_ONLY === '1';
const formatPrice = (price: number) =>
  new Intl.NumberFormat('zh-CN', {
    maximumFractionDigits: 2,
    useGrouping: false,
  }).format(price);
const number = (id: number) => String(id).padStart(2, '0');
function useExpired(product?: Product) {
  const [today, setToday] = useState('');
  const expires = product?.expiresOn;
  useEffect(() => {
    if (!expires) return;
    const update = () =>
      setToday(
        new Intl.DateTimeFormat('en-CA', {
          timeZone: 'Asia/Shanghai',
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
        }).format(new Date()),
      );
    update();
    const timer = window.setInterval(update, 60000);
    return () => window.clearInterval(timer);
  }, [expires]);
  return {
    expired: Boolean(expires && today && today > expires),
    checking: Boolean(expires && !today),
  };
}
function ValidityNotice({ product }: { product: Product }) {
  const { expired } = useExpired(product);
  return product.validityNote ? (
    <p className="validity-notice">
      {expired ? '本批商品已过期，已停止购买咨询。' : product.validityNote}
    </p>
  ) : null;
}
function Contact({
  product,
  className = '',
}: {
  product?: Product;
  className?: string;
}) {
  const [copied, setCopied] = useState('');
  const { expired, checking } = useExpired(product);
  async function copy() {
    try {
      await navigator.clipboard.writeText(wechat);
      setCopied('微信号已复制');
    } catch {
      setCopied('请长按或选中上方微信号复制');
    }
  }
  return (
    <Dialog onOpenChange={() => setCopied('')}>
      <DialogTrigger
        className={`contact-button ${className}`}
        disabled={expired || checking}
      >
        {product ? (
          expired ? (
            '已过期'
          ) : checking ? (
            '核对有效期'
          ) : product.pendingValidity ? (
            '确认有效期'
          ) : (
            '咨询购买'
          )
        ) : (
          <>
            <MessageCircle size={19} aria-hidden="true" />
            <span>
              客服微信 <b>{wechat}</b>
            </span>
          </>
        )}
      </DialogTrigger>
      <DialogContent className="contact-dialog" showCloseButton={false}>
        <DialogClose className="contact-close" aria-label="关闭微信咨询">
          关闭 ×
        </DialogClose>
        <DialogTitle>
          {product?.pendingValidity ? '确认商品有效期' : '微信咨询购买'}
        </DialogTitle>
        <DialogDescription>
          添加客服微信，确认商品规格与交付时间后再购买。
        </DialogDescription>
        {product && (
          <div className="consult-product">
            <span>
              {product.title}
            </span>
            <strong>¥{formatPrice(product.price)}</strong>
            <small>
              {product.periodLabel}：{product.period} · {product.warranty}
            </small>
          </div>
        )}
        {product && <ValidityNotice product={product} />}
        <p className="wechat-number">{wechat}</p>
        <button className="contact-button" onClick={copy}>
          复制微信号
        </button>
        <output className="copy-status">
          {copied || '在微信中搜索此号码添加好友'}
        </output>
        <p className="contact-help">
          {product
            ? `咨询时请发送商品名称“${product.title}”，方便客服确认规格。`
            : '可发送商品名称或说明您的需求。'}{' '}
          {!catalogOnly && '在线订单支持扫码付款，由商家核实到账后处理。'}
        </p>
      </DialogContent>
    </Dialog>
  );
}
function Purchase({
  product,
  className = '',
}: {
  product: Product;
  className?: string;
}) {
  const { expired, checking } = useExpired(product);
  if (!product.available || product.pendingValidity || expired || checking) {
    return <Contact product={product} className={className} />;
  }
  return (
    <a
      className={`contact-button ${className}`}
      href={`/products/zxa-${product.id}`}
    >
      立即购买
    </a>
  );
}
function ProductCard({ p, eager, sequence }: { p: Product; eager: boolean; sequence: number }) {
  return (
    <article className="product">
      <div className="cover">
        <img src={p.image} alt={p.title} loading={eager ? 'eager' : 'lazy'} />
        <span className="item-number">{number(sequence)}</span>
      </div>
      <div className="product-body">
        <h3>{p.title}</h3>
        <dl className="product-specs">
          <div>
            <dt>服务方式</dt>
            <dd>{p.deliveryType}</dd>
          </div>
          <div>
            <dt>{p.periodLabel}</dt>
            <dd>{p.period}</dd>
          </div>
          <div>
            <dt>质保范围</dt>
            <dd>{p.warranty}</dd>
          </div>
        </dl>
        <div className="product-bottom">
          <ValidityNotice product={p} />
          <div className="price">
            <small>¥</small>
            {formatPrice(p.price)}
          </div>
          <Dialog>
            <DialogTrigger className="detail-button">查看详情</DialogTrigger>
            <DialogContent className="product-dialog" showCloseButton={false}>
              <div className="dialog-top">
                <span>商品详情</span>
                <DialogClose className="close" aria-label="关闭商品详情">
                  关闭 ×
                </DialogClose>
              </div>
              <div className="dialog-scroll">
                <div className="detail-overview">
                  <img src={p.image} alt="商品图片" />
                  <div>
                    <DialogTitle className="detail-title">
                      {p.title}
                    </DialogTitle>
                    <DialogDescription>
                      {p.deliveryType} · {p.periodLabel}：{p.period} ·{' '}
                      {p.warranty}
                    </DialogDescription>
                    <p className="detail-price">¥{formatPrice(p.price)}</p>
                  </div>
                </div>
                <ValidityNotice product={p} />
                {p.sections.map((section) => (
                  <section className="notice-section" key={section.title}>
                    <h4>{section.title}</h4>
                    <ul>
                      {section.paragraphs.map((text) => (
                        <li key={text}>{text}</li>
                      ))}
                    </ul>
                  </section>
                ))}
                {p.images.map((src) => (
                  <img
                    className="detail-image"
                    key={src}
                    src={src}
                    alt="使用说明中的页面示例"
                    loading="lazy"
                  />
                ))}
                <p className="detail-footnote">
                  请确认服务期限、质保范围和使用条件。具体交付步骤由客服随商品提供。
                </p>
              </div>
              <div className="detail-actions">
                <span>
                  {p.title} · ¥{formatPrice(p.price)}
                </span>
                <Purchase product={p} />
              </div>
            </DialogContent>
          </Dialog>
        </div>
        <Purchase product={p} className="card-contact" />
      </div>
    </article>
  );
}
export default function Home() {
  const contentRef = useRef<HTMLDivElement>(null);
  const [category, setCategory] = useState('全部');
  const [query, setQuery] = useState('');
  const [catalog, setCatalog] = useState<Product[]>(products);
  useEffect(() => {
    if (catalogOnly) return;
    let stopped = false;
    let active: AbortController | undefined;
    async function refresh() {
      active?.abort();
      const controller = new AbortController();
      active = controller;
      const timeout = window.setTimeout(() => controller.abort(), 12000);
      try {
        const response = await fetch('/api/v1/public/products?page_size=100', {
          signal: controller.signal,
          cache: 'no-store',
        });
        const payload = (await response.json()) as {
          status_code?: number;
          data?: Array<{
            slug: string;
            price_amount: string | number;
            is_sold_out?: boolean;
          }>;
        };
        if (
          !response.ok ||
          payload.status_code !== 0 ||
          !Array.isArray(payload.data)
        )
          throw new Error('catalog unavailable');
        const live = new Map<
          string,
          { price_amount: string | number; is_sold_out?: boolean }
        >(
          payload.data.map(
            (p: {
              slug: string;
              price_amount: string | number;
              is_sold_out?: boolean;
            }) => [p.slug, p],
          ),
        );
        if (!stopped && active === controller)
          setCatalog(
            products.map((p) => {
              const current = live.get(`zxa-${p.id}`);
              const price = Number(current?.price_amount);
              return {
                ...p,
                price:
                  current && Number.isFinite(price) && price > 0
                    ? price
                    : p.price,
                available: Boolean(
                  current &&
                  Number.isFinite(price) &&
                  price > 0 &&
                  !current.is_sold_out,
                ),
              };
            }),
          );
      } catch {
        if (!stopped && active === controller)
          setCatalog(products.map((p) => ({ ...p, available: false })));
      } finally {
        window.clearTimeout(timeout);
      }
    }
    void refresh();
    const timer = window.setInterval(() => void refresh(), 60000);
    const onFocus = () => void refresh();
    window.addEventListener('focus', onFocus);
    return () => {
      stopped = true;
      active?.abort();
      window.clearInterval(timer);
      window.removeEventListener('focus', onFocus);
    };
  }, []);
  const search = query.trim().toLowerCase();
  const matched = catalog.filter((p) =>
    [
      p.title,
      p.category,
      p.period,
      p.warranty,
      p.deliveryType,
    ]
      .join(' ')
      .toLowerCase()
      .includes(search),
  );
  const visible = matched.filter(
    (p) => category === '全部' || p.category === category,
  );
  return (
    <>
      <header className="masthead">
        <Link href="/" className="brand">
          智享<span>A</span>社<span className="brand-note">AI SERVICES</span>
        </Link>
        <nav className="header-links" aria-label="商城信息">
          {!catalogOnly && <>
            <a href="/auth/login">登录 / 注册</a>
            <a href="/me/orders">我的订单</a>
            <a href="/guest/orders">游客查单</a>
          </>}
          <Contact className="header-contact" />
          <a href="#about">关于我们</a>
          <a href="#help">购买帮助</a>
        </nav>
      </header>
      <main className="storefront">
        <Tabs
          value={category}
          onValueChange={(value) => {
            setCategory(String(value));
            contentRef.current?.scrollTo({ top: 0 });
          }}
          className="store-layout"
        >
          <aside className="store-sidebar">
            <div className="sidebar-sticky">
              <h2>平台导航</h2>
              <TabsList className="platform-list" aria-label="商品分类">
                {categories.map((c) => (
                  <TabsTrigger key={c} value={c}>
                    {c}
                  </TabsTrigger>
                ))}
              </TabsList>
              <section className="sidebar-contact">
                <h3>
                  <Headphones size={21} aria-hidden="true" />
                  咨询购买
                </h3>
                <p>
                  选好商品后在线下单，扫码付款并提交核实；有疑问可联系微信客服。
                </p>
                <span>客服微信</span>
                <Contact className="sidebar-wechat" />
              </section>
            </div>
          </aside>
          <div
            className="store-content"
            ref={contentRef}
            tabIndex={0}
            role="region"
            aria-label="商品与购买说明"
          >
            <section className="store-intro">
              <div>
                <h1>你的AI服务，一站选好。</h1>
                <p>账号与充值服务清晰对比，在线下单，随时查看订单进度。</p>
              </div>
              <div className="search-field">
                <Search size={20} aria-hidden="true" />
                <Input
                  aria-label="搜索商品名称或质保"
                  type="search"
                  placeholder="搜索商品名称或质保"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                />
                {query && (
                  <button onClick={() => setQuery('')} aria-label="清空搜索">
                    <X size={18} />
                  </button>
                )}
              </div>
            </section>
            <div className="catalog-heading">
              <h2>
                {category === '全部' ? '全部商品' : category}
                <span>{visible.length}</span>
              </h2>
              <output className="catalog-status">
                {search
                  ? '找到 ' + visible.length + ' 件商品'
                  : '按服务类型与质保区分，选择更清楚'}
              </output>
            </div>
            {categories.map((c) => (
              <TabsContent key={c} value={c}>
                {c === category &&
                  (visible.length ? (
                    <section className="catalog" aria-label={c + '商品目录'}>
                      {visible.map((p, i) => (
                        <ProductCard p={p} eager={i < 6} sequence={i + 1} key={p.id} />
                      ))}
                    </section>
                  ) : (
                    <div className="empty-state">
                      <h3>没有找到符合条件的商品</h3>
                      <p>试试其他名称，或清除分类和搜索条件。</p>
                      <button
                        className="contact-button"
                        onClick={() => {
                          setCategory('全部');
                          setQuery('');
                        }}
                      >
                        查看全部商品
                      </button>
                    </div>
                  ))}
              </TabsContent>
            ))}
            <section className="shopping-guide" id="help">
              <h2>购买前，先确认这三件事</h2>
              <div>
                <p>
                  <strong>01 / 选对服务</strong>
                  成品账号、自有账号代充值与API密钥的使用方式不同。
                </p>
                <p>
                  <strong>02 / 看清质保</strong>
                  首登、激活、24小时和20天质保各有范围，以商品详情为准。
                </p>
                <p>
                  <strong>{catalogOnly ? '03 / 确认交付' : '03 / 下单与查进度'}</strong>
                  {catalogOnly ? '购买前联系微信客服，确认库存、交付时间和使用说明。' : '下单后扫码支付并提交核实。商家确认后处理，卡密和使用说明在订单中查看。'}
                </p>
              </div>
            </section>
            <section className="about-store" id="about">
              <h2>关于智享A社</h2>
              <p>
                {catalogOnly ? '提供AI服务与邮箱账号咨询。商品的期限、质保和使用条件均可在详情中查看，购买前请联系微信客服确认。' : '提供AI大模型账号、订阅充值、API与AI创作工具服务咨询。商品的期限、质保和使用条件均可在详情中查看，付款由商家人工核实，订单页会显示商家是否已确认；超出预计时间可联系微信客服。'}
              </p>
            </section>
            <footer className="store-footer">
              <Info size={18} aria-hidden="true" />
              <p>购买方式：联系微信客服确认后购买，本站暂未开放在线支付。</p>
              <span>服务期限与质保期不同，请查看商品详情。</span>
            </footer>
          </div>
        </Tabs>
      </main>
    </>
  );
}
