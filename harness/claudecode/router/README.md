> **Outer plane (Go):** router plane (`server/router`, mount `/router`) + proxy plane (`server/proxy`, mount `/control-plane/proxy`) — both in the `smarsh-cp` monolith on `127.0.0.1:8787`. Tiers: local-llm / sonnet / opus / fable. Ops: `server/router/docs/outer-runbook.md`.

# Claude Code → Anthropic proxy (`/control-plane/proxy`)

Claude Code speaks the **Anthropic Messages API**. Point it at the outer proxy, not at raw vLLM.

## Project `.claude/settings.local.json` (or user settings) env

### Router mode (recommended)

```json
{
  "env": {
    "ANTHROPIC_BASE_URL": "https://axon.compeller.ai/control-plane/proxy",
    "ANTHROPIC_AUTH_TOKEN": "local",
    "ANTHROPIC_API_KEY": "local",
    "ANTHROPIC_MODEL": "auto",
    "ANTHROPIC_DEFAULT_OPUS_MODEL": "auto",
    "ANTHROPIC_DEFAULT_SONNET_MODEL": "auto",
    "ANTHROPIC_DEFAULT_HAIKU_MODEL": "auto:light",
    "ANTHROPIC_SMALL_FAST_MODEL": "auto:light",
    "CLAUDE_CODE_SUBAGENT_MODEL": "auto"
  }
}
```

Prefer the public edge URL above. Local monolith without nginx: `http://127.0.0.1:8787/control-plane/proxy`.

### Session model switches

Claude Code `/model` — pins the tier for the session:

- `auto` — classify each request (routes LIGHT / MEDIUM / HIGH)  
- `auto:light` / `local-llm` / `light` — **local-llm** (LIGHT)  
- `auto:medium` / `sonnet` / `medium` — **claude-sonnet-5** (MEDIUM)  
- `auto:high` / `opus` / `high` — **claude-opus-4-8** (HIGH)  
- `auto:max` / `fable` / `max` — **claude-fable-5** (MAX)  

**MAX is force-only** — `auto` never selects it; reach it with `/model max`. (Legacy `heavy` still maps to HIGH.) Each final reply is footered with the tier that actually served it (e.g. `— HIGH · claude-opus-4-8`), so downgrades are visible.

Full portable ops: `server/router/docs/outer-runbook.md`.

## Also set knowledge planes (orthogonal)

```json
"LIBRARIAN_BASE": "https://axon.compeller.ai/librarian",
"BRAIN_BASE": "https://axon.compeller.ai/continuity"
```

SessionStart Librarian hook: `../hooks/session_start_librarian.sh`.

## Desktop

If Claude Desktop supports a custom Anthropic-compatible base URL, use the same `/control-plane/proxy` settings.  
If not, Desktop uses its own Anthropic account and **skips** this proxy; it can still use Librarian MCP for playbook.
