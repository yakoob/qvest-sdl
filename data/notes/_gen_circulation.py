#!/usr/bin/env python3
"""One-off generator for fictional circulation.csv. Not the product."""
import csv
import random
from collections import Counter
from datetime import date, timedelta
from pathlib import Path

root = Path(__file__).resolve().parent

students = list(csv.DictReader((root / "students.csv").open()))
for s in students:
    for k, v in list(s.items()):
        if isinstance(v, str):
            s[k] = v.strip()

books = list(csv.DictReader((root / "catalog.csv").open()))
by_cluster: dict[str, list] = {}
for b in books:
    by_cluster.setdefault(b["cluster"], []).append(b)

adjacent = {
    "graphic": ["short", "realistic"],
    "fantasy": ["mystery", "short"],
    "realistic": ["animals", "sports", "graphic"],
    "sports": ["realistic", "short"],
    "animals": ["realistic", "short"],
    "mystery": ["fantasy", "realistic"],
    "unknown": ["short", "graphic"],
    "short": ["graphic", "sports"],
}


def ok_grade(b, grade: int) -> bool:
    return int(b["grade_min"]) <= grade <= int(b["grade_max"])


def pick(rng, pool, exclude):
    cand = [b for b in pool if b["book_id"] not in exclude]
    rng.shuffle(cand)
    return cand[:1]


def weekdays(start, end):
    d = start
    out = []
    while d <= end:
        if d.weekday() < 5:
            out.append(d)
        d += timedelta(days=1)
    return out


def sample_days(pool, n, r):
    if not pool or n <= 0:
        return []
    n = min(n, len(pool))
    return sorted(r.sample(pool, n))


spring = weekdays(date(2026, 1, 13), date(2026, 6, 5))
fall = weekdays(date(2026, 8, 18), date(2026, 9, 3))

planted = {
    "S-406": ["B-003", "B-001", "B-002", "B-004", "B-005", "B-001", "B-006"],
    "S-301": ["B-001", "B-003", "B-007", "B-002"],
    "S-402": ["B-010", "B-015", "B-012", "B-014", "B-010"],
    "S-401": ["B-010", "B-011", "B-015"],
    "S-405": ["B-062"],
    "S-509": [],
    "S-504": ["B-001", "B-002", "B-007", "B-025"],
    "S-403": ["B-026", "B-032", "B-030"],
    "S-502": ["B-016", "B-012", "B-017", "B-013"],
}

rows = []


def add_event(sid, bid, d, staff, rng):
    ret = d + timedelta(days=rng.choice([7, 7, 14, 14, 21]))
    rows.append(
        {
            "student_id": sid,
            "book_id": bid,
            "checkout_date": d.isoformat(),
            "due_date": (d + timedelta(days=14)).isoformat(),
            "return_date": ret.isoformat() if ret <= date(2026, 9, 4) else "",
            "staff_id": staff,
            "channel": "desk",
        }
    )


for s in students:
    sid = s["student_id"]
    grade = int(s["grade"])
    cluster = s["cluster"]
    new = s["new_this_year"].lower() == "true"
    rng_s = random.Random(int(sid.replace("S-", "")) * 97 + 42)

    if sid in planted:
        seq = planted[sid][:]
    else:
        n = rng_s.randint(5, 11)
        if new:
            n = rng_s.randint(0, 2)
        primary = [b for b in by_cluster.get(cluster, []) if ok_grade(b, grade)]
        adj = []
        for c in adjacent.get(cluster, []):
            adj.extend([b for b in by_cluster.get(c, []) if ok_grade(b, grade)])
        seq = []
        exclude = set()
        for _ in range(n):
            roll = rng_s.random()
            if roll < 0.7:
                pool = primary
            elif roll < 0.9:
                pool = adj
            else:
                pool = [b for b in books if ok_grade(b, grade)]
            if not pool:
                pool = [b for b in books if ok_grade(b, grade)]
            choice = pick(rng_s, pool, exclude)
            if not choice:
                choice = pick(rng_s, pool, set())
            if choice:
                seq.append(choice[0]["book_id"])
                exclude.add(choice[0]["book_id"])

    if not seq:
        continue

    if new:
        days = fall[:]
        if sid == "S-405":
            days = [date(2026, 8, 26)]
        elif sid == "S-509":
            continue
    else:
        days = spring + fall

    if (not new) and len(seq) >= 3:
        spring_n = len(seq) - rng_s.randint(1, 2)
    else:
        spring_n = len(seq)

    spring_days = [d for d in days if d < date(2026, 8, 1)]
    fall_days = [d for d in days if d >= date(2026, 8, 1)]
    s_days = sample_days(spring_days, spring_n if spring_days else 0, rng_s)
    f_need = len(seq) - len(s_days)
    f_days = sample_days(fall_days, f_need, rng_s)
    used_days = s_days + f_days
    if len(used_days) < len(seq):
        extra_pool = [d for d in days if d not in used_days]
        used_days += sample_days(extra_pool, len(seq) - len(used_days), rng_s)
    used_days = sorted(used_days)[: len(seq)]

    for bid, d in zip(seq, used_days):
        staff = "L-002" if rng_s.random() < 0.45 else "L-001"
        add_event(sid, bid, d, staff, rng_s)

rows.sort(key=lambda r: (r["checkout_date"], r["student_id"], r["book_id"]))
out_rows = []
for i, r in enumerate(rows, 1):
    item = {"event_id": f"C-{i:04d}", **r}
    out_rows.append(item)

path = root / "circulation.csv"
fields = [
    "event_id",
    "student_id",
    "book_id",
    "checkout_date",
    "due_date",
    "return_date",
    "staff_id",
    "channel",
]
with path.open("w", newline="") as handle:
    writer = csv.DictWriter(handle, fieldnames=fields)
    writer.writeheader()
    writer.writerows(out_rows)

print(f"events={len(out_rows)} students={len({r['student_id'] for r in out_rows})}")
print("top", Counter(r["student_id"] for r in out_rows).most_common(6))
print("Olivia", sum(1 for r in out_rows if r["student_id"] == "S-509"))
print("Priya", [(r["book_id"], r["checkout_date"]) for r in out_rows if r["student_id"] == "S-405"])
print("Mateo", [(r["book_id"], r["checkout_date"]) for r in out_rows if r["student_id"] == "S-406"])
print("Aisha", [(r["book_id"], r["checkout_date"]) for r in out_rows if r["student_id"] == "S-402"])
print("open_returns", sum(1 for r in out_rows if r["return_date"] == ""))
