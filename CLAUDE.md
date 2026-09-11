# Agent notes — ShelfMate

Read `README.md` (assignment + current layout) and `plan.md` first. Do not turn this into a student-facing Netflix clone.

## Goal

Runnable librarian console + hybrid retrieve + eval goldens + optional grounded LLM explanations + partner/tech decks. Scale librarian impact without scaling headcount; reduce desk workload. Presentation materials: `docs/deck-partners.html` (non-tech), `docs/deck.html` (tech), Graft at `http://127.0.0.1:4400/`.

## Constraints

- Language: Go. Single binary `cmd/shelfmate`. No Python in the product path (`scripts/csv_to_json.py` is data prep only).
- Closed catalog: every rec `book_id` must exist in `data/json/catalog.json`.
- Never recommend `copies_available == 0` (B-008 The Bad Guys is the fixture).
- LLM optional and downstream of retrieve. If the model invents a title, drop it. Ranking never changes on LLM success or failure.
- FERPA: no last names, no DOB. Model payloads: `student_id` only, never first names.
- Kill LLM (`SHELFMATE_LLM=off`): recommendations unchanged.
- Do not claim reading gains from the PoC extract.

## Data

Use `data/json/*.json`. Index: `data/json/sources.json`. Do not scrape the web for real children's PII. This extract is fictional.

## Keep green

1. `go test ./...` stays green.
2. Goldens in `testdata/golden/cases.json` (Mateo / Aisha / Priya / Olivia) pass for the right reasons.
3. Librarian UI: lookup, history, recs, stretch, NL box, talking points, My day / outcomes. Tom must complete a lookup without training.
4. TemplateExplainer default; optional Axon behind the interface.
5. Audit JSONL + payload privacy tests.
6. Decks stay honest: partners deck in school-admin voice; tech deck links Graft. Docs match code.

## Commands

```bash
go test ./...
go run ./cmd/shelfmate recommend -student S-406
go run ./cmd/shelfmate serve -addr 127.0.0.1:8088
./run.sh
```

## Do not

- Add a student login or public feed
- Download or generate book full text
- Mix unrelated patent / REACT claims into these decks
- Mark work done without `go test ./...` passing
- Present unfinished analytics as working
