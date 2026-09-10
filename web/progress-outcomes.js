/* The portfolio uses the same academic projections as each student workspace. */
window.engagement.loadStudentProgress = async function (target, navigation) {
  this.metricsSeq = (this.metricsSeq || 0) + 1;
  const seq = this.progressSeq = (this.progressSeq || 0) + 1;
  const source = el("select", {}, [el("option", {value:"scenario", text:"Illustrative historical portfolio"}), el("option", {value:"extract", text:"Extract + session borrowing / optional academics"})]);
  source.value = this.progressSource || "scenario";
  source.addEventListener("change", () => { this.progressSource = source.value; this.progressPeriod = null; this.loadOutcomes().catch(e => this.error(e)); });
  target.replaceChildren(navigation, el("label", {}, ["Student data source", source]), el("p", {role:"status", text:"Loading student progress…"}));
  const response = await fetch(`/api/metrics/progress?${new URLSearchParams({source:source.value})}`);
  const report = await response.json();
  if (seq !== this.progressSeq || this.view !== "outcomes" || this.outcomeSection !== "progress") return;
  target.replaceChildren(navigation, el("label", {}, ["Student data source", source]));
  if (!response.ok) { target.append(el("p", {role:"alert", text:report.error || "Could not load progress"})); return; }
  const table = (headers, rows) => el("div", {class:"table-scroll", tabindex:"0"}, [el("table", {class:"engagement-table"}, [el("thead", {}, [el("tr", {}, headers.map(h => el("th", {scope:"col", text:h})))]), el("tbody", {}, rows.map(row => el("tr", {}, row.map(value => el("td", {}, [value == null ? "—" : typeof value === "object" ? value : String(value)])))))])]);
  const studentLink = id => this.button(this.name(id), async () => {
    await selectStudent(id); switchTab("progress", true);
    if (source.value === "extract") { const details = document.querySelector(".extract-coverage"); if (details) { details.open = true; details.scrollIntoView({block:"start"}); } }
  });
  const gradeOrder = ["A+","A","A-","B+","B","B-","C+","C","C-","D+","D","D-","F"];
  target.append(el("h2", {text:"Student progress · all students"}), el("p", {class:"hint", text:report.note}), el("p", {class:"hint", text:"This portfolio is not attributed to a librarian. Missing borrowing windows are unknown, not zero; partial-window totals are observed events only."}));
  target.append(table(["Period", "Students with records", "Coverage: full / partial / missing", "Observed checkout events", "Student–title pairs", "District distinct titles", "English grade counts / missing"], report.periods.map(p => [
    `${p.id} (${p.start} – ${p.end})`, p.students, `${p.covered} / ${p.partial} / ${p.missing}`, p.checkouts, p.student_titles, p.distinct_titles,
    `${gradeOrder.filter(g => p.grades[g]).map(g => `${g}: ${p.grades[g]}`).join(" · ") || "No grades"} / missing ${p.missing_grades}`,
  ])));
  const period = el("select", {}, [el("option", {value:"",text:"All periods"}), ...report.periods.map(p => el("option", {value:p.id,text:`${p.id}: ${p.start} – ${p.end}`}))]);
  period.value = this.progressPeriod ?? report.periods.at(-1)?.id ?? "";
  const detail = el("div");
  const renderDetails = () => {
    const rows = report.rows.filter(r => !period.value || r.period === period.value);
    const selectedPeriod = report.periods.find(p => p.id === period.value);
    const readings = report.reading.filter(r => !selectedPeriod || (r.date >= selectedPeriod.start && r.date <= selectedPeriod.end));
    detail.replaceChildren(el("h3", {text:"Student records behind the totals"}), table(["Student", "Period", "Coverage", "Observed checkouts", "Unique titles", "English", "Borrowed titles"], rows.map(r => [studentLink(r.student_id), r.period, r.coverage, r.coverage === "missing" && !r.checkouts ? "Unknown" : r.checkouts, r.coverage === "missing" && !r.unique_titles ? "Unknown" : r.unique_titles, r.english || "Not posted", (r.borrowed || []).length ? el("details", {}, [el("summary", {text:`${r.borrowed.length} events`}), el("ul", {}, r.borrowed.map(b => el("li", {text:`${b.title} · ${b.checkout_date}${b.renewal_or_repeat ? " · repeat/renewal" : ""}`})))]) : "No observed events"])), el("h3", {text:"Reading checks · compatible forms only"}), table(["Student", "Date", "Instrument / scale / grade form", "Result", "Prior comparable observation", "Change (same form)"], readings.map(r => [studentLink(r.student_id), r.date, `${r.instrument} / ${r.scale} / ${r.form}`, r.result ?? "Missing", r.prior_date ? `${r.prior_date}: ${r.prior}` : "No comparable prior", r.delta == null ? "Not comparable" : r.delta > 0 ? `+${r.delta}` : r.delta])));
  };
  period.addEventListener("change", () => { this.progressPeriod = period.value; renderDetails(); });
  target.append(el("label", {}, ["Drill-down period", period]), detail);
  renderDetails();
  if (!report.periods.length) target.append(el("p", {text:"No optional academic records available. The librarian workflow and recommendations remain available."}));
};
