# ShelfMate

## Assignment

You are an engineering leader in a consulting organization. There is a desire to provide support to a school district to increase the number of books that the students are reading. During initial interviews with librarians, it was noted repeatedly that students would read books that were recommended by the librarian, and they usually relied on anecdotal data about other children enjoying books based on what they read previously.

You are expected to be at an internal meeting with the partners of the firm to present a proposal for a system they could use to increase the number of books the students are reading. The expectation for the content of the proposal would include diagrams describing how it works, and descriptions of how it would be used, rolled out and managed moving forward. Since that is obviously not enough time to have a concrete plan for all those things, we would like to see your best effort for this, as the project picked for this years pro-bono work will be picked from these presentations. Estimates for how long this project would take should also be included.

The librarians have said they could provide the history of books borrowed and also their electronic card catalog.

The partners include both highly technical and non-technical members, skilled in delivery, change management, user experience and software architecture and development. You can expect varying expectations for delivery format from code to slide deck. You know from previous experience that if the technical members are not convinced you have a workable architecture, it will not be approved.

Side note: This is entirely fictional — this is not a real client or scenario.

---

## What this repo is

Librarian-in-the-loop next-book assistant on a **fictional** Willow Bend School District extract (lighthouse: Maple Street Elementary, grades 3–5 pilot).

**Goal:** give librarians tools to scale their impact without scaling headcount — while reducing desk workload — so more students leave with a book they’ll actually read.

**Pipeline:** retrieve → policy → optional explain → audit → human approve.

- Go module: `school_district_reading` (Go 1.22+)
- Single binary: `cmd/shelfmate` (`recommend` | `serve` | `eval`) · PoC tag `0.1.0-poc`
- Not a student app. Not a Destiny/Alexandria replacement.

