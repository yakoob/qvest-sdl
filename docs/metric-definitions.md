# Engagement metric contracts

ShelfMate is a fictional, localhost librarian prototype. These are descriptive activity and book-feedback measures, not evidence that librarian contact causes academic improvement. Staff selection is attribution, not authorization.

## Scope

Operational activity reports process-memory records from **This server session**. Restart clears those actions and restores inventory. The separate **Students served · observed changes** section supports illustrative historical contacts and a session cohort. Historical contacts and explicit window coverage come from `engagement_demo.json`; they never seed live appointments, loans or feedback. Generic student check-ins are not engagement evidence.

All metrics must derive from typed records, not stored KPI totals. Summary and supporting records use the same source, period, report-as-of instant and facilitator filter. Time ranges are half-open `[start, end)`; display dates use the school timezone. Exclude evidence recorded after report-as-of. Return exact counts and use an unavailable ratio, not zero percent, for an empty denominator.

## Activity cohort and attribution

- **Completed conversations:** interactions explicitly completed in the reporting period, using `completed_at`, attributed to the actual facilitator. Opening a conversation or checking out does not complete it.
- **Students served:** distinct student IDs in those completed interactions. District totals deduplicate across facilitators. Per-librarian distinct counts may overlap and cannot be added into a district distinct count.
- **Students choosing a book:** distinct served students with accepted choices in cohort interactions. A recorded “none today” is visible but is not a book choice.
- **Linked checkouts:** successful loans explicitly linked to an accepted choice in cohort interactions; never join on book/date heuristics. Returns do not erase the original linkage. Unlinked legacy desk loans do not become engagement checkouts.
- **Appointments:** show scheduled, in-progress, completed, cancelled and no-show separately. A reschedule changes the original appointment and appends a trace event, not another completed conversation. Appointment ownership and circulation staff are not substitutes for actual facilitator attribution.

## Acceptance and conversion

- **Recommendation acceptance:** completed cohort interactions with at least one accepted choice divided by completed cohort interactions with an offered candidate set. Explicit in-catalog librarian choices must be distinguishable from accepting displayed recommendations.
- **Choice-to-checkout conversion:** accepted choice records with a full 14-day observation window form the denominator (unit: choices, not students). Numerator choices have an explicit checkout link dated from acceptance through 14 days afterward. Report pending-window choices separately. A checkout later than 14 days is a linked checkout but not a conversion within the window. Never label this conversion of all recommendations.

## Book feedback

Feedback attaches to an interaction/book pair, optionally its loan. Recorder, timestamp and source are retained. Student-reported responses and staff observations are distinct; staff observations must not be presented as student reports. Generic legacy follow-ups remain generic and never count as book completion or enjoyment.

- **Reading status:** finished, reading, not started, stopped, unknown. Use the latest explicit response per eligible interaction/book pair available by report-as-of. Repeated reports do not inflate the pair count. Completion is finished / known reading-status pairs. Unknown or absent reports are not unfinished.
- **Enjoyment:** yes, no, neutral, unknown. Positive enjoyment is yes / (yes + no). Neutral and unknown remain visible outside that denominator.
- **Response coverage:** report the number of eligible book pairs with a report alongside all eligible book pairs. Report reading-known and enjoyment-known counts separately; a response of unknown is contact evidence but not a known outcome.
- **Follow-up coverage:** completed due follow-up records / eligible due follow-up records in the reporting period, observed by report-as-of. Future due records are pending, not overdue. Only an explicit completed follow-up counts as contact; a checkout, return or generic action does not.

## Reservation-backed follow-ups

Booking a follow-up creates a real appointment under the same conflict validator used for other meetings. New bookings use available five-minute-start slots with explicit staff confirmation. Completing the linked follow-up conversation records contact; cancellation and no-show do not. The originating interaction retains outcome attribution even when another staff member conducts the follow-up.

Reservation history is dated. Reports use the due time known at report-as-of; once a due time has passed, a later rebooking cannot remove that overdue obligation from its original reporting cohort. The agenda shows the current reservation, which can differ from the metric's retained due date. One fulfilled obligation counts once.

## Student-progress portfolio

Outcomes also projects the same `academics.Catalog.View` records used by each student workspace. Portfolio data is not attributed to a librarian. Extract plus session borrowing and the isolated historical scenario are separate sources.

- Checkout events include repeated borrowing/renewals. Student–title pairs sum unique titles per student; district distinct titles deduplicate book IDs across students in a period.
- Full, partial and missing coverage counts account for every roster student per period. Missing windows are unknown. An observed event outside full coverage does not establish coverage for the entire window.
- English results are exact letter-grade distributions, never averaged GPA or a percentage gain.
- Reading observation deltas compare only the same student, instrument, scale and grade form. Different forms are not subtracted.
- All six 84-day scenario windows remain intact. Per-student records and period aggregates reconcile; source values are not changed to create a preferred trend.

## Served-cohort paired observations

`GET /api/metrics/paired` uses `source=historical|session`, contact-period `start`/exclusive `end`, optional `staff_id`, and an RFC3339 `as_of` no later than now. Historical defaults cover 2025-07-01 through 2026-07-01. Session defaults cover the current server session. The UI's optional as-of date means the start of that UTC day, explicitly labelled.

- Select the first completed contact per student in the period before applying the facilitator filter. Later completed contacts stay visible; cancelled/no-show contacts do not establish service.
- Historical borrowing uses the nearest eligible pre-contact and first eligible post-contact 84-day windows within ±365 days. Both must have declared enrollment/coverage and be fully observed. Counts include repeats/renewals; these are events, not distinct titles.
- English uses the nearest valid pre-contact and first compatible post-contact observation within ±180 days, matching course, school grade and letter scale. Letter ordering classifies direction only.
- Reading uses the same ±180-day rule and matching instrument/scale/grade form. No pooled score or cross-form subtraction.
- Date-only observations must be strictly before or after the school-local contact date. They become available only after their school day ends. No same-day ordering is invented.
- Each measure reports eligible N, increased/unchanged/decreased counts and excluded reasons, calculated directly from the returned student evidence rows. Increases are descriptive, not proof of benefit.
- Session contacts cannot borrow historical scenario evidence. Without declared full enrollment/export coverage, session pairs are explicitly excluded. Missing optional academics or historical fixtures do not block the librarian workflow.

The historical fixture is limited to contacts and coverage links for paired observations. It does not seed historical recommendation offers, loan conversions, or book-feedback funnels. Its historical meeting times are explicitly illustrative, not validated against current-year staff shifts.

## Evidence and limitations

Every reported number needs supporting IDs and dates, and ratio rows need their numerator, denominator and exclusions. The source and filter labels must remain visible in the UI and drill-down. Missing academic data does not exclude students from the activity cohort. A newly completed session conversation has no mature 14-day conversion window and no months-later outcomes.

Production requires district-approved access controls, sensitive-cohort suppression, retention policy and export coverage validation. None are implied by localhost staff selectors. Feedback-based retraining, production calendar/ILS integration, and causal effectiveness claims are outside this prototype.
