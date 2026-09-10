#!/usr/bin/env bash
# Claude Code SessionStart hook — Smarsh boot parity pack.
#
# Injects into additionalContext:
#   1) Continuity (Brain) run_state for this agent_id
#   2) OKF Librarian playbook boot
#   3) Tool Shed health (tool_count + names)
#
# Wired from tracked .claude/settings.json → hooks.SessionStart
# Env: LIBRARIAN_BASE, BRAIN_BASE, TOOLSHED_BASE, AGENT / AGENT_ID, CLAUDE_PROJECT_DIR
#
# Always exits 0 so a plane blip never blocks session start.

set -u

PROJECT_DIR="${CLAUDE_PROJECT_DIR:-}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
# shellcheck source=session_visibility_lib.sh
source "${SCRIPT_DIR}/session_visibility_lib.sh"
VIS_HOOK_SESSION_ID="$(vis_hook_session_id)"
export VIS_HOOK_SESSION_ID
if [[ -z "${PROJECT_DIR}" || ! -d "${PROJECT_DIR}" ]]; then
  # hooks/ → claudecode/ → harness/ → repo root
  PROJECT_DIR="$(cd "$SCRIPT_DIR/../../.." && pwd)"
fi

export LIBRARIAN_BASE="${LIBRARIAN_BASE:-https://axon.compeller.ai/librarian}"
export BRAIN_BASE="${BRAIN_BASE:-https://axon.compeller.ai/continuity}"
export TOOLSHED_BASE="${TOOLSHED_BASE:-https://axon.compeller.ai/toolshed}"

# --- Agent visibility: open session on Brain ---
VIS_LIB="${PROJECT_DIR}/harness/claudecode/hooks/session_visibility_lib.sh"
if [[ -f "${VIS_LIB}" ]]; then
  # shellcheck source=session_visibility_lib.sh
  source "${VIS_LIB}"
  VIS_SESSION_ID="$(vis_open_session || true)"
else
  VIS_SESSION_ID=""
fi

# Resolve agent_id: AGENT / AGENT_ID / project .agent-id / default koob-claudecode
resolve_agent() {
  if [[ -n "${AGENT:-}" ]]; then
    echo "${AGENT}" | tr -d '[:space:]'
    return
  fi
  if [[ -n "${AGENT_ID:-}" ]]; then
    echo "${AGENT_ID}" | tr -d '[:space:]'
    return
  fi
  if [[ -f "${PROJECT_DIR}/.agent-id" ]]; then
    tr -d '[:space:]' <"${PROJECT_DIR}/.agent-id"
    return
  fi
  echo "koob-claudecode"
}

AGENT_ID="$(resolve_agent)"
if [[ -z "${AGENT_ID}" ]]; then
  AGENT_ID="koob-claudecode"
fi
export AGENT="${AGENT_ID}"
export AGENT_ID

MAX_CHARS="${LIBRARIAN_BOOT_MAX_CHARS:-10000}"
TMP="$(mktemp -t okf-boot.XXXXXX 2>/dev/null || mktemp)"
ERR="$(mktemp -t okf-boot-err.XXXXXX 2>/dev/null || mktemp)"
CTX_FILE="$(mktemp -t brain-ctx.XXXXXX 2>/dev/null || mktemp)"
TH_FILE="$(mktemp -t toolshed.XXXXXX 2>/dev/null || mktemp)"
PACK="$(mktemp -t boot-pack.XXXXXX 2>/dev/null || mktemp)"
cleanup() { rm -f "$TMP" "$ERR" "$CTX_FILE" "$TH_FILE" "$PACK" "${PACK}.brain" "${PACK}.lib" "${PACK}.th" 2>/dev/null || true; }
trap cleanup EXIT

# --- Continuity summary (small) ---
context_headers=(-H "X-Agent-Id: ${AGENT_ID}" -H "X-Harness: ${VIS_HARNESS}")
[[ -n "${VIS_SESSION_ID:-}" ]] && context_headers+=(-H "X-Session-Id: ${VIS_SESSION_ID}")
agent_path="$(AGENT_VALUE="${AGENT_ID}" python3 -c 'import os, urllib.parse; print(urllib.parse.quote(os.environ["AGENT_VALUE"], safe=""))')"
curl -sS --max-time 6 "${context_headers[@]}" "${BRAIN_BASE%/}/brain/agent/${agent_path}/context" -o "$CTX_FILE" 2>/dev/null || true
python3 - "$CTX_FILE" "$AGENT_ID" "$BRAIN_BASE" >"${PACK}.brain" <<'PY'
import json, sys
from pathlib import Path
path, agent_id, brain_base = sys.argv[1], sys.argv[2], sys.argv[3]
print("## Continuity (Brain history)")
print(f"agent_id: **`{agent_id}`** · `{brain_base}`")
print("")
p = Path(path)
if not p.exists() or p.stat().st_size == 0:
    print("**DEGRADED:** Brain unreachable. Continue; retry context when edge is up.")
    print("")
    sys.exit(0)
