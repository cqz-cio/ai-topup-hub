set -euo pipefail
CONF=/etc/nginx/conf.d/zhixiang.conf
PACKAGE=/home/ubuntu/zhixiang-icp-catalog-20260916.tar.gz
test -s "$CONF"
test -s "$PACKAGE"
nginx -t
BACKUP=$(mktemp /etc/nginx/zhixiang-icp-backup-XXXXXXXX)
cp -a "$CONF" "$BACKUP"
rollback() {
  trap - ERR
  cp -a "$BACKUP" "$CONF"
  nginx -t && systemctl reload nginx
  echo 'Update failed; previous configuration restored.'
}
trap rollback ERR
export RELEASE=$(mktemp -d /var/www/zhixiang/release-icp-XXXXXXXX)
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
for DOMAIN in qzweb.cloud www.qzweb.cloud; do
  curl --noproxy '*' -fsS --retry 5 --retry-connrefused --retry-delay 1 --max-time 15 --resolve "$DOMAIN:443:127.0.0.1" "https://$DOMAIN/" | cmp - "$RELEASE/index.html"
  echo "$DOMAIN: homepage and ICP footer verified."
done
trap - ERR
echo 'Deployment complete: 8 products, ICP footer; HTTPS preserved.'
