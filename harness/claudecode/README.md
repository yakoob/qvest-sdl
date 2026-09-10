> Control plane: **one Go binary — `smarsh-cp`** — serving all six planes (continuity, toolshed, librarian, router, proxy, web) on a single listener `127.0.0.1:8787`. Toolshed calls need agent identity. Brain desktop UI: **Axon** (`clients/axon`).

# Claude Code harness adapter

Claude Code–only. OpenClaw / Codex / Desktop use other trees under `harness/`.

## Quick setup (agent `koob-claudecode`)

The session/activity **hooks ship tracked** in [`.claude/settings.json`](../../.claude/settings.json)
at the repo root — they wire automatically on clone/pull, so the audit tape can't silently
stop being captured because someone forgot to copy a local file. You only add a
**machine-local** file for your env / identity:

From the **agentic-sf repo root** (the directory that contains `harness/` + `clients/`):

```bash
cp harness/claudecode/examples/settings.local.json .claude/settings.local.json
# set your agent lane (identity) — must match env.AGENT_ID
echo koob-claudecode > .agent-id
```

Claude Code layers the two files by disjoint top-level keys: **hooks** from the tracked
`.claude/settings.json`, **env** (incl. `AGENT_ID`) from your gitignored
`.claude/settings.local.json`. No merge ambiguity, and no shared `AGENT_ID` default that
would misattribute activity to one lane.

Open a **new** Claude Code session in this project. SessionStart only fires on `startup|resume`.

## Env (project `.claude/settings.local.json`)

See [`examples/settings.local.json`](examples/settings.local.json). Important keys:

| Key | Smarsh edge default |
|-----|---------------------|
| `AGENT_ID` | `koob-claudecode` |
| `BRAIN_BASE` | `https://axon.compeller.ai/continuity` |
| `LIBRARIAN_BASE` | `https://axon.compeller.ai/librarian` |
| `TOOLSHED_BASE` | `https://axon.compeller.ai/toolshed` |
| `ANTHROPIC_BASE_URL` | `https://axon.compeller.ai/control-plane/proxy` |
| `ANTHROPIC_AUTH_TOKEN` / `ANTHROPIC_API_KEY` | `local` (lab proxy placeholder) |

Model routing: `/model auto` · `auto:light` / `local-llm` · `auto:medium` / `sonnet` · `auto:high` / `opus` · `auto:max` / `fable` (MAX force-only). Details: [`router/README.md`](router/README.md).

## Hooks (all under `hooks/`)

| Event | Script | Purpose |
|-------|--------|---------|
| `SessionStart` (`startup\|resume`) | `session_start_librarian.sh` | Open Brain session + inject Continuity + Librarian + Tool Shed boot pack |
| `UserPromptSubmit` | `session_prompt_hook.sh` | Append a bounded, secret-redacted prompt activity |
| `PostToolUse` | `session_activity_hook.sh` | Append secret-redacted activity tape (`tool.post`) |
| `PostToolUseFailure` | `session_activity_hook.sh fail` | Append secret-redacted activity tape (`tool.fail`) |
| `PreCompact` | `session_precompact_hook.sh` | Re-inject budgeted continuity + recent tape |
| `Stop` | `session_response_hook.sh` | Append the bounded, secret-redacted completed assistant response |
| `SessionEnd` | `session_end_hook.sh` | Auto handoff draft + close Brain session |

Wired via the **tracked** [`.claude/settings.json`](../../.claude/settings.json) (not the
gitignored `.local`), so every checkout captures the audit tape automatically — this is the
fix for "session activity not showing up": a stale local settings file used to carry only
`SessionStart`, so sessions opened but no `tool.post` was ever written.

Shared helpers: `session_visibility_lib.sh` (not wired as a hook).

**Paths are Smarsh layout** — never Compeller’s `agentic/shared/scripts/…`:

```bash
bash "${CLAUDE_PROJECT_DIR}/harness/claudecode/hooks/session_start_librarian.sh"
```

`CLAUDE_PROJECT_DIR` must be the agentic-sf root (tree that contains `harness/` + `clients/`).

Session id cache: `~/.cache/smarsh/agent-sessions/{agent}-claude-code.session_id`.
Local adapter health: `~/.cache/smarsh/agent-sessions/{agent}-claude-code.health.json` records only operation timestamps and a bounded error category—never request bodies or credentials. Brain writes are bounded and best-effort, so an edge outage never blocks Claude Code. Prompt, tool, and assistant response fields are redacted before truncation. The adapter records only the completed `last_assistant_message` supplied to the `Stop` hook; it never reads or uploads Claude Code transcript files. Context and handoff requests carry the Brain session ID, agent ID, and harness so the Continuity API can attribute real node reads and writes in the session's **Nodes touched** section.

After changing `AGENT_ID` or `.agent-id`, start a new Claude Code session. The identity is part of the Brain owner ACL and the local cache filename; changing it mid-session intentionally starts a separate lane.

## Manual checks

```bash
# scripts exist + executable
ls -la harness/claudecode/hooks/

# boot pack (non-blocking even if edge down)
AGENT_ID=koob-claudecode bash harness/claudecode/hooks/session_start_librarian.sh | head -c 600

# librarian only
./clients/cli/librarian_client.sh --agent koob-claudecode boot | head -c 400

# brain context
curl -sS -H 'X-Agent-Id: koob-claudecode' "$BRAIN_BASE/brain/agent/koob-claudecode/context" | head -c 400

# current session, recent tape, and local health
sid="$(cat "$HOME/.cache/smarsh/agent-sessions/koob-claudecode-claude-code.session_id")"
curl -sS -H 'X-Agent-Id: koob-claudecode' "$BRAIN_BASE/brain/sessions/$sid" | jq '{id,agent_id,harness,harness_session_key,status}'
curl -sS -H 'X-Agent-Id: koob-claudecode' "$BRAIN_BASE/brain/sessions/$sid/activity?limit=30" | jq '.events[] | {seq,kind,tool_name}'
jq . "$HOME/.cache/smarsh/agent-sessions/koob-claudecode-claude-code.health.json"
```

## Do not

- Copy Compeller `.claude/settings.local.json` wholesale (wrong hosts, `AGENT_ID=bae`, wrong hook paths under `agentic/shared/…`).
- Point hooks at `agentic/shared/scripts/session_start_librarian.sh` — that path does not exist in this repo.
- Put `env` / `AGENT_ID` in the tracked `.claude/settings.json` — identity is **per-operator** and belongs only in the gitignored `.local`; a shared `AGENT_ID` would attribute everyone's audit tape to one lane.
- Re-add a `hooks` block to `.claude/settings.local.json` — hooks now live only in the tracked `.claude/settings.json`; duplicating them risks double-posting activity.