try:
    d = json.loads(p.read_text(encoding="utf-8", errors="replace"))
except Exception as e:
    print(f"**DEGRADED:** Brain parse failed: {e}")
    print("")
    sys.exit(0)
st = ((d.get("run_state") or {}).get("state") or {})
if not d.get("has_state") and not st:
    print("No active run_state.")
else:
    print(f"- **Goal:** {st.get('goal') or 'none'}")
    print(f"- **Status:** {st.get('status') or 'none'}")
    print(f"- **Current task:** {st.get('current_task') or 'none'}")
    blockers = st.get("blockers") or []
    print(f"- **Blockers:** {', '.join(blockers) if blockers else 'none'}")
    ns = st.get("next_steps") or []
    if ns:
        print("- **Next steps:**")
        for s in ns[:8]:
            print(f"  - {s}")
ev = d.get("recent_events") or []
if ev:
    print("")
    print("### Recent events")
    for e in ev[:6]:
        print(f"- [{e.get('event_type','?')}] {e.get('title','')} ({str(e.get('created_at',''))[:16]})")
print("")
print("Write path: atomic decision/blocker/milestone · handoff at session end. Do not store OKF playbook in Brain.")
print("")
PY

# --- Librarian playbook ---
STATUS="ok"
CLIENT="${PROJECT_DIR}/clients/cli/librarian_client.sh"
if [[ -f "$CLIENT" ]]; then
  if AGENT_ID="$AGENT_ID" LIBRARIAN_BASE="$LIBRARIAN_BASE" bash "$CLIENT" boot "$AGENT_ID" >"$TMP.raw" 2>"$ERR"; then
    if python3 - "$TMP.raw" >"$TMP" 2>>"$ERR" <<'PY'
import sys, json
from pathlib import Path
raw = Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace")
try:
    d = json.loads(raw)
except Exception as e:
    print("Librarian boot JSON parse failed: " + str(e))
    sys.exit(1)
md = d.get("prompt_markdown") or ""
if not md:
    lines = ["# Session boot (OKF Librarian)", ""]
    pb = d.get("playbook") or {}
    for section in ("identity", "policy", "feedback_rules", "feedback"):
        for x in pb.get(section) or []:
            if not isinstance(x, dict):
                continue
            lines.append("## " + (x.get("title") or x.get("id") or section))
            lines.append((x.get("text") or x.get("summary") or "")[:1500])
            lines.append("")
    md = "\n".join(lines)
print(md)
PY
    then
      STATUS="cli"
    else
      STATUS="fallback"
    fi
  else
    STATUS="fallback"
  fi
else
  STATUS="missing-client"
fi

if [[ ! -s "$TMP" ]]; then
  if command -v curl >/dev/null 2>&1; then
    if curl -sS --max-time 12 "${LIBRARIAN_BASE%/}/v1/boot?agent_id=${AGENT_ID}" 2>>"$ERR" \
      | python3 -c '
import sys, json
try:
    d = json.load(sys.stdin)
except Exception as e:
    print("Librarian boot JSON parse failed: " + str(e))
    sys.exit(1)
print(d.get("prompt_markdown") or json.dumps({"agent": d.get("agent_id"), "counts": {
    "identity": len((d.get("playbook") or {}).get("identity") or []),
    "policy": len((d.get("playbook") or {}).get("policy") or []),
    "feedback": len((d.get("playbook") or {}).get("feedback_rules") or []),
}}, indent=2))
' >"$TMP" 2>>"$ERR"; then
      STATUS="rest"
    else
      STATUS="error"
    fi
  else
    STATUS="error"
  fi
fi