| Audience | Open |
|----------|------|
| Partners (non-tech) | [`docs/deck-partners.html`](docs/deck-partners.html) |
| Architects (tech) | [`docs/deck.html`](docs/deck.html) |
| Live system / code map | `graft viz` → [http://127.0.0.1:4400/](http://127.0.0.1:4400/) |
| Desk console | `./run.sh` → [http://127.0.0.1:8088](http://127.0.0.1:8088) |

---

## Quick start

```bash
go test ./...
go run ./cmd/shelfmate recommend -student S-406
go run ./cmd/shelfmate recommend -student S-402 -stretch
go run ./cmd/shelfmate recommend -student S-405 -query "funny, reluctant 4th, under 150 pages, we have copies"
./run.sh
# console: http://127.0.0.1:8088

graft viz          # architecture map: http://127.0.0.1:4400/
```

| Make target | What |
|-------------|------|
| `make test` | `go test ./...` |
| `make eval` | `go test -v ./internal/eval` |
| `make recommend` | Mateo CLI recommendation |
| `make serve` | librarian console (`serve`) |
| `make json` | regenerate `data/json` from `data/csv` via `scripts/csv_to_json.py` |

Default listen address is `127.0.0.1:8088`. LLM stays off unless `SHELFMATE_LLM=on` (see [runbook](docs/runbook.md)).

---

## Extract (what’s actually in `data/json`)

Index: [`data/json/sources.json`](data/json/sources.json) · extract date `2026-09-04` · school year `2026-27` · ILS assumed Follett Destiny (nightly CSV).

| Source | Records | Role |
|--------|--------:|------|
| `catalog.json` | 45 | Closed-world card catalog (only legal recommendation titles) |
| `students.json` | 28 | Borrowers grades 3–5 (8 / 10 / 10) · opaque `student_id` |
| `circulation.json` | 176 | Checkout history (CF signal) |
| `librarians.json` | 3 | Elena `L-001`, Tom `L-002`, Priya Shah `L-003` |
| `homerooms.json` | 6 | Grade 3–5 classes |
| `desk_shifts.json` | 11 | Who is on the desk |
| `open_circulation.json` | 14 | High-volume walk-up windows |
| `library_hours.json` | 7 | Open/close |
| `class_visits.json` | 6 | Specials / whole-class visits |
| `calendar_exceptions.json` | 10 | Closures |
| `book_clubs.json` | 3 | Optional programs |
| `engagement_schedule.json` | 13 | Proposed 12-week lighthouse plan |
| `district.json` | 1 | Site, year, ILS, data owner, privacy officer |
| `personas.json` | 4 | Demo walkthrough personas |
| `as_is_notes.json` | 1 | Sticky-note “as-is” notebook we’re replacing |
| `academic_demo.json` | (fixture) | Synthetic English + reading-check scenarios (isolated from retrieve) |
| `engagement_demo.json` | (fixture) | Read-only historical contacts for Reading changes (not live desk) |
| `support_demo.json` | (fixture) | Support-triage demo context |

Original CSVs live under `data/csv/`. Notes/personas prose under `data/notes/`. Do **not** edit frozen extracts just to make goldens pass.

---

## What’s in the tree

| Path | What |
|------|------|
| `cmd/shelfmate` | CLI/binary entrypoint |
| `internal/domain` | Book / Student / Recommendation contracts |
| `internal/store` | Load `data/json` once |
| `internal/retrieve` | Hybrid item-item CF (0.55) + TF-IDF (0.45 / 1.0 if sparse) |
| `internal/policy` | Catalog-closed, copies &gt; 0, grade/stretch, already-read, page cap |
| `internal/engine` | Snapshot orchestration; LLM cannot reorder scores |
| `internal/explain` | TemplateExplainer default; optional Axon |
| `internal/session` | Process-local inventory + engagement coordinator |
| `internal/engagement` | Calendar / shifts / availability types |
| `internal/httpapi` | JSON API + same-origin mutation guard |
| `internal/audit` | JSONL; opaque IDs; no raw query |
| `internal/metrics` | Pure projections: session / portfolio / paired |
| `internal/academics` | Synthetic fixtures; isolated from retrieve |
| `internal/support` | Desk support triage (not model input) |
| `internal/eval` | Golden invariants |
| `internal/version` | `0.1.0-poc` |
| `web/` | Desk SPA: My day, Books, Support, Progress, Outcomes |
| `graft/` | Context + code graph (`graft viz` on `:4400`) |
| `testdata/golden/` | Mateo, Aisha (×2), Priya, Olivia |
| `docs/deck-partners.html` | Non-tech partner proposal |
| `docs/deck.html` | Technical architecture (links Graft) |
| `docs/architecture.md` | Request path + package detail |
| `docs/privacy.md` | FERPA-minded controls vs production gaps |
| `docs/runbook.md` | Operate the PoC |
| `docs/demo-script.md` | Live desk walkthrough |
| `docs/metric-definitions.md` | Outcomes / engagement contracts |
| `docs/pilot-proposal.md` | Rollout, ownership, ~11–16 week lighthouse |
| `plan.md` | Thesis, scope, artifacts |
| `CLAUDE.md` | Agent working notes |

---

## Librarian support loop

**My day → Schedule → Conversation → Choose together → Checkout → Book feedback → Outcomes**

- Available-slot pickers with school-local conflict checks. Follow-ups reserve time; finishing the linked conversation records contact.
- Choosing a book does **not** consume inventory. Linked checkout + conversation association are atomic.
- Explicit conversation completion + structured book feedback drive session-only activity tables (exact denominators).
- **Outcomes → Our work** — live session activity (cleared on restart).
- **Outcomes → Student trends** — roster borrowing / English / reading checks (extract vs scenario sources labeled).
- **Outcomes → Reading changes** — descriptive historical pairs from `engagement_demo.json` (isolated from live desk).
- `serve` preloads a small live morning via `session.SeedLiveDesk`. `SHELFMATE_EMPTY_SESSION=1` boots a blank desk.

Walkthrough: [runbook](docs/runbook.md) · [demo script](docs/demo-script.md) · [metrics](docs/metric-definitions.md). Staff selection is **attribution**, not authentication.

---

## Rules that do not move

- Closed-world catalog — every recommended `book_id` ∈ `catalog.json`.
- `copies_available == 0` is never a spoken recommendation (fixture: B-008 The Bad Guys).
- LLM off by default (`SHELFMATE_LLM`). Recommendations still work; ranking unchanged if the model is on and fails.
- Model payloads (when enabled): `student_id` + candidate metadata only — no first names, raw query, or blurbs.
- Librarian speaks. The binary does not talk to kids.
- Demo checkout is process memory. Restart restores the frozen extract.
- Do not claim reading gains from this PoC extract.

---

## Demo IDs

| ID | Role |
|----|------|
| Mateo `S-406` | Graphic cluster / Friday rush; Cat Kid path; B-008 out of copies |
| Aisha `S-402` | Stretch toggle (grade-band relaxation) |
| Priya `S-405` | Sparse history + NL query (`under 150`) |
| Olivia `S-509` | Zero history → labeled popularity fallback |
| Sofia `S-305` | Primary isolated multi-year progress story (`academic_demo`) |
| Tyler `S-504` | Check-in-first / support path |
| Elena `L-001` | Library Media Specialist (primary user) |
| Tom `L-002` | Library Aide |
| Priya Shah `L-003` | District Library Coordinator / data owner |

Fictional children and staff. Not real student data.
