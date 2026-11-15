"""Read-only acceptance checks for SEO v4, after approved copy is published."""
import argparse,datetime,json,re,unicodedata,urllib.request,urllib.parse,xml.etree.ElementTree as ET
from decimal import Decimal
from html.parser import HTMLParser
from pathlib import Path

ROOT=Path(__file__).resolve().parents[1]
class Page(HTMLParser):
 def __init__(self,text):
  super().__init__(convert_charrefs=True);self.stack=[];self.meta={};self.canonical='';self.title='';self.h1='';self.visible='';self.body='';self.peers=[];self.links=[];self.schemas=[];self.script='';self.feed(text)
 def handle_starttag(self,tag,attrs):
  a=dict(attrs)
  if tag=='meta':self.meta[a.get('name') or a.get('property')]=a.get('content','')
  if tag=='link' and a.get('rel')=='canonical':self.canonical=a.get('href','')
  if tag=='a':
   self.links.append(a.get('href',''))
   if any(t=='aside' and x.get('aria-label')=='其他 Grok 档位' for t,x in self.stack):self.peers.append(a.get('href',''))
  if tag not in ['meta','link','img','br','hr','input','source','wbr','area','base','embed']:self.stack.append((tag,a))
  if tag=='script':self.script=''
 def handle_endtag(self,tag):
  if tag=='script' and self.stack and self.stack[-1][1].get('type')=='application/ld+json':self.schemas.append(json.loads(self.script))
  for i in range(len(self.stack)-1,-1,-1):
   if self.stack[i][0]==tag:self.stack=self.stack[:i];break
 def handle_data(self,data):
  tags=[t for t,a in self.stack]
  if 'script' in tags:self.script+=data;return
  if 'style' in tags:return
  if 'title' in tags:self.title+=data
  if 'h1' in tags:self.h1+=data
  if any(t=='main' and a.get('id')=='public-document' for t,a in self.stack):self.visible+=data+' '
  if any(t=='section' and a.get('aria-label')=='完整正文' for t,a in self.stack):self.body+=data+' '

parser=argparse.ArgumentParser(description=__doc__)
parser.add_argument('--base',default='http://127.0.0.1:8080')
parser.add_argument('--out',default='work/v4-local-public.json')
parser.add_argument('--content-date',default=datetime.datetime.now(datetime.timezone.utc).strftime('%Y-%m-%d'),help='UTC date of the approved content update')
args=parser.parse_args()
base=args.base.rstrip('/');catalog=json.loads((ROOT/'docs/catalog/seo-v4/grok-copy.json').read_text('utf-8'))
now=datetime.datetime.now(datetime.timezone.utc)
report={'base':base,'time':now.isoformat(),'beijing_time':now.astimezone(datetime.timezone(datetime.timedelta(hours=8))).isoformat(),'checks':[],'pages':[],'discoverable':[],'errors':[]}
def shingles(text):
 text=unicodedata.normalize('NFKC',text).lower()
 text=re.sub(r'\b(ultra|super|premium|plus|monthly|yearly|cny|rmb)\b|年付|月付|暂时缺货|缺货|有货|库存','',text)
 text=''.join(c for c in text if not c.isspace() and unicodedata.category(c)[0] not in 'NPS')
 return {text[i:i+3] for i in range(len(text)-2)}
def get(path,method='GET'):
 with urllib.request.urlopen(urllib.request.Request(base+path,method=method),timeout=30) as r:
  report['checks'].append({'path':path,'method':method,'status':r.status});return r.status,dict(r.headers),r.read().decode('utf-8')
