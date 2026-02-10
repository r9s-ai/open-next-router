#!/usr/bin/env bash
set -euo pipefail

REPO_SLUG="r9s-ai/open-next-router"
PROJECT_NAME="open-next-router"
DOCKER_IMAGE="ghcr.io/r9s-ai/open-next-router"
SCRIPT_MARKER="Managed by tools/install_onr_service.sh"

MODE="" # service | docker | docker-compose (required)
VERSION=""
SERVICE_NAME="onr"
CONTAINER_NAME="onr"
SERVICE_USER="onr"
SERVICE_GROUP="onr"
LISTEN=":3000"
HOST_PORT=""
API_KEY=""
FORCE=0
NO_START=0
DRY_RUN=0

CONFIG_DIR="/etc/onr"
STATE_DIR="/var/lib/onr"
BIN_DIR="/usr/local/bin"

CONFIG_FILE="${CONFIG_DIR}/onr.yaml"
ENV_FILE="${CONFIG_DIR}/onr.env"
PROVIDERS_DIR="${CONFIG_DIR}/providers"
KEYS_FILE="${CONFIG_DIR}/keys.yaml"
MODELS_FILE="${CONFIG_DIR}/models.yaml"
COMPOSE_FILE="${CONFIG_DIR}/docker-compose.yml"
COMPOSE_DRIVER=""

log() {
  echo "[onr-install] $*"
}

warn() {
  echo "[onr-install] warning: $*" >&2
}

die() {
  echo "[onr-install] error: $*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
One-click installer for open-next-router (ONR).

Usage:
  tools/install_onr_service.sh [options]

Options:
  --mode <service|docker|docker-compose> Install mode (required)
  --version <vX.Y.Z|X.Y.Z>     Runtime release tag only (default: latest runtime v*)
  --api-key <value>            ONR API key (required on first install)
  --listen <addr>              ONR listen address (default: :3000)
  --host-port <port>           Docker host port (default: same as listen port)
  --service-name <name>        systemd service name / default container name (default: onr)
  --container-name <name>      Docker container name (default: onr)
  --user <name>                Runtime user for service mode (default: onr)
  --group <name>               Runtime group for service mode (default: onr)
  --force                      Overwrite managed config files and seed files
  --no-start                   Install but do not start service/container
  --dry-run                    Print actions without writing changes
  -h, --help                   Show help

Examples:
  # Install systemd service from latest release
  sudo tools/install_onr_service.sh --mode service --api-key 'change-me'

  # Install docker mode from a specific release
  sudo tools/install_onr_service.sh --mode docker --version v0.10.0 --api-key 'change-me'

  # Install docker-compose mode from latest release
  sudo tools/install_onr_service.sh --mode docker-compose --api-key 'change-me'

Notes:
  - Config bundle is downloaded from GitHub Release asset:
    open-next-router_config_vX.Y.Z.tar.gz
  - Provider DSL, keys.example.yaml and models.example.yaml are seeded from that bundle
EOF
}

run_cmd() {
  if (( DRY_RUN == 1 )); then
    printf '+'
    printf ' %q' "$@"
    printf '\n'
    return 0
  fi
  "$@"
}

need_cmd() {
  local cmd="$1"
  command -v "${cmd}" >/dev/null 2>&1 || die "missing required command: ${cmd}"
}

find_checksum_for_asset() {
  local sums_file="$1"
  local asset_name="$2"
  awk -v target="${asset_name}" '
    NF >= 2 {
      file = $2
      n = split(file, parts, "/")
      base = parts[n]
      if (base == target) {
        print $1
        exit
      }
    }
  ' "${sums_file}"
}

is_managed_file() {
  local path="$1"
  [[ -f "${path}" ]] || return 1
  grep -q "${SCRIPT_MARKER}" "${path}"
}

backup_if_exists() {
  local path="$1"
  [[ -f "${path}" ]] || return 0
  local ts
  ts="$(date '+%Y%m%d-%H%M%S')"
  run_cmd cp -a "${path}" "${path}.bak.${ts}"
}

normalize_version() {
  local v="$1"
  v="$(echo "${v}" | xargs)"
  if [[ -z "${v}" ]]; then
    echo ""
    return 0
  fi
  if [[ "${v}" == onr-core/* ]]; then
    echo "[onr-install] error: invalid --version '${v}': onr-core tags are not installable by this script" >&2
    return 1
  fi
  if [[ "${v}" == */* ]]; then
    echo "[onr-install] error: invalid --version '${v}': only runtime tags like v0.10.0 are supported" >&2
    return 1
  fi
  if [[ "${v}" == v* ]]; then
    echo "${v}"
    return 0
  fi
  echo "v${v}"
}

