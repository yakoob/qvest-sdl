#!/usr/bin/env bash
# Shared helpers for Smarsh agent visibility (session registry + activity tape).
# Sourced by session hooks — not invoked directly as a Claude hook.

set -u

VIS_BRAIN_BASE="${BRAIN_BASE:-https://axon.compeller.ai/continuity}"
VIS_HARNESS="${AXON_HARNESS:-${COMPELLER_HARNESS:-claude-code}}"
VIS_CACHE_DIR="${AXON_SESSION_CACHE:-${COMPELLER_SESSION_CACHE:-${HOME}/.cache/smarsh/agent-sessions}}"
VIS_DEFAULT_AGENT="${AXON_DEFAULT_AGENT:-koob-claudecode}"
VIS_HOOK_SESSION_ID="${VIS_HOOK_SESSION_ID:-}"

vis_resolve_agent() {
  if [[ -n "${AGENT:-}" ]]; then
    echo "${AGENT}" | tr -d '[:space:]'
    return
  fi
  if [[ -n "${AGENT_ID:-}" ]]; then
    echo "${AGENT_ID}" | tr -d '[:space:]'
    return
  fi
  local project_dir="${CLAUDE_PROJECT_DIR:-}"
  if [[ -n "${project_dir}" && -f "${project_dir}/.agent-id" ]]; then
    tr -d '[:space:]' <"${project_dir}/.agent-id"
    return
  fi
  if [[ -n "${project_dir}" ]]; then
    local slug hash_dir
    slug="$(printf '%s' "${project_dir}" | sed 's|/|-|g; s|^-||')"
    hash_dir="${HOME}/.claude/projects/-${slug}"
    if [[ -f "${hash_dir}/.agent-id" ]]; then
      tr -d '[:space:]' <"${hash_dir}/.agent-id"
      return
    fi
  fi
  echo "${VIS_DEFAULT_AGENT}"
}

vis_cache_path() {
  local agent="$1" suffix="$2"
  mkdir -p -m 700 "${VIS_CACHE_DIR}" 2>/dev/null || mkdir -p "${VIS_CACHE_DIR}"
  printf '%s/%s-%s.%s' "${VIS_CACHE_DIR}" "${agent}" "${VIS_HARNESS}" "${suffix}"
}

vis_session_file() { vis_cache_path "$1" session_id; }
vis_health_file() { vis_cache_path "$1" health.json; }

vis_read_session_id() {
  local agent f
  agent="$(vis_resolve_agent)"
  f="$(vis_session_file "${agent}")"
  [[ -f "${f}" ]] && tr -d '[:space:]' <"${f}"
}

vis_write_session_id() {
  local f
  f="$(vis_session_file "$1")"
  (umask 077; printf '%s\n' "$2" >"${f}")
}

vis_clear_session_id() { rm -f "$(vis_session_file "$1")"; }

vis_health() {
  local operation="$1" result="$2" category="${3:-}" agent file
  agent="$(vis_resolve_agent)"
  file="$(vis_health_file "${agent}")"
  VIS_OPERATION="${operation}" VIS_RESULT="${result}" VIS_CATEGORY="${category}" VIS_HEALTH_FILE="${file}" python3 - <<'PY' 2>/dev/null || true
import json, os
from datetime import datetime, timezone
path = os.environ["VIS_HEALTH_FILE"]
try:
    with open(path, encoding="utf-8") as f:
        state = json.load(f)
except Exception:
    state = {}
now = datetime.now(timezone.utc).isoformat()
op = os.environ.get("VIS_OPERATION", "unknown")[:32]
result = os.environ.get("VIS_RESULT", "failure")
state["last_attempt_at"] = now
state["last_operation"] = op
if result == "success":
    state["last_success_at"] = now
    state["last_success_operation"] = op
    state["last_error_category"] = None
else:
    state["last_failure_at"] = now
    state["last_error_category"] = (os.environ.get("VIS_CATEGORY") or "request_failed")[:80]
tmp = path + ".tmp"
with open(tmp, "w", encoding="utf-8") as f:
    json.dump(state, f, separators=(",", ":"))
os.chmod(tmp, 0o600)
os.replace(tmp, path)
PY
}

