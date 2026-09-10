# Engagement metric contracts

ShelfMate is a fictional, localhost librarian prototype. These are descriptive activity and book-feedback measures, not evidence that librarian contact causes academic improvement. Staff selection is attribution, not authorization.

## Scope

The Friday-critical slice reports process-memory engagement records from **This server session**. Restart clears newly recorded engagement actions and restores operational inventory. Existing academic/borrowing scenarios and generic student check-ins are not engagement events and must not be silently included. Historical engagement examples, if supplied, require an explicitly separate source and declared coverage. Paired borrowing and academic aggregates are deferred.

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

## Evidence and limitations

Every reported number needs supporting IDs and dates, and ratio rows need their numerator, denominator and exclusions. The source and filter labels must remain visible in the UI and drill-down. Missing academic data does not exclude students from the activity cohort. A newly completed session conversation has no mature 14-day conversion window and no months-later outcomes.

Production requires district-approved access controls, sensitive-cohort suppression, retention policy and export coverage validation. None are implied by localhost staff selectors. Feedback-based retraining, production calendar/ILS integration, and causal effectiveness claims are outside this prototype.
