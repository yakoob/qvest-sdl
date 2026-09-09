# Architecture (skeleton)

```
cmd/shelfmate
    recommend  → engine.Recommend → stdout JSON
    serve      → httpapi + web/

engine
    retrieve.Hybrid   item-item CF + TF-IDF + cluster/series bonus
    policy.Filter     copies>0, grade band, already-read, short-query
    explain           TemplateExplainer (LLM hook, default off)
    audit             JSONL student_id + book_ids
```

Closed world: `data/json/catalog.json` is the only title source.

Kill LLM: do not set `SHELFMATE_LLM=on`. Recs do not change.
