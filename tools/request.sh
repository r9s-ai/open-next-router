#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Usage:
  tools/request.sh [METHOD] <path-or-url> [json_body] [options]

Env (auto source repo .env if exists):
  ONR_BASE_URL    Default: derived from ONR_LISTEN or http://127.0.0.1:3300
  ONR_LISTEN      e.g. :3300
  ONR_ACCESS_KEY_DEFAULT  preferred auth key (Authorization: Bearer <key>)
  ONR_API_KEY             legacy fallback auth key

Gemini shortcut options:
  --gemini-text <text>    Add a Gemini text part. Repeatable.
  --gemini-image <path>   Add a Gemini image part via inlineData. Repeatable.
  --gemini-audio <path>   Add a Gemini audio part via inlineData. Repeatable.
  --gemini-system <text>  Set Gemini systemInstruction text.
  --gemini-role <role>    Set Gemini content role. Default: user.

Examples:
  # Basic
  tools/request.sh /healthz --no-auth
  tools/request.sh /v1/models
  tools/request.sh 'http://127.0.0.1:3300/v1/models' -- -i

  # Text generation / chat
  tools/request.sh /v1/responses '{"model":"gpt-5.1-codex-mini","input":"hello"}'
  tools/request.sh /v1/responses '{"model":"gpt-5.1-codex-mini","input":"hello"}' --stream
  tools/request.sh /v1/responses '{"model":"gpt-5.1-codex-mini", "stream":true, "instructions":"You are helpful.","input":[{"role":"user","content":[{"type":"input_text","text":"reply with exactly OK"}]}]}' --provider codex
  tools/request.sh /v1/chat/completions '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}' --provider openai
  tools/request.sh /v1/chat/completions '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}' --stream
  tools/request.sh /v1/chat/completions '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"timeout test"}]}' -- --max-time 15
  tools/request.sh /v1/messages '{"model":"claude-haiku-4-5","max_tokens":128,"messages":[{"role":"user","content":"hi"}]}' --provider anthropic
  tools/request.sh /v1/messages '{"model":"claude-haiku-4-5","max_tokens":128,"messages":[{"role":"user","content":"hi"}]}' --stream --provider anthropic

  # Embeddings
  tools/request.sh /v1/embeddings '{"model":"text-embedding-3-small","input":"hello world"}'

  # Images
  tools/request.sh /v1/images/generations '{"model":"gpt-image-1-mini","prompt":"a red fox in snow"}'

  # Audio
  tools/request.sh /v1/audio/speech '{"model":"gpt-4o-mini-tts","voice":"alloy","input":"hello"}'

  # Gemini native
  tools/request.sh '/v1beta/models/gemini-2.5-flash:generateContent' '{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}'
  tools/request.sh '/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse' '{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}'
  tools/request.sh '/v1beta/models/gemini-2.5-flash:generateContent' --gemini-text 'Describe this audio.' --gemini-audio ./path/to/sample.mp3
  tools/request.sh '/v1beta/models/gemini-2.5-flash:generateContent' --gemini-text 'Compare the image and audio.' --gemini-image ./path/to/cat.png --gemini-audio ./path/to/sample.mp3

EOF
  exit 2
}

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GET_REQUEST_DUMP_BY_ID="${ROOT}/tools/get_request_dump_by_id.sh"
BUILD_GEMINI_MULTIMODAL_BODY="${ROOT}/tools/build_gemini_multimodal_body.sh"

if [[ -f "${ROOT}/.env" ]]; then
  set -a
  # shellcheck source=/dev/null
  source "${ROOT}/.env"
  set +a
fi

die() {
  echo "error: $*" >&2
  exit 1
}

require_jq() {
  command -v jq >/dev/null 2>&1 || die "jq is required for this operation"
}

merge_stream_true_if_needed() {
  local payload="$1"
  require_jq
  printf '%s' "${payload}" | jq -c 'if type == "object" and (has("stream") | not) then . + {"stream": true} else . end' \
    || die "invalid JSON payload"
}

