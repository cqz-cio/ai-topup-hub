"""Read-only SEO v5 acceptance, defaulting to the isolated local preview.

Product copy must have been approved/applied locally or independently deployed.
No indexing requests, database writes or external services are triggered.
"""
import argparse,datetime,json,re,urllib.request,urllib.parse,xml.etree.ElementTree as ET
from html.parser import HTMLParser
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]

class Document(HTMLParser):
    def __init__(self,raw):
        super().__init__(convert_charrefs=True)
        self.stack=[];self.meta={};self.styles=[];self.links=[];self.footer=[];self.peers=[];self.images=[];self.schemas=[];self.canonical='';self.title='';self.body='';self.visible='';self.script=''
        self.feed(raw)
    def handle_starttag(self,tag,attrs):
        a=dict(attrs)
        if tag=='meta':self.meta[a.get('name') or a.get('property')]=a.get('content','')
        if tag=='link':
            if a.get('rel')=='canonical':self.canonical=a.get('href','')
            if a.get('rel')=='stylesheet':self.styles.append(a.get('href',''))
        if tag=='a':
            self.links.append(a.get('href',''))
            if any(t=='footer' for t,_ in self.stack):self.footer.append(a.get('href',''))
            if any(t=='aside' and x.get('aria-label','').startswith('其他') for t,x in self.stack):self.peers.append(a.get('href',''))
        if tag=='img':self.images.append(a)
        if tag=='script':self.script=''
        if tag not in {'meta','link','img','br','hr','input','source','wbr','area','base','embed'}:self.stack.append((tag,a))
    def handle_endtag(self,tag):
        if tag=='script' and self.stack and self.stack[-1][1].get('type')=='application/ld+json':
            value=json.loads(self.script);self.schemas.extend(value.get('@graph',[value]))
        for i in range(len(self.stack)-1,-1,-1):
            if self.stack[i][0]==tag:self.stack=self.stack[:i];break
    def handle_data(self,data):
        if any(t=='script' for t,_ in self.stack):self.script+=data;return
        if any(t=='style' for t,_ in self.stack):return
        if any(t=='h1' for t,_ in self.stack):self.title+=data
        if any(t=='main' and a.get('id')=='public-document' for t,a in self.stack):self.visible+=data+' '
        if any(t=='section' and a.get('aria-label')=='完整正文' for t,a in self.stack):self.body+=data+' '

