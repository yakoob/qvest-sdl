#!/usr/bin/env bash
# Claude Code SessionEnd / Stop hook — close visibility session + draft handoff.
# Always exit 0.
set -u

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=session_visibility_lib.sh
source "${SCRIPT_DIR}/session_visibility_lib.sh"

export BRAIN_BASE="${BRAIN_BASE:-https://axon.compeller.ai/continuity}"
VIS_BRAIN_BASE="${BRAIN_BASE}"
VIS_HOOK_SESSION_ID="$(vis_hook_session_id)"
export VIS_HOOK_SESSION_ID

agent="$(vis_resolve_agent)"
project="${CLAUDE_PROJECT_DIR:-$(pwd)}"

branch="$(git -C "${project}" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")"
head="$(git -C "${project}" rev-parse --short HEAD 2>/dev/null || echo "")"
dirty="$(git -C "${project}" status --porcelain 2>/dev/null | head -40 || true)"
dirty_n="$(printf '%s\n' "${dirty}" | grep -c . || true)"

sid="$(vis_read_session_id)"
context_headers=(-H "X-Agent-Id: ${agent}" -H "X-Harness: ${VIS_HARNESS}")
[[ -n "${sid}" ]] && context_headers+=(-H "X-Session-Id: ${sid}")
agent_path="$(AGENT_VALUE="${agent}" python3 -c 'import os, urllib.parse; print(urllib.parse.quote(os.environ["AGENT_VALUE"], safe=""))')"
ctx="$(curl -sS --max-time 8 "${context_headers[@]}" "${VIS_BRAIN_BASE%/}/brain/agent/${agent_path}/context" 2>/dev/null || echo '{}')"

handoff_body="$(
CTX_JSON="${ctx}" BRANCH="${branch}" HEAD_SHA="${head}" DIRTY_N="${dirty_n}" PROJECT="${project}" python3 - <<'PY' 2>/dev/null || true
import json, os
ctx = {}
try:
    ctx = json.loads(os.environ.get("CTX_JSON") or "{}")
except Exception:
    pass
st = {}
rs = ctx.get("run_state") or {}
if isinstance(rs, dict):
    st = rs.get("state") or rs
    if not isinstance(st, dict):
        st = {}
goal = st.get("goal") or "session (auto handoff)"
status = st.get("status") or "Session ended (auto)"
task = st.get("current_task") or "Session closed by SessionEnd hook"
issue = st.get("current_issue") or ""
blockers = st.get("blockers") if isinstance(st.get("blockers"), list) else []
next_steps = st.get("next_steps") if isinstance(st.get("next_steps"), list) else []
decisions = st.get("recent_decisions") if isinstance(st.get("recent_decisions"), list) else []
artifacts = []
if isinstance(st.get("context"), dict) and isinstance(st["context"].get("artifacts"), list):
    artifacts = list(st["context"]["artifacts"])
branch = os.environ.get("BRANCH") or ""
head = os.environ.get("HEAD_SHA") or ""
proj = os.environ.get("PROJECT") or ""
dirty_n = os.environ.get("DIRTY_N") or "0"
if branch or head:
    artifacts.append(f"git {branch}@{head} dirty={dirty_n} (auto SessionEnd)")
if proj:
    artifacts.append(f"project {proj}")
artifacts = artifacts[-12:]
print(json.dumps({
    "goal": goal,
    "status": status,
    "current_task": task,
    "current_issue": issue,
    "blockers": blockers,
    "next_steps": next_steps,
    "recent_decisions": decisions + ["SessionEnd auto-handoff (visibility plane)"],
    "context": {"artifacts": artifacts},
}))
PY
)"

if [[ -n "${handoff_body}" ]]; then
  handoff_headers=(-H "Content-Type: application/json" -H "X-Agent-Id: ${agent}" -H "X-Harness: ${VIS_HARNESS}")
  [[ -n "${sid}" ]] && handoff_headers+=(-H "X-Session-Id: ${sid}")
  curl -sS --max-time 10 -X POST "${VIS_BRAIN_BASE%/}/brain/agent/${agent_path}/handoff" \
    "${handoff_headers[@]}" \
    -d "${handoff_body}" >/dev/null 2>&1 || true
fi

vis_close_session "closed"
exit 0
