#!/usr/bin/env bash
# Claude Code Stop hook → bounded Brain assistant response activity.
# Always exits 0 and never reads the local transcript.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=session_visibility_lib.sh
source "${SCRIPT_DIR}/session_visibility_lib.sh"

INPUT="$(cat || true)"
parsed="$(INPUT_JSON="${INPUT}" python3 - <<'PY' 2>/dev/null || true
import json, os
try:
    d = json.loads(os.environ.get("INPUT_JSON") or "{}")
except Exception:
    d = {}
for value in (
    str(d.get("session_id") or "")[:256],
    str(d.get("last_assistant_message") or "")[:32000],
    bool(d.get("stop_hook_active")),
):
    print(json.dumps(value))
PY
)"
if [[ -n "${parsed}" ]]; then
  mapfile -t fields <<<"${parsed}"
  decode() { python3 -c 'import json,sys; sys.stdout.write(str(json.loads(sys.argv[1])))' "$1" 2>/dev/null || true; }
  VIS_HOOK_SESSION_ID="$(decode "${fields[0]:-\"\"}")"
  export VIS_HOOK_SESSION_ID
  response="$(decode "${fields[1]:-\"\"}")"
  stop_hook_active="$(decode "${fields[2]:-false}")"
  if [[ "${stop_hook_active}" != "True" && -n "${response}" ]]; then
    vis_post_activity "assistant.response" "" "" "${response}" '{"metadata":{"role":"assistant","hook_event_name":"Stop"}}'
  fi
fi
exit 0
