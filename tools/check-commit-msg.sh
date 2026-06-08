#!/usr/bin/env sh
set -eu

if [ "$#" -lt 1 ]; then
  echo "commit message file is required" >&2
  exit 2
fi

msg_file=$1
subject=$(sed -n '1p' "$msg_file" | tr -d '\r')

types='build|chore|ci|docs|feat|fix|perf|refactor|revert|style|test'
pattern="^(${types})(\\([a-z0-9._/-]+\\))?!?: [A-Za-z0-9][ -~]*$"

if [ -z "$subject" ]; then
  echo "commit subject must not be empty" >&2
  exit 1
fi

if ! printf '%s\n' "$subject" | LC_ALL=C grep -Eq '^[ -~]+$'; then
  echo "commit subject must be English ASCII only" >&2
  echo "example: feat: add provider validation" >&2
  exit 1
fi

non_ascii_line=$(sed '/^[[:space:]]*#/d' "$msg_file" | tr -d '\r' | LC_ALL=C grep -n '[^ -~]' | head -n 1 || true)
if [ -n "$non_ascii_line" ]; then
  echo "commit message must be English ASCII only" >&2
  echo "first non-ASCII line: $non_ascii_line" >&2
  exit 1
fi

if ! printf '%s\n' "$subject" | LC_ALL=C grep -Eq "$pattern"; then
  echo "commit subject must match Conventional Commits format" >&2
  echo "example: feat: add provider validation" >&2
  echo "allowed types: build, chore, ci, docs, feat, fix, perf, refactor, revert, style, test" >&2
  exit 1
fi
