#!/usr/bin/env bash
set -euo pipefail

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "golangci-lint is required but was not found in PATH" >&2
  exit 1
fi

args=(run --timeout=3m --allow-serial-runners)

if [[ "$#" -eq 0 ]]; then
  exec golangci-lint "${args[@]}"
fi

run_all=false
package_dirs=()
skipped_dirs=()

should_lint_dir() {
  local dir="${1#./}"
  case "${dir}" in
    onr-lsp|onr-lsp/*)
      return 1
      ;;
  esac
  return 0
}

for path in "$@"; do
  path="${path#./}"
  case "${path}" in
    go.mod|go.sum|go.work|go.work.sum)
      run_all=true
      ;;
    onr-core/go.mod|onr-core/go.sum)
      package_dirs+=("./onr-core/...")
      ;;
    *.go)
      dir="$(dirname "${path}")"
      if ! should_lint_dir "${dir}"; then
        skipped_dirs+=("./${dir}")
      elif [[ "${dir}" == "." ]]; then
        package_dirs+=(".")
      elif [[ -d "${dir}" ]]; then
        package_dirs+=("./${dir}")
      fi
      ;;
  esac
done

if [[ "${run_all}" == "true" ]]; then
  exec golangci-lint "${args[@]}"
fi

if [[ "${#package_dirs[@]}" -eq 0 ]]; then
  if [[ "${#skipped_dirs[@]}" -gt 0 ]]; then
    echo "golangci-lint skipped: changed Go files are outside the root prek lint scope"
  else
    echo "golangci-lint skipped: no existing Go package directories in changed files"
  fi
  exit 0
fi

packages=()
while IFS= read -r package; do
  packages+=("${package}")
done < <(printf '%s\n' "${package_dirs[@]}" | sort -u)

exec golangci-lint "${args[@]}" "${packages[@]}"
