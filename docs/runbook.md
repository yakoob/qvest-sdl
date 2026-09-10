# Runbook (PoC)

Local desk laptop. Not a production deploy.

## One command

```bash
go test ./...
go run ./cmd/shelfmate serve
```

Console: http://127.0.0.1:8088

CLI:

```bash
go run ./cmd/shelfmate recommend -student S-406
go run ./cmd/shelfmate recommend -student S-402
go run ./cmd/shelfmate recommend -student S-402 -stretch
go run ./cmd/shelfmate recommend -student S-405 -query "funny, reluctant 4th, under 150 pages, we have copies"
go run ./cmd/shelfmate recommend -student S-509
```

Default listen address is `127.0.0.1:8088`. A bare `-addr :8088` is rewritten to localhost.

## Axon (optional)

Default off. Recs do not change when the model is missing.

```bash
export SHELFMATE_LLM=on
export LLM_BASE=http://127.0.0.1:8791/router   # client appends /v1/chat/completions
export LLM_MODEL=your-model
export LLM_API_KEY=                           # optional; never commit
```

Kill switch: unset `SHELFMATE_LLM` or set `SHELFMATE_LLM=off`.

Audit (optional path):

```bash
export SHELFMATE_AUDIT=/tmp/shelfmate-audit.jsonl
```

Failed audit writes set `audit_error` on the response; they are not swallowed.

## Demo students

| ID | Who | Point |
|----|-----|--------|
| S-406 | Mateo | Cat Kid; Bad Guys out of copies |
| S-402 | Aisha | Fantasy cluster; stretch is grade policy |
| S-405 | Priya | Sparse history + under-150 query |
| S-509 | Olivia | Zero history fallback |

Staff: Elena L-001, Tom L-002, Priya Shah L-003.

## Code walkthrough (10 minutes)

1. `internal/retrieve/hybrid.go` — CF + TF-IDF, evidence labels, fallback.
2. `internal/policy/filter.go` — catalog, copies, grade/stretch, already-read, page caps.
3. `internal/explain/` — TemplateExplainer; `axon.go` mock-tested.
4. `internal/audit/log.go` — mutex JSONL, no raw query.
5. `web/app.js` — lookup, stale-request guard, copy drafts.
6. `testdata/golden/cases.json` — executable goldens (`notes`, not a `show` field).

## Rehearsal if the model is down

Keep LLM off. The Friday (or any) talk still has retrieve, policy, console, and eval.
