# Claude Code — ShelfMate

You are finishing a Go PoC in this folder. Read `plan.md` and `readme.md` first. Do not turn this into a student-facing Netflix clone.

## Goal

Runnable librarian console + hybrid retrieve + eval goldens + (optional) grounded LLM explanations. Presentation is Friday 2026-09-11.

## Constraints

- Language: Go. Single binary `cmd/shelfmate`. No Python in the product path (`scripts/csv_to_json.py` is data prep only).
- Closed catalog: every rec `book_id` must exist in `data/json/catalog.json`.
- Never recommend `copies_available == 0` (B-008 The Bad Guys is the fixture).
- LLM optional and downstream of retrieve. If the model invents a title, drop it.
- FERPA: no last names, no DOB. Model payloads: `student_id` only, never first names.
- Kill LLM (`SHELFMATE_LLM=off`): recommendations unchanged.

## Data

Use `data/json/*.json`. Index: `data/json/sources.json`. Do not scrape the web for real children's PII. This extract is fictional.

## Finish order

1. Keep `go test ./...` green.
2. Tune retrieve so Mateo / Aisha / Priya goldens in `testdata/golden/cases.json` pass for the right reasons (read the `show` field).
3. Librarian UI: student lookup, history, recs, stretch toggle, NL box, copy talking points. Tom must complete a lookup without training.
4. Grounded LLM explainer behind an interface. TemplateExplainer stays default.
5. Audit JSONL + a test that model payload has no first names.
6. Only then: `docs/deck.html` for the Friday talk.

## Commands

```bash
go test ./...
go run ./cmd/shelfmate recommend -student S-406
go run ./cmd/shelfmate serve -addr :8088
```

## Do not

- Add a student login or public feed
- Download or generate book full text
- Mix Compeller REACT / patent-pending claims into this deck
- Mark work done without `go test ./...` passing
