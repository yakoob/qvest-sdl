# ShelfMate — plan (Go PoC)

**Assignment:** Sony EIP hiring presentation, Friday 2026-09-11  
**Product:** librarian-in-the-loop next-book assistant for a fictional K-5 district  
**Language:** Go, single binary  
**This folder:** `/mnt/compeller/ai/school_district_reading`  
**Status:** data + skeleton. Finish retrieve polish, grounded LLM, librarian UI, HTML deck in Claude Code.

This is not Netflix-for-kids. Students already take librarian recs. ShelfMate makes Elena's sticky-note notebook queryable in the 20-second desk window.

---

## 1. Thesis (do not drift)

1. **Human in the loop.** Librarian speaks the rec. The model never talks to a child.
2. **Retrieve first.** Item-item CF from circulation + content/TF-IDF from catalog. Closed world: every recommended `book_id` exists in `catalog.json`.
3. **LLM last, and optional.** Used for intent parse + 20-second talking points. Never for inventing titles. Kill the LLM: recs still work.
4. **Availability is a feature.** `copies_available = 0` is not a spoken rec (Bad Guys / B-008).
5. **Privacy first.** FERPA / minors. LLM prompts get `student_id` only. First names stay in the librarian console. Audit JSONL for every rec.
6. **Handoff, not slideware.** Binary + CSV/JSON contract + runbook + eval. Parallel to EIP: memory, policy, audit, human approve.

---

## 2. Demo (must work, in this order)

| # | Student | Point |
|---|---------|--------|
| 1 | Mateo S-406 | CF lands on Cat Kid / Investigators. Bad Guys is out of copies — must not be spoken. |
| 2 | Aisha S-402 | Default stays adjacent fantasy. Stretch toggle → Gregor / Amari / Endling. Not Westing Game. |
| 3 | Priya S-405 | One checkout (Sharks). CF is empty. Content + NL query carries it. |
| 4 | Olivia S-509 | Zero checkouts. Honest cold start: NL / grade-band popular. |

Three real UX moments, not "kids open an app":

- Tom, 07:40-08:15 and lunch — lookup, no conversation
- Elena, last 8 minutes of specials — 4-5 kids, 3-minute talks
- Elena, Friday 14:20 — weekend rush; availability wins

If it fails those three, it fails.

---

## 3. Data (already converted)

Canonical machine files: `data/json/*.json`  
Index: `data/json/sources.json`  
Original CSVs kept: `data/csv/`  
Notes: `data/notes/`

Regenerate JSON:

```bash
python3 scripts/csv_to_json.py
```

Fictional district: Willow Bend / Maple Street Elementary, grades 3-5 pilot, 28 students, 45 titles, 176 circ events. Production would be Destiny/Alexandria nightly CSV, same columns, bigger N.

---

## 4. Architecture

```
librarian console (local)
        │
        ▼
   HTTP API  (cmd/shelfmate serve)
        │
        ├─ store.Store          load JSON once
        ├─ retrieve.Hybrid      CF + TF-IDF + series/cluster boost
        ├─ policy.Filter        grade band, copies>0, catalog-closed, no recent dupes
        ├─ explain.Explainer    template now; grounded LLM later
        └─ audit.Log            JSONL, student_id only toward model
```

Kill-switch: `SHELFMATE_LLM=off` (default). Retrieval path does not import an LLM client.

### Retrieval mix

| Signal | Source | Weight (default) |
|--------|--------|------------------|
| Item-item CF | circulation co-checkout cosine | 0.55 when history >= 2 |
| TF-IDF content | title/author/blurb/subjects/genre/series | 0.45, or 1.0 on cold start |
| NL query | librarian typed intent | blended into content query |
| Cluster / series | student.cluster, series continuation | additive bonus, not a new title source |
| Stretch | grade/lexile relaxation | policy, not generator |

Scores are min-max normalized per signal before mix. Then policy filter. Then top-k (default 5).

### Policy (hard)

- Recommend only `book_id` in catalog
- Drop `copies_available == 0`
- Grade: `grade_min <= grade <= grade_max` (stretch: `grade_min <= grade+1` and `grade_max >= grade`)
- Do not re-recommend the last 3 returned titles unless series continuation and librarian asked
- Never put peer-child names in student-facing slips (there are no student-facing slips in v1)