{
  echo "## OKF Librarian boot (SessionStart, source=${STATUS})"
  echo "LIBRARIAN_BASE=${LIBRARIAN_BASE} · agent_id=\`${AGENT_ID}\`"
  echo ""
  if [[ ! -s "$TMP" ]]; then
    echo "OKF Librarian SessionStart: unavailable (${STATUS})."
    echo "stderr: $(head -c 400 "$ERR" 2>/dev/null || true)"
    echo ""
    echo "Continue with Brain continuity. Manual:"
    echo "  ./clients/cli/librarian_client.sh --agent ${AGENT_ID} boot"
  else
    python3 - "$TMP" "$MAX_CHARS" <<'PY'
import sys
path, max_chars = sys.argv[1], int(sys.argv[2])
text = open(path, encoding="utf-8", errors="replace").read()
if len(text) > max_chars:
    text = text[:max_chars] + f"\n\n…[truncated at {max_chars} chars — run librarian_client.sh boot for full]\n"
sys.stdout.write(text)
if not text.endswith("\n"):
    sys.stdout.write("\n")
PY
  fi
  echo ""
} >"${PACK}.lib"

# --- Tool Shed ---
curl -sS --max-time 5 "${TOOLSHED_BASE%/}/health" -o "$TH_FILE" 2>/dev/null || true
python3 - "$TH_FILE" "$TOOLSHED_BASE" >"${PACK}.th" <<'PY'
import json, sys
from pathlib import Path
path, base = sys.argv[1], sys.argv[2]
print("## Tool Shed")
print(f"`{base}`")
print("")
p = Path(path)
if not p.exists() or p.stat().st_size == 0:
    print("**DEGRADED:** Tool Shed unreachable.")
    print("")
    sys.exit(0)
try:
    d = json.loads(p.read_text(encoding="utf-8", errors="replace"))
except Exception as e:
    print(f"**DEGRADED:** parse failed: {e}")
    print("")
    sys.exit(0)
tools = d.get("tools") or []
print(f"- status: {d.get('status')}")
print(f"- tool_count: {len(tools)}")
if tools:
    print(f"- tools: {', '.join(tools)}")
print("")
print("Toolshed calls need agent identity (X-Agent / agent envelope).")
print("")
PY

# --- Assemble ---
{
  echo "# Smarsh session boot pack"
  echo ""
  echo "Harness: **Claude Code** · agent_id: **\`${AGENT_ID}\`**"
  echo "Ritual: Continuity → Librarian → Tool Shed"
  echo "Docs: \`harness/claudecode/README.md\` · edge: axon.compeller.ai"
  if [[ -n "${VIS_SESSION_ID:-}" ]]; then
    echo "Visibility session_id: **\`${VIS_SESSION_ID}\`** (activity tape on Brain)"
  else
    echo "Visibility session_id: _(not opened — Brain sessions API may be down)_"
  fi
  echo ""
  cat "${PACK}.brain"
  cat "${PACK}.lib"
  cat "${PACK}.th"
  echo "## Identity reminder"
  echo ""
  echo "- This session is **\`${AGENT_ID}\`** on the Smarsh agentic-sf lane."
  echo "- Compeller lab (bae/lisa) is a separate control plane — do not cross-write."
  echo "- If Continuity above is empty/degraded, re-query Brain before assuming state."
  echo ""
} >"$PACK"

# Persist the same bounded, redacted representation that is hashed. The store's
# per-session stable hash makes repeated SessionStart delivery idempotent.
if [[ -n "${VIS_SESSION_ID:-}" ]]; then
  BOOT_CAPTURE="$(vis_redact_json "$(cat "$PACK")" 14000)"
  BOOT_HASH="$(printf '%s' "$BOOT_CAPTURE" | sha256sum | cut -d' ' -f1)"
  vis_post_activity "system.init" "" "" "$BOOT_CAPTURE" "{\"hash\":\"system.init:$BOOT_HASH\",\"metadata\":{\"hook_event_name\":\"SessionStart\",\"capture\":\"bounded-redacted-boot\"}}"
fi

# Emit SessionStart hook payload
python3 - "$PACK" <<'PY'
import json, sys
from pathlib import Path
ctx = Path(sys.argv[1]).read_text(encoding="utf-8", errors="replace")
# Keep tab/lf/cr; drop other C0 controls for strict JSON consumers
ctx = "".join(ch if (ord(ch) >= 32 or ch in "\t\n\r") else " " for ch in ctx)
if len(ctx) > 14000:
    ctx = ctx[:14000] + "\n\n…[boot pack truncated]\n"
json.dump({
    "continue": True,
    "suppressOutput": True,
    "hookSpecificOutput": {
        "hookEventName": "SessionStart",
        "additionalContext": ctx,
    },
}, sys.stdout, ensure_ascii=True)
sys.stdout.write("\n")
PY

exit 0
