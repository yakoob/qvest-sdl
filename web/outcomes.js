/* Tables keep denominators and supporting records visible without color coding. */
window.engagement.loadOutcomes = async function () {
  const target = document.getElementById("outcomes-body");
  const navigation = el("div", { class: "engagement-actions", "aria-label": "Outcome sections" }, [
    this.button("Librarian activity", () => { this.outcomeSection = "activity"; return this.loadOutcomes(); }),
    this.button("Student progress", () => { this.outcomeSection = "progress"; return this.loadOutcomes(); }),
  ]);
  if (this.outcomeSection === "progress") return this.loadStudentProgress(target, navigation);
  this.progressSeq = (this.progressSeq || 0) + 1;
  const staff = el("select", {}, [el("option", { value: "", text: "All librarians (deduplicated)" }), ...[...document.getElementById("staff").options].map(o => el("option", { value: o.value, text: o.text }))]);
  staff.value = this.metricsStaff || "";
  const start = el("input", { type: "date", value: this.metricsStart || "" });
  const end = el("input", { type: "date", value: this.metricsEnd || "" });
  const controls = el("div", { class: "agenda-controls" }, [el("label", {}, ["Actual facilitator", staff]), el("label", {}, ["From (blank: session start)", start]), el("label", {}, ["Until (exclusive; blank: tomorrow)", end]), this.button("Apply filters", () => { this.metricsStaff = staff.value; this.metricsStart = start.value; this.metricsEnd = end.value; return this.loadOutcomes(); })]);
  const seq = this.metricsSeq = (this.metricsSeq || 0) + 1;
  const res = await fetch(`/api/metrics?${new URLSearchParams({ staff_id: staff.value, start: start.value, end: end.value, source: "session" })}`);
  const d = await res.json(); if (seq !== this.metricsSeq || this.view !== "outcomes") return;
  target.replaceChildren(navigation, controls);
  if (!res.ok) { target.append(el("p", { role: "alert", text: d.error || "Could not load outcomes" })); return; }
  const ratio = v => v.denominator ? `${v.numerator} / ${v.denominator} (${Math.round(100*v.numerator/v.denominator)}%)` : `${v.numerator} / 0 · not yet available`;
  const table = (headers, rows) => el("div", { class: "table-scroll", tabindex: "0" }, [el("table", { class: "engagement-table" }, [el("thead", {}, [el("tr", {}, headers.map(h => el("th", { scope: "col", text: h })))]), el("tbody", {}, rows.map(row => el("tr", {}, row.map(v => el("td", { text: v })))))] )]);
  target.append(el("p", { class: "hint", text: `${d.source} · ${this.local(d.filter.start)} to ${this.local(d.filter.end)} (exclusive) · as of ${this.local(d.filter.as_of)}. Facilitator: ${d.filter.staff_id || "all"}. Per-staff distinct totals may overlap; never sum them.` }), table(["Measure", "Count / denominator", "Definition"], [
    ["Students served", d.students_served, "Distinct students with explicit completed conversations in period"],
    ["Completed conversations", d.completed, "Attributed to actual facilitator, not appointment owner or circulation staff"],
    ["Students choosing a book", d.students_choosing, "Distinct served students with accepted book choices"],
    ["None today", d.none_today, "Explicit no-book choices"],
    ["Linked checkouts", d.linked_checkouts, "Exact choice-to-loan links; returns do not erase them"],
    ["Recommendation acceptance", ratio(d.acceptance), "Completed interactions accepting a book / completed interactions where candidates were offered"],
    ["Choice-to-checkout in 14 days", ratio(d.conversion), `${d.pending_choices} choices pending full observation window; unit is choices`],
    ["Student-reported completion", ratio(d.reading_completion), `${d.unknown_reading} unknown/absent reading statuses; never inferred from returns`],
    ["Student-reported enjoyment", ratio(d.enjoyment), `${d.neutral} neutral, ${d.unknown_enjoyment} unknown/absent; denominator is yes + no`],
    ["Student response coverage", ratio(d.response_coverage), "Latest student reports / accepted interaction-book pairs in completion cohort"],
    ["Staff-observed book pairs", d.staff_observations, "Separate from student reports"],
    ["Due follow-up coverage", ratio(d.followup_coverage), `${d.overdue} overdue, ${d.pending_followups} pending; due-date cohort, attributed to original facilitator`],
  ]));
  target.append(el("h2", { text: "Supporting conversations" }), table(["Interaction", "Student", "Facilitator", "Completed"], d.interactions.map(i => [i.id, this.name(i.student_id), i.facilitator, this.local(i.completed_at)])));
  target.append(el("h2", { text: "Choices and loan links" }), table(["Choice / interaction", "Book / source", "Accepted", "Loan / checkout"], d.choices.map(c => [`${c.id} / ${c.interaction_id}`, `${c.book_id || "None today"} / ${c.source}`, this.local(c.at), `${c.loan_id || "Not linked"}${c.checkout_at ? " / "+this.local(c.checkout_at) : ""}`])));
  target.append(el("h2", { text: "Latest student-reported book evidence" }), table(["Interaction / student", "Book / loan", "Reading", "Enjoyment", "Source"], d.pairs.map(p => [`${p.interaction_id} / ${this.name(p.student_id)}`, `${p.book_id} / ${p.loan_id || "no loan"}`, p.reading, p.enjoyment, p.source])));
  target.append(el("h2", { text: "Follow-up records" }), table(["Follow-up / interaction", "Due", "Contact completed", "Recorder"], d.followups.map(f => [`${f.id} / ${f.interaction_id}`, this.local(f.due), f.completed_at ? this.local(f.completed_at) : "Not recorded", f.staff_id || "—"])));
  target.append(el("p", { class: "hint", text: "Paired borrowing and academic outcomes and illustrative historical engagement are deferred. Existing student academic scenarios stay separate. These counts do not establish causal staff effectiveness." }));
};
