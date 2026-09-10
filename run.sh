#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
PORT=8088
BINARY="${TMPDIR:-/tmp}/shelfmate-demo"

command -v go >/dev/null || { printf 'Go is required.\n' >&2; exit 1; }
command -v lsof >/dev/null || { printf 'lsof is required.\n' >&2; exit 1; }

# Stop only ShelfMate listeners, never an unrelated app using this port.
pids="$(lsof -nP -tiTCP:"$PORT" -sTCP:LISTEN || true)"
for pid in $pids; do
  executable="$(ps -p "$pid" -o comm=)"
  case "${executable##*/}" in
    shelfmate|shelfmate-demo) ;;
    *) printf 'Port %s is occupied by %s (PID %s); refusing to stop it.\n' "$PORT" "$executable" "$pid" >&2; exit 1 ;;
  esac
done
for pid in $pids; do
  printf 'Stopping ShelfMate (PID %s)…\n' "$pid"
  kill -TERM "$pid"
done
for ((attempt=0; attempt<50; attempt++)); do
  if ! lsof -nP -tiTCP:"$PORT" -sTCP:LISTEN >/dev/null; then
    break
  fi
  sleep 0.1
done
if lsof -nP -tiTCP:"$PORT" -sTCP:LISTEN >/dev/null; then
  printf 'Port %s is still occupied. Stop the process manually and retry.\n' "$PORT" >&2
  exit 1
fi

cd -- "$ROOT"
printf 'Building ShelfMate…\n'
go build -o "$BINARY" ./cmd/shelfmate
printf 'Starting ShelfMate at http://127.0.0.1:%s (Ctrl+C to stop)\n' "$PORT"
exec "$BINARY" serve -addr "127.0.0.1:$PORT"
