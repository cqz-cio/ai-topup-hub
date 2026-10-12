set -euo pipefail
CONF=/etc/nginx/conf.d/zhixiang.conf
PACKAGE=/home/ubuntu/zhixiang-prices-20260909.tar.gz
test -s "$CONF"
test -s "$PACKAGE"
nginx -t
BACKUP=$(mktemp /etc/nginx/zhixiang-price-backup-XXXXXXXX)
cp -a "$CONF" "$BACKUP"
rollback() {
  trap - ERR
  cp -a "$BACKUP" "$CONF"
  nginx -t && systemctl reload nginx
  echo 'Update failed; previous configuration restored.'
}
trap rollback ERR
export RELEASE=$(mktemp -d /var/www/zhixiang/release-price-XXXXXXXX)
tar --no-same-owner --no-same-permissions -xzf "$PACKAGE" -C "$RELEASE"
test -s "$RELEASE/index.html"
test -s "$RELEASE/brand-icon-v2.svg"
chown -R root:root "$RELEASE"
find "$RELEASE" -type d -exec chmod 755 {} +
find "$RELEASE" -type f -exec chmod 644 {} +
python3 <<'PY'
import os, pathlib, re
p = pathlib.Path('/etc/nginx/conf.d/zhixiang.conf')
s = p.read_text()
pattern = r'(?m)^(\s*root\s+)/var/www/zhixiang/[^;\s]+;'
if len(re.findall(pattern, s)) != 1:
    raise SystemExit('Unexpected site root; stopped without overwriting it.')
p.write_text(re.sub(pattern, lambda m:m.group(1)+os.environ['RELEASE']+';',s))
PY
nginx -t
systemctl reload nginx
sleep 2
curl --noproxy '*' -fsS --retry 5 --retry-connrefused --retry-delay 1 --max-time 15 --resolve qzweb.cloud:443:127.0.0.1 https://qzweb.cloud/brand-icon-v2.svg | cmp - "$RELEASE/brand-icon-v2.svg"
curl --noproxy '*' -fsS --max-time 15 --resolve qzweb.cloud:443:127.0.0.1 https://qzweb.cloud/ | cmp - "$RELEASE/index.html"
trap - ERR
echo 'Website prices updated successfully: 4=136, 5=688, 6=1250, 8=8.88.'
