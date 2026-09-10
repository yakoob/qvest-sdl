# Pilot proposal and delivery gates

## Thesis

Help librarians scale trusted conversations and connect students with books they want to keep reading. Borrowing history and an electronic catalog are the guaranteed inputs. Teacher guidance, academic records and counselor context are optional extensions requiring separate approvals; they are not prerequisites for the reading workflow.

The prototype is a fictional, local demonstration—not a production district service and not evidence of effectiveness. The target librarian journey is:

```text
Needs attention → Schedule → Conversation → Choose together
                                               ↓
Review outcomes ← Book-specific follow-up ← Checkout
```

Check the runbook and implemented UI for current scope. Do not present a planned feature as built.

## Architecture and production boundary

```text
Catalog + borrowing extract → retrieve → availability/policy → candidate books
                                      ↓                          ↓
Internal appointments → librarian conversation → recorded choice → inventory loan
                                      ↓                          ↓
                               explicit completion → book feedback → descriptive totals

Optional explanation drafts are downstream of retrieve; no model is needed for the core path.
```

```text
LOCAL FICTIONAL PROTOTYPE                  DISTRICT PILOT (approval-gated)
JSON sources + process memory             Approved exports + retention
Staff attribution selector                SSO + district role permissions
Internal appointment book                 Validated staff calendar / school constraints
Demo checkout                             Approved ILS integration or read-only export
Explicit source/coverage labels           Suppression + authorized drill-downs
Default template explanation              Approved model egress, if any
Restart resets session                    Backups, deletion, monitoring, support ownership
```

A staff selector is not access control. Academic scenario curves do not demonstrate that librarian conversations caused improvement. Book feedback is explicit evidence; checkouts and returns alone are not completion or enjoyment reports. The current retrieval policy does not learn from newly recorded feedback.

## Delivery estimate — not a fixed-price promise

| Phase | Elapsed planning range | Gate / assumption |
| --- | --- | --- |
| Privacy, export discovery and librarian co-design | 1–2 weeks | District library lead and privacy/IT contacts available; examine actual export quality |
| Bounded pilot build and integration | 3–4 weeks | Two engineers; fractional UX/delivery; stable catalog/borrowing schema |
| Onboarding and rehearsal | 1–2 weeks | Protected staff time; fallback procedure rehearsed |
| Measured school pilot | 6–8 weeks | Agreed baseline, response coverage and burden measures |
| Total | Approximately 11–16 weeks | Re-estimate at discovery gate; holidays and approvals can extend elapsed time |

Finishing the local PoC is a separate estimate: approximately 4–6 focused engineer-days for the full proposed engagement extension and regression, assuming no external integrations and an otherwise stable app. A Friday-critical cut defers historical paired outcomes rather than presenting unfinished analytics as working.

Grades/counselor connectors, calendar writes, notifications and production ILS writeback are separately scoped. Hosting/model/support costs need volume assumptions and provider quotes; no dollar figure is implied here.

## Ownership

- District library lead: reading policy, workflow acceptance and librarian co-design.
- District IT: access controls, approved imports, hosting and operational integration.
- Named pilot engineering team: defects, monitoring, incident response and handover materials.
- Privacy lead: retention, permissions, small-cohort disclosure rules and model egress approvals.
- School leadership: protected librarian time, training and feedback collection.

## Evaluation and decision gates

Establish a baseline before rollout. Measure explicit student reading/enjoyment reports with response coverage, librarian effort, adoption, distinct students served and linked choices/checkouts with mature observation windows. Missing responses remain unknown. Separate descriptive student outcomes from claims about staff effectiveness.

Where feasible, use a comparison or staggered rollout to investigate confounding: popular titles, shelf access, prior motivation and teacher support may all affect observed change. Any academic pairing needs compatible instrument/course conditions, full observation windows and exclusion reasons. Do not infer grade gains from illustrative fixtures.

Proceed from discovery to a bounded pilot only after data/privacy acceptance and librarian usability checks. The final decision is adopt, revise or stop based on evidence and staff burden—not a synthetic chart. Handover includes runbooks, named support ownership and a supported fallback when the optional model is unavailable.
