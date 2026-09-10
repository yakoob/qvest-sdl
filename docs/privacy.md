# Privacy (FERPA / minors)

Fictional extract. Treat it as if it were real. Implemented controls are not a legal opinion and do not prove compliance.

## In the JSON

Included: opaque `student_id`, grade, homeroom, reading band, first name + last initial.  
Excluded: last names, DOB, address, parent contact, state ID, free/reduced lunch, discipline.

## In the product (implemented)

- Librarian console may show first name and the desk anecdote. That stays on localhost.
- HTTP student DTO is allowlisted (no lexile, no last name, no DOB).
- Anything toward a model: `student_id` + candidate book metadata/evidence + stretch/page flags. No raw query, no blurbs (a fixture blurb contains a student name), no anecdotes, no first names. Test: `TestPayloadPrivacy`.
- Audit JSONL: `student_id`, `staff_id`, ranked `book_ids`, dropped reasons, explain mode, version, query *flags* (present / under_150 / short). No raw query, names, talking-point prose, or model bodies.
- Talking points are labeled librarian-reviewed drafts. ID validation is not a claim that the prose is true.
- No student-facing UI.
- Synthetic academics (`academic_demo.json`) stay on the desk. They are not sent to a model and are not written to audit JSONL. First names still do not leave the console toward a model.
- Demo checkout is localhost process memory. It is not a production ILS write.
- Support bands and teacher observations are local display context, not recommendation/model/audit inputs. Only explicit neutral reading-theme choices affect local catalog search.
- Counselor fixtures contain deliberately shared reading themes and approval dates, not clinical records, diagnoses or disciplinary histories. They never influence support bands.
- Queue responses omit note bodies; full guidance is available on selected-student detail. This is data minimization, NOT authorization: selecting a staff name is attribution only. Real records require district-approved role-based access.
- Follow-ups are structured process-memory events. Restart clears them; check-in and enjoyment reports do not change academic evidence.

## Kill switch

`SHELFMATE_LLM=off` (default). Retrieval still runs. Optional Axon uses `LLM_BASE`, `LLM_MODEL`, `LLM_API_KEY` (or `OPENAI_API_KEY`). Keys are never written to source or audit.

## Not implemented (production work)

- District SSO / staff auth
- Destiny/Alexandria nightly export + redaction
- Privacy-officer written approval before a live model
- Network egress controls beyond “bind localhost”
- Retention / deletion policy for audit files
