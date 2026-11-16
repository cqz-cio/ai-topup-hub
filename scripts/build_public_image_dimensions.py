"""Record intrinsic sizes from checked-in public assets; never resize originals."""
import json
from pathlib import Path
import re
import xml.etree.ElementTree as ET
from PIL import Image

ROOT=Path(__file__).resolve().parents[1]
PUBLIC=ROOT/'frontend/user/public'

def dimensions(path):
    if path.suffix.lower()=='.svg':
        root=ET.parse(path).getroot()
        values=[root.get('width',''),root.get('height','')]
        if not all(re.fullmatch(r'\d+(?:\.\d+)?(?:px)?',v) for v in values):
            box=root.get('viewBox','').replace(',',' ').split()
            values=box[2:] if len(box)==4 else []
        if len(values)==2:
            size=tuple(float(v.removesuffix('px')) for v in values)
            if all(v.is_integer() and v>0 for v in size):return tuple(map(int,size))
        return None
    with Image.open(path) as image:return image.size

def main():
    result={}
    verified=ROOT/'internal/platform/publiccontent/verified-upload-dimensions.json'
    if verified.exists():
        result.update({path:{key:value[key] for key in ['width','height']} for path,value in json.loads(verified.read_text('utf-8')).items()})
    for path in sorted(PUBLIC.rglob('*')):
        if path.suffix.lower() not in {'.svg','.jpg','.jpeg','.png','.gif','.webp','.avif'}:continue
        try:size=dimensions(path)
        except (ValueError,OSError,ET.ParseError):continue
        if size:result['/'+path.relative_to(PUBLIC).as_posix()]={'width':size[0],'height':size[1]}
    target=ROOT/'internal/platform/publiccontent/image-dimensions.json'
    target.write_text(json.dumps(result,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    print(f'{len(result)} verified public image dimensions')

if __name__=='__main__':main()
