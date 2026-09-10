#!/usr/bin/env bash
# Claude Code PostToolUse / PostToolUseFailure hook → Brain activity tape.
# Always exits 0. Reads documented hook JSON fields from stdin.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=session_visibility_lib.sh
source "${SCRIPT_DIR}/session_visibility_lib.sh"

INPUT="$(cat || true)"
KIND="tool.post"
if [[ "${AXON_HOOK_EVENT:-${COMPELLER_HOOK_EVENT:-}}" == *"Failure"* ]] || [[ "${1:-}" == "fail" ]]; then
  KIND="tool.fail"
fi

parsed="$(INPUT_JSON="${INPUT}" python3 - <<'PY' 2>/dev/null || true
import json, os
try:
    d = json.loads(os.environ.get("INPUT_JSON") or "{}")
except Exception:
    d = {}
tool = d.get("tool_name") or ""
inp = d.get("tool_input")
out = d.get("tool_response")
if out is None:
    out = d.get("error") or ""
sid = str(d.get("session_id") or "")[:256]
extras = {"metadata": {"hook_event_name": d.get("hook_event_name") or None}}
for source, target in (("duration_ms", "duration_ms"), ("exit_code", "exit_code"), ("tool_use_id", "hash")):
    value = d.get(source)
    if value is not None:
        extras[target] = value
files = []
if isinstance(inp, dict):
    for key in ("file_path", "notebook_path"):
        value = inp.get(key)
        if isinstance(value, str) and value:
            files.append(value[:1024])
if files:
    extras["files"] = files
def dump(value):
    if value is None:
        return ""
    if isinstance(value, (dict, list)):
        return json.dumps(value, default=str, separators=(",", ":"))
    return str(value)
for value in (sid, str(tool), dump(inp), dump(out), json.dumps(extras, default=str, separators=(",", ":"))):
    print(json.dumps(value))
PY
)"

if [[ -n "${parsed}" ]]; then
  mapfile -t fields <<<"${parsed}"
  decode() { python3 -c 'import json,sys; sys.stdout.write(str(json.loads(sys.argv[1])))' "$1" 2>/dev/null || true; }
  VIS_HOOK_SESSION_ID="$(decode "${fields[0]:-\"\"}")"
  export VIS_HOOK_SESSION_ID
  tool_name="$(decode "${fields[1]:-\"\"}")"
  input_r="$(decode "${fields[2]:-\"\"}")"
  output_r="$(decode "${fields[3]:-\"\"}")"
  extras="$(decode "${fields[4]}")"
  vis_post_activity "${KIND}" "${tool_name}" "${input_r}" "${output_r}" "${extras}"
fi
exit 0
