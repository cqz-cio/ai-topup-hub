#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly MANAGER_VERSION="1.0.0"
readonly GITHUB_REPOSITORY="dujiao-next/dujiao-next"
readonly GITHUB_API_URL="https://api.github.com/repos/${GITHUB_REPOSITORY}/releases/latest"
readonly MANAGER_SOURCE_URL="https://raw.githubusercontent.com/${GITHUB_REPOSITORY}/main/scripts/dujiao-next-manager.sh"
readonly SERVICE_USER="dujiao"
readonly SERVICE_GROUP="dujiao"
readonly APP_SERVICE="dujiao-next.service"
readonly REDIS_SERVICE="dujiao-next-redis.service"
readonly MIN_ARCHIVE_BYTES=1048576
readonly MAX_ARCHIVE_BYTES=536870912
readonly MIN_BINARY_BYTES=1048576
readonly MAX_BINARY_BYTES=314572800

ROOT_PREFIX="${DUJIAO_ROOT_PREFIX:-}"

root_path() {
  printf '%s%s' "$ROOT_PREFIX" "$1"
}

INSTALL_DIR="$(root_path /opt/dujiao-next)"
CONFIG_FILE="${INSTALL_DIR}/config.yml"
CONFIG_EXAMPLE="${INSTALL_DIR}/config.yml.example"
APP_BINARY="${INSTALL_DIR}/dujiao-next"
ETC_DIR="$(root_path /etc/dujiao-next)"
STATE_FILE="${ETC_DIR}/install-state.json"
REDIS_CONFIG="${ETC_DIR}/redis.conf"
REDIS_DATA_DIR="$(root_path /var/lib/dujiao-next/redis)"
BACKUP_DIR="$(root_path /var/backups/dujiao-next)"
APP_UNIT_FILE="$(root_path /etc/systemd/system/${APP_SERVICE})"
REDIS_UNIT_FILE="$(root_path /etc/systemd/system/${REDIS_SERVICE})"
NGINX_AVAILABLE="$(root_path /etc/nginx/sites-available/dujiao-next.conf)"
NGINX_ENABLED="$(root_path /etc/nginx/sites-enabled/dujiao-next.conf)"
NGINX_TRANSITION_AVAILABLE="$(root_path /etc/nginx/sites-available/dujiao-next-transition.conf)"
NGINX_TRANSITION_ENABLED="$(root_path /etc/nginx/sites-enabled/dujiao-next-transition.conf)"
ACME_ROOT="$(root_path /var/www/dujiao-next-acme)"
CERTBOT_DEPLOY_HOOK="$(root_path /etc/letsencrypt/renewal-hooks/deploy/dujiao-next-nginx)"
MANAGER_BIN="$(root_path /usr/local/sbin/dujiao-next-manager)"
LOCK_FILE="$(root_path /run/dujiao-next-manager.lock)"

RUN_TMP=""

log_info() {
  printf '[INFO] %s\n' "$*" >&2
}

log_warn() {
  printf '[WARN] %s\n' "$*" >&2
}

log_error() {
  printf '[ERROR] %s\n' "$*" >&2
}

die() {
  log_error "$*"
  exit 1
}

cleanup() {
  if [[ -n "$RUN_TMP" && -d "$RUN_TMP" ]]; then
    rm -rf -- "$RUN_TMP"
  fi
}

on_error() {
  local exit_code=$?
  local line_no=${1:-unknown}
  log_error "命令在第 ${line_no} 行失败，安装状态已保留，可修复问题后重新运行。"
  exit "$exit_code"
}

init_runtime() {
  [[ -n "$RUN_TMP" ]] || RUN_TMP="$(mktemp -d "${TMPDIR:-/tmp}/dujiao-next-manager.XXXXXX")"
  trap cleanup EXIT
  trap 'on_error $LINENO' ERR
}

require_root() {
  if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
    die "请使用 root 运行，例如：sudo dujiao-next-manager"
  fi
}

acquire_lock() {
  mkdir -p -- "$(dirname "$LOCK_FILE")"
  exec 9>"$LOCK_FILE"
  if ! flock -n 9; then
    die "另一个 Dujiao-Next 管理进程正在运行，请稍后重试。"
  fi
}

has_tty() {
  [[ -r /dev/tty && -w /dev/tty ]]
}

can_use_whiptail() {
  has_tty && command -v whiptail >/dev/null 2>&1 && [[ "${TERM:-dumb}" != "dumb" ]]
}

ui_message() {
  local title=$1
  local message
  message=$(printf '%b' "$2")
  if can_use_whiptail; then
    whiptail --title "$title" --msgbox "$message" 16 74
  elif has_tty; then
    printf '\n== %s ==\n%s\n' "$title" "$message" > /dev/tty
  else
    printf '\n== %s ==\n%s\n' "$title" "$message"
  fi
}

ui_yesno() {
  local title=$1
  local message
  message=$(printf '%b' "$2")
  if can_use_whiptail; then
    whiptail --title "$title" --yesno "$message" 16 74
    return $?
  fi

  local answer
  while true; do
    printf '\n== %s ==\n%s [y/N]: ' "$title" "$message" > /dev/tty
    IFS= read -r answer < /dev/tty || return 1
    case "$(lowercase "$answer")" in
      y|yes) return 0 ;;
      ''|n|no) return 1 ;;
    esac
  done
}

ui_input() {
  local title=$1
  local prompt=$2
  local default_value=${3:-}
  local value
  if can_use_whiptail; then
    value=$(whiptail --title "$title" --inputbox "$prompt" 12 74 "$default_value" 3>&1 1>&2 2>&3) || return 130
  else
    printf '\n== %s ==\n%s' "$title" "$prompt" > /dev/tty
    if [[ -n "$default_value" ]]; then
      printf ' [%s]' "$default_value" > /dev/tty
    fi
    printf ': ' > /dev/tty
    IFS= read -r value < /dev/tty || return 130
    [[ -n "$value" ]] || value=$default_value
  fi
  printf '%s' "$value"
}

ui_password() {
  local title=$1
  local prompt=$2
  local value
  if can_use_whiptail; then
    value=$(whiptail --title "$title" --passwordbox "$prompt" 12 74 3>&1 1>&2 2>&3) || return 130
  else
    printf '\n== %s ==\n%s: ' "$title" "$prompt" > /dev/tty
    IFS= read -rs value < /dev/tty || return 130
    printf '\n' > /dev/tty
  fi
  printf '%s' "$value"
}

