#!/usr/bin/env bash
# Point .claude/settings.json at a named customer pack.
# Usage: use-customer.sh compeller|smarsh
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../../.." && pwd)"
CLAUDE="$ROOT/.claude"
NAME="${1:-}"

usage() {
  echo "usage: $0 compeller|smarsh" >&2
  exit 2
}

case "$NAME" in
  compeller|smarsh) ;;
  *) usage ;;
esac

src="$CLAUDE/settings.$NAME.json"
dst="$CLAUDE/settings.json"
local="$CLAUDE/settings.local.json"

if [[ ! -f "$src" ]]; then
  echo "missing $src" >&2
  exit 1
fi

cp -f "$src" "$dst"
python3 - "$local" "$NAME" <<'PY'
import json, pathlib, sys
path = pathlib.Path(sys.argv[1])
customer = sys.argv[2]
data = {}
if path.exists():
    try:
        data = json.loads(path.read_text())
    except Exception:
        data = {}
if not isinstance(data, dict):
    data = {}
env = data.get("env")
if not isinstance(env, dict):
    env = {}
env["AXON_CUSTOMER"] = customer
if customer == "compeller":
    env.setdefault("AXON_CUSTOMER_CP", "compeller-cp")
    env.setdefault("AXON_AGENT", "lisa")
else:
    env.setdefault("AXON_CUSTOMER_CP", "smarsh-cp")
    env.setdefault("AXON_AGENT", "ada")
data["env"] = env
path.write_text(json.dumps(data, indent=2) + "\n")
PY

echo "active $dst <- settings.$NAME.json"
echo "local  $local AXON_CUSTOMER=$NAME"
echo "restart Claude Code so env reloads"
