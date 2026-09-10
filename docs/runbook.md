# Runbook (PoC)

Local desk laptop. Not a production deploy.

## One command

```bash
go test ./...
./run.sh
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

Serve does **not** append session checkouts or session-backed recommendations to that file. Demo circulation is process memory. Restart restores `catalog.json` / `circulation.json` / `students.json`. CLI `recommend` still honors `SHELFMATE_AUDIT`.

## Demo checkout

```bash
# after serve is up
curl -s http://127.0.0.1:8088/api/health
curl -s -X POST http://127.0.0.1:8088/api/checkouts \
  -H 'Content-Type: application/json' \
  -d '{"student_id":"S-406","book_id":"B-007","staff_id":"L-002","retry_id":"demo-1"}'
```

409 means the title is already out, copies are 0, or returning it would exceed `copies_total`. Refresh the student. There is no reset URL.

## Demo students

| ID | Who | Point |
|----|-----|--------|
| S-406 | Mateo | Cat Kid; Bad Guys out of copies |
| S-402 | Aisha | Fantasy cluster; stretch is grade policy |
| S-405 | Priya | Sparse history + under-150 query |
| S-509 | Olivia | Zero history fallback |
| S-305 | Sofia | Synthetic illustrative improving case (same-form 2→3; English B then B+) |

Staff: Elena L-001, Tom L-002, Priya Shah L-003.

## Code walkthrough (10 minutes)

1. `internal/retrieve/hybrid.go` — CF + TF-IDF, evidence labels, fallback.
2. `internal/policy/filter.go` — catalog, copies, grade/stretch, already-read, page caps.
3. `internal/explain/` — TemplateExplainer; `axon.go` mock-tested.
4. `internal/audit/log.go` — mutex JSONL, no raw query.
5. `web/app.js` — lookup, checkout/return, revision guard, copy drafts.
6. `internal/session` — in-memory snapshot checkout/return.
7. `internal/academics` + `data/json/academic_demo.json` — synthetic grades; not ranking.
8. `testdata/golden/cases.json` — executable goldens (`notes`, not a `show` field).

## Support and progress walkthrough

1. Select Tyler (`S-504`, search by name/ID) or filter **Check in first**. Expand **Why this band?**: the explicit teacher request triggers prompt attention. The C grade and fictional reading check are separately shown.
2. Inspect strengths and counselor-approved reading themes. Click **Explore sports** to confirm a neutral local catalog preference. Raw teacher/counselor text is not copied into the query or model payload.
3. Check out an available book. The loan and copy count change; academic results and support band do not.
4. Record a check-in only if it happened. The dated follow-up is process-local and separate from academic evidence.
5. Open **Reading & learning**: letter-grade categories, separate assessment grade forms, and observed borrowing counts. Hover/focus marks or expand the data tables. Partial exports are not complete reading rates; borrowed/returned does not mean read/finished.
6. Shortcut **Sofia · illustrative (S-305)** is the labeled improving case (same-form reading check 2→3; English B then existing B+). It does not change ranking or the latest support band. Mateo/Aisha remain the stable/missing comparators; Priya/Olivia stay insufficient-evidence.
7. Compare Aisha (`S-402`, no current flags), Mateo (`S-406`, check in soon), and Priya/Olivia (insufficient evidence). Missing records never become zeros or an inactivity penalty.
8. Restart with `./run.sh`: loans, copy counts, activity and follow-ups reset. Base JSON is unchanged.

Support rules in `internal/support/evaluate.go` are **unvalidated synthetic demo rules**, reviewed as of 2026-09-04—not a clinical screen or failure prediction. The UI has no real authentication/RBAC; staff choice is attribution only. Counselor data is deliberately shared reading guidance, not clinical records. Live Axon remains unverified; mock failure/privacy tests cover the integration.

## Rehearsal if the model is down

Keep LLM off. The Friday (or any) talk still has retrieve, policy, console, and eval.
