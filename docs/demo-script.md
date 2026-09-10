# Desk demo (about 3 minutes)

Staff: Elena (L-001) or Tom (L-002). Do this live from the console, not as slides.

Start:

```bash
go run ./cmd/shelfmate serve
```

Open http://127.0.0.1:8088 — default bind is localhost.

1. **Mateo S-406** — demo chip, no query, Next book.
   - Spoken rec includes **Cat Kid Comic Club (B-007)**.
   - **Investigators (B-006)** is already in his history, so policy drops it (`already checked out`).
   - **The Bad Guys (B-008)** is `copies_available=0` — show it only as an unavailable exclusion. Do not copy it as a talking point.
2. **Aisha S-402** — default stays fantasy (Harry Potter, Endling, Gregor, Amari in this extract).
   - Toggle Stretch. Ranking in this extract **does not change**: Gregor / Amari / Endling are already grade-eligible at grade 4. Stretch is a grade-band relaxation, not an ability diagnosis.
   - **Westing Game (B-051)** stays out of the spoken list (grade_min 5; even when stretch makes it eligible, retrieve does not prefer it).
3. **Priya S-405** — type `funny, reluctant 4th, under 150 pages, we have copies`.
   - One sharks checkout. CF weight is zero (history < 2). Content + query carry it.
   - `under 150` means fewer than 150 pages. This run returned Encyclopedia Brown (96), I Survived (112), Gravity in Pictures (64), Who Was Jackie Robinson (112), Soccer Sunday (128).
4. **Olivia S-509** — empty history, empty query.
   - Honest fallback: unique borrowers in the grade band, labeled `grade-band popularity fallback`. This run: Dog Man, Wild Robot, Mr. Popper's Penguins (7 unique borrowers), then Cat Kid / Gravity in Pictures (6).

Copy talking points after Mateo. Say out loud they are drafts.

5. **Checkout** — on Mateo's Cat Kid card, **Check out to Mateo**.
   - Confirmation shows the title, copies left, and current loan.
   - Activity pane records checkout · student · book · copy change.
   - Next book no longer offers Cat Kid (already borrowed).
   - Switch to Aisha: Cat Kid's shelf count dropped for everyone in this process.
6. **Return** — on Mateo's Current loans, Return. Shelf count comes back. History keeps the title (borrowed, not finished).
7. **Reading & learning** — synthetic fixture. Mateo has a posted spring English C and a current term with no grade. Willow Bend Reading Check (fictional 1–4) is flat in grade 3; the grade 4 form is not a delta. Checkout does not change grades.
8. **Support** — select Tyler S-504; see Check in first and expand its evidence. The current teacher request determines prompt attention; counselor themes do not influence the band. Explore sports/underdogs as an explicit reading preference. Record a real check-in or student-reported enjoyment separately; the band and academic values stay unchanged.
9. **Progress** — inspect separate letter-grade, assessment-form and observed-borrowing charts. Use keyboard focus or hover for detail; each chart has a data table. No claim that borrowing caused a grade change.
10. Restart the server with `./run.sh`: demo loans, activity and follow-ups vanish; original source loans remain and the frozen extract is back.

If Tom cannot finish step 1 without a walkthrough, the UX is not done.

## Offline fallback rehearsal

Leave `SHELFMATE_LLM` unset. Recs still return. If you enable `SHELFMATE_LLM=on` without `LLM_BASE`, explain mode is `fallback` and ranking is unchanged.
