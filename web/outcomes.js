/* Outcomes presents the existing API metrics without changing their units. */
window.engagement.loadOutcomes = async function () {
  const target = document.getElementById("outcomes-body");
  this.outcomeSection ||= "activity";
  const navigation = el("nav", { class: "outcome-tabs", "aria-label": "Outcome sections" });
  for (const [key, label] of [["activity", "Our work"], ["paired", "Reading changes"], ["progress", "Student trends"]]) {
    const button = this.button(label, () => { this.outcomeSection = key; return this.loadOutcomes(); });
    button.setAttribute("aria-pressed", String(key === this.outcomeSection));
    navigation.append(button);
  }
  if (this.outcomeSection === "paired") return this.loadPairedOutcomes(target, navigation);
  if (this.outcomeSection === "progress") return this.loadStudentProgress(target, navigation);
  const staff = el("select", {}, [el("option", { value: "", text: "All librarians" }), ...[...document.getElementById("staff").options].map(o => el("option", { value: o.value, text: o.text }))]);
  staff.value = this.metricsStaff || "";
  const preset = el("select", {}, [["session", "This session"], ["today", "Today"], ["week", "Last 7 days"], ["custom", "Custom dates"]].map(([value, text]) => el("option", { value, text })));
  preset.value = this.workPreset || "session";
  const start = el("input", { type: "date", value: this.metricsStart || "" });
  const end = el("input", { type: "date", value: this.metricsEnd || "" });
  const apply = () => { this.metricsStaff = staff.value; this.workPreset = preset.value; this.metricsStart = start.value; this.metricsEnd = end.value; return this.loadOutcomes(); };
  const custom = charts.disclosure("Custom dates", el("div", { class: "agenda-controls" }, [el("label", {}, ["From", start]), el("label", {}, ["Through", end]), this.button("Apply dates", () => { preset.value = "custom"; return apply(); })]));
  custom.open = preset.value === "custom";
  for (const select of [staff, preset]) select.addEventListener("change", () => apply().catch(e => this.error(e)));
  const controls = el("div", { class: "outcome-filters" }, [el("label", {}, ["Period", preset]), el("label", {}, ["Librarian", staff]), custom]);
  let from = "", until = "";
  if (preset.value === "today" || preset.value === "week") { from = charts.shiftDate(this.day(), preset.value === "week" ? -6 : 0); until = charts.shiftDate(this.day(), 1); }
  if (preset.value === "custom") { from = start.value; until = end.value ? charts.shiftDate(end.value, 1) : ""; }
  return charts.report(target, `/api/metrics?${new URLSearchParams({ staff_id: staff.value, start: from, end: until, source: "session" })}`, d => {
    const ratio = r => r.denominator ? `${r.numerator} / ${r.denominator}` : "Not enough information";
    target.replaceChildren(navigation, controls, el("div", { class: "outcome-intro" }, [el("span", { class: "source-badge", text: "Live session" }), el("h2", { text: "Every conversation is a start." }), el("p", { text: `${this.day(d.filter.start)} through ${charts.shiftDate(this.day(d.filter.end), -1)} · ${charts.staff(d.filter.staff_id) || "All librarians"}. Restart clears live activity.` })]),
      el("div", { class: "stat-grid" }, [charts.stat("Students helped", d.students_served, "Distinct readers with a completed conversation"), charts.stat("Conversations completed", d.completed, "Explicitly finished visits"), charts.stat("Linked checkouts", d.linked_checkouts, "Loans linked to a recorded book choice")]),
      el("div", { class: "chart-grid" }, [charts.ratio("Reported finished", d.reading_completion, `${d.unknown_reading} book pairs have no known reading status`), charts.ratio("Reported enjoyment", d.enjoyment, `${d.neutral} neutral · ${d.unknown_enjoyment} unknown; yes/no reports only`), charts.ratio("Follow-ups completed", d.followup_coverage, `${d.overdue} overdue · ${d.pending_followups} not yet due`)]),
      el("p", { class: "coverage-note", text: `Student feedback received for ${ratio(d.response_coverage)} chosen book pairs. Borrowing is not proof of finishing or enjoyment.` }));
    if (!d.completed) target.append(el("div", { class: "empty-state" }, [el("h3", { text: "Your next conversation starts the story." }), el("p", { text: "Complete a visit to see it here. Nothing is filled in automatically." }), this.button("Find a student", () => this.showView("students")), this.button("See illustrative reading changes", () => { this.outcomeSection = "paired"; return this.loadOutcomes(); })]));
    target.append(charts.disclosure("More activity measures", charts.table(["Measure", "Result"], [["Students choosing a book", d.students_choosing], ["None today", d.none_today], ["Recommendation acceptance", ratio(d.acceptance)], ["Choice-to-checkout within 14 days", `${ratio(d.conversion)} · ${d.pending_choices} choices still being observed`], ["Staff-observed book pairs (separate)", d.staff_observations]])),
      charts.disclosure("View records", el("h3", { text: "Conversations" }), charts.table(["Student", "Librarian", "Completed", "Record"], d.interactions.map(i => [this.name(i.student_id), charts.staff(i.facilitator), this.local(i.completed_at), i.id])), el("h3", { text: "Choices and loans" }), charts.table(["Choice", "Conversation", "Book", "Source", "Chosen", "Loan / checkout"], d.choices.map(c => [c.id, c.interaction_id, c.book_id || "None today", c.source, this.local(c.at), `${c.loan_id || "Not linked"} ${c.checkout_at ? this.local(c.checkout_at) : ""}`])), el("h3", { text: "Latest student reports" }), charts.table(["Student", "Conversation / book", "Reading", "Enjoyment", "Source"], d.pairs.map(p => [this.name(p.student_id), `${p.interaction_id} / ${p.book_id}`, p.reading, p.enjoyment, p.source])), el("h3", { text: "Follow-ups" }), charts.table(["Record", "Due", "Completed", "Recorder"], d.followups.map(f => [f.id, this.local(f.due), f.completed_at ? this.local(f.completed_at) : "Not yet", charts.staff(f.staff_id)]))),
      charts.disclosure("How this is counted", el("p", { text: "Students are deduplicated across completed conversations. Each ratio has its own unit and denominator; these cards are not a funnel. Choice conversion requires a complete 14-day observation window. Unknown feedback is not a negative report. Follow-ups use the due-date cohort and original facilitator. These records describe activity, not causal impact." })));
  });
};
