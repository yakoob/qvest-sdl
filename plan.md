# ShelfMate — plan (Go PoC)

**Assignment:** Internal partner pro-bono proposal (see README). Fictional school-district reading support — not a real client.  
**Product:** librarian-in-the-loop next-book assistant for a fictional K-5 district  
**Language:** Go, single binary  
**Decks:** `docs/deck-partners.html` (non-tech) · `docs/deck.html` (tech) · Graft map `http://127.0.0.1:4400/`  
**Status:** retrieve + policy + eval + librarian console + optional Axon explainer + audit + partner/tech decks + Graft. My day supports available-slot bookings and reserved follow-up conversations with atomic choice/loan linkage. Outcomes separates **Our work** (live session activity), **Reading changes** (descriptive historical served-cohort pairs) and **Student trends** (whole-roster rollups) with coverage/comparability exclusions. Historical offer/feedback funnels and production integrations remain deferred; see docs/runbook.md and docs/metric-definitions.md.

This is not Netflix-for-kids. Students already take librarian recs. ShelfMate makes Elena's sticky-note notebook queryable in the 20-second desk window. Goal: scale librarian impact without scaling headcount, while reducing workload.

---

## 1. Thesis (do not drift)

1. **Human in the loop.** Librarian speaks the rec. The model never talks to a child.
2. **Retrieve first.** Item-item CF from circulation + content/TF-IDF from catalog. Closed world: every recommended `book_id` exists in `catalog.json`.
3. **LLM last, and optional.** Talking points only. Never for inventing titles. Kill the LLM: recs still work.
4. **Availability is a feature.** `copies_available = 0` is not a spoken rec (Bad Guys / B-008).
5. **Privacy first.** FERPA / minors. LLM prompts get `student_id` plus candidate metadata. First names stay in the librarian console. Audit JSONL for every rec.
6. **Handoff, not slideware.** Binary + JSON contract + runbook + eval.

---

## 2. Demo (must work, in this order)

| # | Student | Point |
|---|---------|--------|
| 1 | Mateo S-406 | CF / cluster lands on Cat Kid. Investigators already borrowed. Bad Guys is out of copies — must not be spoken. |
| 2 | Aisha S-402 | Default stays adjacent fantasy. Stretch is grade-band relaxation. In this extract Gregor / Amari / Endling are already eligible at grade 4, so the spoken list need not change. Not Westing Game. |
| 3 | Priya S-405 | One checkout (Sharks). CF weight is zero. Content + NL query carries it. `under 150` means fewer than 150 pages. |
| 4 | Olivia S-509 | Zero checkouts. Honest fallback: unique-borrower popularity, labeled. |

Three real UX moments, not "kids open an app":

- Tom, 07:40-08:15 and lunch — lookup, no conversation
- Elena, last 8 minutes of specials — 4-5 kids, 3-minute talks
- Elena, Friday 14:20 — weekend rush; availability wins

If it fails those three, it fails.

---

## 3. Data (frozen)

Canonical machine files: `data/json/*.json`  
Index: `data/json/sources.json`  
Original CSVs kept: `data/csv/`  
Notes: `data/notes/`

Do not edit source extracts to make goldens pass.

Fictional district: Willow Bend / Maple Street Elementary, grades 3-5 pilot, 28 students, 45 titles, 176 circ events. Production would be Destiny/Alexandria nightly CSV, same columns, bigger N.

---

## 4. Architecture

See `docs/architecture.md` and Graft (`http://127.0.0.1:4400/`). Kill-switch: `SHELFMATE_LLM=off` (default).

### Policy (hard)

- Recommend only `book_id` in catalog (store lookup, not candidate trust)
- Drop `copies_available == 0`
- Grade: `grade_min <= grade <= grade_max` (stretch: `grade_min <= grade+1` and `grade_max >= grade`)
- Conservative: do not re-recommend any previously borrowed title
- `under 150 pages` = strictly fewer than 150; bare `short` uses a looser ~180 page default
- Never put peer-child names in student-facing slips (there are no student-facing slips)

### LLM contract (when enabled)

Input: `student_id`, stretch/page flags, allowlisted themes, retrieved candidate records (no blurbs, no raw query, no notes).  
Output: talking points keyed by `book_id`, plus optional enjoy IDs from that same set.  
If the model emits an id not in the retrieved set: drop it, keep retrieve ranking. Classification may only emit `ThemeNames()`.

---

## 5. Implementation status

Core path is in place: goldens, console, grounded explainer, audit, engagement loop, outcomes, partner + tech decks, Graft map. Keep `go test ./...` green. Do not claim reading gains.

---

## 6. Eval (must stay honest)

`go test ./internal/eval ./internal/retrieve ./internal/policy ./internal/explain`

Goldens live in `testdata/golden/cases.json` (`notes`, not `show`).

Pass:

- Mateo top-5 includes Cat Kid (B-007) or Investigators (B-006)
- Mateo top-5 does **not** include Bad Guys (B-008)
- Aisha default stays fantasy/cluster; stretch *may* include B-013/B-016/B-017 — in this extract they already appear without stretch
- Priya and Olivia still return catalog-closed recs
- Every rec id ∈ catalog

Do not claim "reading gains" from 12 weeks of lighthouse data.

---

## 7. Engagement & lighthouse

Desk engagement (My day / conversations / linked checkout / outcomes) is implemented in this binary as process-local demo state. A district lighthouse is still a separate engagement: see `data/json/engagement_schedule.json` and `docs/pilot-proposal.md`. Week 0 is privacy + data contract. LLM off until written privacy-officer yes.

Estimates (lab, not a bid): retrieve 3–5 eng-days, console 2–3, LLM+audit 2–3, Destiny export ~1 week with a media specialist; district lighthouse ~11–16 elapsed weeks, constrained by librarian hours.

---

## 8. Out of scope (say no)

- Student-facing app, accounts, or "For You" feed
- Generating or downloading full book text
- Training on Battle of the Books lists
- Putting recs on a public Family Literacy Night screen
- Replacing Destiny/Alexandria as ILS
- Claiming patented tech from unrelated work

---

## 9. Artifact list

1. This repo, runnable: `go run ./cmd/shelfmate recommend -student S-406` / `./run.sh`
2. Eval: `go test ./internal/eval` (and `go test ./...`)
3. Librarian UI on localhost (`http://127.0.0.1:8088`)
4. Non-tech partner deck: `docs/deck-partners.html`
5. Technical architecture deck: `docs/deck.html` (links Graft)
6. Graft system/code map: `graft/` → `http://127.0.0.1:4400/`
7. Kill LLM, recs remain
8. FERPA notes: `docs/privacy.md` — what leaves the desk toward a model
