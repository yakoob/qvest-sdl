#!/usr/bin/env bash
# Claude Code PreCompact hook — re-inject budgeted session + Brain context.
# Emits additionalContext JSON. Always exit 0.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=session_visibility_lib.sh
source "${SCRIPT_DIR}/session_visibility_lib.sh"

export BRAIN_BASE="${BRAIN_BASE:-https://axon.compeller.ai/continuity}"
VIS_BRAIN_BASE="${BRAIN_BASE}"
VIS_HOOK_SESSION_ID="$(vis_hook_session_id)"
export VIS_HOOK_SESSION_ID

agent="$(vis_resolve_agent)"
sid="$(vis_read_session_id)"

if [[ -n "${sid}" ]]; then
  vis_post_activity "compact" "" "precompact" ""
fi

ctx="$(curl -sS --max-time 8 "${VIS_BRAIN_BASE%/}/brain/agent/${agent}/context" 2>/dev/null || echo '{}')"
act=""
if [[ -n "${sid}" ]]; then
  act="$(curl -sS --max-time 8 -H "X-Agent-Id: ${agent}" -H "X-Harness: ${VIS_HARNESS}" -H "X-Session-Id: ${sid}" \
    "${VIS_BRAIN_BASE%/}/brain/sessions/${sid}/activity?limit=30" 2>/dev/null || echo '{}')"
fi

pack="$(
AGENT="${agent}" SID="${sid}" CTX="${ctx}" ACT="${act}" python3 - <<'PY' 2>/dev/null || true
import json, os
agent = os.environ.get("AGENT") or "koob-claudecode"
sid = os.environ.get("SID") or ""
try:
    ctx = json.loads(os.environ.get("CTX") or "{}")
except Exception:
    ctx = {}
try:
    act = json.loads(os.environ.get("ACT") or "{}")
except Exception:
    act = {}
st = {}
rs = ctx.get("run_state") or {}
if isinstance(rs, dict):
    st = rs.get("state") or {}
lines = [
    f"## PreCompact continuity ({agent})",
    f"session_id: {sid or '(none)'}",
    f"goal: {st.get('goal','')}",
    f"status: {st.get('status','')}",
    f"current_task: {st.get('current_task','')}",
]
ns = st.get("next_steps") or []
if isinstance(ns, list) and ns:
    lines.append("next_steps: " + "; ".join(str(x) for x in ns[:5]))
ev = act.get("events") or []
if isinstance(ev, list) and ev:
    lines.append("recent_activity:")
    for e in ev[-12:]:
        if not isinstance(e, dict):
            continue
        lines.append(f"- [{e.get('kind')}] {e.get('tool_name') or ''} {(e.get('input_redacted') or '')[:120]}")
text = "\n".join(lines)
if len(text) > 6000:
    text = text[:5990] + "\n…"
print(json.dumps({
    "continue": True,
    "suppressOutput": True,
    "hookSpecificOutput": {
        "hookEventName": "PreCompact",
        "additionalContext": text,
    },
}, ensure_ascii=True))
PY
)"

if [[ -n "${pack}" ]]; then
  printf '%s\n' "${pack}"
else
  printf '%s\n' '{"continue":true,"suppressOutput":true}'
fi
exit 0
