#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'EOF'
Build a Gemini native generateContent request body.

Usage:
  tools/build_gemini_multimodal_body.sh [options]

Options:
  --text <text>       Add a text part. Repeatable.
  --image <path>      Add an image file as inlineData. Repeatable.
  --audio <path>      Add an audio file as inlineData. Repeatable.
  --system <text>     Add systemInstruction text.
  --role <role>       Content role. Default: user.
  --pretty            Pretty-print JSON output.
  -h, --help          Show this help.

  Examples:
  tools/build_gemini_multimodal_body.sh \
    --text 'Describe this image.' \
    --image ./path/to/cat.png

  tools/build_gemini_multimodal_body.sh \
    --text 'Transcribe this audio and summarize it in 3 bullets.' \
    --audio ./path/to/sample.mp3
EOF
  exit 2
}

die() {
  echo "error: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "$1 is required"
}

infer_mime_type_from_extension() {
  local path="${1:?missing path}"

  case "${path##*.}" in
    png) printf 'image/png\n' ;;
    jpg|jpeg) printf 'image/jpeg\n' ;;
    webp) printf 'image/webp\n' ;;
    gif) printf 'image/gif\n' ;;
    mp3) printf 'audio/mpeg\n' ;;
    wav) printf 'audio/wav\n' ;;
    flac) printf 'audio/flac\n' ;;
    m4a|mp4) printf 'audio/mp4\n' ;;
    ogg|oga) printf 'audio/ogg\n' ;;
    webm) printf 'audio/webm\n' ;;
    aac) printf 'audio/aac\n' ;;
    *) return 1 ;;
  esac
}

infer_mime_type() {
  local path="${1:?missing path}"
  local detected=""
  local fallback=""

  if command -v file >/dev/null 2>&1; then
    detected="$(file --brief --mime-type -- "${path}" 2>/dev/null || true)"
  fi

  fallback="$(infer_mime_type_from_extension "${path}" || true)"

  if [[ -n "${detected}" && "${detected}" != "text/plain" && "${detected}" != "application/octet-stream" ]]; then
    printf '%s\n' "${detected}"
    return
  fi

  if [[ -n "${fallback}" ]]; then
    printf '%s\n' "${fallback}"
    return
  fi

  if [[ -n "${detected}" ]]; then
    printf '%s\n' "${detected}"
    return
  fi

  printf 'application/octet-stream\n'
}

encode_file_base64() {
  local path="${1:?missing path}"
  base64 <"${path}" | tr -d '\n'
}

append_text_part() {
  local current="${1:?missing current}"
  local text="${2:?missing text}"
  jq -cn --argjson current "${current}" --arg text "${text}" \
    '$current + [{"text": $text}]'
}

append_inline_data_part() {
  local current="${1:?missing current}"
  local path="${2:?missing path}"
  local expected_prefix="${3:?missing expected prefix}"
  local mime
  local data

  [[ -f "${path}" ]] || die "file not found: ${path}"

  mime="$(infer_mime_type "${path}")"
  if [[ "${mime}" != "${expected_prefix}"/* ]]; then
    die "file ${path} detected as ${mime}, expected ${expected_prefix}/*"
  fi

  data="$(encode_file_base64 "${path}")"
  jq -cn \
    --argjson current "${current}" \
    --arg mime "${mime}" \
    --arg data "${data}" \
    '$current + [{"inlineData": {"mimeType": $mime, "data": $data}}]'
}

main() {
  require_cmd jq
  require_cmd base64

  local role="user"
  local system_text=""
  local pretty="false"
  local parts='[]'

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --text)
        shift
        [[ $# -gt 0 ]] || die "missing value for --text"
        parts="$(append_text_part "${parts}" "$1")"
        shift
        ;;
      --image)
        shift
        [[ $# -gt 0 ]] || die "missing value for --image"
        parts="$(append_inline_data_part "${parts}" "$1" "image")"
        shift
        ;;
      --audio)
        shift
        [[ $# -gt 0 ]] || die "missing value for --audio"
        parts="$(append_inline_data_part "${parts}" "$1" "audio")"
        shift
        ;;
      --system)
        shift
        [[ $# -gt 0 ]] || die "missing value for --system"
        system_text="$1"
        shift
        ;;
      --role)
        shift
        [[ $# -gt 0 ]] || die "missing value for --role"
        role="$1"
        shift
        ;;
      --pretty)
        pretty="true"
        shift
        ;;
      -h|--help)
        usage
        ;;
      *)
        die "unknown argument: $1"
        ;;
    esac
  done

  [[ "${parts}" != "[]" ]] || die "at least one --text, --image, or --audio part is required"

  if [[ "${pretty}" == "true" ]]; then
    jq -n \
      --arg role "${role}" \
      --arg system_text "${system_text}" \
      --argjson parts "${parts}" \
      '
      {
        contents: [
          {
            role: $role,
            parts: $parts
          }
        ]
      }
      + (if $system_text != "" then
          {
            systemInstruction: {
              parts: [
                {text: $system_text}
              ]
            }
          }
        else
          {}
        end)
      '
    return
  fi

  jq -cn \
    --arg role "${role}" \
    --arg system_text "${system_text}" \
    --argjson parts "${parts}" \
    '
    {
      contents: [
        {
          role: $role,
          parts: $parts
        }
      ]
    }
    + (if $system_text != "" then
        {
          systemInstruction: {
            parts: [
              {text: $system_text}
            ]
          }
        }
      else
        {}
      end)
    '
}

main "$@"
