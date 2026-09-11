# ShelfMate

## Assignment

You are an engineering leader in a consulting organization. There is a desire to provide support to a school district to increase the number of books that the students are reading. During initial interviews with librarians, it was noted repeatedly that students would read books that were recommended by the librarian, and they usually relied on anecdotal data about other children enjoying books based on what they read previously.

You are expected to be at an internal meeting with the partners of the firm to present a proposal for a system they could use to increase the number of books the students are reading. The expectation for the content of the proposal would include diagrams describing how it works, and descriptions of how it would be used, rolled out and managed moving forward. Since that is obviously not enough time to have a concrete plan for all those things, we would like to see your best effort for this, as the project picked for this years pro-bono work will be picked from these presentations. Estimates for how long this project would take should also be included.

The librarians have said they could provide the history of books borrowed and also their electronic card catalog.

The partners include both highly technical and non-technical members, skilled in delivery, change management, user experience and software architecture and development. You can expect varying expectations for delivery format from code to slide deck. You know from previous experience that if the technical members are not convinced you have a workable architecture, it will not be approved.

Side note: This is entirely fictional - this is not a real client or scenario.

---

Librarian-in-the-loop next-book assistant. Fictional Willow Bend School District extract. Built as a EIP-style PoC: retrieve, eval, audit, human approve. Not a student app.

**Canonical path:** `/mnt/compeller/ai/school_district_reading`  
(`/mnt/ai/...` is not writable on oc-lisa; this is the NAS folder Koob created.)

## What is here

| Path | What |
|------|------|
| `plan.md` | Product thesis, architecture, Claude Code finish order |
| `CLAUDE.md` | Instructions for Claude Code |
| `data/json/` | All data sources as JSON (use these) |
| `data/json/sources.json` | Catalog of every source + FERPA notes |
| `data/csv/` | Original extracts |
| `data/notes/` | Personas, as-is sticky notes, generator |
| `cmd/shelfmate` | Single binary: `recommend`, `serve`, `eval` |
| `internal/` | Retrieve / policy / explain / audit / HTTP |
| `web/` | Librarian desk: lookup, recs, checkout/return, activity |
| `testdata/golden/` | Demo cases (Mateo, Aisha, Priya, Olivia) |
| `data/json/academic_demo.json` | Synthetic semester English + fictional reading check |

## Quick start

```bash
cd /mnt/compeller/ai/school_district_reading
go test ./...
go run ./cmd/shelfmate recommend -student S-406
go run ./cmd/shelfmate recommend -student S-402 -stretch
go run ./cmd/shelfmate recommend -student S-405 -query "funny, reluctant 4th, under 150 pages, we have copies"
go run ./cmd/shelfmate serve -addr :8088
```

Console: http://127.0.0.1:8088

Regenerate JSON from CSV:

```bash
python3 scripts/csv_to_json.py
```

## Librarian support loop

**My day → Schedule → Conversation → Choose together → Checkout → Book feedback → Outcomes** is available in the local console. Existing student Books/Support/Progress tabs remain intact.

- Appointments and follow-ups use available-slot pickers with school-local conflict checks and explicit confirmation. Follow-ups reserve time; completing the linked conversation records contact.
- Book choices do not consume inventory. Linked checkout and its conversation association are atomic.
- Explicit conversation completion and structured book feedback drive session-only activity tables, with exact denominators and supporting records.
- Outcomes → Student trends rolls up individual borrowing, English grades and compatible reading checks, with separate extract/scenario sources and student drill-downs.
- Outcomes → Reading changes compares eligible historical borrowing/academic pairs, with sample sizes, exclusions and first-contact attribution. Historical contacts are explicitly fictional and isolated from live actions.
- `serve` preloads a small live morning for My day and Outcomes → Our work. Restart still clears it. `SHELFMATE_EMPTY_SESSION=1` boots a blank desk. Historical Reading changes stay on the separate fixture.

See [the walkthrough](docs/runbook.md#complete-librarian-support-loop-session-only), [metric definitions](docs/metric-definitions.md), and [pilot proposal](docs/pilot-proposal.md). Staff selection is attribution, not authentication.

## Rules that do not move

- Closed-world catalog. No invented titles.
- `copies_available == 0` is never a spoken rec.
- LLM off by default. Recs still work.
- Prompts (when enabled) use `student_id` only.
- Elena approves. The binary does not talk to kids.
- Demo checkout is in-memory. Restart restores the frozen extract.

## Demo IDs

- Mateo `S-406` — graphic cluster, Friday rush; isolated 6-window improving scenario (C-→A-, 2→11 checkouts). Operational spring English stays C.
- Aisha `S-402` — stretch toggle; strong-stable isolated scenario (A/A-, 4 checkouts × 6 windows)
- Priya `S-405` — cold start (sharks); empty local-history windows
- Olivia `S-509` — true zero history; empty local-history windows
- Sofia `S-305` — primary isolated 3-year story: 6×84-day windows, D→B+, 2→10 checkouts. Operational spring English stays B+.
- Tyler `S-504` — Check in first; isolated improving scenario (D+→A, 1→9 checkouts). Operational spring English stays C.
- Elena `L-001` / Tom `L-002` / Priya Shah `L-003`

Fictional children. Not real student data.