ui_menu() {
  local title=$1
  local prompt=$2
  shift 2
  local choice
  if can_use_whiptail; then
    choice=$(whiptail --title "$title" --menu "$prompt" 22 78 14 "$@" 3>&1 1>&2 2>&3) || return 130
    printf '%s' "$choice"
    return 0
  fi

  local -a tags=()
  local -a labels=()
  while (($# >= 2)); do
    tags+=("$1")
    labels+=("$2")
    shift 2
  done
  printf '\n== %s ==\n%s\n' "$title" "$prompt" > /dev/tty
  local i
  for ((i = 0; i < ${#tags[@]}; i++)); do
    printf '  %d) %s\n' "$((i + 1))" "${labels[$i]}" > /dev/tty
  done
  while true; do
    printf '请选择 [1-%d]: ' "${#tags[@]}" > /dev/tty
    IFS= read -r choice < /dev/tty || return 130
    if [[ "$choice" =~ ^[0-9]+$ ]] && ((choice >= 1 && choice <= ${#tags[@]})); then
      printf '%s' "${tags[$((choice - 1))]}"
      return 0
    fi
  done
}

trim() {
  local value=$1
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

lowercase() {
  printf '%s' "$1" | tr '[:upper:]' '[:lower:]'
}

validate_domain() {
  local domain
  domain=$(lowercase "$1")
  [[ ${#domain} -le 253 ]] || return 1
  [[ "$domain" != *://* && "$domain" != */* && "$domain" != \** ]] || return 1
  [[ "$domain" == *.* ]] || return 1
  [[ "$domain" =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$ ]]
}

validate_email() {
  local email=$1
  [[ ${#email} -le 254 ]] || return 1
  [[ "$email" =~ ^[^[:space:]@]+@[^[:space:]@]+\.[^[:space:]@]+$ ]]
}

validate_smtp_host() {
  local host=$1
  [[ -n "$host" && ${#host} -le 253 ]] || return 1
  [[ "$host" != *..* ]] || return 1
  [[ "$host" =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?$ ]]
}

validate_port() {
  local port=$1
  [[ "$port" =~ ^[0-9]+$ ]] && ((port >= 1024 && port <= 65535))
}

validate_network_port() {
  local port=$1
  [[ "$port" =~ ^[0-9]+$ ]] && ((port >= 1 && port <= 65535))
}

validate_os_version() {
  local os_id=$1
  local version=$2
  case "$os_id" in
    ubuntu) dpkg --compare-versions "$version" ge "22.04" ;;
    debian) dpkg --compare-versions "$version" ge "12" ;;
    *) return 1 ;;
  esac
}

validate_admin_username() {
  local username=$1
  [[ "$username" =~ ^[A-Za-z0-9._@-]{3,64}$ ]]
}

validate_admin_password() {
  local password=$1
  (( ${#password} >= 8 )) || return 1
  [[ "$password" =~ [A-Z] ]] || return 1
  [[ "$password" =~ [a-z] ]] || return 1
  [[ "$password" =~ [0-9] ]] || return 1
  case "$(lowercase "$password")" in
    admin|admin123|password|password123|change-me|changeme) return 1 ;;
  esac
  return 0
}

validate_destroy_phrase() {
  [[ "$1" == "DELETE DUJIAO DATA" ]]
}

validate_admin_path() {
  local path=$1
  [[ "$path" == /* && "$path" != / && "$path" != */ ]] || return 1
  local without_slash=${path#/}
  [[ "$without_slash" != *//* ]] || return 1
  local segment
  local old_ifs=$IFS
  IFS=/
  # shellcheck disable=SC2206
  local segments=( $without_slash )
  IFS=$old_ifs
  [[ ${#segments[@]} -gt 0 ]] || return 1
  for segment in "${segments[@]}"; do
    [[ -n "$segment" && "$segment" != "." && "$segment" != ".." ]] || return 1
    [[ "$segment" =~ ^[A-Za-z0-9._~@-]+$ ]] || return 1
  done
  local reserved
  for reserved in /api /uploads /health; do
    [[ "$path" != "$reserved" ]] || return 1
    [[ "$path" != "$reserved"/* ]] || return 1
    [[ "$reserved" != "$path"/* ]] || return 1
  done
  return 0
}

platform_asset_arch() {
  local machine=$1
  case "$machine" in
    x86_64|amd64) printf 'x86_64' ;;
    aarch64|arm64) printf 'arm64' ;;
    *) return 1 ;;
  esac
}

validate_release_tag() {
  [[ "$1" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]
}

archive_name_for() {
  local tag=$1
  local arch=$2
  validate_release_tag "$tag" || return 1
  [[ "$arch" == "x86_64" || "$arch" == "arm64" ]] || return 1
  printf 'dujiao-next_%s_Linux_%s.tar.gz' "$tag" "$arch"
}

validate_download_url() {
  local url=$1
  [[ "$url" =~ ^https://github\.com/dujiao-next/dujiao-next/releases/download/[^/]+/[^/?#]+$ ]]
}

validate_effective_download_url() {
  local url=$1
  [[ "$url" =~ ^https://(github\.com|release-assets\.githubusercontent\.com|objects\.githubusercontent\.com|github-releases\.githubusercontent\.com)(/|$) ]]
}

download_release_asset() {
  local url=$1
  local destination=$2
  local max_time=$3
  local effective_url
  effective_url=$(curl --fail --silent --show-error --location \
    --proto '=https' --proto-redir '=https' --tlsv1.2 \
    --connect-timeout 10 --max-time "$max_time" \
    --write-out '%{url_effective}' "$url" -o "$destination") || return 1
  validate_effective_download_url "$effective_url"
}

resolve_release_metadata() {
  local metadata=$1
  local arch=$2
  [[ -s "$metadata" ]] || return 1
  jq -e 'type == "object" and (.assets | type == "array")' "$metadata" >/dev/null || return 1

  local tag asset_name checksum_name archive_count checksum_count archive_url checksum_url
  tag=$(jq -er '.tag_name | select(type == "string")' "$metadata") || return 1
  validate_release_tag "$tag" || return 1
  asset_name=$(archive_name_for "$tag" "$arch") || return 1
  checksum_name="dujiao-next_${tag#v}_checksums.txt"
  archive_count=$(jq -r --arg name "$asset_name" '[.assets[] | select(.name == $name)] | length' "$metadata")
  checksum_count=$(jq -r --arg name "$checksum_name" '[.assets[] | select(.name == $name)] | length' "$metadata")
  [[ "$archive_count" == "1" && "$checksum_count" == "1" ]] || return 1
  archive_url=$(jq -er --arg name "$asset_name" '.assets[] | select(.name == $name) | .browser_download_url | select(type == "string")' "$metadata") || return 1
  checksum_url=$(jq -er --arg name "$checksum_name" '.assets[] | select(.name == $name) | .browser_download_url | select(type == "string")' "$metadata") || return 1
  validate_download_url "$archive_url" || return 1
  validate_download_url "$checksum_url" || return 1
  printf '%s\n%s\n%s\n%s\n' "$tag" "$asset_name" "$archive_url" "$checksum_url"
}

verify_release_checksum() {
  local archive=$1
  local checksums=$2
  local asset_name=$3
  local expected actual
  expected=$(awk -v name="$asset_name" '$2 == name || $2 == "*" name {print $1; exit}' "$checksums")
  [[ "$expected" =~ ^[0-9a-fA-F]{64}$ ]] || return 1
  actual=$(sha256sum "$archive" | awk '{print $1}')
  [[ "$(lowercase "$actual")" == "$(lowercase "$expected")" ]]
}

validate_archive_listing() {
  local entry
  while IFS= read -r entry; do
    [[ -n "$entry" ]] || continue
    [[ "$entry" != /* ]] || return 1
    [[ "$entry" != *$'\n'* && "$entry" != *$'\r'* ]] || return 1
    case "/$entry/" in
      */../*|*/./*) return 1 ;;
    esac
  done
  return 0
}

validate_archive_types() {
  local line type
  while IFS= read -r line; do
    [[ -n "$line" ]] || continue
    type=${line:0:1}
    [[ "$type" == "-" || "$type" == "d" ]] || return 1
  done
  return 0
}

select_archive_member() {
  local listing=$1
  local basename=$2
  awk -F/ -v basename="$basename" '
    $NF == basename {found = $0; count++}
    END {
      if (count == 1) {
        print found
        exit 0
      }
      exit 1
    }
  ' "$listing"
}

certificate_name_for_domain() {
  local domain=$1
  local digest
  digest=$(printf '%s' "$domain" | sha256sum | awk '{print substr($1,1,16)}')
  printf 'dujiao-next-%s' "$digest"
}

phase_rank() {
  case "$1" in
    prerequisites) printf '1' ;;
    acme_ready) printf '2' ;;
    cert_issued) printf '3' ;;
    services_ready) printf '4' ;;
    installed) printf '5' ;;
    *) printf '0' ;;
  esac
}

phase_at_least() {
  local current=$1
  local expected=$2
  (( $(phase_rank "$current") >= $(phase_rank "$expected") ))
}

state_exists() {
  [[ -f "$STATE_FILE" ]]
}

state_get() {
  local field=$1
  jq -er --arg field "$field" '.[$field] // empty' "$STATE_FILE"
}

state_set() {
  local field=$1
  local value=$2
  local tmp="${RUN_TMP}/state.json"
  jq --arg field "$field" --arg value "$value" '.[$field] = $value' "$STATE_FILE" > "$tmp" || return 1
  install -o root -g root -m 0600 "$tmp" "$STATE_FILE"
}

state_init() {
  local domain=$1
  local acme_email=$2
  local app_port=$3
  local redis_port=$4
  local admin_path=$5
  local cert_name=$6
  local tmp="${RUN_TMP}/state-init.json"
  mkdir -p -- "$ETC_DIR"
  jq -n \
    --arg installer_version "$MANAGER_VERSION" \
    --arg phase "prerequisites" \
    --arg domain "$domain" \
    --arg acme_email "$acme_email" \
    --arg app_port "$app_port" \
    --arg redis_port "$redis_port" \
    --arg admin_path "$admin_path" \
    --arg cert_name "$cert_name" \
    --arg release_version "" \
    --arg user_created "false" \
    --arg redis_package_preexisting "false" \
    --arg install_dir "/opt/dujiao-next" \
    --arg app_service "$APP_SERVICE" \
    --arg redis_service "$REDIS_SERVICE" \
    --arg app_unit "$APP_UNIT_FILE" \
    --arg redis_unit "$REDIS_UNIT_FILE" \
    --arg nginx_site "$NGINX_AVAILABLE" \
    --arg nginx_enabled "$NGINX_ENABLED" \
    --arg nginx_transition "$NGINX_TRANSITION_AVAILABLE" \
    --arg nginx_transition_enabled "$NGINX_TRANSITION_ENABLED" \
    --arg redis_config "$REDIS_CONFIG" \
    --arg deploy_hook "$CERTBOT_DEPLOY_HOOK" \
    --arg manager_bin "$MANAGER_BIN" \
    --arg config_file "$CONFIG_FILE" \
    --arg app_binary "$APP_BINARY" \
    --arg redis_data "$(dirname "$REDIS_DATA_DIR")" \
    --arg acme_root "$ACME_ROOT" \
    '{installer_version:$installer_version,phase:$phase,domain:$domain,acme_email:$acme_email,app_port:$app_port,redis_port:$redis_port,admin_path:$admin_path,cert_name:$cert_name,release_version:$release_version,user_created:$user_created,redis_package_preexisting:$redis_package_preexisting,install_dir:$install_dir,app_service:$app_service,redis_service:$redis_service,managed_files:[$app_binary,$config_file,$app_unit,$redis_unit,$nginx_site,$nginx_enabled,$nginx_transition,$nginx_transition_enabled,$redis_config,$deploy_hook,$manager_bin],managed_directories:[$install_dir,$redis_data,$acme_root]}' > "$tmp"
  install -o root -g root -m 0600 "$tmp" "$STATE_FILE"
}

write_yaml_helper() {
  local helper="${RUN_TMP}/config_helper.py"
  if [[ -f "$helper" ]]; then
    printf '%s' "$helper"
    return 0
  fi
  cat > "$helper" <<'PY'
import os
import pathlib
import sys
import tempfile

import yaml


def read_fields():
    raw = sys.stdin.buffer.read()
    if not raw:
        return []
    fields = raw.split(b"\0")
    if fields and fields[-1] == b"":
        fields.pop()
    return [field.decode("utf-8") for field in fields]


def load(path):
    with open(path, "r", encoding="utf-8") as handle:
        data = yaml.safe_load(handle)
    if data is None:
        return {}
    if not isinstance(data, dict):
        raise ValueError("config root must be a mapping")
    return data


def write_atomic(path, data):
    target = pathlib.Path(path)
    target.parent.mkdir(parents=True, exist_ok=True)
    fd, temporary = tempfile.mkstemp(prefix=".config.yml.", dir=str(target.parent))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            yaml.safe_dump(data, handle, allow_unicode=True, sort_keys=False, default_flow_style=False)
            handle.flush()
            os.fsync(handle.fileno())
        os.chmod(temporary, 0o640)
        os.replace(temporary, target)
    except Exception:
        try:
            os.unlink(temporary)
        except FileNotFoundError:
            pass
        raise


action = sys.argv[1]

if action == "generate":
    template, destination = sys.argv[2], sys.argv[3]
    fields = read_fields()
    if len(fields) != 18:
        raise ValueError(f"generate expects 18 fields, got {len(fields)}")
    (
        app_port,
        redis_port,
        app_secret,
        jwt_secret,
        user_jwt_secret,
        redis_password,
        admin_username,
        admin_password,
        admin_path,
        smtp_enabled,
        smtp_host,
        smtp_port,
        smtp_username,
        smtp_password,
        smtp_from,
        smtp_from_name,
        smtp_use_tls,
        smtp_use_ssl,
    ) = fields
    data = load(template)
    data.setdefault("app", {})["secret_key"] = app_secret
    data.setdefault("server", {}).update({"host": "127.0.0.1", "port": str(app_port), "mode": "release"})
    data["database"] = {
        "driver": "sqlite",
        "dsn": "./db/dujiao.db",
        "pool": {
            "max_open_conns": 1,
            "max_idle_conns": 1,
            "conn_max_lifetime_seconds": 0,
            "conn_max_idle_time_seconds": 0,
        },
    }
    data.setdefault("jwt", {})["secret"] = jwt_secret
    data.setdefault("user_jwt", {})["secret"] = user_jwt_secret
    data["bootstrap"] = {
        "default_admin_username": admin_username,
        "default_admin_password": admin_password,
    }
    data["redis"] = {
        "enabled": True,
        "host": "127.0.0.1",
        "port": int(redis_port),
        "password": redis_password,
        "db": 0,
        "prefix": "dj",
    }
    data["queue"] = {
        "enabled": True,
        "host": "127.0.0.1",
        "port": int(redis_port),
        "password": redis_password,
        "db": 1,
        "concurrency": 10,
        "queues": {"default": 10, "critical": 5},
        "upstream_sync_interval": "5m",
    }
    email = data.setdefault("email", {})
    email.update(
        {
            "enabled": smtp_enabled == "true",
            "host": smtp_host,
            "port": int(smtp_port),
            "username": smtp_username,
            "password": smtp_password,
            "from": smtp_from,
            "from_name": smtp_from_name,
            "use_tls": smtp_use_tls == "true",
            "use_ssl": smtp_use_ssl == "true",
        }
    )
    email.setdefault(
        "verify_code",
        {"expire_minutes": 10, "send_interval_seconds": 60, "max_attempts": 5, "length": 6},
    )
    data["web"] = {"admin_path": admin_path}
    write_atomic(destination, data)
elif action == "patch-admin-path":
    path = sys.argv[2]
    fields = read_fields()
    if len(fields) != 1:
        raise ValueError("patch-admin-path expects one field")
    data = load(path)
    data.setdefault("web", {})["admin_path"] = fields[0]
    write_atomic(path, data)
elif action == "clear-bootstrap":
    path = sys.argv[2]
    data = load(path)
    data["bootstrap"] = {"default_admin_username": "", "default_admin_password": ""}
    write_atomic(path, data)
elif action == "disable-email":
    path = sys.argv[2]
    data = load(path)
    data.setdefault("email", {})["enabled"] = False
    write_atomic(path, data)
elif action == "read":
    path, dotted = sys.argv[2], sys.argv[3]
    value = load(path)
    for component in dotted.split("."):
        if not isinstance(value, dict) or component not in value:
            sys.exit(3)
        value = value[component]
    if isinstance(value, bool):
        print("true" if value else "false")
    elif value is None:
        print("")
    elif isinstance(value, (str, int, float)):
        print(value)
    else:
        raise ValueError("requested value is not scalar")
else:
    raise ValueError(f"unknown action: {action}")
PY
  chmod 0700 "$helper"
  printf '%s' "$helper"
}

generate_config() {
  local template=$1
  local destination=$2
  shift 2
  local helper
  helper=$(write_yaml_helper)
  printf '%s\0' "$@" | python3 "$helper" generate "$template" "$destination" || return 1
  chown "$SERVICE_USER:$SERVICE_GROUP" "$destination" || return 1
  chmod 0640 "$destination"
}

config_patch_admin_path() {
  local admin_path=$1
  local helper
  helper=$(write_yaml_helper)
  printf '%s\0' "$admin_path" | python3 "$helper" patch-admin-path "$CONFIG_FILE" || return 1
  chown "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_FILE" || return 1
  chmod 0640 "$CONFIG_FILE"
}

config_clear_bootstrap() {
  local helper
  helper=$(write_yaml_helper)
  python3 "$helper" clear-bootstrap "$CONFIG_FILE" || return 1
  chown "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_FILE" || return 1
  chmod 0640 "$CONFIG_FILE"
}

config_disable_email() {
  local helper
  helper=$(write_yaml_helper)
  python3 "$helper" disable-email "$CONFIG_FILE" || return 1
  chown "$SERVICE_USER:$SERVICE_GROUP" "$CONFIG_FILE" || return 1
  chmod 0640 "$CONFIG_FILE"
}

config_read() {
  local key=$1
  local helper
  helper=$(write_yaml_helper)
  python3 "$helper" read "$CONFIG_FILE" "$key"
}

atomic_install_text() {
  local destination=$1
  local mode=$2
  local owner=$3
  local group=$4
  local temporary
  temporary="${RUN_TMP}/$(basename "$destination").tmp"
  cat > "$temporary" || return 1
  mkdir -p -- "$(dirname "$destination")" || return 1
  install -o "$owner" -g "$group" -m "$mode" "$temporary" "$destination"
}

assert_safe_managed_paths() {
  [[ "$INSTALL_DIR" == "$(root_path /opt/dujiao-next)" ]] || die "安装目录安全校验失败"
  [[ "$ETC_DIR" == "$(root_path /etc/dujiao-next)" ]] || die "配置目录安全校验失败"
  [[ "$REDIS_DATA_DIR" == "$(root_path /var/lib/dujiao-next/redis)" ]] || die "Redis 目录安全校验失败"
  [[ -n "$INSTALL_DIR" && "$INSTALL_DIR" != "/" && "$INSTALL_DIR" != "$ROOT_PREFIX" ]] || die "拒绝使用危险安装目录"
}

check_os_support() {
  local os_release
  os_release="$(root_path /etc/os-release)"
  [[ -r "$os_release" ]] || die "无法读取 /etc/os-release"
  local ID=""
  local VERSION_ID=""
  # shellcheck disable=SC1090
  source "$os_release"
  validate_os_version "$ID" "$VERSION_ID" || die "当前系统 ${ID:-unknown} ${VERSION_ID:-unknown} 不受支持；首版仅支持 Ubuntu 22.04+ 与 Debian 12+"
  platform_asset_arch "$(uname -m)" >/dev/null || die "仅支持 amd64/x86_64 与 arm64/aarch64"
  [[ "$(ps -p 1 -o comm= | tr -d '[:space:]')" == "systemd" ]] || die "当前系统不是由 systemd 启动，无法使用官方安装器"
}

check_resources() {
  local free_mb
  free_mb=$(df -Pm /opt 2>/dev/null | awk 'NR==2 {print $4}')
  if [[ -n "$free_mb" ]] && ((free_mb < 2048)); then
    die "/opt 可用磁盘不足 2GB"
  fi
  local mem_kb
  mem_kb=$(awk '/MemTotal/ {print $2}' /proc/meminfo 2>/dev/null || printf '0')
  if ((mem_kb > 0 && mem_kb < 524288)); then
    die "可用内存低于最低要求 512MB"
  fi
  if ((mem_kb > 0 && mem_kb < 1048576)); then
    log_warn "内存低于推荐值 1GB，低并发场景可以运行，但请关注 OOM。"
  fi
}

port_in_use() {
  local port=$1
  ss -ltnH 2>/dev/null | awk '{print $4}' | grep -Eq ":${port}$"
}

port_bound_only_to_loopback() {
  local port=$1
  local addresses
  addresses=$(ss -ltnH 2>/dev/null | awk -v suffix=":${port}" '$4 ~ suffix "$" {print $4}')
  [[ -n "$addresses" ]] || return 1
  local address
  while IFS= read -r address; do
    if [[ "$address" != "127.0.0.1:${port}" && "$address" != "[::1]:${port}" && "$address" != "::1:${port}" ]]; then
      return 1
    fi
  done <<< "$addresses"
}

nginx_has_domain_conflict() {
  local domain=$1
  [[ -x "$(command -v nginx 2>/dev/null || true)" ]] || return 1
  nginx -T 2>/dev/null | awk -v domain="$domain" '
    $1 == "server_name" {
      for (i = 2; i <= NF; i++) {
        value = $i
        gsub(/;/, "", value)
        if (value == domain) found = 1
      }
    }
    END { exit(found ? 0 : 1) }
  '
}

check_web_listener_conflicts() {
  local port listeners
  for port in 80 443; do
    port_in_use "$port" || continue
    listeners=$(ss -ltnp 2>/dev/null | awk -v suffix=":${port}" '$4 ~ suffix "$"')
    if [[ "$listeners" != *nginx* ]]; then
      die "TCP ${port} 已被非 Nginx 进程占用；官方安装器不会接管或停止该服务。"
    fi
  done
}

check_unmanaged_conflicts() {
  if state_exists; then
    return 0
  fi
  local path
  for path in \
    "$INSTALL_DIR" \
    "$ETC_DIR" \
    "$(dirname "$REDIS_DATA_DIR")" \
    "$APP_UNIT_FILE" \
    "$REDIS_UNIT_FILE" \
    "$NGINX_AVAILABLE" \
    "$NGINX_ENABLED" \
    "$NGINX_TRANSITION_AVAILABLE" \
    "$NGINX_TRANSITION_ENABLED" \
    "$ACME_ROOT" \
    "$CERTBOT_DEPLOY_HOOK" \
    "$MANAGER_BIN"; do
    [[ ! -e "$path" ]] || die "检测到非本安装器管理的路径：${path}；不会自动接管，请先按迁移文档处理。"
  done
  if getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
    die "系统用户 $SERVICE_USER 已存在，但没有安装器状态；为避免接管旧部署，已停止。"
  fi
}

install_dependencies() {
  local redis_preexisting=false
  if dpkg-query -W -f='${Status}' redis-server 2>/dev/null | grep -q 'install ok installed'; then
    redis_preexisting=true
  fi
  log_info "安装运行依赖（Nginx、Certbot、Redis、whiptail、PyYAML 等）..."
  apt-get update
  DEBIAN_FRONTEND=noninteractive apt-get install -y \
    ca-certificates certbot curl iproute2 jq nginx openssl python3 python3-yaml redis-server tar util-linux whiptail
  if state_exists; then
    if [[ -z "$(state_get redis_package_preexisting 2>/dev/null || true)" ]]; then
      state_set redis_package_preexisting "$redis_preexisting"
    fi
  else
    REDIS_PACKAGE_PREEXISTING=$redis_preexisting
  fi
  if [[ "$redis_preexisting" == "false" ]]; then
    systemctl disable --now redis-server.service >/dev/null 2>&1 || true
  fi
}

install_manager_copy() {
  local source=${BASH_SOURCE[0]:-}
  mkdir -p -- "$(dirname "$MANAGER_BIN")"
  if [[ -n "$source" && -r "$source" && "$source" != "$MANAGER_BIN" ]]; then
    install -o root -g root -m 0755 "$source" "$MANAGER_BIN"
  elif [[ "$source" == "$MANAGER_BIN" ]]; then
    chmod 0755 "$MANAGER_BIN"
  else
    curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
      --connect-timeout 10 --max-time 60 "$MANAGER_SOURCE_URL" -o "${RUN_TMP}/manager"
    install -o root -g root -m 0755 "${RUN_TMP}/manager" "$MANAGER_BIN"
  fi
}

create_service_user() {
  if ! getent passwd "$SERVICE_USER" >/dev/null 2>&1; then
    useradd --system --user-group --home-dir /opt/dujiao-next --shell /usr/sbin/nologin "$SERVICE_USER"
    state_set user_created "true"
  fi
  mkdir -p -- "$INSTALL_DIR" "${INSTALL_DIR}/db" "${INSTALL_DIR}/uploads" "${INSTALL_DIR}/logs" "$REDIS_DATA_DIR" "$ETC_DIR"
  chown -R "$SERVICE_USER:$SERVICE_GROUP" "$INSTALL_DIR" "$(dirname "$REDIS_DATA_DIR")"
  chown root:"$SERVICE_GROUP" "$ETC_DIR"
  chmod 0750 "$INSTALL_DIR" "$REDIS_DATA_DIR" "$ETC_DIR"
}

render_redis_config() {
  local redis_port=$1
  local redis_password=$2
  printf 'bind 127.0.0.1 -::1\n'
  printf 'protected-mode yes\n'
  printf 'port %s\n' "$redis_port"
  printf 'daemonize no\n'
  printf 'supervised no\n'
  printf 'dir %s\n' "$REDIS_DATA_DIR"
  printf 'appendonly yes\n'
  printf 'save 900 1\n'
  printf 'save 300 10\n'
  printf 'save 60 10000\n'
  printf 'requirepass %s\n' "$redis_password"
}

write_redis_config() {
  render_redis_config "$1" "$2" | atomic_install_text "$REDIS_CONFIG" 0640 root "$SERVICE_GROUP"
}

render_redis_unit() {
  cat <<EOF
[Unit]
Description=Dujiao-Next dedicated Redis
After=network.target
Before=${APP_SERVICE}

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
ExecStart=/usr/bin/redis-server ${REDIS_CONFIG}
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${REDIS_DATA_DIR}
UMask=0077

[Install]
WantedBy=multi-user.target
EOF
}

render_app_unit() {
  cat <<EOF
[Unit]
Description=Dujiao-Next
Wants=network-online.target
After=network-online.target ${REDIS_SERVICE}
Requires=${REDIS_SERVICE}

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${APP_BINARY}
Restart=always
RestartSec=3
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=${INSTALL_DIR}
UMask=0027

[Install]
WantedBy=multi-user.target
EOF
}

write_systemd_units() {
  render_redis_unit | atomic_install_text "$REDIS_UNIT_FILE" 0644 root root
  render_app_unit | atomic_install_text "$APP_UNIT_FILE" 0644 root root
  systemctl daemon-reload
}

fetch_release() {
  local arch
  arch=$(platform_asset_arch "$(uname -m)")
  local metadata="${RUN_TMP}/release.json"
  curl --fail --silent --show-error --location --proto '=https' --proto-redir '=https' --tlsv1.2 \
    --connect-timeout 10 --max-time 60 \
    -H 'Accept: application/vnd.github+json' \
    -H 'X-GitHub-Api-Version: 2022-11-28' \
    "$GITHUB_API_URL" -o "$metadata"

  local resolution tag asset_name archive_url checksum_url
  resolution=$(resolve_release_metadata "$metadata" "$arch") || die "Release 元数据异常，或缺少唯一的当前架构附件/版本校验文件"
  tag=$(sed -n '1p' <<< "$resolution")
  asset_name=$(sed -n '2p' <<< "$resolution")
  archive_url=$(sed -n '3p' <<< "$resolution")
  checksum_url=$(sed -n '4p' <<< "$resolution")

  local archive="${RUN_TMP}/${asset_name}"
  local checksums="${RUN_TMP}/checksums.txt"
  download_release_asset "$archive_url" "$archive" 600 || die "Release 附件下载失败或最终下载域名不受信任"
  download_release_asset "$checksum_url" "$checksums" 60 || die "Release 校验文件下载失败或最终下载域名不受信任"

  local size
  size=$(stat -c '%s' "$archive")
  ((size >= MIN_ARCHIVE_BYTES && size <= MAX_ARCHIVE_BYTES)) || die "Release 归档大小异常：${size} bytes"
  verify_release_checksum "$archive" "$checksums" "$asset_name" || die "Release SHA-256 校验失败或 checksums.txt 缺少有效条目"

  tar -tzf "$archive" > "${RUN_TMP}/archive.list"
  validate_archive_listing < "${RUN_TMP}/archive.list" || die "Release 归档包含不安全路径"
  tar -tvzf "$archive" > "${RUN_TMP}/archive.verbose"
  validate_archive_types < "${RUN_TMP}/archive.verbose" || die "Release 归档包含符号链接、硬链接或设备文件"
  local binary_member config_member
  binary_member=$(select_archive_member "${RUN_TMP}/archive.list" dujiao-next) || die "Release 归档缺少唯一的 dujiao-next 二进制"
  config_member=$(select_archive_member "${RUN_TMP}/archive.list" config.yml.example) || die "Release 归档缺少唯一的 config.yml.example"
  local extract_dir="${RUN_TMP}/extract"
  mkdir -p -- "$extract_dir"
  tar -xzf "$archive" --no-same-owner -C "$extract_dir" -- "$binary_member" "$config_member"
  local binary_path="${extract_dir}/${binary_member}"
  local config_path="${extract_dir}/${config_member}"
  [[ -f "$binary_path" && ! -L "$binary_path" && -f "$config_path" && ! -L "$config_path" ]] || die "Release 关键文件类型异常"
  local binary_size
  binary_size=$(stat -c '%s' "$binary_path")
  ((binary_size >= MIN_BINARY_BYTES && binary_size <= MAX_BINARY_BYTES)) || die "Release 二进制大小异常"

  install -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0755 "$binary_path" "$APP_BINARY"
  install -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0640 "$config_path" "$CONFIG_EXAMPLE"
  state_set release_version "$tag"
}

render_acme_site() {
  local domain=$1
  cat <<EOF
# Managed by dujiao-next-manager. Do not edit by hand.
server {
    listen 80;
    listen [::]:80;
    server_name ${domain};

    location ^~ /.well-known/acme-challenge/ {
        root ${ACME_ROOT};
        default_type text/plain;
        try_files \$uri =404;
    }

    location / {
        return 404;
    }
}
EOF
}

write_acme_site() {
  local domain=$1
  render_acme_site "$domain" | atomic_install_text "$NGINX_AVAILABLE" 0644 root root
  ln -sfn "$NGINX_AVAILABLE" "$NGINX_ENABLED"
  nginx -t
  systemctl enable --now nginx
  systemctl reload nginx
}

write_transition_acme_site() {
  local domain=$1
  cat <<EOF | atomic_install_text "$NGINX_TRANSITION_AVAILABLE" 0644 root root
# Temporary ACME site managed by dujiao-next-manager.
server {
    listen 80;
    listen [::]:80;
    server_name ${domain};

    location ^~ /.well-known/acme-challenge/ {
        root ${ACME_ROOT};
        default_type text/plain;
        try_files \$uri =404;
    }

    location / {
        return 404;
    }
}
EOF
  ln -sfn "$NGINX_TRANSITION_AVAILABLE" "$NGINX_TRANSITION_ENABLED"
  nginx -t
  systemctl reload nginx
}

remove_transition_site() {
  rm -f -- "$NGINX_TRANSITION_ENABLED" "$NGINX_TRANSITION_AVAILABLE"
  if command -v nginx >/dev/null 2>&1 && nginx -t >/dev/null 2>&1; then
    systemctl reload nginx || true
  fi
}

render_final_nginx_site() {
  local domain=$1
  local app_port=$2
  local cert_name=$3
  local live_dir
  live_dir="$(root_path "/etc/letsencrypt/live/${cert_name}")"
  cat <<EOF
# Managed by dujiao-next-manager. Do not edit by hand.
server {
    listen 80;
    listen [::]:80;
    server_name ${domain};

    location ^~ /.well-known/acme-challenge/ {
        root ${ACME_ROOT};
        default_type text/plain;
        try_files \$uri =404;
    }

    location / {
        return 301 https://\$host\$request_uri;
    }
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${domain};

    ssl_certificate ${live_dir}/fullchain.pem;
    ssl_certificate_key ${live_dir}/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:${app_port};
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_connect_timeout 10s;
        proxy_read_timeout 120s;
        proxy_send_timeout 120s;
    }
}
EOF
}

write_final_nginx_site() {
  local domain=$1
  local app_port=$2
  local cert_name=$3
  local live_dir
  live_dir="$(root_path "/etc/letsencrypt/live/${cert_name}")"
  [[ -s "${live_dir}/fullchain.pem" && -s "${live_dir}/privkey.pem" ]] || die "证书文件不存在：$live_dir"
  render_final_nginx_site "$domain" "$app_port" "$cert_name" | atomic_install_text "$NGINX_AVAILABLE" 0644 root root || return 1
  ln -sfn "$NGINX_AVAILABLE" "$NGINX_ENABLED" || return 1
  nginx -t || return 1
  systemctl reload nginx
}

write_certbot_deploy_hook() {
  cat <<'EOF' | atomic_install_text "$CERTBOT_DEPLOY_HOOK" 0755 root root
#!/usr/bin/env bash
set -euo pipefail
nginx -t
systemctl reload nginx
EOF
}

verify_acme_reachability() {
  local domain=$1
  local token
  token=$(openssl rand -hex 16)
  local challenge_dir="${ACME_ROOT}/.well-known/acme-challenge"
  mkdir -p -- "$challenge_dir"
  printf '%s' "$token" > "${challenge_dir}/${token}"
  chmod 0644 "${challenge_dir}/${token}"
  local response=""
  response=$(curl --fail --silent --show-error --connect-timeout 8 --max-time 20 \
    "http://${domain}/.well-known/acme-challenge/${token}") || true
  rm -f -- "${challenge_dir}/${token}"
  [[ "$response" == "$token" ]]
}

issue_certificate() {
  local domain=$1
  local email=$2
  local cert_name=$3
  local live_dir
  live_dir="$(root_path "/etc/letsencrypt/live/${cert_name}")"
  if [[ -s "${live_dir}/fullchain.pem" && -s "${live_dir}/privkey.pem" ]]; then
    return 0
  fi
  local -a certbot_args=(
    certonly --webroot -w "$ACME_ROOT"
    -d "$domain" --cert-name "$cert_name"
    --email "$email" --agree-tos --no-eff-email --non-interactive
  )
  if [[ "${DUJIAO_ACME_STAGING:-0}" == "1" ]]; then
    certbot_args+=(--staging)
  fi
  certbot "${certbot_args[@]}"
  [[ -s "${live_dir}/fullchain.pem" && -s "${live_dir}/privkey.pem" ]]
}

maybe_open_ufw() {
  command -v ufw >/dev/null 2>&1 || return 0
  ufw status 2>/dev/null | grep -q '^Status: active' || return 0
  if ui_yesno "防火墙" "检测到 UFW 已启用。SSL 申请和站点访问需要开放 80/443，是否执行 ufw allow 'Nginx Full'？"; then
    ufw allow 'Nginx Full'
  else
    log_warn "未修改 UFW。请自行确保 TCP 80/443 已开放，否则证书申请会失败。"
  fi
}

wait_for_local_health() {
  local app_port=$1
  local attempt
  for ((attempt = 1; attempt <= 40; attempt++)); do
    if curl --fail --silent --show-error --max-time 3 "http://127.0.0.1:${app_port}/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  journalctl -u "$APP_SERVICE" -n 80 --no-pager >&2 || true
  return 1
}

extract_first_asset_url() {
  local html_file=$1
  local base_url=$2
  python3 - "$html_file" "$base_url" <<'PY'
import sys
import urllib.parse
from html.parser import HTMLParser

class AssetFinder(HTMLParser):
    def __init__(self):
        super().__init__()
        self.asset = ""

    def handle_starttag(self, _tag, attrs):
        for name, value in attrs:
            if name in ("src", "href") and value and (
                "/assets/" in value or value.startswith("./assets/") or value.startswith("assets/")
            ):
                self.asset = urllib.parse.urljoin(sys.argv[2], value)
                return

finder = AssetFinder()
with open(sys.argv[1], encoding="utf-8") as handle:
    finder.feed(handle.read())
print(finder.asset)
PY
}

verify_public_site() {
  local domain=$1
  local admin_path=$2
  curl --fail --silent --show-error --connect-timeout 10 --max-time 30 "https://${domain}/health" >/dev/null || return 1
  curl --fail --silent --show-error --connect-timeout 10 --max-time 30 "https://${domain}/" >/dev/null || return 1
  local admin_html="${RUN_TMP}/public-admin.html"
  curl --fail --silent --show-error --connect-timeout 10 --max-time 30 "https://${domain}${admin_path}/" -o "$admin_html" || return 1
  local asset_url
  asset_url=$(extract_first_asset_url "$admin_html" "https://${domain}${admin_path}/") || return 1
  [[ -n "$asset_url" ]] || return 1
  curl --fail --silent --show-error --connect-timeout 10 --max-time 30 "$asset_url" >/dev/null || return 1
  local status
  status=$(curl --silent --output /dev/null --connect-timeout 10 --max-time 30 --write-out '%{http_code}' "http://${domain}/") || return 1
  [[ "$status" == "301" || "$status" == "302" || "$status" == "307" || "$status" == "308" ]]
}

verify_admin_exists() {
  local username=$1
  local output="${RUN_TMP}/admins.txt"
  if ! runuser -u "$SERVICE_USER" -- bash -c 'cd "$1" && ./dujiao-next admin list-admins' _ "$INSTALL_DIR" > "$output" 2>&1; then
    cat "$output" >&2
    return 1
  fi
  awk -v username="$username" 'NR > 1 && $2 == username {found=1} END {exit(found ? 0 : 1)}' "$output"
}

smtp_test() {
  local app_port=$1
  local username=$2
  local password=$3
  local recipient=$4
  local login_request="${RUN_TMP}/admin-login.json"
  local test_request="${RUN_TMP}/smtp-test.json"
  local login_response="${RUN_TMP}/admin-login-response.json"
  local test_response="${RUN_TMP}/smtp-test-response.json"
  printf '%s\0%s\0' "$username" "$password" | python3 -c '
import json, sys
parts=sys.stdin.buffer.read().split(b"\0")
print(json.dumps({"username":parts[0].decode(),"password":parts[1].decode()}))
' > "$login_request"
  chmod 0600 "$login_request"
  curl --fail --silent --show-error --max-time 20 \
    -H 'Content-Type: application/json' --data-binary "@${login_request}" \
    "http://127.0.0.1:${app_port}/api/v1/admin/login" -o "$login_response" || return 1
  local token
  token=$(jq -er '.data.token // .token // empty' "$login_response") || return 1
  printf '%s\0' "$recipient" | python3 -c '
import json, sys
recipient=sys.stdin.buffer.read().split(b"\0")[0].decode()
print(json.dumps({"to_email":recipient,"subject":"Dujiao-Next SMTP 安装测试","body":"SMTP 配置测试成功。"}, ensure_ascii=False))
' > "$test_request"
  chmod 0600 "$test_request"
  curl --fail --silent --show-error --max-time 45 \
    -H 'Content-Type: application/json' -H "Authorization: Bearer ${token}" \
    --data-binary "@${test_request}" \
    "http://127.0.0.1:${app_port}/api/v1/admin/settings/smtp/test" -o "$test_response" || return 1
  jq -e '(.status_code == 0) and (.data.sent == true)' "$test_response" >/dev/null
}

ensure_loopback_bindings() {
  local app_port=$1
  local redis_port=$2
  port_bound_only_to_loopback "$app_port" || die "应用端口 ${app_port} 未按预期仅监听回环地址"
  port_bound_only_to_loopback "$redis_port" || die "Redis 端口 ${redis_port} 未按预期仅监听回环地址"
}

prompt_validated() {
  local validator=$1
  local title=$2
  local prompt=$3
  local default_value=${4:-}
  local value
  while true; do
    value=$(ui_input "$title" "$prompt" "$default_value") || exit 130
    value=$(trim "$value")
    if "$validator" "$value"; then
      printf '%s' "$value"
      return 0
    fi
    ui_message "输入无效" "请检查后重新输入。"
  done
}

prompt_password_pair() {
  local first second
  while true; do
    first=$(ui_password "管理员密码" "至少 8 位，必须同时包含大写字母、小写字母和数字") || exit 130
    if ! validate_admin_password "$first"; then
      ui_message "密码不安全" "密码至少 8 位，且必须同时包含大写字母、小写字母和数字；不能使用常见默认密码。"
      continue
    fi
    second=$(ui_password "确认管理员密码" "请再次输入管理员密码") || exit 130
    if [[ "$first" == "$second" ]]; then
      printf '%s' "$first"
      return 0
    fi
    ui_message "两次输入不一致" "请重新输入。"
  done
}

pick_free_port() {
  local title=$1
  local preferred=$2
  local value=$preferred
  while port_in_use "$value"; do
    value=$(prompt_validated validate_port "$title" "端口 ${value} 已被占用，请输入一个未占用的回环端口" "$((value + 1))")
  done
  printf '%s' "$value"
}

collect_smtp_config() {
  SMTP_ENABLED=false
  SMTP_HOST=""
  SMTP_PORT="587"
  SMTP_USERNAME=""
  SMTP_PASSWORD=""
  SMTP_FROM=""
  SMTP_FROM_NAME="Dujiao-Next"
  SMTP_USE_TLS=true
  SMTP_USE_SSL=false
  SMTP_TEST_RECIPIENT=""

  if ! ui_yesno "SMTP（可选）" "是否现在配置 SMTP？跳过时商城可以运行，但邮箱验证码注册要在后台完成 SMTP 配置后才能使用。"; then
    return 0
  fi
  SMTP_ENABLED=true
  SMTP_HOST=$(prompt_validated validate_smtp_host "SMTP 主机" "例如 smtp.example.com" "")
  SMTP_PORT=$(prompt_validated validate_network_port "SMTP 端口" "常用端口：STARTTLS=587，SSL=465" "587")
  SMTP_USERNAME=$(ui_input "SMTP 用户名" "允许留空（取决于服务商）" "") || exit 130
  SMTP_PASSWORD=$(ui_password "SMTP 密码" "允许留空（取决于服务商）；推荐填写授权码而非账户登录密码") || exit 130
  SMTP_FROM=$(prompt_validated validate_email "发件地址" "请输入发件人邮箱" "")
  SMTP_FROM_NAME=$(ui_input "发件人名称" "邮件中显示的名称" "Dujiao-Next") || exit 130
  local encryption
  encryption=$(ui_menu "SMTP 加密" "请选择连接方式" \
    starttls "STARTTLS（通常为 587）" \
    ssl "隐式 SSL/TLS（通常为 465）" \
    none "不加密（仅可信内网）") || exit 130
  case "$encryption" in
    starttls) SMTP_USE_TLS=true; SMTP_USE_SSL=false ;;
    ssl) SMTP_USE_TLS=false; SMTP_USE_SSL=true ;;
    none) SMTP_USE_TLS=false; SMTP_USE_SSL=false ;;
  esac
  SMTP_TEST_RECIPIENT=$(prompt_validated validate_email "SMTP 测试收件人" "安装后可发送一封测试邮件" "$SMTP_FROM")
}

collect_install_input() {
  DOMAIN=$(prompt_validated validate_domain "商城域名" "请输入已解析到本服务器的单个域名（不含 https:// 和路径）" "")
  DOMAIN=$(lowercase "$DOMAIN")
  ACME_EMAIL=$(prompt_validated validate_email "证书邮箱" "用于 Lets Encrypt 到期通知" "")
  ADMIN_USERNAME=$(prompt_validated validate_admin_username "管理员账号" "3-64 位，仅允许字母、数字、点、下划线、@ 和连字符" "admin")
  ADMIN_PASSWORD=$(prompt_password_pair)
  local generated_path
  generated_path="/dj-$(openssl rand -hex 6)"
  ADMIN_PATH=$(prompt_validated validate_admin_path "后台入口" "使用随机路径可减少自动化扫描；允许多级路径" "$generated_path")
  APP_PORT=$(pick_free_port "应用端口" 8080)
  REDIS_PORT=$(pick_free_port "Redis 端口" 6380)
  [[ "$APP_PORT" != "$REDIS_PORT" ]] || REDIS_PORT=$(pick_free_port "Redis 端口" 6381)
  collect_smtp_config
  CERT_NAME=$(certificate_name_for_domain "$DOMAIN")
}

show_install_summary() {
  local smtp_text="未配置（安装后在后台设置）"
  [[ "$SMTP_ENABLED" == "true" ]] && smtp_text="已填写，将在启动后测试"
  local summary
  summary=$(cat <<EOF
系统：$(. /etc/os-release && printf '%s %s' "$PRETTY_NAME" "$(uname -m)")
域名：https://${DOMAIN}
后台：https://${DOMAIN}${ADMIN_PATH}/
应用：127.0.0.1:${APP_PORT}
Redis：127.0.0.1:${REDIS_PORT}（独立实例）
数据库：SQLite
SMTP：${smtp_text}

脚本将安装系统依赖、创建 systemd 服务、申请 Lets Encrypt 证书并写入 Nginx 配置。
EOF
)
  ui_yesno "确认安装" "$summary"
}

check_domain_preconditions() {
  local domain=$1
  getent ahosts "$domain" >/dev/null 2>&1 || die "域名 $domain 当前无法解析；请先完成 DNS 解析。"
  if nginx_has_domain_conflict "$domain" && [[ ! -e "$NGINX_AVAILABLE" ]]; then
    die "Nginx 已有其他站点声明域名 ${domain}；为避免覆盖，安装已停止。"
  fi
  local cert_name live_dir
  cert_name=$(certificate_name_for_domain "$domain")
  live_dir=$(root_path "/etc/letsencrypt/live/${cert_name}")
  if ! state_exists && [[ -e "$live_dir" ]]; then
    die "检测到非本安装器管理的同名证书目录：${live_dir}；拒绝接管。"
  fi
}

prepare_application_files() {
  local app_port=$1
  local redis_port=$2
  local admin_path=$3
  local admin_username=${4:-}
  local admin_password=${5:-}

  create_service_user
  if [[ ! -x "$APP_BINARY" || ! -s "$CONFIG_EXAMPLE" ]]; then
    fetch_release
  fi

  local redis_password=""
  if [[ -s "$REDIS_CONFIG" ]]; then
    redis_password=$(awk '$1 == "requirepass" {print $2; exit}' "$REDIS_CONFIG")
  fi
  [[ -n "$redis_password" ]] || redis_password=$(openssl rand -hex 32)
  write_redis_config "$redis_port" "$redis_password"

  if [[ ! -s "$CONFIG_FILE" ]]; then
    [[ -n "$admin_username" && -n "$admin_password" ]] || die "缺少首次管理员凭据，无法生成配置"
    local app_secret jwt_secret user_jwt_secret
    app_secret=$(openssl rand -hex 32)
    jwt_secret=$(openssl rand -hex 32)
    user_jwt_secret=$(openssl rand -hex 32)
    [[ "$app_secret" != "$jwt_secret" && "$app_secret" != "$user_jwt_secret" && "$jwt_secret" != "$user_jwt_secret" ]] || die "随机密钥生成异常"
    generate_config "$CONFIG_EXAMPLE" "$CONFIG_FILE" \
      "$app_port" "$redis_port" "$app_secret" "$jwt_secret" "$user_jwt_secret" "$redis_password" \
      "$admin_username" "$admin_password" "$admin_path" \
      "$SMTP_ENABLED" "$SMTP_HOST" "$SMTP_PORT" "$SMTP_USERNAME" "$SMTP_PASSWORD" \
      "$SMTP_FROM" "$SMTP_FROM_NAME" "$SMTP_USE_TLS" "$SMTP_USE_SSL"
  fi
  write_systemd_units
  install_manager_copy
}

resume_values_from_state() {
  DOMAIN=$(state_get domain)
  ACME_EMAIL=$(state_get acme_email)
  APP_PORT=$(state_get app_port)
  REDIS_PORT=$(state_get redis_port)
  ADMIN_PATH=$(state_get admin_path)
  CERT_NAME=$(state_get cert_name)
}

run_install() {
  check_unmanaged_conflicts

  local new_install=false
  if ! state_exists; then
    new_install=true
    check_os_support
    check_resources
    if ! ui_yesno "安装准备" "将通过 apt 安装 Nginx、Certbot、Redis、whiptail、PyYAML 等依赖。是否继续？"; then
      die "用户取消安装"
    fi
    REDIS_PACKAGE_PREEXISTING=false
    install_dependencies
    check_web_listener_conflicts
    collect_install_input
    check_domain_preconditions "$DOMAIN"
    show_install_summary || die "用户取消安装"
    state_init "$DOMAIN" "$ACME_EMAIL" "$APP_PORT" "$REDIS_PORT" "$ADMIN_PATH" "$CERT_NAME"
    state_set redis_package_preexisting "$REDIS_PACKAGE_PREEXISTING"
  else
    check_os_support
    resume_values_from_state
    if [[ "$(state_get phase)" == "installed" ]]; then
      ui_message "已经安装" "Dujiao-Next 已安装完成。\n商城：https://${DOMAIN}\n后台：https://${DOMAIN}${ADMIN_PATH}/"
      return 0
    fi
    install_dependencies
    check_web_listener_conflicts
    log_info "检测到中断的安装，将从阶段 $(state_get phase) 继续。"
  fi

  maybe_open_ufw
  check_domain_preconditions "$DOMAIN"

  if [[ ! -s "$CONFIG_FILE" ]]; then
    if [[ "$new_install" != "true" ]]; then
      ADMIN_USERNAME=$(prompt_validated validate_admin_username "管理员账号" "恢复安装需要重新填写首次管理员账号" "admin")
      ADMIN_PASSWORD=$(prompt_password_pair)
      collect_smtp_config
    fi
  else
    ADMIN_USERNAME=$(config_read bootstrap.default_admin_username 2>/dev/null || true)
    ADMIN_PASSWORD=$(config_read bootstrap.default_admin_password 2>/dev/null || true)
    SMTP_ENABLED=$(config_read email.enabled 2>/dev/null || printf 'false')
    SMTP_TEST_RECIPIENT=""
  fi

  prepare_application_files "$APP_PORT" "$REDIS_PORT" "$ADMIN_PATH" "${ADMIN_USERNAME:-}" "${ADMIN_PASSWORD:-}"

  local phase
  phase=$(state_get phase)
  if ! phase_at_least "$phase" acme_ready; then
    mkdir -p -- "${ACME_ROOT}/.well-known/acme-challenge"
    chmod -R 0755 "$ACME_ROOT"
    write_acme_site "$DOMAIN"
    write_certbot_deploy_hook
    state_set phase acme_ready
    phase=acme_ready
  fi

  if ! phase_at_least "$phase" cert_issued; then
    if ! verify_acme_reachability "$DOMAIN"; then
      die "无法通过 http://${DOMAIN}/.well-known/acme-challenge/ 回读挑战文件。已保留 ACME 站点，请检查 DNS、TCP 80、安全组和 CDN 回源后重新运行。"
    fi
    issue_certificate "$DOMAIN" "$ACME_EMAIL" "$CERT_NAME" || die "证书申请失败；未开放 HTTP 商城，可修复后重新运行。"
    state_set phase cert_issued
    phase=cert_issued
  fi

  if ! phase_at_least "$phase" services_ready; then
    systemctl enable --now "$REDIS_SERVICE"
    systemctl enable --now "$APP_SERVICE"
    wait_for_local_health "$APP_PORT" || die "Dujiao-Next 本机健康检查失败"
    ensure_loopback_bindings "$APP_PORT" "$REDIS_PORT"

    ADMIN_USERNAME=$(config_read bootstrap.default_admin_username 2>/dev/null || true)
    ADMIN_PASSWORD=$(config_read bootstrap.default_admin_password 2>/dev/null || true)
    if [[ -n "$ADMIN_USERNAME" && -n "$ADMIN_PASSWORD" ]]; then
      verify_admin_exists "$ADMIN_USERNAME" || die "未确认默认管理员初始化成功，保留 bootstrap 凭据并停止安装"
      if [[ "$(config_read email.enabled 2>/dev/null || printf 'false')" == "true" ]]; then
        local recipient=${SMTP_TEST_RECIPIENT:-}
        if [[ -z "$recipient" ]]; then
          recipient=$(config_read email.from 2>/dev/null || true)
        fi
        if [[ -n "$recipient" ]] && ui_yesno "SMTP 测试" "是否向 ${recipient} 发送测试邮件？失败时脚本会自动禁用 SMTP，避免邮箱注册处于假可用状态。"; then
          if ! smtp_test "$APP_PORT" "$ADMIN_USERNAME" "$ADMIN_PASSWORD" "$recipient"; then
            log_warn "SMTP 测试失败，已将 SMTP 保持为禁用状态；请安装后在后台重新配置。"
            config_disable_email
            systemctl restart "$APP_SERVICE"
            wait_for_local_health "$APP_PORT" || die "禁用 SMTP 后应用未恢复健康"
          fi
        fi
      fi
      config_clear_bootstrap
    fi
    state_set phase services_ready
    phase=services_ready
  fi

  write_final_nginx_site "$DOMAIN" "$APP_PORT" "$CERT_NAME"
  verify_public_site "$DOMAIN" "$ADMIN_PATH" || die "公网 HTTPS 验证失败；证书和服务已保留，请检查 CDN、DNS 或出口网络后重新运行。"
  ensure_loopback_bindings "$APP_PORT" "$REDIS_PORT"
  state_set phase installed

  local smtp_note=""
  if [[ "$(config_read email.enabled 2>/dev/null || printf 'false')" != "true" ]]; then
    smtp_note=$'\n\n注意：SMTP 尚未启用。请登录后台完成 SMTP 配置后再开放邮箱验证码注册。'
  fi
  ui_message "安装成功" "商城：https://${DOMAIN}\n后台：https://${DOMAIN}${ADMIN_PATH}/\n管理命令：sudo dujiao-next-manager${smtp_note}"
}

require_installed_state() {
  state_exists || die "未找到由本安装器管理的 Dujiao-Next"
}

show_status() {
  require_installed_state
  resume_values_from_state
  local app_status redis_status nginx_status health version cert_expiry
  app_status=$(systemctl is-active "$APP_SERVICE" 2>/dev/null || true)
  redis_status=$(systemctl is-active "$REDIS_SERVICE" 2>/dev/null || true)
  nginx_status=$(systemctl is-active nginx 2>/dev/null || true)
  health="失败"
  curl --fail --silent --max-time 3 "http://127.0.0.1:${APP_PORT}/health" >/dev/null 2>&1 && health="正常"
  version=$(journalctl -u "$APP_SERVICE" -n 250 --no-pager 2>/dev/null | awk '/Version:/ {value=$NF} END {print value}')
  [[ -n "$version" ]] || version=$(state_get release_version 2>/dev/null || printf 'unknown')
  cert_expiry="未知"
  local certificate
  certificate=$(root_path "/etc/letsencrypt/live/${CERT_NAME}/cert.pem")
  if [[ -s "$certificate" ]]; then
    cert_expiry=$(openssl x509 -in "$certificate" -noout -enddate 2>/dev/null | cut -d= -f2-)
  fi
  local message
  message=$(cat <<EOF
安装阶段：$(state_get phase)
应用版本：${version}
应用服务：${app_status}
Redis 服务：${redis_status}
Nginx 服务：${nginx_status}
本机健康：${health}
商城地址：https://${DOMAIN}
后台地址：https://${DOMAIN}${ADMIN_PATH}/
应用监听：127.0.0.1:${APP_PORT}
Redis 监听：127.0.0.1:${REDIS_PORT}
证书到期：${cert_expiry}
EOF
)
  ui_message "Dujiao-Next 状态" "$message"
}

show_logs() {
  require_installed_state
  local target=${1:-}
  if [[ -z "$target" ]]; then
    target=$(ui_menu "查看日志" "请选择日志来源" \
      app "Dujiao-Next 应用" \
      redis "独立 Redis" \
      nginx "Nginx" \
      certbot "Certbot") || return 0
  fi
  case "$target" in
    app) journalctl -u "$APP_SERVICE" -n 200 --no-pager ;;
    redis) journalctl -u "$REDIS_SERVICE" -n 200 --no-pager ;;
    nginx) journalctl -u nginx -n 200 --no-pager ;;
    certbot) journalctl -u certbot.service -u certbot.timer -n 200 --no-pager ;;
    *) die "未知日志目标：$target" ;;
  esac
}

service_action() {
  local action=$1
  require_installed_state
  case "$action" in
    start)
      systemctl start "$REDIS_SERVICE" "$APP_SERVICE"
      ;;
    stop)
      systemctl stop "$APP_SERVICE" "$REDIS_SERVICE"
      ;;
    restart)
      systemctl restart "$REDIS_SERVICE"
      systemctl restart "$APP_SERVICE"
      ;;
    *) die "未知服务操作：$action" ;;
  esac
  show_status
}

restart_application_service() {
  systemctl restart "$APP_SERVICE"
}

verify_local_admin_path() {
  local app_port=$1
  local admin_path=$2
  wait_for_local_health "$app_port" || return 1
  local base_url="http://127.0.0.1:${app_port}${admin_path}/"
  local admin_html="${RUN_TMP}/local-admin.html"
  curl --fail --silent --max-time 10 "$base_url" -o "$admin_html" || return 1
  local asset_url
  asset_url=$(extract_first_asset_url "$admin_html" "$base_url") || return 1
  [[ -n "$asset_url" ]] || return 1
  curl --fail --silent --max-time 10 "$asset_url" >/dev/null
}

restore_application_config() {
  local backup=$1
  install -o "$SERVICE_USER" -g "$SERVICE_GROUP" -m 0640 "$backup" "$CONFIG_FILE"
}

apply_admin_path_change() {
  local new_path=$1
  local backup="${RUN_TMP}/config.yml.before-admin-path"
  cp -p -- "$CONFIG_FILE" "$backup" || return 1
  config_patch_admin_path "$new_path" || return 1
  if ! restart_application_service || ! verify_local_admin_path "$APP_PORT" "$new_path"; then
    log_warn "新后台路径验证失败，正在恢复原配置。"
    restore_application_config "$backup" || log_error "恢复旧配置文件失败，请立即检查 config.yml"
    restart_application_service || true
    wait_for_local_health "$APP_PORT" || true
    return 1
  fi
  if ! state_set admin_path "$new_path"; then
    log_warn "新后台路径已启动，但安装状态写入失败，正在恢复原配置。"
    restore_application_config "$backup" || log_error "恢复旧配置文件失败，请立即检查 config.yml"
    restart_application_service || true
    wait_for_local_health "$APP_PORT" || true
    return 1
  fi
  ADMIN_PATH=$new_path
}

configure_admin_path() {
  require_installed_state
  resume_values_from_state
  local new_path
  new_path=$(prompt_validated validate_admin_path "修改后台入口" "请输入新的后台路径" "$ADMIN_PATH")
  [[ "$new_path" != "$ADMIN_PATH" ]] || return 0
  apply_admin_path_change "$new_path" || die "后台路径修改失败，原配置已恢复"
  verify_public_site "$DOMAIN" "$ADMIN_PATH" || log_warn "本机修改成功，但公网后台地址验证失败，请检查 CDN 缓存。"
  ui_message "修改成功" "新的后台地址：https://${DOMAIN}${ADMIN_PATH}/"
}

configure_domain() {
  require_installed_state
  resume_values_from_state
  local old_domain=$DOMAIN
  local old_cert=$CERT_NAME
  local new_domain new_email new_cert
  new_domain=$(prompt_validated validate_domain "修改域名" "请输入已解析到本服务器的新域名" "$DOMAIN")
  new_domain=$(lowercase "$new_domain")
  [[ "$new_domain" != "$old_domain" ]] || return 0
  new_email=$(prompt_validated validate_email "证书邮箱" "用于新证书到期通知" "$ACME_EMAIL")
  new_cert=$(certificate_name_for_domain "$new_domain")
  getent ahosts "$new_domain" >/dev/null 2>&1 || die "新域名当前无法解析"
  if nginx_has_domain_conflict "$new_domain"; then
    die "Nginx 已有站点声明新域名，拒绝接管。"
  fi
  local new_live
  new_live=$(root_path "/etc/letsencrypt/live/${new_cert}")
  [[ ! -e "$new_live" ]] || die "证书名 ${new_cert} 已存在但不在当前安装状态中，拒绝接管。"

  write_transition_acme_site "$new_domain"
  if ! verify_acme_reachability "$new_domain"; then
    remove_transition_site
    die "新域名 ACME 挑战无法回读，旧域名保持不变。"
  fi
  if ! issue_certificate "$new_domain" "$new_email" "$new_cert"; then
    remove_transition_site
    die "新域名证书申请失败，旧域名保持不变。"
  fi

  local nginx_backup="${RUN_TMP}/nginx-before-domain.conf"
  cp -p -- "$NGINX_AVAILABLE" "$nginx_backup"
  remove_transition_site
  if ! write_final_nginx_site "$new_domain" "$APP_PORT" "$new_cert" || \
     ! verify_public_site "$new_domain" "$ADMIN_PATH"; then
    install -o root -g root -m 0644 "$nginx_backup" "$NGINX_AVAILABLE"
    ln -sfn "$NGINX_AVAILABLE" "$NGINX_ENABLED"
    nginx -t && systemctl reload nginx
    certbot delete --cert-name "$new_cert" --non-interactive >/dev/null 2>&1 || true
    die "新域名切换验证失败，旧域名已恢复。"
  fi

  state_set domain "$new_domain"
  state_set acme_email "$new_email"
  state_set cert_name "$new_cert"
  certbot delete --cert-name "$old_cert" --non-interactive >/dev/null 2>&1 || log_warn "旧证书未自动删除，可稍后手工清理：$old_cert"
  ui_message "域名切换成功" "商城：https://${new_domain}\n后台：https://${new_domain}${ADMIN_PATH}/"
}

renew_certificate() {
  require_installed_state
  resume_values_from_state
  certbot renew --cert-name "$CERT_NAME" --deploy-hook "$CERTBOT_DEPLOY_HOOK"
  ui_message "证书续签" "Certbot 已完成检查。未到期的证书会保持不变；成功续签时 Nginx 已自动重载。"
}

admin_recovery() {
  local operation=$1
  require_installed_state
  local username
  username=$(prompt_validated validate_admin_username "管理员恢复" "请输入目标管理员用户名" "admin")
  systemctl stop "$APP_SERVICE"
  local result=0
  case "$operation" in
    password)
      local password
      password=$(prompt_password_pair)
      if ! printf '%s\n' "$password" | runuser -u "$SERVICE_USER" -- bash -c 'cd "$1" && ./dujiao-next admin reset-password --username "$2"' _ "$INSTALL_DIR" "$username"; then
        result=1
      fi
      ;;
    2fa)
      if ! runuser -u "$SERVICE_USER" -- bash -c 'cd "$1" && ./dujiao-next admin reset-2fa --username "$2"' _ "$INSTALL_DIR" "$username"; then
        result=1
      fi
      ;;
    *) result=1 ;;
  esac
  systemctl start "$APP_SERVICE"
  wait_for_local_health "$(state_get app_port)" || true
  ((result == 0)) || die "管理员恢复操作失败"
  ui_message "管理员恢复" "操作已完成，应用服务已重新启动。"
}

create_uninstall_backup() {
  local timestamp
  timestamp=$(date -u +%Y%m%dT%H%M%SZ)
  mkdir -p -- "$BACKUP_DIR" || return 1
  chmod 0700 "$BACKUP_DIR" || return 1
  local staging="${RUN_TMP}/backup-staging"
  mkdir -p -- "$staging" || return 1
  [[ -s "$CONFIG_FILE" ]] || return 1
  cp -p -- "$CONFIG_FILE" "${staging}/config.yml" || return 1
  cp -a -- "${INSTALL_DIR}/db" "${staging}/db" || return 1
  cp -a -- "${INSTALL_DIR}/uploads" "${staging}/uploads" || return 1
  cp -p -- "$STATE_FILE" "${staging}/install-state.json" || return 1
  cat > "${staging}/README.txt" <<EOF || return 1
Dujiao-Next uninstall backup
Created: ${timestamp}
Restore requires config.yml, db/, and uploads/ together.
The app.secret_key in config.yml is required to decrypt stored secrets.
EOF
  local temporary="${RUN_TMP}/dujiao-next-uninstall-${timestamp}.tar.gz"
  tar -czf "$temporary" -C "$staging" . || return 1
  tar -tzf "$temporary" >/dev/null || return 1
  local listing
  listing=$(tar -tzf "$temporary") || return 1
  grep -q '^\./config.yml$' <<< "$listing" || return 1
  grep -q '^\./db/' <<< "$listing" || return 1
  grep -q '^\./uploads/' <<< "$listing" || return 1
  grep -q '^\./install-state.json$' <<< "$listing" || return 1
  local destination="${BACKUP_DIR}/dujiao-next-uninstall-${timestamp}.tar.gz"
  install -o root -g root -m 0600 "$temporary" "$destination" || return 1
  printf '%s' "$destination"
}

uninstall_managed() {
  require_installed_state
  resume_values_from_state
  if ! ui_yesno "安全卸载" "卸载会停止服务并移除应用、独立 Redis、Nginx站点和安装器签发的证书。脚本会先创建强制备份；系统软件包不会卸载。是否继续？"; then
    return 0
  fi
  local app_was_active=false redis_was_active=false
  local user_created
  user_created=$(state_get user_created 2>/dev/null || printf 'false')
  systemctl is-active --quiet "$APP_SERVICE" && app_was_active=true
  systemctl is-active --quiet "$REDIS_SERVICE" && redis_was_active=true
  systemctl stop "$APP_SERVICE" || true
  systemctl stop "$REDIS_SERVICE" || true

  local backup
  if ! backup=$(create_uninstall_backup); then
    log_error "卸载备份失败，不会删除任何数据。"
    [[ "$redis_was_active" == "true" ]] && systemctl start "$REDIS_SERVICE" || true
    [[ "$app_was_active" == "true" ]] && systemctl start "$APP_SERVICE" || true
    die "卸载已取消"
  fi

  systemctl disable "$APP_SERVICE" "$REDIS_SERVICE" >/dev/null 2>&1 || true
  rm -f -- "$APP_UNIT_FILE" "$REDIS_UNIT_FILE"
  systemctl daemon-reload
  rm -f -- "$NGINX_ENABLED" "$NGINX_AVAILABLE" "$NGINX_TRANSITION_ENABLED" "$NGINX_TRANSITION_AVAILABLE" "$CERTBOT_DEPLOY_HOOK"
  if command -v nginx >/dev/null 2>&1 && nginx -t >/dev/null 2>&1; then
    systemctl reload nginx || true
  fi
  certbot delete --cert-name "$CERT_NAME" --non-interactive >/dev/null 2>&1 || true
  assert_safe_managed_paths
  rm -rf -- "$INSTALL_DIR" "$ETC_DIR" "$(dirname "$REDIS_DATA_DIR")" "$ACME_ROOT"
  if [[ "$user_created" == "true" ]]; then
    userdel "$SERVICE_USER" >/dev/null 2>&1 || true
  fi

  local destroy=false phrase=""
  if ui_yesno "保留恢复点" "卸载备份已保存到：\n${backup}\n\n是否连该备份也彻底销毁？默认建议保留。"; then
    phrase=$(ui_input "最终确认" "输入 DELETE DUJIAO DATA 才会删除唯一恢复备份" "") || true
    validate_destroy_phrase "$phrase" && destroy=true
  fi
  if [[ "$destroy" == "true" ]]; then
    rm -f -- "$backup"
    log_warn "卸载备份已按确认彻底删除，无法通过本安装器恢复。"
  else
    log_info "恢复备份保留在：$backup"
  fi
  rm -f -- "$MANAGER_BIN"
  ui_message "卸载完成" "Dujiao-Next 已卸载。\n恢复备份：$([[ "$destroy" == "true" ]] && printf '已销毁' || printf '%s' "$backup")"
}

show_smtp_hint() {
  require_installed_state
  resume_values_from_state
  ui_message "SMTP 设置" "安装后的 SMTP 配置以数据库中的后台设置为准。\n请打开：https://${DOMAIN}${ADMIN_PATH}/\n进入 设置 → SMTP 邮件 完成修改和测试。"
}

main_menu() {
  while true; do
    local choice
    if ! state_exists; then
      choice=$(ui_menu "Dujiao-Next 管理器" "未检测到已完成安装" \
        install "安装或继续中断的安装" \
        exit "退出") || return 0
    else
      choice=$(ui_menu "Dujiao-Next 管理器" "请选择操作" \
        status "查看状态" \
        logs "查看日志" \
        start "启动服务" \
        stop "停止服务" \
        restart "重启服务" \
        domain "修改域名和证书" \
        admin_path "修改后台入口" \
        smtp "前往后台配置 SMTP" \
        renew "检查并续签证书" \
        reset_password "重置管理员密码" \
        reset_2fa "重置管理员 2FA" \
        uninstall "安全卸载" \
        exit "退出") || return 0
    fi
    case "$choice" in
      install) run_install ;;
      status) show_status ;;
      logs) show_logs ;;
      start|stop|restart) service_action "$choice" ;;
      domain) configure_domain ;;
      admin_path) configure_admin_path ;;
      smtp) show_smtp_hint ;;
      renew) renew_certificate ;;
      reset_password) admin_recovery password ;;
      reset_2fa) admin_recovery 2fa ;;
      uninstall) uninstall_managed; return 0 ;;
      exit) return 0 ;;
    esac
  done
}

usage() {
  cat <<'EOF'
Dujiao-Next 官方安装与运维管理器

用法：
  dujiao-next-manager                         打开交互式管理菜单
  dujiao-next-manager install                 安装或继续中断的安装
  dujiao-next-manager status                  查看状态
  dujiao-next-manager logs [app|redis|nginx|certbot]
  dujiao-next-manager start|stop|restart
  dujiao-next-manager configure-domain
  dujiao-next-manager configure-admin-path
  dujiao-next-manager renew-cert
  dujiao-next-manager admin-reset-password
  dujiao-next-manager admin-reset-2fa
  dujiao-next-manager uninstall
EOF
}

main() {
  require_root
  init_runtime
  acquire_lock
  assert_safe_managed_paths
  local command=${1:-menu}
  case "$command" in
    menu) main_menu ;;
    install) run_install ;;
    status) show_status ;;
    logs) show_logs "${2:-}" ;;
    start|stop|restart) service_action "$command" ;;
    configure-domain) configure_domain ;;
    configure-admin-path) configure_admin_path ;;
    renew-cert) renew_certificate ;;
    admin-reset-password) admin_recovery password ;;
    admin-reset-2fa) admin_recovery 2fa ;;
    uninstall) uninstall_managed ;;
    -h|--help|help) usage ;;
    --version) printf 'dujiao-next-manager %s\n' "$MANAGER_VERSION" ;;
    *) usage; die "未知命令：$command" ;;
  esac
}

if [[ "${DUJIAO_MANAGER_TESTING:-0}" != "1" ]]; then
  main "$@"
fi
