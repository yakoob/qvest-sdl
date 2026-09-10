# Architecture

```
librarian console (web/, localhost)
        │
        ▼
   HTTP API  (cmd/shelfmate serve)
        │
        ├─ store.Store          load data/json once
        ├─ retrieve.Hybrid      item-item CF + catalog TF-IDF
        ├─ policy.Filter        catalog-closed, copies>0, grade, already-read, page cap
        ├─ explain.Explainer    TemplateExplainer default; optional Axon
        └─ audit.Log            JSONL, opaque IDs, no raw query
```

Closed world: `data/json/catalog.json` is the only title source. Policy re-reads the store for membership and `copies_available`; candidate structs are not trusted.

Serve path adds a process-local `session.Service`. Checkout and return clone Books + Circulation, rebuild retrieve/policy, and publish a new immutable snapshot with a monotonic revision. Restart reloads the frozen extract. Session activity is memory-only and is not appended to the optional CLI audit JSONL.

Synthetic academics (`data/json/academic_demo.json`) load beside the store. Isolated matched-window scenarios (six 84-day windows over three completed years) never enter retrieve, ranking, support, policy, explainer payloads, or audit records. Operational semester rows that feed support stay unchanged.

## Request sequence

1. Librarian looks up a student (desk laptop).
2. `Hybrid.Recommend` scores the catalog. Item-item cosine from circulation (weight 0.55 when history ≥ 2) plus TF-IDF from title/author/blurb/subjects/genre/series (0.45, or 1.0 on sparse history). Query tokens are a separate content vector from history tokens. Cluster/series bonuses are additive, not a new title source. “Same series” is not “next volume” (no sequence metadata).
3. Zero history and no query match: deterministic unique-borrower popularity, `book_id` tie-break, labeled as fallback.
4. `policy.Filter` drops unknown IDs, `copies_available == 0`, out-of-band grades, any previously borrowed title (conservative; not only last-3), and page-cap matches (`under 150` = strictly fewer than 150 pages; bare `short` ≈ 180 pages).
5. Stretch is a librarian checkbox: `grade_min <= grade+1` and `grade_max >= grade`. It does not change retrieve weights or invent ability.
6. Explainer writes talking points for the already-ranked IDs. Kill switch `SHELFMATE_LLM=off` (default) keeps TemplateExplainer. `SHELFMATE_LLM=on` may call Axon; ranking is unchanged on success or failure.
7. Audit JSONL appends opaque IDs, policy outcomes, explain mode, version. Write errors surface; they are not ignored.
8. Optional demo checkout (`POST /api/checkouts`) decrements live copies in the session snapshot and records an open loan. Return restores one copy. The UI shows confirmation, current loans, and activity. Stale recommendations include `revision` so the console can refetch.

## Demo session (serve only)

- One `session.Service` per process. All student / recommend / inventory reads use the same snapshot kind.
- Checkout validates student, staff, book, no duplicate open loan, copies > 0, and a retry key under the writer lock.
- Return validates loan ID + student. If restoring a copy would exceed `copies_total` (imported inventory is authoritative at boot), the call fails with 409 rather than corrupting counts.
- Recommendation and explanation run after the lock is released. Older in-flight recs may finish; checkout always re-checks current inventory.
- No reset HTTP endpoint. Restart is the reset.

## Why hybrid, not CF-only or LLM-first

| Approach | Why not alone |
|----------|----------------|
| Circulation CF | Empty for Priya (one checkout) and Olivia (zero). |
| Content / TF-IDF only | Misses Mateo’s graphic neighborhood that co-checkout cosine catches. |
| LLM as generator | Invented titles, no shelf truth, FERPA surface area. |
| Hybrid retrieve → policy → optional talk | Closed catalog, availability as a feature, model optional. |

## LLM boundary

Retrieval does not import the HTTP client as a required path. When enabled:

`POST $LLM_BASE/v1/chat/completions` (client appends that path if `LLM_BASE` is an origin or `/v1`).

Payload allowlist: `student_id`, stretch/page flags, candidate `book_id` + catalog fields + retrieve reasons. No first names, anecdotes, blurbs, or raw query. Unknown IDs from the model are dropped; talking points are drafts, not a factuality proof.

Kill LLM: recs and scores stay the same. Proven in `internal/engine` against a mock that fails and a mock that succeeds.
