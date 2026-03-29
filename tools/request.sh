#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

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

merge_stream_true_if_needed() {
  local payload="$1"
  if ! command -v jq >/dev/null 2>&1; then
    die "--stream with JSON payload requires jq (install jq or set stream in JSON manually)"
  fi
  printf '%s' "${payload}" | jq -c 'if type == "object" and (has("stream") | not) then . + {"stream": true} else . end' \
    || die "invalid JSON payload"
}

usage() {
  cat >&2 <<'EOF'
Usage:
  tools/request.sh [-X METHOD] [--provider NAME] [--no-auth] [--json JSON | --json-file FILE] [--stream] <path-or-url> [-- curl_args...]

Env (auto source repo .env if exists):
  ONR_BASE_URL    Default: derived from ONR_LISTEN or http://127.0.0.1:3300
  ONR_LISTEN      e.g. :3300
  ONR_ACCESS_KEY_DEFAULT  preferred auth key (Authorization: Bearer <key>)
  ONR_API_KEY             legacy fallback auth key

Examples:
  # Basic
  tools/request.sh /healthz --no-auth
  tools/request.sh /v1/models
  tools/request.sh 'http://127.0.0.1:3300/v1/models' -- -i

  # Text generation / chat
  tools/request.sh /v1/responses --json '{"model":"gpt-5.1-codex-mini","input":"hello"}'
  tools/request.sh /v1/responses --json '{"model":"gpt-5.1-codex-mini","input":"hello"}' --stream
  tools/request.sh /v1/responses --json '{"model":"gpt-5.1-codex-mini", "stream":true, "instructions":"You are helpful.","input":[{"role":"user","content":[{"type":"input_text","text":"reply with exactly OK"}]}]}' --provider codex
  tools/request.sh /v1/chat/completions --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}' --provider openai
  tools/request.sh /v1/chat/completions --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"hi"}]}' --stream
  tools/request.sh /v1/chat/completions --json '{"model":"gpt-4o-mini","messages":[{"role":"user","content":"timeout test"}]}' -- --max-time 15
  tools/request.sh /v1/messages --json '{"model":"claude-haiku-4-5","max_tokens":128,"messages":[{"role":"user","content":"hi"}]}' --provider anthropic
  tools/request.sh /v1/messages --json '{"model":"claude-haiku-4-5","max_tokens":128,"messages":[{"role":"user","content":"hi"}]}' --stream --provider anthropic

  # Embeddings
  tools/request.sh /v1/embeddings --json '{"model":"text-embedding-3-small","input":"hello world"}'

  # Images
  tools/request.sh /v1/images/generations --json '{"model":"gpt-image-1.5","prompt":"a red fox in snow"}'

  # Audio
  tools/request.sh /v1/audio/speech --json '{"model":"gpt-4o-mini-tts","voice":"alloy","input":"hello"}'

  # Gemini native
  tools/request.sh '/v1beta/models/gemini-2.5-flash:generateContent' --json '{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}'
  tools/request.sh '/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse' --stream --json '{"contents":[{"role":"user","parts":[{"text":"hello"}]}]}'

EOF
  exit 2
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

method="GET"
provider=""
no_auth="false"
json=""
json_file=""
stream="false"
target=""
extra_curl_args=()

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
    --json)
      shift
      [[ $# -gt 0 ]] || die "missing value for $0 --json"
      json="$1"
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
    --)
      shift
      # Remaining args go to curl
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
      else
        extra_curl_args+=("$1")
      fi
      shift
      ;;
  esac
done

[[ -n "${target}" ]] || usage

url=""
if [[ "${target}" == http://* || "${target}" == https://* ]]; then
  url="${target}"
else
  base="$(infer_base_url)"
  if [[ "${target}" != /* ]]; then
    target="/${target}"
  fi
  url="${base}${target}"
fi

if [[ "${no_auth}" != "true" ]]; then
  auth_key="${ONR_ACCESS_KEY_DEFAULT:-${ONR_API_KEY:-}}"
  if [[ -n "${auth_key:-}" ]]; then
    auth_header=("Authorization: Bearer ${auth_key}")
  else
    die "ONR_ACCESS_KEY_DEFAULT/ONR_API_KEY is empty (set it in .env or use --no-auth)"
  fi
fi

if [[ -n "${provider}" ]]; then
  provider_header=("x-onr-provider: ${provider}")
fi

if [[ -n "${json}" && -n "${json_file}" ]]; then
  die "use only one of --json or --json-file"
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

data_args=()
headers=()

if [[ -n "${json_file}" ]]; then
  [[ -f "${json_file}" ]] || die "json file not found: ${json_file}"
  headers+=("Content-Type: application/json")
  data_args+=("--data-binary" "@${json_file}")
  if [[ "${method}" == "GET" ]]; then
    method="POST"
  fi
elif [[ -n "${json}" ]]; then
  headers+=("Content-Type: application/json")
  data_args+=("--data-raw" "${json}")
  if [[ "${method}" == "GET" ]]; then
    method="POST"
  fi
fi

curl_args=()
curl_args+=("-sS")
curl_args+=("-X" "${method}")
if [[ "${stream}" == "true" ]]; then
  curl_args+=("-N")
fi
if [[ -n "${auth_header:-}" ]]; then
  curl_args+=("-H" "${auth_header}")
fi
if [[ -n "${provider_header:-}" ]]; then
  curl_args+=("-H" "${provider_header}")
fi
for h in "${headers[@]}"; do
  curl_args+=("-H" "${h}")
done
curl_args+=("${data_args[@]}")

curl "${curl_args[@]}" "${extra_curl_args[@]}" "${url}"
