#!/usr/bin/env python3
"""Convert the Willow Bend CSV extracts into typed JSON for the Go PoC."""
from __future__ import annotations

import csv
import json
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
CSV = ROOT / "data" / "csv"
JSON = ROOT / "data" / "json"
NOTES = ROOT / "data" / "notes"

INT_FIELDS = {
    "year",
    "pages",
    "lexile_approx",
    "grade_min",
    "grade_max",
    "copies_total",
    "copies_available",
    "grade",
    "class_size",
    "fte",
    "est_checkouts",
    "elena_hours",
    "tom_hours",
    "priya_hours",
    "week",
}
FLOAT_FIELDS = {"fte"}
BOOL_FIELDS = {
    "english_learner",
    "new_this_year",
    "active",
}
LIST_SEMI = {"subjects", "specialties"}
LIST_SPACE = {"students_in_sample"}


def coerce(key: str, value):
    if value is None:
        return None
    if isinstance(value, list):
        value = ", ".join(str(v) for v in value if v is not None)
    raw = str(value).strip()
    if raw == "":
        return None
    if key in BOOL_FIELDS:
        return raw.lower() == "true"
    if key in FLOAT_FIELDS:
        return float(raw)
    if key in INT_FIELDS:
        if re.fullmatch(r"-?\d+", raw):
            return int(raw)
        return raw
    if key in LIST_SEMI:
        return [p.strip() for p in re.split(r"[;,]\s*", raw) if p.strip()]
    if key in LIST_SPACE:
        return [p.strip() for p in raw.split() if p.strip()]
    return raw


def align_row(headers, raw):
    n = len(headers)
    if len(raw) == n:
        return raw
    if len(raw) < n:
        return raw + [""] * (n - len(raw))
    year_i = headers.index("year") if "year" in headers else None
    if year_i is not None and year_i < len(raw) and re.fullmatch(r"\d{4}", raw[year_i].strip()):
        return raw[: n - 1] + [", ".join(raw[n - 1 :])]
    for i, cell in enumerate(raw):
        if re.fullmatch(r"\d{4}", cell.strip()) and i >= 3:
            title = ", ".join(x.strip() for x in raw[2 : i - 1])
            author = raw[i - 1].strip()
            combined = raw[:2] + [title, author] + raw[i:]
            if len(combined) == n:
                return combined
            if len(combined) > n:
                return combined[: n - 1] + [", ".join(combined[n - 1 :])]
            return combined + [""] * (n - len(combined))
    return raw[: n - 1] + [", ".join(raw[n - 1 :])]


def load_csv(name: str) -> list[dict]:
    path = CSV / name
    with path.open(newline="", encoding="utf-8") as handle:
        reader = csv.reader(handle)
        headers = next(reader)
        rows = []
        for raw in reader:
            if not raw or all(not c.strip() for c in raw):
                continue
            raw = align_row(headers, raw)
            item = {k: coerce(k, v) for k, v in zip(headers, raw)}
            rows.append(item)
        return rows


def dump(name: str, payload) -> None:
    path = JSON / name
    path.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    print(f"wrote {path.relative_to(ROOT)} ({path.stat().st_size} bytes)")


