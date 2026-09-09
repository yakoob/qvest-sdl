# ShelfMate

Librarian-in-the-loop next-book assistant. Fictional Willow Bend School District extract. Built as a Sony EIP-style PoC: retrieve, eval, audit, human approve. Not a student app.

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
| `web/` | Librarian console skeleton |
| `testdata/golden/` | Demo cases (Mateo, Aisha, Priya, Olivia) |

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

## Rules that do not move

- Closed-world catalog. No invented titles.
- `copies_available == 0` is never a spoken rec.
- LLM off by default. Recs still work.
- Prompts (when enabled) use `student_id` only.
- Elena approves. The binary does not talk to kids.

## Demo IDs

- Mateo `S-406` — graphic cluster, Friday rush
- Aisha `S-402` — stretch toggle
- Priya `S-405` — cold start (sharks)
- Olivia `S-509` — true zero history
- Elena `L-001` / Tom `L-002` / Priya Shah `L-003`

Fictional children. Not real student data.
