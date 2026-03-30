#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage:
  tools/get_request_dump_by_id.sh [request_id]

Description:
  Resolve the traffic dump log path for a request id using onr.yaml and
  ONR_TRAFFIC_DUMP_* environment overrides.

Output:
  - absolute dump log path
  - or a readable placeholder such as:
    (traffic_dump disabled)
    (request_id missing)
    (unresolved file_path template)
EOF
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -f "${ROOT}/.env" ]]; then
  set -a
  # shellcheck source=/dev/null
  source "${ROOT}/.env"
  set +a
fi

trim() {
  local value="${1-}"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

unquote_scalar() {
  local value
  value="$(trim "${1-}")"
  if [[ "${#value}" -ge 2 ]]; then
    if [[ "${value}" == \"*\" && "${value}" == *\" ]]; then
      value="${value:1:${#value}-2}"
    elif [[ "${value}" == \'*\' && "${value}" == *\' ]]; then
      value="${value:1:${#value}-2}"
    fi
  fi
  printf '%s' "${value}"
}

normalize_bool() {
  local value
  value="$(trim "${1-}")"
  value="${value,,}"
  case "${value}" in
    1|true|yes|y|on)
      printf 'true\n'
      ;;
    0|false|no|n|off)
      printf 'false\n'
      ;;
    *)
      ;;
  esac
}

read_traffic_dump_yaml_value() {
  local config_path="${1:?missing config path}"
  local field="${2:?missing field}"
  [[ -f "${config_path}" ]] || return 0

  awk -v want="${field}" '
    function trim(s) {
      sub(/^[[:space:]]+/, "", s)
      sub(/[[:space:]]+$/, "", s)
      return s
    }

    /^[[:space:]]*#/ { next }

    {
      line = $0
      if (in_block && line ~ /^[^[:space:]#][^:]*:[[:space:]]*$/) {
        exit
      }
      if (line ~ /^traffic_dump:[[:space:]]*$/) {
        in_block = 1
        next
      }
      if (!in_block) {
        next
      }
      if (line ~ /^  [A-Za-z0-9_]+:[[:space:]]*/) {
        sub(/^  /, "", line)
        key = line
        sub(/:.*/, "", key)
        if (key != want) {
          next
        }
        sub(/^[^:]+:[[:space:]]*/, "", line)
        sub(/[[:space:]]+#.*$/, "", line)
        print trim(line)
        exit
      }
    }
  ' "${config_path}"
}

resolve_traffic_dump_config() {
  local enabled="false"
  local dump_dir="./dumps"
  local file_path="{{.request_id}}.log"
  local config_path="${ROOT}/onr.yaml"
  local yaml_enabled=""
  local yaml_dir=""
  local yaml_file_path=""
  local normalized=""

  yaml_enabled="$(read_traffic_dump_yaml_value "${config_path}" "enabled")"
  yaml_dir="$(read_traffic_dump_yaml_value "${config_path}" "dir")"
  yaml_file_path="$(read_traffic_dump_yaml_value "${config_path}" "file_path")"

  normalized="$(normalize_bool "${yaml_enabled}")"
  if [[ -n "${normalized}" ]]; then
    enabled="${normalized}"
  fi
  if [[ -n "${yaml_dir}" ]]; then
    dump_dir="$(unquote_scalar "${yaml_dir}")"
  fi
  if [[ -n "${yaml_file_path}" ]]; then
    file_path="$(unquote_scalar "${yaml_file_path}")"
  fi

  normalized="$(normalize_bool "${ONR_TRAFFIC_DUMP_ENABLED:-}")"
  if [[ -n "${normalized}" ]]; then
    enabled="${normalized}"
  fi
  if [[ -n "${ONR_TRAFFIC_DUMP_DIR:-}" ]]; then
    dump_dir="${ONR_TRAFFIC_DUMP_DIR}"
  fi
  if [[ -n "${ONR_TRAFFIC_DUMP_FILE_PATH:-}" ]]; then
    file_path="${ONR_TRAFFIC_DUMP_FILE_PATH}"
  fi

  printf '%s\t%s\t%s\n' "${enabled}" "${dump_dir}" "${file_path}"
}

to_absolute_path() {
  local candidate="${1:?missing path}"
  if command -v realpath >/dev/null 2>&1; then
    realpath -m "${candidate}"
    return
  fi
  if command -v python3 >/dev/null 2>&1; then
    python3 - "${candidate}" <<'PY'
import os
import sys

print(os.path.abspath(sys.argv[1]))
PY
    return
  fi
  printf '%s\n' "${candidate}"
}

join_dump_path() {
  local dump_dir="${1:?missing dump dir}"
  local file_path="${2:?missing file path}"
  if [[ "${dump_dir}" == */ ]]; then
    printf '%s%s\n' "${dump_dir}" "${file_path#/}"
    return
  fi
  printf '%s/%s\n' "${dump_dir}" "${file_path#/}"
}

render_dump_relative_path() {
  local request_id="${1:?missing request id}"
  local template="${2:?missing file path template}"
  local rendered

  rendered="${template//\{\{.request_id\}\}/${request_id}}"
  if [[ "${rendered}" == *"{{"* || "${rendered}" == *"}}"* ]]; then
    return 1
  fi
  printf '%s\n' "${rendered}"
}

resolve_dump_log_output() {
  local request_id="${1-}"
  local rendered=""
  local joined=""

  IFS=$'\t' read -r TRAFFIC_DUMP_ENABLED TRAFFIC_DUMP_DIR TRAFFIC_DUMP_FILE_PATH < <(resolve_traffic_dump_config)

  if [[ "${TRAFFIC_DUMP_ENABLED}" != "true" ]]; then
    printf '(traffic_dump disabled)\n'
    return
  fi
  if [[ -z "${request_id}" ]]; then
    printf '(request_id missing)\n'
    return
  fi

  if ! rendered="$(render_dump_relative_path "${request_id}" "${TRAFFIC_DUMP_FILE_PATH}")"; then
    printf '(unresolved file_path template)\n'
    return
  fi

  joined="$(join_dump_path "${TRAFFIC_DUMP_DIR}" "${rendered}")"
  if [[ "${joined}" != /* ]]; then
    joined="${ROOT}/${joined}"
  fi
  to_absolute_path "${joined}"
}

main() {
  if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
    usage
    exit 0
  fi

  if [[ $# -gt 1 ]]; then
    usage
    exit 2
  fi

  resolve_dump_log_output "${1-}"
}

main "$@"