def personas_json() -> dict:
    return {
        "walkthrough_order": ["S-406", "S-402", "S-405", "S-509"],
        "moments": [
            {
                "id": "tom_lookup",
                "who": "L-002",
                "when": "07:40-08:15 and lunch",
                "job": "lookup by student, no conversation",
            },
            {
                "id": "elena_specials",
                "who": "L-001",
                "when": "last 8 minutes of specials",
                "job": "4-5 kids, 3-minute talks",
            },
            {
                "id": "friday_rush",
                "who": "L-001",
                "when": "Friday 14:20",
                "job": "weekend rush, availability is the feature",
            },
        ],
        "personas": [
            {
                "student_id": "S-406",
                "name": "Mateo S",
                "role": "demo_star",
                "why": "the thing librarians already do",
                "grade": 4,
                "homeroom_id": "H-4B",
                "reading_band": "below",
                "cluster": "graphic",
                "history_titles": [
                    "Dog Man",
                    "Wimpy Kid",
                    "Captain Underpants",
                    "Big Nate",
                    "Lunch Lady",
                ],
                "lost_him_on": "prose chapter book in March",
                "when": "Tuesday 10:05 class visit, or Friday 14:20 weekend rush",
                "show": [
                    "item-item CF lands on Cat Kid / Investigators / Bad Guys",
                    "Bad Guys has copies_available=0 — it must not be the spoken rec",
                    "talking points Elena can say in 20 seconds",
                ],
            },
            {
                "student_id": "S-402",
                "name": "Aisha B",
                "role": "stretch_case",
                "why": "stretch toggle",
                "grade": 4,
                "homeroom_id": "H-4A",
                "reading_band": "above",
                "cluster": "fantasy",
                "history_titles": [
                    "Lightning Thief",
                    "Wings of Fire",
                    "Wild Robot",
                    "Dealing with Dragons",
                ],
                "show": [
                    "default recs stay in-series / adjacent fantasy",
                    "stretch toggle → Gregor, Amari, Endling — still catalog-closed",
                    "do not jump her to Westing Game just because lexile is high",
                ],
            },
            {
                "student_id": "S-405",
                "name": "Priya N",
                "role": "cold_start",
                "why": "CF empty; content + NL query must carry it",
                "grade": 4,
                "homeroom_id": "H-4B",
                "reading_band": "on",
                "cluster": "unknown",
                "history_titles": ["National Geographic Kids: Sharks"],
                "arrived": "2026-08-25",
                "show": [
                    "librarian types: funny, reluctant 4th, under 150 pages, we have copies",
                    "if you only had CF, this child gets nothing useful",
                ],
            },
            {
                "student_id": "S-509",
                "name": "Olivia I",
                "role": "cold_start_5",
                "why": "true cold start: zero checkouts",
                "grade": 5,
                "homeroom_id": "H-5B",
                "reading_band": "on",
                "cluster": "unknown",
                "history_titles": [],
                "arrived": "2026-09-02",
                "show": [
                    "pure NL / grade-band popular",
                    "honest about the limit",
                ],
            },
        ],
        "partners": [
            {"staff_id": "L-001", "name": "Elena", "watches": "Does the why sound like her, or like a bot"},
            {"staff_id": "L-002", "name": "Tom", "watches": "Can he do it in the lunch window without a training class"},
            {"staff_id": "L-003", "name": "Priya Shah", "watches": "Is student_id the only thing that leaves the building toward a model"},
        ],
    }


def as_is_json() -> dict:
    return {
        "owner": "Elena Vasquez",
        "role": "what we are replacing",
        "sticky_notes": [
            "Mateo / Jayden / Maya — Dog Man shelf, do not hand Charlotte's Web again",
            "Aisha — give her #2 and #3 the same day or she comes back tomorrow angry",
            "DeShawn — Ghost was a fluke, try Crossover",
            "New girl Priya — sharks, no idea yet",
            "Bad Guys is always out. Stop recommending it until copies come back.",
        ],
        "notebook_2026_05": [
            "Kids who liked Wimpy Kid this year also ate Big Nate and Lunch Lady. Investigators worked for two of them. The Bad Guys would have worked if we had copies.",
            "Grade 5 graphic readers (Tyler) get embarrassed if I walk them to the 2nd grade bins. Keep them in Dog Man / New Kid / Cat Kid and do not announce the level.",
            "Teachers email me 'something for a reluctant boy in 3B.' I guess. I want their last three checkouts in one screen.",
        ],
        "job": "Make that notebook queryable. Do not retire Elena.",
    }


