# Demo personas (use these three, in this order)

All IDs are in `students.csv`. Anecdotes are Elena's, not the model's.

## 1. Mateo S-406 — the thing librarians already do

- Grade 4, H-4B, below band, cluster `graphic`
- History: Dog Man, Wimpy Kid, Captain Underpants, Big Nate, Lunch Lady
- Lost him on a prose chapter book in March
- When: Tuesday 10:05 class visit, or Friday 14:20 weekend rush
- **Show:** item-item CF lands on Cat Kid / Investigators / Bad Guys
- **Show:** Bad Guys has `copies_available=0` — it must not be the spoken rec
- **Show:** talking points Elena can say in 20 seconds ("funny, pictures, you can finish it this weekend")

## 2. Aisha S-402 — stretch

- Grade 4, H-4A, above band, cluster `fantasy`
- History: Lightning Thief, Wings of Fire, Wild Robot, Dealing with Dragons
- **Show:** default recs stay in-series / adjacent fantasy
- **Show:** stretch toggle → Gregor, Amari, Endling — still catalog-closed, still grade_max >= 4
- **Do not:** jump her to Westing Game just because lexile is high. Different muscle.

## 3. Priya N S-405 — cold start (and Olivia S-509 even colder)

- Grade 4, arrived 2026-08-25, one checkout: Sharks (B-062)
- CF is empty. Content/TF-IDF + Elena's NL query has to carry it
- **Show:** librarian types `funny, reluctant 4th, under 150 pages, we have copies`
- **Show:** if you only had CF, this child gets nothing useful. That is why hybrid.

Olivia S-509 transferred 2026-09-02, **zero** checkouts. Pure NL / grade-band popular. Honest about the limit.

## Who is in the room (partners)

| Person | What they watch |
|---|---|
| Elena L-001 | Does the why sound like her, or like a bot |
| Tom L-002 | Can he do it in the lunch window without a training class |
| Priya L-003 | Is student_id the only thing that leaves the building toward a model |
| Change partner | Friday 14:20 is the real UX, not the Monday specials lesson |
| Architect | Kill LLM, recs remain |

## When ShelfMate is actually used

Not "students open an app." Three moments:

1. **Tom, 07:40-08:15 and lunch** — lookup by student, no conversation
2. **Elena, last 8 minutes of specials** — 4-5 kids, 3-minute talks
3. **Elena, Friday 14:20** — weekend rush, availability is the feature

If it does not work in those three, it does not work.
