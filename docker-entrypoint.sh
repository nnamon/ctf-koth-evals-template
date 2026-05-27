#!/bin/sh
# Auto-register any bundled challenges on first `serve` startup. Idempotent —
# register-challenge is a no-op when the (name, version) pair already exists.
set -e

if [ "$1" = "serve" ]; then
  mkdir -p "$(dirname "${CTF_EVALS_DB#sqlite://}")" 2>/dev/null || true
  if [ -d /app/challenges ]; then
    for dir in /app/challenges/*/; do
      [ -f "${dir}manifest.json" ] || continue
      /app/ctf-evals register-challenge "$dir" || true
    done
  fi
fi

exec /app/ctf-evals "$@"
