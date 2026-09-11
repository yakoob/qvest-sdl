# ShelfMate

## Assignment

You are an engineering leader in a consulting organization. There is a desire to provide support to a school district to increase the number of books that the students are reading. During initial interviews with librarians, it was noted repeatedly that students would read books that were recommended by the librarian, and they usually relied on anecdotal data about other children enjoying books based on what they read previously.

You are expected to be at an internal meeting with the partners of the firm to present a proposal for a system they could use to increase the number of books the students are reading. The expectation for the content of the proposal would include diagrams describing how it works, and descriptions of how it would be used, rolled out and managed moving forward. Since that is obviously not enough time to have a concrete plan for all those things, we would like to see your best effort for this, as the project picked for this years pro-bono work will be picked from these presentations. Estimates for how long this project would take should also be included.

The librarians have said they could provide the history of books borrowed and also their electronic card catalog.

The partners include both highly technical and non-technical members, skilled in delivery, change management, user experience and software architecture and development. You can expect varying expectations for delivery format from code to slide deck. You know from previous experience that if the technical members are not convinced you have a workable architecture, it will not be approved.

Side note: This is entirely fictional — this is not a real client or scenario.

---

## What this repo is

Librarian-in-the-loop next-book assistant for a fictional Willow Bend / Maple Street extract.

**Goal:** help librarians scale their impact without scaling headcount, while reducing desk workload — so more students leave with a book they’ll actually read.

Built as a runnable PoC: **retrieve → policy → optional explain → audit → human approve**. Not a student app. Not a Destiny/Alexandria replacement.

| Audience | Open |
|----------|------|
| Partners (non-tech) | [`docs/deck-partners.html`](docs/deck-partners.html) |
| Architects (tech) | [`docs/deck.html`](docs/deck.html) |
| Live system / code map | [Graft](http://127.0.0.1:4400/) (`graft/` · `http://127.0.0.1:4400/`) |
| Desk console | `./run.sh` → http://127.0.0.1:8088 |

---

## Quick start

```bash
cd /Users/compeller/Documents/GitHub/qvest-sdl   # or your clone
go test ./...
go run ./cmd/shelfmate recommend -student S-406
go run ./cmd/shelfmate recommend -student S-402 -stretch
go run ./cmd/shelfmate recommend -student S-405 -query "funny, reluctant 4th, under 150 pages, we have copies"
./run.sh                                        # or: go run ./cmd/shelfmate serve -addr 127.0.0.1:8088
```

| Make target | What |
|-------------|------|
| `make test` | `go test ./...` |
| `make eval` | golden eval (`./internal/eval`) |
| `make recommend` | Mateo CLI rec |
| `make serve` | librarian console |
| `make json` | regenerate `data/json` from CSV |

Regenerate JSON from CSV: `python3 scripts/csv_to_json.py`

---

## What’s in the tree

| Path | What |
|------|------|
| `plan.md` | Product thesis, architecture notes, scope |
| `CLAUDE.md` | Agent instructions for working in this repo |
| `cmd/shelfmate` | Single binary: `recommend`, `serve`, `eval` |
| `internal/` | store, retrieve, policy, explain, engine, session, httpapi, audit, metrics, academics, support, domain |
| `web/` | Librarian desk SPA (My day, Books, Support, Progress, Outcomes) |
| `data/json/` | Canonical sources (catalog, circulation, students, …) |
| `data/json/sources.json` | Catalog of every source + FERPA notes |
| `data/csv/` | Original extracts |
| `data/notes/` | Personas, as-is sticky notes |
| `testdata/golden/` | Demo cases (Mateo, Aisha, Priya, Olivia) |
| `graft/` | System + code graph nodes (served at `:4400`) |
| `docs/deck-partners.html` | Non-tech partner proposal deck |
| `docs/deck.html` | Technical architecture deck (links Graft) |
| `docs/architecture.md` | Package and request-path detail |
| `docs/privacy.md` | FERPA-minded controls vs production gaps |
| `docs/runbook.md` | Operate the PoC |
| `docs/demo-script.md` | Live desk walkthrough |
| `docs/metric-definitions.md` | Outcomes / engagement contracts |
| `docs/pilot-proposal.md` | Rollout, ownership, ~11–16 week lighthouse |

---

## Librarian support loop

**My day → Schedule → Conversation → Choose together → Checkout → Book feedback → Outcomes**

- Appointments and follow-ups use available-slot pickers with school-local conflict checks. Follow-ups reserve time; finishing the linked conversation records contact.
- Choosing a book does **not** consume inventory. Linked checkout + conversation association are atomic.
- Explicit conversation completion and structured book feedback drive session-only activity tables (exact denominators).
- **Outcomes → Our work** — live session activity.
- **Outcomes → Student trends** — roster borrowing / English / reading checks (extract vs scenario sources labeled).
- **Outcomes → Reading changes** — descriptive historical served-cohort pairs (fictional fixture; isolated from live desk).
- `serve` preloads a small live morning. Restart clears it. `SHELFMATE_EMPTY_SESSION=1` boots a blank desk.

Walkthrough: [runbook](docs/runbook.md#complete-librarian-support-loop-session-only) · [metrics](docs/metric-definitions.md) · [pilot](docs/pilot-proposal.md). Staff selection is attribution, not authentication.

---

## Rules that do not move

- Closed-world catalog. No invented titles.
- `copies_available == 0` is never a spoken recommendation.
- LLM off by default (`SHELFMATE_LLM`). Recommendations still work.
- Model payloads (when enabled) use `student_id` only — no first names, no raw query, no blurbs.
- Librarian speaks. The binary does not talk to kids.
- Demo checkout is in-memory. Restart restores the frozen extract.
- Do not claim reading gains from this PoC extract.

---

## Demo IDs

| ID | Role in the demo |
|----|------------------|
| Mateo `S-406` | Graphic cluster / Friday rush; Cat Kid path; B-008 out of copies |
| Aisha `S-402` | Stretch toggle (grade-band relaxation) |
| Priya `S-405` | Sparse history + NL query (`under 150`) |
| Olivia `S-509` | Zero history → labeled popularity fallback |
| Sofia `S-305` | Primary isolated multi-year progress story |
| Tyler `S-504` | Check-in-first / support path |
| Elena `L-001` / Tom `L-002` / Priya Shah `L-003` | Staff |

Fictional children. Not real student data.