vis_hook_session_id() {
  local raw
  raw="$(cat || true)"
  HOOK_INPUT="${raw}" python3 - <<'PY' 2>/dev/null || true
import json, os
try:
    data = json.loads(os.environ.get("HOOK_INPUT") or "{}")
except Exception:
    data = {}
print(str(data.get("session_id") or "")[:256])
PY
}

vis_provider_stamp() {
  local base model route provider="unknown"
  base="${ANTHROPIC_BASE_URL:-}"
  model="${ANTHROPIC_MODEL:-${CLAUDE_CODE_SUBAGENT_MODEL:-}}"
  route="${base}"
  if [[ "${base}" == *"/control-plane/proxy"* ]] || [[ "${base}" == *"/proxy"* ]]; then
    provider="anthropic-proxy"
  elif [[ "${base}" == *"router"* ]]; then
    provider="model-router"
  elif [[ -n "${base}" ]]; then
    provider="anthropic-compatible"
  fi
  printf '%s|%s|%s' "${provider}" "${model}" "${route}"
}

vis_redact_json() {
  VIS_RAW="${1:-}" VIS_LIMIT="${2:-8000}" python3 - <<'PY' 2>/dev/null || true
import json, os, re
raw = os.environ.get("VIS_RAW") or ""
limit = max(0, int(os.environ.get("VIS_LIMIT") or "8000"))
secret_key = re.compile(r"(^|[_-])(api[_-]?key|auth|authorization|cookie|credential|password|passwd|secret|token)([_-]|$)", re.I)
patterns = [
    (re.compile(r"(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]+"), r"\1 [REDACTED]"),
    (re.compile(r"(?i)(https?://[^\s:/]+:)[^@\s/]+@"), r"\1[REDACTED]@"),
    (re.compile(r"(?i)\b(ANTHROPIC_API_KEY|ANTHROPIC_AUTH_TOKEN|API_KEY|AUTH_TOKEN|PASSWORD|SECRET|TOKEN)=([^\s]+)"), r"\1=[REDACTED]"),
]
def clean(value, key=""):
    if secret_key.search(str(key)):
        return "[REDACTED]"
    if isinstance(value, dict):
        return {str(k): clean(v, str(k)) for k, v in value.items()}
    if isinstance(value, list):
        return [clean(v) for v in value]
    if isinstance(value, str):
        for pattern, replacement in patterns:
            value = pattern.sub(replacement, value)
        return value
    return value
try:
    value = json.loads(raw)
except Exception:
    value = raw
value = clean(value)
text = json.dumps(value, ensure_ascii=False, separators=(",", ":")) if not isinstance(value, str) else value
print(text[:limit], end="")
PY
}

vis_headers() {
  local agent="$1" sid="${2:-}"
  printf '%s\n' "X-Agent-Id: ${agent}" "X-Harness: ${VIS_HARNESS}"
  [[ -n "${sid}" ]] && printf '%s\n' "X-Session-Id: ${sid}"
}

