# Privacy (FERPA / minors)

Fictional extract. Treat it as if it were real.

## In the JSON

Included: opaque `student_id`, grade, homeroom, reading band, first name + last initial.  
Excluded: last names, DOB, address, parent contact, state ID, free/reduced lunch, discipline.

## In the product

- Librarian console may show first name. That stays on the desk laptop.
- Anything toward a model: `student_id` + `book_id`s only.
- Audit JSONL records `student_id`, `staff_id`, recommended `book_ids`, dropped reasons. No first names.
- LLM default off. District privacy officer written yes before it goes on (engagement week 0).
- No student-facing UI in year 1.

## Kill switch

`SHELFMATE_LLM=off` (default). Retrieval does not import an HTTP LLM client.