def verify(base,out):
    catalog=json.loads((ROOT/'docs/catalog/seo-v5/copy.json').read_text('utf-8'))
    clusters=json.loads((ROOT/'internal/platform/publiccontent/product-clusters.json').read_text('utf-8'))
    report={'base':base,'checked_at':datetime.datetime.now(datetime.timezone.utc).isoformat(),'checks':[],'pages':[],'errors':[]}
    def get(path,method='GET',ua='SEO-v5-local-verification'):
        with urllib.request.urlopen(urllib.request.Request(base+path,headers={'User-Agent':ua},method=method),timeout=15) as response:
            report['checks'].append({'path':path,'method':method,'status':response.status})
            return dict(response.headers),response.read().decode('utf-8')
    try:
        raw_categories=json.loads(get('/api/v1/public/categories')[1])['data']
        def flatten(rows):
            for row in rows:
                yield row
                yield from flatten(row.get('children',[]))
        products=json.loads(get('/api/v1/public/products?page=1&page_size=100')[1])['data']
        populated={p['category_id'] for p in products}
        categories=[c for c in flatten(raw_categories) if c['id'] in populated]
        footer={'/products','/blog','/notice','/about','/privacy','/terms'}|{'/categories/'+c['slug'] for c in categories}
        for item in catalog['products']:
            path='/products/'+item['slug'];headers,raw=get(path);doc=Document(raw)
            assert doc.meta['description']==item['seo_description'] and doc.meta['og:description']==doc.meta['description'],path
            assert len(doc.meta['description'])>=70 and doc.meta['description']!=doc.title,path
            assert urllib.parse.urlsplit(doc.canonical).path==path,path
            cluster=next(c for c in clusters if item['slug'] in c['slugs'])
            peers={'/products/'+slug for slug in cluster['slugs'] if slug!=item['slug']}
            assert peers<=set(doc.peers) and path not in doc.peers,(path,doc.peers)
            assert footer<=set(doc.footer),(path,footer-set(doc.footer))
            assert not {'/cart','/checkout','/me'}&set(doc.footer),path
            assert all(re.fullmatch(r'/assets/[\w/-]+\.css',style) for style in doc.styles),doc.styles
            faq=next(s for s in doc.schemas if s.get('@type')=='FAQPage')
            assert len(faq['mainEntity'])>=3,path
            visible=''.join(doc.body.split())
            for qa in faq['mainEntity']:
                assert ''.join(qa['name'].split()) in visible and ''.join(qa['acceptedAnswer']['text'].split()) in visible,(path,qa)
            schema=next(s for s in doc.schemas if s.get('@type')=='Product')
            assert schema.get('offers') and any(re.search(r'\.(jpg|png|webp|jpeg)$',image) for image in schema['image']),path
            crumbs=next(s for s in doc.schemas if s.get('@type')=='BreadcrumbList')['itemListElement']
            assert len(crumbs)==3 and '/categories/' in crumbs[1]['item'],path
            assert all(img.get('width') and img.get('height') for img in doc.images),(path,doc.images)
            assert 'public, max-age=60' in headers.get('Cache-Control',''),headers
            for ua in ['Googlebot','OAI-SearchBot']:
                assert get(path,ua=ua)[1]==raw,('UA content differs',path)
            get(path,'HEAD')
            report['pages'].append({'path':path,'description':doc.meta['description'],'body':doc.body,'visible':doc.visible,'faq_count':len(faq['mainEntity']),'peer_links':doc.peers,'styles':doc.styles,'dimensions':doc.images})
        for path in ['/terms','/privacy','/blog','/notice','/about','/products']+['/categories/'+c['slug'] for c in categories]:
            doc=Document(get(path)[1]);desc=doc.meta['description']
            assert 60<=len(desc)<=130 and desc!=doc.title,(path,len(desc),desc)
            assert desc==doc.meta['og:description'] and footer<=set(doc.footer),path
            report['pages'].append({'path':path,'description':desc,'description_chars':len(desc)})
        for item in catalog['media']:
            path=('/products/' if item['table']=='products' else '/categories/')+item['slug']
            doc=Document(get(path)[1])
            assert doc.images and urllib.parse.urlsplit(doc.images[0]['src']).path==item['image'],path
            assert doc.meta['og:image'].endswith(item['image']) and doc.images[0]['width']=='1200' and doc.images[0]['height']=='630',path
        namespace={'s':'http://www.sitemaps.org/schemas/sitemap/0.9'}
        urls=ET.fromstring(get('/sitemap.xml')[1]).findall('s:url',namespace)
        dates={urllib.parse.urlsplit(node.findtext('s:loc',namespaces=namespace)).path:node.findtext('s:lastmod',namespaces=namespace) for node in urls}
        for path in ['/','/products','/blog','/notice','/terms','/privacy']:assert dates.get(path),(path,'missing actual date')
        assert dates['/terms']=='2026-10-14' and dates['/privacy']=='2026-10-13',dates
        report['static_lastmod']={path:dates.get(path) for path in ['/','/products','/blog','/notice','/terms','/privacy','/about']}
        report['status']='passed'
    except Exception as error:
        report['status']='failed';report['errors'].append(repr(error));raise
    finally:
        out.parent.mkdir(parents=True,exist_ok=True);out.write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
        print(json.dumps({'status':report.get('status'),'requests':len(report['checks']),'pages':len(report['pages']),'errors':report['errors']},ensure_ascii=False))

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__);parser.add_argument('--base',default='http://127.0.0.1:5188');parser.add_argument('--out',default='work/seo-v5/http-checks.json')
    args=parser.parse_args();verify(args.base.rstrip('/'),ROOT/args.out)