vis_open_session() {
  local agent project cwd branch head stamp provider model route body resp sid rest hook_sid sess_status
  agent="$(vis_resolve_agent)"
  project="${CLAUDE_PROJECT_DIR:-$(pwd)}"
  cwd="$(pwd 2>/dev/null || echo "${project}")"
  branch="$(git -C "${project}" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
  head="$(git -C "${project}" rev-parse --short HEAD 2>/dev/null || true)"
  stamp="$(vis_provider_stamp)"
  provider="${stamp%%|*}"; rest="${stamp#*|}"; model="${rest%%|*}"; route="${rest#*|}"
  hook_sid="${VIS_HOOK_SESSION_ID:0:256}"
  body="$(VIS_AGENT="${agent}" VIS_PROJECT="${project}" VIS_CWD="${cwd}" VIS_BRANCH="${branch}" VIS_HEAD="${head}" VIS_PROVIDER="${provider}" VIS_MODEL="${model}" VIS_ROUTE="${route}" VIS_VENDOR_SESSION="${hook_sid}" python3 - <<'PY' 2>/dev/null)" || body=""
import json, os
project = os.environ.get("VIS_PROJECT", "")
print(json.dumps({
  "agent_id": os.environ.get("VIS_AGENT", ""),
  "harness": os.environ.get("VIS_HARNESS", "claude-code"),
  "harness_session_key": os.environ.get("VIS_VENDOR_SESSION") or None,
  "project_root": project,
  "cwd": os.environ.get("VIS_CWD", ""),
  "git_branch": os.environ.get("VIS_BRANCH", ""),
  "git_head": os.environ.get("VIS_HEAD", ""),
  "provider": os.environ.get("VIS_PROVIDER", ""),
  "model": os.environ.get("VIS_MODEL", ""),
  "proxy_route": os.environ.get("VIS_ROUTE", ""),
  "metadata": {"source": "session_start_hook", "lane": "smarsh", "claude_session_id": os.environ.get("VIS_VENDOR_SESSION") or None},
}))
PY
  [[ -z "${body}" ]] && { vis_health open failure serialize_failed; return 0; }
  if ! resp="$(curl --fail -sS --connect-timeout 3 --max-time 8 -X POST "${VIS_BRAIN_BASE%/}/brain/sessions" -H "Content-Type: application/json" -H "X-Agent-Id: ${agent}" -H "X-Harness: ${VIS_HARNESS}" -d "${body}" 2>/dev/null)"; then
    vis_health open failure request_failed
    return 0
  fi
  sid="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("id", ""))' <<<"${resp}" 2>/dev/null || true)"
  if [[ -n "${sid}" ]]; then
    sess_status="$(python3 -c 'import json,sys; print(json.load(sys.stdin).get("status", "open"))' <<<"${resp}" 2>/dev/null || true)"
    if [[ "${sess_status}" != "open" ]]; then
      # Brain CreateSession is idempotent on (agent, harness, harness_session_key)
      # and hands back the existing terminal session; PATCH it open so the tape
      # continues on the same session instead of 4xxing every later post.
      if curl --fail -sS --connect-timeout 3 --max-time 8 -X PATCH "${VIS_BRAIN_BASE%/}/brain/sessions/${sid}" -H "Content-Type: application/json" -H "X-Agent-Id: ${agent}" -H "X-Harness: ${VIS_HARNESS}" -H "X-Session-Id: ${sid}" -d '{"status":"open"}' >/dev/null 2>&1; then
        vis_health open success
      else
        vis_health open failure reopen_failed
        return 0
      fi
    fi
    vis_write_session_id "${agent}" "${sid}"
    vis_health open success
    printf '%s' "${sid}"
  else
    vis_health open failure invalid_response
  fi
}

VIS_ACTIVITY_CODE=""
VIS_ACTIVITY_BODY=""

vis_activity_post() {
  local agent="$1" sid="$2" body="$3" out
  out="$(curl -sS --connect-timeout 3 --max-time 5 -X POST "${VIS_BRAIN_BASE%/}/brain/sessions/${sid}/activity" \
    -H "Content-Type: application/json" -H "X-Agent-Id: ${agent}" -H "X-Harness: ${VIS_HARNESS}" -H "X-Session-Id: ${sid}" \
    -d "${body}" -w '\n%{http_code}' 2>/dev/null || true)"
  VIS_ACTIVITY_BODY="${out%%$'\n'*}"
  VIS_ACTIVITY_CODE="${out##*$'\n'}"
  [[ "${VIS_ACTIVITY_CODE}" =~ ^[0-9]{3}$ ]] || VIS_ACTIVITY_CODE="000"
}

