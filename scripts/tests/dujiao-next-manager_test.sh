#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
MANAGER_SCRIPT=$(cd "${SCRIPT_DIR}/.." && pwd)/dujiao-next-manager.sh

TEST_TMP=$(mktemp -d "${TMPDIR:-/tmp}/dujiao-next-manager-test.XXXXXX")
trap 'rm -rf -- "$TEST_TMP"' EXIT
export DUJIAO_MANAGER_TESTING=1
export DUJIAO_ROOT_PREFIX="${TEST_TMP}/root"
# shellcheck source=../dujiao-next-manager.sh
source "$MANAGER_SCRIPT"

RUN_TMP="${TEST_TMP}/runtime"
mkdir -p "$RUN_TMP"

passed=0
failed=0
skipped=0

pass() {
  printf 'ok - %s\n' "$1"
  passed=$((passed + 1))
}

fail() {
  printf 'not ok - %s\n' "$1" >&2
  failed=$((failed + 1))
}

skip() {
  printf 'ok - %s # SKIP %s\n' "$1" "$2"
  skipped=$((skipped + 1))
}

assert_success() {
  local name=$1
  shift
  if "$@"; then
    pass "$name"
  else
    fail "$name"
  fi
}

assert_failure() {
  local name=$1
  shift
  if "$@"; then
    fail "$name"
  else
    pass "$name"
  fi
}

assert_equal() {
  local name=$1
  local expected=$2
  local actual=$3
  if [[ "$expected" == "$actual" ]]; then
    pass "$name"
  else
    printf '  expected: %s\n  actual:   %s\n' "$expected" "$actual" >&2
    fail "$name"
  fi
}

assert_contains() {
  local name=$1
  local haystack=$2
  local needle=$3
  if [[ "$haystack" == *"$needle"* ]]; then
    pass "$name"
  else
    printf '  missing: %s\n' "$needle" >&2
    fail "$name"
  fi
}

assert_not_contains() {
  local name=$1
  local haystack=$2
  local needle=$3
  if [[ "$haystack" == *"$needle"* ]]; then
    printf '  unexpected: %s\n' "$needle" >&2
    fail "$name"
  else
    pass "$name"
  fi
}

quiet() {
  "$@" >/dev/null
}

assert_success "accepts ordinary FQDN" validate_domain "shop.example.com"
assert_success "accepts uppercase FQDN after normalization" validate_domain "Shop.Example.COM"
assert_failure "rejects URL as domain" validate_domain "https://shop.example.com"
assert_failure "rejects wildcard domain" validate_domain "*.example.com"
assert_failure "rejects single-label host" validate_domain "localhost"
assert_failure "rejects leading hyphen label" validate_domain "-shop.example.com"

assert_success "accepts ACME email" validate_email "admin@example.com"
assert_failure "rejects malformed email" validate_email "admin@example"
assert_success "accepts SMTP hostname" validate_smtp_host "smtp.example.com"
assert_failure "rejects SMTP hostname with spaces" validate_smtp_host "smtp example.com"
assert_failure "rejects SMTP hostname with empty label" validate_smtp_host "smtp..example.com"
assert_success "accepts loopback application port" validate_port "8080"
assert_failure "rejects privileged application port" validate_port "443"
assert_success "accepts SMTP port 465" validate_network_port "465"
assert_failure "rejects out-of-range network port" validate_network_port "65536"
if command -v dpkg >/dev/null 2>&1; then
  assert_success "accepts Ubuntu 22.04" validate_os_version ubuntu 22.04
  assert_success "accepts Ubuntu 24.04" validate_os_version ubuntu 24.04
  assert_failure "rejects Ubuntu 20.04" validate_os_version ubuntu 20.04
  assert_success "accepts Debian 12" validate_os_version debian 12
  assert_failure "rejects Debian 11" validate_os_version debian 11
  assert_failure "rejects unsupported distro" validate_os_version fedora 42
else
  skip "Debian and Ubuntu version matrix" "dpkg is not installed"
fi

assert_success "accepts generated admin path" validate_admin_path "/dj-a1b2c3"
assert_success "accepts multi-segment admin path" validate_admin_path "/ops/private-console"
assert_failure "rejects reserved API path" validate_admin_path "/api/private"
assert_failure "rejects prefix of reserved path" validate_admin_path "/api"
assert_failure "rejects duplicate slash" validate_admin_path "/ops//private"
assert_failure "rejects Gin wildcard" validate_admin_path "/ops/*private"
assert_failure "rejects path normalization segment" validate_admin_path "/ops/../private"