def sources_index(counts: dict[str, int]) -> dict:
    return {
        "district": "Willow Bend School District (fictional)",
        "lighthouse": "Maple Street Elementary",
        "extract_date": "2026-09-04",
        "school_year": "2026-27",
        "ferpa": {
            "included": [
                "opaque student_id",
                "grade",
                "homeroom",
                "reading band",
                "first name + last initial",
            ],
            "excluded": [
                "last names",
                "DOB",
                "address",
                "parent contact",
                "state ID",
                "free/reduced lunch",
                "discipline",
            ],
            "llm_rule": "Prompts use student_id only. First names stay in the librarian console.",
        },
        "files": [
            {
                "id": "district",
                "json": "data/json/district.json",
                "origin": "hand-authored",
                "records": 1,
                "description": "Site, year, ILS, data owner, privacy officer",
            },
            {
                "id": "catalog",
                "json": "data/json/catalog.json",
                "csv": "data/csv/catalog.csv",
                "records": counts["catalog"],
                "description": "Card catalog slice. Closed world for recommendations.",
            },
            {
                "id": "students",
                "json": "data/json/students.json",
                "csv": "data/csv/students.csv",
                "records": counts["students"],
                "description": "Sample borrowers grades 3-5. Opaque IDs.",
            },
            {
                "id": "circulation",
                "json": "data/json/circulation.json",
                "csv": "data/csv/circulation.csv",
                "records": counts["circulation"],
                "description": "Checkouts spring 2026 + first three weeks of fall. CF source.",
            },
            {
                "id": "librarians",
                "json": "data/json/librarians.json",
                "csv": "data/csv/librarians.csv",
                "records": counts["librarians"],
                "description": "Staff who would actually touch the tool",
            },
            {
                "id": "homerooms",
                "json": "data/json/homerooms.json",
                "csv": "data/csv/homerooms.csv",
                "records": counts["homerooms"],
                "description": "Grade 3-5 classes in the specials rotation",
            },
            {
                "id": "library_hours",
                "json": "data/json/library_hours.json",
                "csv": "data/csv/library_hours.csv",
                "records": counts["library_hours"],
                "description": "Open/close",
            },
            {
                "id": "class_visits",
                "json": "data/json/class_visits.json",
                "csv": "data/csv/class_visits.csv",
                "records": counts["class_visits"],
                "description": "Weekly specials when a whole class is in the room",
            },
            {
                "id": "desk_shifts",
                "json": "data/json/desk_shifts.json",
                "csv": "data/csv/desk_shifts.csv",
                "records": counts["desk_shifts"],
                "description": "Who is on the desk",
            },
            {
                "id": "open_circulation",
                "json": "data/json/open_circulation.json",
                "csv": "data/csv/open_circulation.csv",
                "records": counts["open_circulation"],
                "description": "High-volume walk-up windows. ShelfMate's real job.",
            },
            {
                "id": "book_clubs",
                "json": "data/json/book_clubs.json",
                "csv": "data/csv/book_clubs.csv",
                "records": counts["book_clubs"],
                "description": "Optional programs",
            },
            {
                "id": "calendar_exceptions",
                "json": "data/json/calendar_exceptions.json",
                "csv": "data/csv/calendar_exceptions.csv",
                "records": counts["calendar_exceptions"],
                "description": "Closures",
            },
            {
                "id": "engagement_schedule",
                "json": "data/json/engagement_schedule.json",
                "csv": "data/csv/engagement_schedule.csv",
                "records": counts["engagement_schedule"],
                "description": "Proposed 12-week pro-bono lighthouse",
            },
            {
                "id": "personas",
                "json": "data/json/personas.json",
                "notes": "data/notes/personas.md",
                "records": 4,
                "description": "Demo walkthrough (Mateo, Aisha, Priya, Olivia)",
            },
            {
                "id": "as_is_notes",
                "json": "data/json/as_is_notes.json",
                "notes": "data/notes/as_is_notes.md",
                "records": 1,
                "description": "Elena's sticky notes and notebook. The system we are replacing.",
            },
        ],
    }


def main() -> None:
    JSON.mkdir(parents=True, exist_ok=True)
    district = json.loads((CSV / "district.source.json").read_text(encoding="utf-8"))
    dump("district.json", district)

    mapping = {
        "catalog.json": "catalog.csv",
        "students.json": "students.csv",
        "circulation.json": "circulation.csv",
        "librarians.json": "librarians.csv",
        "homerooms.json": "homerooms.csv",
        "library_hours.json": "library_hours.csv",
        "class_visits.json": "class_visits.csv",
        "desk_shifts.json": "desk_shifts.csv",
        "open_circulation.json": "open_circulation.csv",
        "book_clubs.json": "book_clubs.csv",
        "calendar_exceptions.json": "calendar_exceptions.csv",
        "engagement_schedule.json": "engagement_schedule.csv",
    }
    counts = {}
    for json_name, csv_name in mapping.items():
        rows = load_csv(csv_name)
        dump(json_name, rows)
        counts[csv_name.replace(".csv", "")] = len(rows)

    dump("personas.json", personas_json())
    dump("as_is_notes.json", as_is_json())
    dump("sources.json", sources_index(counts))


if __name__ == "__main__":
    main()
