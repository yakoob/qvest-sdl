#!/usr/bin/env bash
# Claude Code UserPromptSubmit hook → bounded Brain prompt activity.
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
for value in (str(d.get("session_id") or "")[:256], str(d.get("prompt") or "")[:16000]):
    print(json.dumps(value))
PY
)"
if [[ -n "${parsed}" ]]; then
  mapfile -t fields <<<"${parsed}"
  decode() { python3 -c 'import json,sys; sys.stdout.write(str(json.loads(sys.argv[1])))' "$1" 2>/dev/null || true; }
  VIS_HOOK_SESSION_ID="$(decode "${fields[0]:-\"\"}")"
  export VIS_HOOK_SESSION_ID
  prompt="$(decode "${fields[1]:-\"\"}")"
  [[ -n "${prompt}" ]] && vis_post_activity "prompt" "" "${prompt}" "" '{"metadata":{"role":"user","hook_event_name":"UserPromptSubmit"}}'
fi
exit 0