assert_success "accepts strong bootstrap password" validate_admin_password "StrongPass123"
assert_failure "rejects default bootstrap password" validate_admin_password "admin123"
assert_failure "rejects password without uppercase" validate_admin_password "lowercase123"
assert_failure "rejects password without number" validate_admin_password "NoNumberHere"
assert_success "accepts safe admin username" validate_admin_username "root-admin"
assert_failure "rejects whitespace in admin username" validate_admin_username "root admin"
assert_success "accepts exact backup destruction phrase" validate_destroy_phrase "DELETE DUJIAO DATA"
assert_failure "rejects incomplete backup destruction phrase" validate_destroy_phrase "DELETE DUJIAO"

assert_equal "maps amd64 release architecture" "x86_64" "$(platform_asset_arch amd64)"
assert_equal "maps aarch64 release architecture" "arm64" "$(platform_asset_arch aarch64)"
assert_failure "rejects unsupported architecture" platform_asset_arch riscv64
assert_success "accepts semantic release tag" validate_release_tag "v1.5.0"
assert_failure "rejects partial release tag" validate_release_tag "v1.5"
assert_equal "builds exact GoReleaser asset name" \
  "dujiao-next_v1.5.0_Linux_x86_64.tar.gz" \
  "$(archive_name_for v1.5.0 x86_64)"
assert_success "accepts trusted GitHub asset URL" validate_download_url \
  "https://github.com/dujiao-next/dujiao-next/releases/download/v1.5.0/dujiao-next_v1.5.0_Linux_x86_64.tar.gz"
assert_failure "rejects lookalike GitHub host" validate_download_url \
  "https://github.com.evil.invalid/dujiao-next/dujiao-next/releases/download/v1.5.0/file.tar.gz"
assert_success "accepts GitHub release asset CDN" validate_effective_download_url \
  "https://release-assets.githubusercontent.com/github-production-release-asset/example"
assert_failure "rejects lookalike release asset CDN" validate_effective_download_url \
  "https://release-assets.githubusercontent.com.evil.invalid/payload"

release_json="${TEST_TMP}/release.json"
cat > "$release_json" <<'JSON'
{
  "tag_name": "v1.5.0",
  "assets": [
    {
      "name": "dujiao-next_v1.5.0_Linux_x86_64.tar.gz",
      "browser_download_url": "https://github.com/dujiao-next/dujiao-next/releases/download/v1.5.0/dujiao-next_v1.5.0_Linux_x86_64.tar.gz"
    },
    {
      "name": "dujiao-next_1.5.0_checksums.txt",
      "browser_download_url": "https://github.com/dujiao-next/dujiao-next/releases/download/v1.5.0/dujiao-next_1.5.0_checksums.txt"
    }
  ]
}
JSON
assert_success "resolves exact release metadata" quiet resolve_release_metadata "$release_json" x86_64
jq 'del(.assets[0])' "$release_json" > "${TEST_TMP}/release-missing.json"
assert_failure "rejects release missing architecture asset" resolve_release_metadata "${TEST_TMP}/release-missing.json" x86_64
jq '.assets += [.assets[0]]' "$release_json" > "${TEST_TMP}/release-duplicate.json"
assert_failure "rejects duplicate release architecture assets" resolve_release_metadata "${TEST_TMP}/release-duplicate.json" x86_64
printf '%s\n' '{"tag_name":false,"assets":"broken"}' > "${TEST_TMP}/release-malformed.json"
assert_failure "rejects malformed release JSON" resolve_release_metadata "${TEST_TMP}/release-malformed.json" x86_64

printf 'verified release payload' > "${TEST_TMP}/archive.tar.gz"
archive_digest=$(sha256sum "${TEST_TMP}/archive.tar.gz" | awk '{print $1}')
printf '%s  %s\n' "$archive_digest" 'dujiao-next_v1.5.0_Linux_x86_64.tar.gz' > "${TEST_TMP}/checksums.txt"
assert_success "accepts matching release checksum" verify_release_checksum \
  "${TEST_TMP}/archive.tar.gz" "${TEST_TMP}/checksums.txt" 'dujiao-next_v1.5.0_Linux_x86_64.tar.gz'