try:
 source_rows={}
 for item in catalog['products']:
  path='/products/'+item['slug'];status,headers,raw=get(path);page=Page(raw)
  assert status==200 and 'noindex' not in page.meta.get('robots','') and 'noindex' not in headers.get('X-Robots-Tag',''),path
  assert urllib.parse.urlsplit(page.canonical).path==path,path
  assert page.h1==item['title'] and page.meta['description']==item['seo_description'],path
  assert item['seo_title'] in page.title,path
  assert item['description'] in page.visible and '常见问题' in page.body and len(page.body)>600,path
  expected={ '/products/'+p['slug'] for p in catalog['products'] if p['id']!=item['id'] }
  assert set(page.peers)==expected and len(page.peers)==7,path
  assert len(re.findall(r'href="/blog/purchase-and-agent-guide(?:#[^"]*)?"',raw))==1,path
  schema=next(v for v in page.schemas if v.get('@type')=='Product')
  assert any(re.search(r'\.(jpe?g|png|webp)$',urllib.parse.urlsplit(image).path,re.I) for image in schema['image']),path
  data=json.loads(get('/api/v1/public/products/'+item['slug'])[2])['data'];source_rows[item['slug']]=data
  assert Decimal(schema['offers']['price'])==Decimal(str(data.get('effective_price_amount') or data['price_amount'])),path
  assert schema['offers']['priceCurrency']=='CNY'
  assert schema['offers']['availability']==('https://schema.org/OutOfStock' if data['stock_status']=='out_of_stock' else 'https://schema.org/InStock')
  if item['id']==66:assert schema['offers']['availability']=='https://schema.org/OutOfStock','Plus monthly changed stock state'
  assert page.meta['twitter:card']=='summary_large_image' and page.meta['og:image:width']=='1200' and page.meta['og:image:height']=='630'
  assert get(path,'HEAD')[0]==200
  report['pages'].append({'slug':item['slug'],'h1':page.h1,'description':page.meta['description'],'canonical':page.canonical,'visible_chars':len(page.visible),'body_chars':len(page.body),'body':page.body,'peer_links':page.peers,'schema':schema})
 sitemap=get('/sitemap.xml')[2]
 ns={'s':'http://www.sitemaps.org/schemas/sitemap/0.9'}
 entries={urllib.parse.urlsplit(n.findtext('s:loc',namespaces=ns)).path:n.findtext('s:lastmod',namespaces=ns) for n in ET.fromstring(sitemap).findall('s:url',ns)}
 today=args.content_date
 for item in catalog['products']:assert entries['/products/'+item['slug']]==today,item['slug']
 targets=['chatgpt-pro-200-renew-ph','telegram-premium-12m','chatgpt-pro-100-ph','chatgpt-pro-200-ph','chatgpt-pro-500-ph','chatgpt-pro-500-egypt','chatgpt-pro-100-egypt','gemini-pro-18m-warranty','gemini-pro-18m-no-warranty','chatgpt-plus-manual','chatgpt-go-manual']
 products=json.loads(get('/api/v1/public/products?page=1&page_size=100')[2])['data'];categories={}
 for slug in targets:
  product=next(p for p in products if p['slug']==slug);category='/categories/'+product['category']['slug']
  if category not in categories:categories[category]=Page(get(category)[2])
  assert '/products/'+slug in categories[category].links and '/products/'+slug in entries,slug
  report['discoverable'].append({'slug':slug,'from':category,'lastmod':entries['/products/'+slug]})
 for old in ['qzweb-5','qzweb-7','86gamestore-10006']:assert '/products/'+old not in entries
 comparisons=[]
 for index,left in enumerate(report['pages']):
  for right in report['pages'][index+1:]:
   a,b=shingles(left['body']),shingles(right['body']);score=len(a&b)/len(a|b)
   comparisons.append({'left':left['slug'],'right':right['slug'],'jaccard':score})
   assert score<=.85,(left['slug'],right['slug'],score)
 report['body_comparisons']=comparisons
 report.update(status='passed',sitemap_content_date=today)
except Exception as error:report.update(status='failed');report['errors'].append(repr(error));raise
finally:
 destination=ROOT/args.out;destination.parent.mkdir(parents=True,exist_ok=True)
 destination.write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
 print(json.dumps({'status':report.get('status'),'requests':len(report['checks']),'pages':len(report['pages']),'discoverable':len(report['discoverable']),'errors':report['errors']},ensure_ascii=False))
