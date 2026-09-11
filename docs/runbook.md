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

Default off. Recs do not change when the model is missing. Classification and enjoy-picks use `auto:medium` (MEDIUM / Qwen 3.8 on the Axon router). Ranking stays local TF-IDF; invented `book_id`s are dropped.

Claude Code proxy (Anthropic Messages):

```bash
export SHELFMATE_LLM=on
export ANTHROPIC_BASE_URL=https://axon.compeller.ai/control-plane/proxy
export LLM_MODEL=auto:medium
export ANTHROPIC_AUTH_TOKEN=local           # never commit
```

OpenAI-compatible router:

```bash
export SHELFMATE_LLM=on
export LLM_BASE=http://127.0.0.1:8791/router   # client appends /v1/chat/completions
export LLM_MODEL=your-model
export LLM_API_KEY=                           # optional; never commit
```

Kill switch: unset `SHELFMATE_LLM` or set `SHELFMATE_LLM=off`. Live Axon is not called from tests.

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
| S-406 | Mateo | Cat Kid recs; isolated improving scenario C-→A- (2→11 checkouts); operational spring A- / Above grade |
| S-402 | Aisha | Fantasy cluster; stretch is grade policy; strong-stable scenario |
| S-405 | Priya | Sparse history + under-150 query; empty local-history windows |
| S-509 | Olivia | Zero history fallback; empty local-history windows |
| S-305 | Sofia | Primary synthetic story: six 84-day windows / three years, 2→10 checkouts, D→B+ |
| S-504 | Tyler | Check in first; isolated improving scenario D+→A (1→9 checkouts); operational spring C |

Staff: Elena L-001, Tom L-002, Priya Shah L-003.

## Code walkthrough (10 minutes)

1. `internal/retrieve/hybrid.go` — CF + TF-IDF, evidence labels, fallback, in-process `Search` catalog index.
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
5. Open **Reading & learning**: exact letter grades (plus/minus labeled), separate assessment grade forms, and observed borrowing. Hover/focus marks or expand the data tables. Incomplete extract coverage is under details, not the primary story.
6. Shortcut **Sofia · illustrative (S-305)** is the primary labeled story: six 84-day matched windows over three completed years, checkouts 2→3→5→6→8→10, English D → D+ → C- → C → B- → B+. Isolated from operational loans, ranking, and support. Latest support band stays No current flags. Mateo and Tyler have distinct improving isolated scenarios; Aisha remains a strong-stable comparator; Priya/Olivia stay insufficient-evidence with empty local-history windows.
7. Compare Aisha (`S-402`, no current flags), Mateo (`S-406`, Above grade), and Priya/Olivia (insufficient evidence). Missing records never become zeros or an inactivity penalty.
8. Restart with `./run.sh`: loans, copy counts, activity and follow-ups reset. Base JSON is unchanged.

Support rules in `internal/support/evaluate.go` are **unvalidated synthetic demo rules**, reviewed as of 2026-09-04—not a clinical screen or failure prediction. The UI has no real authentication/RBAC; staff choice is attribution only. Counselor data is deliberately shared reading guidance, not clinical records. Live Axon remains unverified; mock failure/privacy tests cover the integration.

## Complete librarian support loop (session only)

Use **My day · Students · Outcomes**. The existing Books/Support/Progress tabs are preserved.

1. In My day, review Needs attention; open a student or choose Schedule. All students remain discoverable in Students, even without academics.
2. Scheduling uses `America/Los_Angeles`, whole-minute times, declared shifts minus structured circulation/class/club blocks, closures, early closing, and existing appointments. Suggestions are not guaranteed free time: confirm staff/student availability and prose-only duties before saving. Internal bookings send no notifications.
3. On a future open school day, choose Elena and a suggested slot (Friday 2026-09-11 at 09:00 is a fixture example). Reschedule/cancel keep the appointment ID and append trace events. Booked conversations can be started now from My day or the student card.
4. Select Mateo, start a walk-in, then Find available books. Choose together records the offered book; it does not reserve a copy. Check out chosen book revalidates live inventory and atomically records its loan link.
5. Choose Finish conversation, then Finish now or Book a follow-up. Booking uses the same available-slot picker and reserves staff/student time atomically. There is no editable time field. Past conversations contains Add book feedback; the focused form records reading/enjoyment and source.
6. My day displays each meeting once, with Start/Continue as the main action. More actions contains rescheduling, cancellation and no-show. An unfulfilled cancelled follow-up remains available for rebooking under Needs attention.
7. Outcomes → Our work shows independent count cards and feedback/follow-up ratio bars. Reading changes shows source-labelled paired distributions; Student trends shows borrowing and selected-period grade/reading distributions. View records and How this is counted retain the evidence. Custom Through dates are inclusive in the UI and translated to exclusive API ends. New choices still need their full 14-day observation window.

New API: GET `/api/agenda`, GET `/api/availability`, POST `/api/engagement`, GET `/api/metrics`, GET `/api/metrics/progress?source=scenario|extract`. Engagement commands require `request_id` and `expected_revision`; duplicate matching requests replay, changed payloads or stale revisions fail. Actions: schedule, reschedule, cancel, no_show, start, offer, choose, checkout, complete, feedback, book_followup. The old unreserved `due` field and one-click `followup` command are rejected. Existing checkout/return routes remain available but unlinked desk checkouts do not become engagement conversions.

`serve` preloads a small live morning so My day and Outcomes → Our work are not empty: completed visits (Aisha, Priya, Olivia, Maya), Tyler in progress, Jordan upcoming, and Aisha’s reserved follow-up. Mateo is left free for the live demo. Set `SHELFMATE_EMPTY_SESSION=1` to boot a blank desk. Restart still clears live records; historical Reading changes reload unchanged from `engagement_demo.json`.

Reload preserves state; restart clears live engagement records, retry receipts and inventory changes. Read-only historical contacts reload from `engagement_demo.json` and never occupy live slots or consume inventory. Outcomes → Reading changes offers historical/session sources, contact-period and facilitator filters, and optional evidence-as-of. API: GET `/api/metrics/paired?source=historical|session`. The default historical period has eight students, six eligible borrowing pairs and two exclusions. Open supporting student rows to inspect dates, values and later shared contacts. A session cohort does not inherit scenario evidence. Historical offer/checkout/feedback funnels, preference learning, external calendars, notifications, SSO and production ILS writeback remain outside this cut. See `metric-definitions.md` and `pilot-proposal.md`.

If a transport error leaves an action uncertain, use Retry interrupted action rather than creating another command. This retains the exact request body and is reachable from booking and feedback dialogs. No reset URL exists.

## UI regression check

With Playwright installed, run `node scripts/ui-smoke.cjs`. The script builds and launches an isolated local server with the model disabled, `SHELFMATE_EMPTY_SESSION=1` so empty-start assertions stay valid, and `SHELFMATE_TEST_DRIVER=1` so an injected clock can book a follow-up and later start it. It checks chart/API parity, Tom-style lookup without conversation, conversation/feedback actions, in-dialog retry, stale responses, mobile layout and available-slot booking, then stops its server. If Playwright or Chromium is installed outside the project, set `PLAYWRIGHT_MODULE` to the module path and `CHROMIUM_PATH` to the browser executable. Production serve never sets the test driver, so `/api/test/clock` is absent. Booking against live wall-clock availability is explicitly skipped if no future slots remain in the configured school calendar.

## Rehearsal if the model is down

Keep LLM off. The Friday (or any) talk still has retrieve, policy, console, and eval.