printf '%064d  %s\n' 0 'dujiao-next_v1.5.0_Linux_x86_64.tar.gz' > "${TEST_TMP}/bad-checksums.txt"
assert_failure "rejects mismatched release checksum" verify_release_checksum \
  "${TEST_TMP}/archive.tar.gz" "${TEST_TMP}/bad-checksums.txt" 'dujiao-next_v1.5.0_Linux_x86_64.tar.gz'

if printf '%s\n' "dujiao-next" "config.yml.example" "README.md" | validate_archive_listing; then
  pass "accepts safe archive listing in current shell"
else
  fail "accepts safe archive listing in current shell"
fi
if printf '%s\n' "../../etc/passwd" | validate_archive_listing; then
  fail "rejects parent traversal archive entry"
else
  pass "rejects parent traversal archive entry"
fi
if printf '%s\n' "/etc/passwd" | validate_archive_listing; then
  fail "rejects absolute archive entry"
else
  pass "rejects absolute archive entry"
fi
if printf '%s\n' '-rwxr-xr-x root/root 1024 2026-01-01 00:00 dujiao-next' 'drwxr-xr-x root/root 0 2026-01-01 00:00 docs/' | validate_archive_types; then
  pass "accepts regular files and directories in archive"
else
  fail "accepts regular files and directories in archive"
fi
if printf '%s\n' 'lrwxrwxrwx root/root 0 2026-01-01 00:00 binary -> /etc/passwd' | validate_archive_types; then
  fail "rejects symlink archive entry"
else
  pass "rejects symlink archive entry"
fi
printf '%s\n' 'dujiao-next' 'config.yml.example' > "${TEST_TMP}/archive-members.txt"
assert_equal "selects unique expected archive member" "dujiao-next" \
  "$(select_archive_member "${TEST_TMP}/archive-members.txt" dujiao-next)"
printf '%s\n' 'dujiao-next' 'nested/dujiao-next' > "${TEST_TMP}/duplicate-members.txt"
assert_failure "rejects duplicate critical archive members" select_archive_member \
  "${TEST_TMP}/duplicate-members.txt" dujiao-next

assert_equal "orders installation phases" "4" "$(phase_rank services_ready)"
assert_success "installed phase satisfies cert stage" phase_at_least installed cert_issued
assert_failure "ACME stage does not satisfy installed" phase_at_least acme_ready installed

redis_config=$(render_redis_config 6380 "RedisSecret123")
assert_contains "Redis binds IPv4 loopback" "$redis_config" "bind 127.0.0.1 -::1"
assert_contains "Redis enables AOF" "$redis_config" "appendonly yes"
assert_contains "Redis writes strong password" "$redis_config" "requirepass RedisSecret123"
assert_not_contains "Redis config stays compatible with Redis 6" "$redis_config" "appenddirname"

app_unit=$(render_app_unit)
assert_contains "app unit always restarts" "$app_unit" "Restart=always"
assert_contains "app unit allows only install directory writes" "$app_unit" "ReadWritePaths=${INSTALL_DIR}"
assert_contains "app unit preserves system protection" "$app_unit" "ProtectSystem=strict"
redis_unit=$(render_redis_unit)
assert_contains "Redis unit uses dedicated config" "$redis_unit" "ExecStart=/usr/bin/redis-server ${REDIS_CONFIG}"
assert_contains "Redis unit runs as service user" "$redis_unit" "User=${SERVICE_USER}"

acme_site=$(render_acme_site "shop.example.com")
assert_contains "ACME site exposes only challenge root" "$acme_site" "location ^~ /.well-known/acme-challenge/"
assert_contains "ACME site rejects storefront traffic" "$acme_site" "return 404;"
final_site=$(render_final_nginx_site "shop.example.com" 8080 "dujiao-next-cert")
assert_contains "final site proxies only to loopback" "$final_site" "proxy_pass http://127.0.0.1:8080;"
assert_contains "final site redirects HTTP to HTTPS" "$final_site" 'return 301 https://$host$request_uri;'
assert_not_contains "final site does not force HSTS" "$final_site" "Strict-Transport-Security"
printf '<script type="module" src="./assets/app.js"></script>\n' > "${TEST_TMP}/admin.html"
assert_equal "resolves admin static asset URL" "https://shop.example.com/dj-safe/assets/app.js" \
  "$(extract_first_asset_url "${TEST_TMP}/admin.html" "https://shop.example.com/dj-safe/")"