infer_base_url() {
  if [[ -n "${ONR_BASE_URL:-}" ]]; then
    echo "${ONR_BASE_URL}"
    return
  fi
  local listen="${ONR_LISTEN:-:3300}"
  listen="$(echo "${listen}" | xargs)"
  if [[ "${listen}" == http://* || "${listen}" == https://* ]]; then
    echo "${listen}"
    return
  fi
  if [[ "${listen}" == :* ]]; then
    echo "http://127.0.0.1${listen}"
    return
  fi
  echo "http://${listen}"
}

extract_header_value() {
  local file="${1:?missing file}"
  local name="${2:?missing name}"
  local want

  want="${name,,}"
  awk -v want="${want}" '
    /^HTTP\// {
      current = ""
      next
    }
    /^[[:space:]]*$/ {
      if (current != "") {
        last = current
      }
      next
    }
    {
      key = $1
      gsub(/:$/, "", key)
      gsub(/\r/, "", key)
      if (tolower(key) != want) {
        next
      }
      value = $0
      sub(/^[^:]+:[[:space:]]*/, "", value)
      gsub(/\r/, "", value)
      current = value
    }
    END {
      if (current != "") {
        last = current
      }
      printf "%s", last
    }
  ' "${file}"
}

extract_status_code() {
  local headers_file="${1:?missing headers file}"
  awk '
    /^HTTP\// {
      status = $2
    }
    END {
      printf "%s", status
    }
  ' "${headers_file}"
}

extract_request_id() {
  local headers_file="${1:?missing headers file}"
  local request_id=""

  request_id="$(extract_header_value "${headers_file}" "X-Onr-Request-Id")"
  if [[ -n "${request_id}" ]]; then
    printf '%s\n' "${request_id}"
    return
  fi
  extract_header_value "${headers_file}" "X-Request-Id"
}

resolve_dump_log_output() {
  if [[ ! -x "${GET_REQUEST_DUMP_BY_ID}" ]]; then
    printf '(helper missing: %s)\n' "${GET_REQUEST_DUMP_BY_ID}"
    return
  fi
  "${GET_REQUEST_DUMP_BY_ID}" "${1-}"
}

