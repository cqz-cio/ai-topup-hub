import sys, tempfile
from pathlib import Path
sys.path.insert(0,str(Path(tempfile.gettempdir())/'zhixiang-qr-libs'))
import qrcode, qrcode.image.svg, zxingcpp
from PIL import Image
url='https://qzweb.cloud'
out=Path('deployment');out.mkdir(exist_ok=True)
qr=qrcode.QRCode(error_correction=qrcode.constants.ERROR_CORRECT_H,box_size=24,border=4)
qr.add_data(url);qr.make(fit=True)
png=out/'qzweb-cloud-qr.png'
qr.make_image(fill_color='black',back_color='white').save(png)
qr.make_image(image_factory=qrcode.image.svg.SvgPathFillImage).save(out/'qzweb-cloud-qr.svg')
result=zxingcpp.read_barcode(Image.open(png))
assert result and result.text==url, 'QR decoding failed'
print(f'Verified: {result.text}; image size: {Image.open(png).size}')