mkdir -p "$(dirname "$APP_UNIT_FILE")"
printf 'foreign unit' > "$APP_UNIT_FILE"
if (check_unmanaged_conflicts); then
  fail "rejects unmanaged installation conflict"
else
  pass "rejects unmanaged installation conflict"
fi
rm -f "$APP_UNIT_FILE"

if (
  port_in_use() { return 0; }
  ss() { printf 'LISTEN 0 511 0.0.0.0:80 0.0.0.0:* users:(("apache2",pid=1,fd=3))\n'; }
  check_web_listener_conflicts
); then
  fail "rejects non-Nginx listener on web ports"
else
  pass "rejects non-Nginx listener on web ports"
fi

public_first_call="${TEST_TMP}/public-first-call"
public_continued="${TEST_TMP}/public-continued"
if (
  curl() {
    if [[ ! -e "$public_first_call" ]]; then
      : > "$public_first_call"
      return 1
    fi
    : > "$public_continued"
    return 0
  }
  verify_public_site "shop.example.com" "/dj-safe"
); then
  fail "public verification propagates first failed probe"
else
  pass "public verification propagates first failed probe"
fi
assert_failure "public verification stops after first failed probe" test -e "$public_continued"

if python3 -c 'import yaml' >/dev/null 2>&1; then
  template="${TEST_TMP}/config.yml.example"
  generated="${TEST_TMP}/config.yml"
  cat > "$template" <<'YAML'
app:
  secret_key: placeholder
server:
  host: 0.0.0.0
  port: 8080
  mode: debug
jwt:
  secret: placeholder
user_jwt:
  secret: placeholder
email:
  enabled: false
web:
  admin_path: /admin
YAML
  helper=$(write_yaml_helper)
  printf '%s\0' \
    "8080" "6380" \
    "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" \
    "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb" \
    "cccccccccccccccccccccccccccccccc" \
    "redis-secret" "root-admin" 'Adm"in\Pass123' "/dj-safe" \
    "true" "smtp.example.com" "465" 'smtp-user' 'smtp:pa#ss"word' \
    "sender@example.com" 'Dujiao: "Next"' "false" "true" \
    | python3 "$helper" generate "$template" "$generated"

  if python3 - "$generated" <<'PY'
import sys, yaml
with open(sys.argv[1], encoding="utf-8") as f:
    cfg = yaml.safe_load(f)
assert cfg["server"] == {"host": "127.0.0.1", "port": "8080", "mode": "release"}
assert cfg["app"]["secret_key"] == "a" * 32
assert cfg["jwt"]["secret"] == "b" * 32
assert cfg["user_jwt"]["secret"] == "c" * 32
assert len({cfg["app"]["secret_key"], cfg["jwt"]["secret"], cfg["user_jwt"]["secret"]}) == 3
assert cfg["bootstrap"]["default_admin_password"] == 'Adm"in\\Pass123'
assert cfg["email"]["password"] == 'smtp:pa#ss"word'
assert cfg["email"]["use_ssl"] is True and cfg["email"]["use_tls"] is False
assert cfg["web"]["admin_path"] == "/dj-safe"
PY
  then
    pass "generates safe production YAML with special characters"
  else
    fail "generates safe production YAML with special characters"
  fi

  cp "$generated" "${generated}.before"
  printf '%s\0' "/new-admin" | python3 "$helper" patch-admin-path "$generated"
  python3 "$helper" clear-bootstrap "$generated"
  if python3 - "$generated" "${generated}.before" <<'PY'
import sys, yaml
with open(sys.argv[1], encoding="utf-8") as f:
    after = yaml.safe_load(f)
with open(sys.argv[2], encoding="utf-8") as f:
    before = yaml.safe_load(f)
assert after["web"]["admin_path"] == "/new-admin"
assert after["bootstrap"] == {"default_admin_username": "", "default_admin_password": ""}
for path in (("app", "secret_key"), ("jwt", "secret"), ("user_jwt", "secret")):
    assert after[path[0]][path[1]] == before[path[0]][path[1]]
PY
  then
    pass "reconfiguration preserves encryption and JWT secrets"
  else
    fail "reconfiguration preserves encryption and JWT secrets"
  fi
else
  skip "YAML generation and secret-preservation tests" "PyYAML is not installed"
fi

