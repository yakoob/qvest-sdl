# Willow Bend sample extract (fictional)

PoC data for **ShelfMate**. Not a real district. Not real children.

**District:** Willow Bend School District (WBSD)  
**Lighthouse:** Maple Street Elementary (grades K-5, 412 students)  
**Pilot population:** grades 3-5 only  
**School year:** 2026-27 (started 2026-08-17)  
**Extract date:** 2026-09-04

This folder is a **minimized sample**, not a full ILS dump:

- 28 students (not 412)
- 3 staff (the people who would actually touch the tool)
- ~40 catalog titles (enough for clusters to show up)
- Circulation from spring 2026 + the first three weeks of fall
- Operating schedule for Maple Street library

Production would be: full Destiny/Alexandria catalog + 12-24 months of circulation, nightly CSV or SIF. Same columns. Same IDs. Bigger N.

## FERPA posture (demo)

Included: opaque student_id, grade, homeroom, reading band, first name + last initial.  
Excluded: last names, DOB, address, parent contact, state ID, free/reduced lunch, discipline.

LLM prompts in the product should use `student_id` only. First names are for the librarian console, not the model.

## Files

| File | What |
|---|---|
| `district.json` | Site, year, ILS, who owns data |
| `librarians.csv` | Staff who run the desk |
| `homerooms.csv` | Grade 3-5 classes in the specials rotation |
| `students.csv` | Sample borrowers + cluster tags for the demo |
| `catalog.csv` | Card catalog slice |
| `circulation.csv` | Checkouts (generated from clusters) |
| `library_hours.csv` | Open/close |
| `class_visits.csv` | Weekly specials (when a whole class is in the room) |
| `desk_shifts.csv` | Who is on the desk |
| `open_circulation.csv` | High-volume walk-up windows (ShelfMate's real job) |
| `book_clubs.csv` | Optional programs |
| `calendar_exceptions.csv` | Closures |
| `engagement_schedule.csv` | Proposed 12-week pro-bono lighthouse |
| `personas.md` | Demo walkthrough (Maya/Mateo, Aisha, Priya) |