vis_post_activity() {
  local kind="$1" tool_name="${2:-}" input_r="${3:-}" output_r="${4:-}" extras="${5-}" agent sid code swept body provider_stamp provider model rest
  [[ -z "${extras}" ]] && extras='{}'
  agent="$(vis_resolve_agent)"; sid="$(vis_read_session_id)"
  [[ -z "${sid}" ]] && sid="$(vis_open_session || true)"
  [[ -z "${sid}" ]] && return 0
  input_r="$(vis_redact_json "${input_r}" 8000)"
  output_r="$(vis_redact_json "${output_r}" 16000)"
  provider_stamp="$(vis_provider_stamp)"; provider="${provider_stamp%%|*}"; rest="${provider_stamp#*|}"; model="${rest%%|*}"
  body="$(VIS_KIND="${kind}" VIS_TOOL="${tool_name}" VIS_IN="${input_r}" VIS_OUT="${output_r}" VIS_EXTRAS="${extras}" VIS_PROVIDER="${provider}" VIS_MODEL="${model}" python3 - <<'PY' 2>/dev/null)" || body=""
import json, os
try:
    extra = json.loads(os.environ.get("VIS_EXTRAS") or "{}")
except Exception:
    extra = {}
secret_key = __import__("re").compile(r"(^|[_-])(api[_-]?key|auth|authorization|cookie|credential|password|passwd|secret|token)([_-]|$)", __import__("re").I)
def safe(value, key=""):
    if secret_key.search(str(key)):
        return "[REDACTED]"
    if isinstance(value, dict):
        return {str(k): safe(v, str(k)) for k, v in value.items()}
    if isinstance(value, list):
        return [safe(v) for v in value]
    return value
extra = safe(extra)
body = {
  "kind": os.environ.get("VIS_KIND", "tool.post"),
  "tool_name": os.environ.get("VIS_TOOL") or None,
  "input_redacted": os.environ.get("VIS_IN") or None,
  "output_redacted": os.environ.get("VIS_OUT") or None,
  "provider": os.environ.get("VIS_PROVIDER") or None,
  "model": os.environ.get("VIS_MODEL") or None,
  "metadata": {"harness": "claude-code", "source": "claude-code-hook"},
}
for key in ("files", "exit_code", "duration_ms", "hash"):
    if key in extra and extra[key] is not None:
        body[key] = extra[key]
if isinstance(extra.get("metadata"), dict):
    body["metadata"].update(extra["metadata"])
print(json.dumps(body, default=str))
PY
  [[ -z "${body}" ]] && { vis_health activity failure serialize_failed; return 0; }
  vis_activity_post "${agent}" "${sid}" "${body}"
  code="${VIS_ACTIVITY_CODE}"
  if [[ "${code}" == 2* ]]; then
    vis_health activity success
    return 0
  fi
  swept=0
  if [[ "${code}" == 4* ]]; then
    swept=1
  elif [[ "${code}" == 5* && "${VIS_ACTIVITY_BODY}" == *"session is"* ]]; then
    # Version skew: servers before the 409 mapping report terminal sessions
    # as 500 with a "session is closed|abandoned" body. Recoverable too.
    swept=1
  fi
  if [[ "${swept}" != 1 ]]; then
    vis_health activity failure request_failed
    return 0
  fi
  # Session is terminal (idle-swept, closed, or agent mismatch): drop the cached
  # id, record why, and retry once on a freshly opened session instead of
  # losing the rest of the day to repeated rejections (PC-0686 follow-up).
  vis_clear_session_id "${agent}"
  vis_health activity failure session_swept
  sid="$(vis_open_session || true)"
  [[ -z "${sid}" ]] && return 0
  vis_activity_post "${agent}" "${sid}" "${body}"
  code="${VIS_ACTIVITY_CODE}"
  if [[ "${code}" == 2* ]]; then
    vis_health activity success
  else
    vis_health activity failure request_failed
  fi
}

vis_close_session() {
  local sess_status="${1:-closed}" agent sid
  agent="$(vis_resolve_agent)"; sid="$(vis_read_session_id)"
  [[ -z "${sid}" ]] && return 0
  if curl --fail -sS --connect-timeout 3 --max-time 8 -X PATCH "${VIS_BRAIN_BASE%/}/brain/sessions/${sid}" -H "Content-Type: application/json" -H "X-Agent-Id: ${agent}" -H "X-Harness: ${VIS_HARNESS}" -H "X-Session-Id: ${sid}" -d "{\"status\":\"${sess_status}\"}" >/dev/null 2>&1; then
    vis_health close success
    vis_clear_session_id "${agent}"
  else
    vis_health close failure request_failed
  fi
}