if [[ $(id -u) -eq 0 ]] && command -v jq >/dev/null 2>&1; then
  mkdir -p "$RUN_TMP" "$INSTALL_DIR/db" "$INSTALL_DIR/uploads"
  state_init "shop.example.com" "admin@example.com" "8080" "6380" "/dj-safe" "dujiao-next-test"
  assert_equal "initial state starts at prerequisites" "prerequisites" "$(state_get phase)"
  state_set phase cert_issued
  assert_success "persisted state supports cert-stage resume" phase_at_least "$(state_get phase)" cert_issued
  state_set phase cert_issued
  assert_equal "repeating completed stage is idempotent" "cert_issued" "$(state_get phase)"
  state_json=$(cat "$STATE_FILE")
  assert_contains "state records managed files" "$state_json" '"managed_files"'
  assert_not_contains "state never stores bootstrap password" "$state_json" "Adm\"in\\Pass123"

  printf 'original admin config' > "$CONFIG_FILE"
  export APP_PORT=8080
  if (
    config_patch_admin_path() { printf 'changed admin config' > "$CONFIG_FILE"; }
    restart_application_service() { return 0; }
    verify_local_admin_path() { return 1; }
    restore_application_config() { cp "$1" "$CONFIG_FILE"; }
    wait_for_local_health() { return 0; }
    apply_admin_path_change "/new-admin"
  ); then
    fail "health failure triggers admin-path rollback"
  else
    pass "health failure triggers admin-path rollback"
  fi
  assert_equal "admin-path rollback restores original config" "original admin config" "$(cat "$CONFIG_FILE")"
  assert_equal "admin-path rollback preserves state" "/dj-safe" "$(state_get admin_path)"

  mkdir -p "$(dirname "$NGINX_ENABLED")" "$(root_path /etc/letsencrypt/live/dujiao-next-test)"
  printf cert > "$(root_path /etc/letsencrypt/live/dujiao-next-test/fullchain.pem)"
  printf key > "$(root_path /etc/letsencrypt/live/dujiao-next-test/privkey.pem)"
  reload_marker="${TEST_TMP}/nginx-reloaded"
  if (
    nginx() { return 1; }
    systemctl() { printf reloaded > "$reload_marker"; }
    write_final_nginx_site "shop.example.com" 8080 "dujiao-next-test"
  ); then
    fail "Nginx validation failure is propagated"
  else
    pass "Nginx validation failure is propagated"
  fi
  assert_failure "Nginx is not reloaded after failed validation" test -e "$reload_marker"

  printf 'app:\n  secret_key: restore-secret\n' > "$CONFIG_FILE"
  printf 'sqlite data' > "${INSTALL_DIR}/db/dujiao.db"
  printf 'upload data' > "${INSTALL_DIR}/uploads/example.txt"
  backup=$(create_uninstall_backup)
  assert_success "uninstall backup is readable" quiet tar -tzf "$backup"
  backup_listing=$(tar -tzf "$backup")
  assert_contains "uninstall backup contains SQLite" "$backup_listing" './db/dujiao.db'
  assert_contains "uninstall backup contains uploads" "$backup_listing" './uploads/example.txt'
  assert_contains "uninstall backup contains decryption config" "$backup_listing" './config.yml'

  if (
    cp() { return 1; }
    create_uninstall_backup
  ); then
    fail "backup copy failure is propagated"
  else
    pass "backup copy failure is propagated"
  fi
  assert_success "copy failure leaves SQLite untouched" test -f "${INSTALL_DIR}/db/dujiao.db"

  rm -f "$CONFIG_FILE"
  assert_failure "backup failure is reported before destructive uninstall" create_uninstall_backup
  assert_success "backup failure leaves SQLite untouched" test -f "${INSTALL_DIR}/db/dujiao.db"
else
  skip "state resume and uninstall-backup safety tests" "requires root and jq"
fi

if command -v flock >/dev/null 2>&1; then
  acquire_lock
  if DUJIAO_MANAGER_TESTING=1 DUJIAO_ROOT_PREFIX="$DUJIAO_ROOT_PREFIX" \
    bash -c 'source "$1"; RUN_TMP=$(mktemp -d); acquire_lock' _ "$MANAGER_SCRIPT" 9>&-; then
    fail "concurrent manager process is rejected"
  else
    pass "concurrent manager process is rejected"
  fi
else
  skip "concurrent manager process is rejected" "flock is not installed"
fi

printf '\n%d passed, %d failed, %d skipped\n' "$passed" "$failed" "$skipped"
((failed == 0))