runtime_image_tag() {
  local release_tag="$1"
  echo "${release_tag#v}"
}

is_runtime_tag() {
  local tag="$1"
  [[ -n "${tag}" ]] || return 1
  [[ "${tag}" == v* ]] || return 1
  [[ "${tag}" == */* ]] && return 1
  return 0
}

resolve_release_tag() {
  local requested="$1"
  if [[ -n "${requested}" ]]; then
    local normalized
    normalized="$(normalize_version "${requested}")" || return 1
    is_runtime_tag "${normalized}" || die "invalid runtime tag: ${normalized}"
    echo "${normalized}"
    return 0
  fi

  local api="https://api.github.com/repos/${REPO_SLUG}/releases?per_page=100"
  local body
  body="$(curl -fsSL "${api}")" || die "failed to query releases from ${api}"

  local tags
  tags="$(printf '%s' "${body}" | grep -o '"tag_name":[[:space:]]*"[^"]*"' | sed 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/')"
  [[ -n "${tags}" ]] || die "failed to parse release tags from GitHub API response"

  local tag=""
  while IFS= read -r t; do
    if is_runtime_tag "${t}"; then
      tag="${t}"
      break
    fi
  done <<< "${tags}"
  [[ -n "${tag}" ]] || die "no runtime release tag (v*) found in recent GitHub releases"
  echo "${tag}"
}

detect_linux_arch() {
  local machine
  machine="$(uname -m)"
  case "${machine}" in
    x86_64|amd64) echo "x86_64" ;;
    aarch64|arm64) echo "arm64" ;;
    *)
      die "unsupported architecture: ${machine} (supported: x86_64, arm64)"
      ;;
  esac
}

extract_listen_port() {
  local listen="$1"
  local port=""
  if [[ "${listen}" =~ :([0-9]+)$ ]]; then
    port="${BASH_REMATCH[1]}"
  elif [[ "${listen}" =~ ^([0-9]+)$ ]]; then
    port="${BASH_REMATCH[1]}"
  fi
  [[ -n "${port}" ]] || die "failed to parse port from --listen '${listen}'"
  echo "${port}"
}

ensure_runtime_dirs() {
  run_cmd install -d -m 0750 "${CONFIG_DIR}" "${PROVIDERS_DIR}" "${STATE_DIR}" "${STATE_DIR}/oauth"
}

copy_seed_file() {
  local src="$1"
  local dst="$2"
  if [[ -f "${dst}" && ${FORCE} -eq 0 ]]; then
    return 0
  fi
  backup_if_exists "${dst}"
  run_cmd cp "${src}" "${dst}"
}

install_config_files_from_release() {
  local release_tag="$1"
  local asset="open-next-router_config_${release_tag}.tar.gz"
  local base_url="https://github.com/${REPO_SLUG}/releases/download/${release_tag}"
  local asset_url="${base_url}/${asset}"
  local checksums_url="${base_url}/checksums.txt"

  if (( DRY_RUN == 1 )); then
    log "[dry-run] download ${asset_url}"
    log "[dry-run] download ${checksums_url}"
    log "[dry-run] extract config/providers/*.conf to ${PROVIDERS_DIR}"
    log "[dry-run] seed ${KEYS_FILE} and ${MODELS_FILE} from config/*.example.yaml"
    return 0
  fi

  local tmpdir
  tmpdir="$(mktemp -d)"

  local tarball="${tmpdir}/${asset}"
  local sums="${tmpdir}/checksums.txt"
  local extract_dir="${tmpdir}/extract"

  log "downloading ${asset}"
  curl -fL --retry 3 --retry-delay 1 -o "${tarball}" "${asset_url}" || die "failed to download config archive: ${asset_url}"
  curl -fL --retry 3 --retry-delay 1 -o "${sums}" "${checksums_url}" || die "failed to download checksums: ${checksums_url}"

  if command -v sha256sum >/dev/null 2>&1; then
    local expected
    expected="$(find_checksum_for_asset "${sums}" "${asset}")"
    [[ -n "${expected}" ]] || die "checksum entry not found for ${asset}"
    local actual
    actual="$(sha256sum "${tarball}" | awk '{print $1}')"
    [[ "${actual}" == "${expected}" ]] || die "checksum mismatch for ${asset}"
  else
    warn "sha256sum not found, skipping checksum verification"
  fi

  mkdir -p "${extract_dir}"
  tar -xzf "${tarball}" -C "${extract_dir}"

  local config_src="${extract_dir}/config"
  local providers_src="${config_src}/providers"
  local keys_src="${config_src}/keys.example.yaml"
  local models_src="${config_src}/models.example.yaml"
  [[ -d "${providers_src}" ]] || die "providers directory not found in archive: ${asset}"
  [[ -f "${keys_src}" ]] || die "keys.example.yaml not found in archive: ${asset}"
  [[ -f "${models_src}" ]] || die "models.example.yaml not found in archive: ${asset}"

  local src
  local copied=0
  shopt -s nullglob
  for src in "${providers_src}"/*.conf; do
    copied=1
    local dst="${PROVIDERS_DIR}/$(basename "${src}")"
    if [[ -f "${dst}" && ${FORCE} -eq 0 ]]; then
      continue
    fi
    backup_if_exists "${dst}"
    cp "${src}" "${dst}"
  done
  shopt -u nullglob
  copy_seed_file "${keys_src}" "${KEYS_FILE}"
  copy_seed_file "${models_src}" "${MODELS_FILE}"
  rm -rf "${tmpdir}"
  (( copied == 1 )) || die "no providers conf files found in archive: ${asset}"
}

write_onr_config() {
  local pid_file="$1"
  local should_write=1
  if [[ -f "${CONFIG_FILE}" ]]; then
    if [[ ${FORCE} -eq 1 ]] || is_managed_file "${CONFIG_FILE}"; then
      should_write=1
    else
      should_write=0
      warn "keeping existing custom config: ${CONFIG_FILE} (use --force to overwrite)"
    fi
  fi
  (( should_write == 1 )) || return 0

  backup_if_exists "${CONFIG_FILE}"
  if (( DRY_RUN == 1 )); then
    log "[dry-run] write ${CONFIG_FILE}"
    return 0
  fi

  cat >"${CONFIG_FILE}" <<EOF
# ${SCRIPT_MARKER}
server:
  listen: "${LISTEN}"
  read_timeout_ms: 60000
  write_timeout_ms: 60000
  pid_file: "${pid_file}"

auth:
  api_key: "change-me"

providers:
  dir: "${PROVIDERS_DIR}"

keys:
  file: "${KEYS_FILE}"

models:
  file: "${MODELS_FILE}"

oauth:
  token_persist:
    enabled: true
    dir: "${STATE_DIR}/oauth"

pricing:
  enabled: false
  file: "${CONFIG_DIR}/price.yaml"
  overrides_file: "${CONFIG_DIR}/price_overrides.yaml"

upstream_proxies:
  by_provider:

usage_estimation:
  enabled: true

logging:
  level: "info"
  access_log: true
  access_log_path: ""
EOF
}

read_env_value() {
  local key="$1"
  local file="$2"
  [[ -f "${file}" ]] || return 1
  awk -F= -v key="${key}" '$1 == key {val=substr($0, index($0, "=")+1)} END {if (val != "") print val}' "${file}"
}

upsert_env_value() {
  local key="$1"
  local value="$2"
  if (( DRY_RUN == 1 )); then
    log "[dry-run] set ${key} in ${ENV_FILE}"
    return 0
  fi

  if [[ ! -f "${ENV_FILE}" ]]; then
    cat >"${ENV_FILE}" <<EOF
# ${SCRIPT_MARKER}
# Runtime env vars for ONR.
${key}=${value}
# Example upstream key override:
# ONR_UPSTREAM_KEY_OPENAI_KEY1=sk-xxxx
EOF
    return 0
  fi

  local tmp
  tmp="$(mktemp)"
  awk -F= -v key="${key}" -v value="${value}" '
    BEGIN { updated = 0 }
    $1 == key { print key "=" value; updated = 1; next }
    { print $0 }
    END { if (updated == 0) print key "=" value }
  ' "${ENV_FILE}" >"${tmp}"
  mv "${tmp}" "${ENV_FILE}"
}

ensure_api_key() {
  if [[ -n "${API_KEY}" ]]; then
    return 0
  fi
  API_KEY="$(read_env_value "ONR_API_KEY" "${ENV_FILE}" || true)"
  [[ -n "${API_KEY}" ]] || die "--api-key is required on first install (or keep ONR_API_KEY in ${ENV_FILE})"
}

set_permissions_service() {
  run_cmd chown -R root:"${SERVICE_GROUP}" "${CONFIG_DIR}"
  run_cmd chmod 0750 "${CONFIG_DIR}" "${PROVIDERS_DIR}"
  run_cmd chmod 0750 "${STATE_DIR}" "${STATE_DIR}/oauth"

  [[ -f "${CONFIG_FILE}" ]] && run_cmd chmod 0640 "${CONFIG_FILE}"
  [[ -f "${ENV_FILE}" ]] && run_cmd chmod 0640 "${ENV_FILE}"
  [[ -f "${KEYS_FILE}" ]] && run_cmd chmod 0640 "${KEYS_FILE}"
  [[ -f "${MODELS_FILE}" ]] && run_cmd chmod 0640 "${MODELS_FILE}"

  local conf
  shopt -s nullglob
  for conf in "${PROVIDERS_DIR}"/*.conf; do
    run_cmd chmod 0640 "${conf}"
  done
  shopt -u nullglob

  run_cmd chown -R "${SERVICE_USER}:${SERVICE_GROUP}" "${STATE_DIR}"
}

set_permissions_docker() {
  run_cmd chown -R 10001:10001 "${CONFIG_DIR}" "${STATE_DIR}"
  run_cmd chmod 0750 "${CONFIG_DIR}" "${PROVIDERS_DIR}" "${STATE_DIR}" "${STATE_DIR}/oauth"

  [[ -f "${CONFIG_FILE}" ]] && run_cmd chmod 0640 "${CONFIG_FILE}"
  [[ -f "${ENV_FILE}" ]] && run_cmd chmod 0640 "${ENV_FILE}"
  [[ -f "${KEYS_FILE}" ]] && run_cmd chmod 0640 "${KEYS_FILE}"
  [[ -f "${MODELS_FILE}" ]] && run_cmd chmod 0640 "${MODELS_FILE}"

  local conf
  shopt -s nullglob
  for conf in "${PROVIDERS_DIR}"/*.conf; do
    run_cmd chmod 0640 "${conf}"
  done
  shopt -u nullglob
}

ensure_service_user_group() {
  if getent group "${SERVICE_GROUP}" >/dev/null 2>&1; then
    :
  else
    run_cmd groupadd --system "${SERVICE_GROUP}"
  fi

  if id -u "${SERVICE_USER}" >/dev/null 2>&1; then
    :
  else
    local nologin="/usr/sbin/nologin"
    [[ -x "${nologin}" ]] || nologin="/sbin/nologin"
    [[ -x "${nologin}" ]] || nologin="/usr/bin/nologin"
    run_cmd useradd --system --gid "${SERVICE_GROUP}" --home-dir "${STATE_DIR}" --no-create-home --shell "${nologin}" "${SERVICE_USER}"
  fi
}

install_release_binaries() {
  local release_tag="$1"
  local arch="$2"
  local release_plain="${release_tag#v}"
  local asset="${PROJECT_NAME}_${release_plain}_linux_${arch}.tar.gz"
  local base_url="https://github.com/${REPO_SLUG}/releases/download/${release_tag}"
  local tar_url="${base_url}/${asset}"
  local checksums_url="${base_url}/checksums.txt"

  if (( DRY_RUN == 1 )); then
    log "[dry-run] download ${tar_url}"
    log "[dry-run] download ${checksums_url}"
    return 0
  fi

  local tmpdir
  tmpdir="$(mktemp -d)"

  local tarball="${tmpdir}/${asset}"
  local sums="${tmpdir}/checksums.txt"
  local extract_dir="${tmpdir}/extract"

  log "downloading ${asset}"
  curl -fL --retry 3 --retry-delay 1 -o "${tarball}" "${tar_url}" || die "failed to download release asset: ${tar_url}"
  curl -fL --retry 3 --retry-delay 1 -o "${sums}" "${checksums_url}" || die "failed to download checksums: ${checksums_url}"

  if command -v sha256sum >/dev/null 2>&1; then
    local expected
    expected="$(find_checksum_for_asset "${sums}" "${asset}")"
    [[ -n "${expected}" ]] || die "checksum entry not found for ${asset}"
    local actual
    actual="$(sha256sum "${tarball}" | awk '{print $1}')"
    [[ "${actual}" == "${expected}" ]] || die "checksum mismatch for ${asset}"
  else
    warn "sha256sum not found, skipping checksum verification"
  fi

  mkdir -p "${extract_dir}"
  tar -xzf "${tarball}" -C "${extract_dir}"

  [[ -f "${extract_dir}/onr" ]] || die "binary not found in release archive: onr"
  [[ -f "${extract_dir}/onr-admin" ]] || die "binary not found in release archive: onr-admin"
  install -m 0755 "${extract_dir}/onr" "${BIN_DIR}/onr"
  install -m 0755 "${extract_dir}/onr-admin" "${BIN_DIR}/onr-admin"
  rm -rf "${tmpdir}"
}

write_systemd_unit() {
  local unit_file="/etc/systemd/system/${SERVICE_NAME}.service"
  if [[ -f "${unit_file}" ]]; then
    if [[ ${FORCE} -eq 1 ]] || is_managed_file "${unit_file}"; then
      :
    else
      die "existing custom unit file detected: ${unit_file} (use --force to overwrite)"
    fi
  fi
  backup_if_exists "${unit_file}"

  if (( DRY_RUN == 1 )); then
    log "[dry-run] write ${unit_file}"
    return 0
  fi

  cat >"${unit_file}" <<EOF
# ${SCRIPT_MARKER}
[Unit]
Description=Open Next Router (${SERVICE_NAME})
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=${SERVICE_USER}
Group=${SERVICE_GROUP}
WorkingDirectory=${STATE_DIR}
EnvironmentFile=-${ENV_FILE}
ExecStart=${BIN_DIR}/onr --config ${CONFIG_FILE}
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=2s
RuntimeDirectory=${SERVICE_NAME}
RuntimeDirectoryMode=0750
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
}

test_config_service() {
  if (( DRY_RUN == 1 )); then
    log "[dry-run] ${BIN_DIR}/onr -t -c ${CONFIG_FILE}"
    return 0
  fi
  "${BIN_DIR}/onr" -t -c "${CONFIG_FILE}"
}

install_service_mode() {
  local release_tag="$1"
  local arch
  arch="$(detect_linux_arch)"

  ensure_service_user_group
  install_release_binaries "${release_tag}" "${arch}"
  set_permissions_service
  write_systemd_unit
  test_config_service

  run_cmd systemctl daemon-reload
  run_cmd systemctl enable "${SERVICE_NAME}.service"
  if (( NO_START == 0 )); then
    run_cmd systemctl restart "${SERVICE_NAME}.service"
    run_cmd systemctl --no-pager --full status "${SERVICE_NAME}.service"
  else
    log "skipped start (--no-start); service installed as ${SERVICE_NAME}.service"
  fi
}

test_config_docker() {
  local image_tag="$1"
  if (( DRY_RUN == 1 )); then
    log "[dry-run] docker run --rm ${DOCKER_IMAGE}:${image_tag} -t -c /etc/onr/onr.yaml"
    return 0
  fi
  docker run --rm \
    --env-file "${ENV_FILE}" \
    -e ONR_PROVIDERS_DIR="/etc/onr/providers" \
    -e ONR_KEYS_FILE="/etc/onr/keys.yaml" \
    -e ONR_MODELS_FILE="/etc/onr/models.yaml" \
    -e ONR_PID_FILE="/tmp/${SERVICE_NAME}.pid" \
    -v "${CONFIG_DIR}:/etc/onr:ro" \
    -v "${STATE_DIR}:${STATE_DIR}" \
    "${DOCKER_IMAGE}:${image_tag}" \
    -t -c /etc/onr/onr.yaml
}

stop_remove_container_if_exists() {
  local name="$1"
  if (( DRY_RUN == 1 )); then
    log "[dry-run] remove existing container ${name} if present"
    return 0
  fi
  if docker ps -a --format '{{.Names}}' | grep -Fxq "${name}"; then
    docker rm -f "${name}" >/dev/null
  fi
}

detect_compose_driver() {
  if docker compose version >/dev/null 2>&1; then
    COMPOSE_DRIVER="docker-compose-plugin"
    return 0
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_DRIVER="docker-compose"
    return 0
  fi
  die "docker compose is not available (install docker compose plugin or docker-compose)"
}

run_compose() {
  if [[ -z "${COMPOSE_DRIVER}" ]]; then
    COMPOSE_DRIVER="docker-compose-plugin"
  fi
  if [[ "${COMPOSE_DRIVER}" == "docker-compose-plugin" ]]; then
    run_cmd docker compose "$@"
    return 0
  fi
  run_cmd docker-compose "$@"
}

write_compose_file() {
  local image_tag="$1"
  local listen_port="$2"
  if [[ -z "${HOST_PORT}" ]]; then
    HOST_PORT="${listen_port}"
  fi

  if [[ -f "${COMPOSE_FILE}" ]]; then
    if [[ ${FORCE} -eq 1 ]] || is_managed_file "${COMPOSE_FILE}"; then
      :
    else
      die "existing custom compose file detected: ${COMPOSE_FILE} (use --force to overwrite)"
    fi
  fi
  backup_if_exists "${COMPOSE_FILE}"

  if (( DRY_RUN == 1 )); then
    log "[dry-run] write ${COMPOSE_FILE}"
    return 0
  fi

  cat >"${COMPOSE_FILE}" <<EOF
# ${SCRIPT_MARKER}
services:
  onr:
    image: ${DOCKER_IMAGE}:${image_tag}
    container_name: ${CONTAINER_NAME}
    restart: unless-stopped
    ports:
      - "${HOST_PORT}:${listen_port}"
    env_file:
      - ${ENV_FILE}
    environment:
      ONR_PROVIDERS_DIR: /etc/onr/providers
      ONR_KEYS_FILE: /etc/onr/keys.yaml
      ONR_MODELS_FILE: /etc/onr/models.yaml
      ONR_PID_FILE: /tmp/${SERVICE_NAME}.pid
    volumes:
      - ${CONFIG_DIR}:/etc/onr:ro
      - ${STATE_DIR}:${STATE_DIR}
    command: ["--config", "/etc/onr/onr.yaml"]
EOF
}

install_docker_mode() {
  local release_tag="$1"
  local image_tag
  image_tag="$(runtime_image_tag "${release_tag}")"
  local listen_port
  listen_port="$(extract_listen_port "${LISTEN}")"
  if [[ -z "${HOST_PORT}" ]]; then
    HOST_PORT="${listen_port}"
  fi

  set_permissions_docker
  run_cmd docker pull "${DOCKER_IMAGE}:${image_tag}"
  test_config_docker "${image_tag}"
  stop_remove_container_if_exists "${CONTAINER_NAME}"

  if (( NO_START == 1 )); then
    log "skipped start (--no-start); image is ready: ${DOCKER_IMAGE}:${image_tag}"
    return 0
  fi

  run_cmd docker run -d \
    --name "${CONTAINER_NAME}" \
    --restart unless-stopped \
    -p "${HOST_PORT}:${listen_port}" \
    --env-file "${ENV_FILE}" \
    -e ONR_PROVIDERS_DIR="/etc/onr/providers" \
    -e ONR_KEYS_FILE="/etc/onr/keys.yaml" \
    -e ONR_MODELS_FILE="/etc/onr/models.yaml" \
    -e ONR_PID_FILE="/tmp/${SERVICE_NAME}.pid" \
    -v "${CONFIG_DIR}:/etc/onr:ro" \
    -v "${STATE_DIR}:${STATE_DIR}" \
    "${DOCKER_IMAGE}:${image_tag}" \
    --config /etc/onr/onr.yaml

  run_cmd docker ps --filter "name=^/${CONTAINER_NAME}$"
}

install_docker_compose_mode() {
  local release_tag="$1"
  local image_tag
  image_tag="$(runtime_image_tag "${release_tag}")"
  local listen_port
  listen_port="$(extract_listen_port "${LISTEN}")"

  set_permissions_docker
  write_compose_file "${image_tag}" "${listen_port}"
  run_cmd docker pull "${DOCKER_IMAGE}:${image_tag}"
  test_config_docker "${image_tag}"

  if (( NO_START == 1 )); then
    log "skipped start (--no-start); compose file is ready: ${COMPOSE_FILE}"
    return 0
  fi

  run_compose -f "${COMPOSE_FILE}" up -d
  run_compose -f "${COMPOSE_FILE}" ps
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --mode)
        [[ $# -ge 2 ]] || die "missing value for --mode"
        MODE="$2"
        shift 2
        ;;
      --version)
        [[ $# -ge 2 ]] || die "missing value for --version"
        VERSION="$2"
        shift 2
        ;;
      --api-key)
        [[ $# -ge 2 ]] || die "missing value for --api-key"
        API_KEY="$2"
        shift 2
        ;;
      --listen)
        [[ $# -ge 2 ]] || die "missing value for --listen"
        LISTEN="$2"
        shift 2
        ;;
      --host-port)
        [[ $# -ge 2 ]] || die "missing value for --host-port"
        HOST_PORT="$2"
        shift 2
        ;;
      --service-name)
        [[ $# -ge 2 ]] || die "missing value for --service-name"
        SERVICE_NAME="$2"
        shift 2
        ;;
      --container-name)
        [[ $# -ge 2 ]] || die "missing value for --container-name"
        CONTAINER_NAME="$2"
        shift 2
        ;;
      --user)
        [[ $# -ge 2 ]] || die "missing value for --user"
        SERVICE_USER="$2"
        shift 2
        ;;
      --group)
        [[ $# -ge 2 ]] || die "missing value for --group"
        SERVICE_GROUP="$2"
        shift 2
        ;;
      --force)
        FORCE=1
        shift
        ;;
      --no-start)
        NO_START=1
        shift
        ;;
      --dry-run)
        DRY_RUN=1
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done
}

main() {
  parse_args "$@"

  [[ -n "${MODE}" ]] || die "--mode is required: choose service, docker, or docker-compose"
  [[ "${MODE}" == "service" || "${MODE}" == "docker" || "${MODE}" == "docker-compose" ]] || die "--mode must be service, docker, or docker-compose"
  [[ -n "${SERVICE_NAME}" ]] || die "--service-name cannot be empty"
  [[ -n "${CONTAINER_NAME}" ]] || CONTAINER_NAME="${SERVICE_NAME}"
  [[ -n "${LISTEN}" ]] || die "--listen cannot be empty"
  if [[ -n "${HOST_PORT}" ]]; then
    [[ "${HOST_PORT}" =~ ^[0-9]+$ ]] || die "--host-port must be a number"
  fi

  if (( DRY_RUN == 0 )) && (( EUID != 0 )); then
    die "run as root (or use sudo), or use --dry-run to preview"
  fi

  need_cmd curl
  need_cmd tar
  if (( DRY_RUN == 0 )); then
    need_cmd install
  fi
  if [[ "${MODE}" == "service" && ${DRY_RUN} -eq 0 ]]; then
    need_cmd systemctl
    need_cmd useradd
    need_cmd groupadd
    need_cmd getent
  fi
  if [[ "${MODE}" == "docker" && ${DRY_RUN} -eq 0 ]]; then
    need_cmd docker
  fi
  if [[ "${MODE}" == "docker-compose" && ${DRY_RUN} -eq 0 ]]; then
    need_cmd docker
    detect_compose_driver
  fi

  local release_tag
  release_tag="$(resolve_release_tag "${VERSION}")"
  log "target release: ${release_tag}"

  ensure_runtime_dirs
  install_config_files_from_release "${release_tag}"

  local pid_file
  if [[ "${MODE}" == "service" ]]; then
    pid_file="/run/${SERVICE_NAME}/${SERVICE_NAME}.pid"
  else
    pid_file="/tmp/${SERVICE_NAME}.pid"
  fi
  write_onr_config "${pid_file}"

  ensure_api_key
  upsert_env_value "ONR_API_KEY" "${API_KEY}"

  if [[ "${MODE}" == "service" ]]; then
    install_service_mode "${release_tag}"
    log "service mode installation complete"
    log "health check: curl -sS http://127.0.0.1$(extract_listen_port "${LISTEN}" | sed 's#^#:#')/v1/models -H 'Authorization: Bearer ${API_KEY}'"
    return 0
  fi

  if [[ "${MODE}" == "docker" ]]; then
    install_docker_mode "${release_tag}"
    log "docker mode installation complete"
    log "health check: curl -sS http://127.0.0.1:${HOST_PORT}/v1/models -H 'Authorization: Bearer ${API_KEY}'"
    return 0
  fi

  install_docker_compose_mode "${release_tag}"
  log "docker-compose mode installation complete"
  log "health check: curl -sS http://127.0.0.1:${HOST_PORT}/v1/models -H 'Authorization: Bearer ${API_KEY}'"
}

main "$@"