### LLM contract (when enabled)

Input: `student_id`, opaque history `book_id`s, retrieved candidate records, optional NL query.  
Output: `{intent, talking_points[book_id], refuse_if_not_in_candidates}`.  
If the model emits a title whose id is not in the retrieved set: drop it, log `ungrounded_title`, keep retrieve ranking.

---

## 5. Implementation order (Claude Code)

Do these in order. Do not start the HTML deck until eval is green.

| Step | Package | Done in skeleton? | Finish |
|------|---------|-------------------|--------|
| 0 | `data/json` | yes | treat as frozen unless Koob adds titles |
| 1 | `internal/domain` + `internal/store` | yes | add Destiny column aliases if a real extract arrives |
| 2 | `internal/retrieve` CF + TF-IDF | yes, baseline | tune weights; add time-decay on circ; better tokenizer |
| 3 | `internal/policy` | yes, baseline | stretch lexile cap; "do not announce level" flag |
| 4 | `internal/eval` goldens | yes, 4 cases | add Tom lunch timing; conversion metric stub |
| 5 | `internal/explain` | template only | grounded LLM using local OpenAI-compatible endpoint |
| 6 | `internal/audit` | JSONL stub | redaction test: no first names in model payload |
| 7 | `internal/httpapi` + `web/` | skeleton UI | 20-second lookup, stretch toggle, copy talking points |
| 8 | presentation | not started | one HTML deck: problem, retrieve, eval, privacy, estimates |

### Suggested LLM hook (do not invent a vendor)

Local first, EIP-shaped:

```
POST $LLM_BASE/v1/chat/completions
model=$LLM_MODEL
```

Default off. Wire to Compeller router only for lab (`http://127.0.0.1:8791/router/v1`). District deploy: their key, their VPC, privacy officer written yes (engagement week 0).

---

## 6. Eval (must stay honest)

`go test ./internal/eval ./internal/retrieve ./internal/policy`

Goldens live in `testdata/golden/cases.json`.

Pass:

- Mateo top-5 includes Cat Kid (B-007) or Investigators (B-006)
- Mateo top-5 does **not** include Bad Guys (B-008)
- Aisha default stays fantasy/cluster; stretch can include B-013/B-016/B-017
- Priya and Olivia still return catalog-closed recs
- Every rec id ∈ catalog

Do not claim "reading gains" from 12 weeks of lighthouse data. Report rec-to-checkout 14-day rate and checkouts/student as directional only.

---

## 7. Engagement (pro-bono lighthouse)

See `data/json/engagement_schedule.json`. Week 0 is privacy + data contract. LLM off until Marcus Ellison (fictional privacy officer) writes yes.

Estimates for the Friday deck (lab, not a bid):

| Slice | Time | Notes |
|-------|------|--------|
| Retrieve + policy + eval | 3-5 days | this skeleton |
| Librarian console | 2-3 days | Tom 20s path is the UX |
| Grounded LLM + audit | 2-3 days | refuse-un grounded is the test |
| Destiny export + redaction | 1 week with Priya | not in PoC |
| 12-week lighthouse | `engagement_schedule.json` | Elena hours are the constraint |

---

## 8. Out of scope (say no)

- Student-facing app, accounts, or "For You" feed
- Generating or downloading full book text
- Training on Battle of the Books lists
- Putting recs on a public Family Literacy Night screen
- Replacing Destiny/Alexandria as ILS
- Claiming patented tech (Compeller patent-pending is unrelated; do not mix)

---

## 9. Friday artifact list

1. This repo, runnable: `go run ./cmd/shelfmate recommend -student S-406`
2. Eval output (screenshot or `go test -v ./internal/eval`)
3. Librarian UI on localhost
4. HTML deck: as-is sticky notes → hybrid retrieve → human approve → audit
5. One slide: kill LLM, recs remain
6. One slide: FERPA (what leaves the building)

---

## 10. Open in Claude Code

```text
Open folder: /mnt/compeller/ai/school_district_reading
Read: CLAUDE.md, plan.md, readme.md, data/json/sources.json
Then: implement step 5+ from this plan. Keep tests green.
```