is_binary_content_type() {
  local content_type="${1:-}"
  [[ "${content_type}" == audio/* ]] || \
    [[ "${content_type}" == image/* ]] || \
    [[ "${content_type}" == video/* ]] || \
    [[ "${content_type}" == application/octet-stream* ]]
}

body_contains_nul() {
  local file="${1:?missing file}"
  python3 - "$file" <<'PY'
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
data = path.read_bytes()
sys.exit(0 if b"\x00" in data else 1)
PY
}

print_response_body() {
  local body_file="${1:?missing body file}"
  local headers_file="${2:?missing headers file}"
  local content_type=""

  content_type="$(extract_header_value "${headers_file}" "Content-Type")"

  if is_binary_content_type "${content_type}" || body_contains_nul "${body_file}"; then
    printf '[binary response omitted: content-type=%s, bytes=%s]\n' \
      "${content_type:-unknown}" \
      "$(wc -c <"${body_file}" | tr -d ' ')"
    return
  fi

  if command -v jq >/dev/null 2>&1 && [[ "${content_type}" == application/json* ]]; then
    jq -C . <"${body_file}" || cat "${body_file}"
    return
  fi

  cat "${body_file}"
}

init_defaults() {
  method="GET"
  provider=""
  no_auth="false"
  json=""
  positional_json=""
  json_file=""
  stream="false"
  target=""
  url=""
  auth_header=()
  provider_header=()
  headers=()
  data_args=()
  curl_args=()
  extra_curl_args=()
  gemini_texts=()
  gemini_images=()
  gemini_audios=()
  gemini_system=""
  gemini_role="user"
  tmp_headers=""
  tmp_body=""
  curl_exit=0
}

parse_args() {
  # Parse args in flexible order:
  # - options can appear before/after target
  # - first non-option token is treated as <path-or-url>
  # - remaining unknown tokens are forwarded to curl
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -h|--help)
        usage
        ;;
      -X|--method)
        shift
        [[ $# -gt 0 ]] || die "missing value for $0 -X/--method"
        method="$1"
        shift
        ;;
      --provider)
        shift
        [[ $# -gt 0 ]] || die "missing value for $0 --provider"
        provider="$1"
        shift
        ;;
      --no-auth)
        no_auth="true"
        shift
        ;;
      --json-file)
        shift
        [[ $# -gt 0 ]] || die "missing value for $0 --json-file"
        json_file="$1"
        shift
        ;;
      --stream)
        stream="true"
        shift
        ;;
      --gemini-text)
        shift
        [[ $# -gt 0 ]] || die "missing value for $0 --gemini-text"
        gemini_texts+=("$1")
        shift
        ;;
      --gemini-image)
        shift
        [[ $# -gt 0 ]] || die "missing value for $0 --gemini-image"
        gemini_images+=("$1")
        shift
        ;;
      --gemini-audio)
        shift
        [[ $# -gt 0 ]] || die "missing value for $0 --gemini-audio"
        gemini_audios+=("$1")
        shift
        ;;
      --gemini-system)
        shift
        [[ $# -gt 0 ]] || die "missing value for $0 --gemini-system"
        gemini_system="$1"
        shift
        ;;
      --gemini-role)
        shift
        [[ $# -gt 0 ]] || die "missing value for $0 --gemini-role"
        gemini_role="$1"
        shift
        ;;
      --)
        shift
        while [[ $# -gt 0 ]]; do
          extra_curl_args+=("$1")
          shift
        done
        break
        ;;
      GET|POST|PUT|PATCH|DELETE|HEAD|OPTIONS)
        method="$1"
        shift
        ;;
      *)
        if [[ -z "${target}" ]]; then
          target="$1"
        elif [[ -z "${positional_json}" && -z "${json_file}" ]]; then
          positional_json="$1"
        else
          extra_curl_args+=("$1")
        fi
        shift
        ;;
    esac
  done
}

validate_inputs() {
  [[ -n "${target}" ]] || usage

  local gemini_part_count=0
  gemini_part_count=$((${#gemini_texts[@]} + ${#gemini_images[@]} + ${#gemini_audios[@]}))

  if [[ -n "${positional_json}" && -n "${json_file}" ]]; then
    die "use only one of positional json_body or --json-file"
  fi

  if [[ "${gemini_part_count}" -gt 0 ]] && [[ -n "${positional_json}" || -n "${json_file}" ]]; then
    die "use either JSON input or --gemini-* shortcut options, not both"
  fi

  if [[ "${gemini_part_count}" -eq 0 ]] && [[ -n "${gemini_system}" || "${gemini_role}" != "user" ]]; then
    die "--gemini-system/--gemini-role requires at least one --gemini-text, --gemini-image, or --gemini-audio"
  fi

  if [[ -n "${positional_json}" ]]; then
    json="${positional_json}"
  fi
}

build_url() {
  if [[ "${target}" == http://* || "${target}" == https://* ]]; then
    url="${target}"
    return
  fi

  local base
  base="$(infer_base_url)"
  if [[ "${target}" != /* ]]; then
    target="/${target}"
  fi
  url="${base}${target}"
}

prepare_auth_header() {
  auth_header=()
  if [[ "${no_auth}" == "true" ]]; then
    return
  fi

  local auth_key="${ONR_ACCESS_KEY_DEFAULT:-${ONR_API_KEY:-}}"
  if [[ -z "${auth_key}" ]]; then
    die "ONR_ACCESS_KEY_DEFAULT/ONR_API_KEY is empty (set it in .env or use --no-auth)"
  fi
  auth_header=("Authorization: Bearer ${auth_key}")
}

prepare_provider_header() {
  provider_header=()
  if [[ -n "${provider}" ]]; then
    provider_header=("x-onr-provider: ${provider}")
  fi
}

build_gemini_shortcut_payload() {
  local cmd=("bash" "${BUILD_GEMINI_MULTIMODAL_BODY}" "--role" "${gemini_role}")
  local item

  [[ -f "${BUILD_GEMINI_MULTIMODAL_BODY}" ]] || die "helper not found: ${BUILD_GEMINI_MULTIMODAL_BODY}"

  if [[ -n "${gemini_system}" ]]; then
    cmd+=("--system" "${gemini_system}")
  fi
  for item in "${gemini_texts[@]}"; do
    cmd+=("--text" "${item}")
  done
  for item in "${gemini_images[@]}"; do
    cmd+=("--image" "${item}")
  done
  for item in "${gemini_audios[@]}"; do
    cmd+=("--audio" "${item}")
  done

  "${cmd[@]}"
}

prepare_payload() {
  headers=()
  data_args=()

  if [[ $((${#gemini_texts[@]} + ${#gemini_images[@]} + ${#gemini_audios[@]})) -gt 0 ]]; then
    json="$(build_gemini_shortcut_payload)"
  fi

  if [[ "${stream}" == "true" ]]; then
    if [[ -n "${json}" ]]; then
      json="$(merge_stream_true_if_needed "${json}")"
    elif [[ -n "${json_file}" ]]; then
      [[ -f "${json_file}" ]] || die "json file not found: ${json_file}"
      json="$(merge_stream_true_if_needed "$(cat "${json_file}")")"
      json_file=""
    fi
  fi

  if [[ -n "${json_file}" ]]; then
    [[ -f "${json_file}" ]] || die "json file not found: ${json_file}"
    headers+=("Content-Type: application/json")
    data_args+=("--data-binary" "@${json_file}")
    if [[ "${method}" == "GET" ]]; then
      method="POST"
    fi
    return
  fi

  if [[ -n "${json}" ]]; then
    headers+=("Content-Type: application/json")
    data_args+=("--data-raw" "${json}")
    if [[ "${method}" == "GET" ]]; then
      method="POST"
    fi
  fi
}

build_curl_args() {
  curl_args=("-sS" "-X" "${method}")

  if [[ "${stream}" == "true" ]]; then
    curl_args+=("-N")
  fi
  if [[ ${#auth_header[@]} -gt 0 ]]; then
    curl_args+=("-H" "${auth_header[0]}")
  fi
  if [[ ${#provider_header[@]} -gt 0 ]]; then
    curl_args+=("-H" "${provider_header[0]}")
  fi
  for h in "${headers[@]}"; do
    curl_args+=("-H" "${h}")
  done
  curl_args+=("${data_args[@]}")
}

cleanup() {
  rm -f "${tmp_headers:-}" "${tmp_body:-}"
}

execute_request() {
  tmp_headers="$(mktemp)"
  tmp_body="$(mktemp)"
  trap cleanup EXIT

  set +e
  if [[ "${stream}" == "true" ]]; then
    curl "${curl_args[@]}" -D "${tmp_headers}" -o >(tee "${tmp_body}") "${extra_curl_args[@]}" "${url}"
    curl_exit=$?
  else
    curl "${curl_args[@]}" -D "${tmp_headers}" -o "${tmp_body}" "${extra_curl_args[@]}" "${url}"
    curl_exit=$?
  fi
  set -e
}

print_response_summary() {
  local status_code=""
  status_code="$(extract_status_code "${tmp_headers}")"
  printf '=> %s %s (%s)\n' "${method}" "${url}" "${status_code:-unknown}"

  if [[ "${stream}" != "true" ]]; then
    print_response_body "${tmp_body}" "${tmp_headers}"
  fi
}

print_request_metadata() {
  local request_id=""
  request_id="$(extract_request_id "${tmp_headers}")"
  printf 'request_id: %s\n' "${request_id}" >&2
  printf 'dump_log: %s\n' "$(resolve_dump_log_output "${request_id}")" >&2
}

main() {
  init_defaults
  parse_args "$@"
  validate_inputs
  build_url
  prepare_auth_header
  prepare_provider_header
  prepare_payload
  build_curl_args
  execute_request
  print_response_summary
  print_request_metadata

  if [[ "${curl_exit}" -ne 0 ]]; then
    exit "${curl_exit}"
  fi
}

main "$@"
