/* Paired changes are descriptive, never a causal staff score. */
window.engagement.loadPairedOutcomes = async function (target, navigation) {
  const source = el("select", {}, [el("option", { value: "historical", text: "Illustrative history" }), el("option", { value: "session", text: "Live session" })]);
  source.value = this.pairedSource || "historical";
  const staff = el("select", {}, [el("option", { value: "", text: "All librarians" }), ...[...document.getElementById("staff").options].map(o => el("option", { value: o.value, text: o.text }))]);
  staff.value = this.pairedStaff || "";
  const period = el("select", {}, (source.value === "historical" ? [["2025", "School year 2025–26"], ["2024", "School year 2024–25"], ["2023", "School year 2023–24"], ["custom", "Custom dates"]] : [["session", "This session"], ["custom", "Custom dates"]]).map(([value,text]) => el("option", {value,text})));
  period.value = this.pairedPeriod || (source.value === "historical" ? "2025" : "session");
  const start = el("input", {type:"date",value:this.pairedStart || ""});
  const end = el("input", {type:"date",value:this.pairedEnd || ""});
  const asof = el("input", {type:"date",value:this.pairedAsOf || ""});
  const apply = () => { this.pairedStaff = staff.value; this.pairedPeriod = period.value; this.pairedStart = start.value; this.pairedEnd = end.value; this.pairedAsOf = asof.value; return this.loadOutcomes(); };
  const more = charts.disclosure("More filters", el("div", {class:"agenda-controls"}, [el("label", {}, ["From",start]),el("label", {}, ["Through",end]),el("label", {}, ["Evidence as of (UTC day start)",asof]),this.button("Apply filters", () => { if (start.value || end.value) period.value="custom"; return apply(); })]));
  more.open = period.value === "custom";
  source.addEventListener("change", () => { this.pairedSource=source.value; this.pairedPeriod=null; this.pairedStart=""; this.pairedEnd=""; this.pairedAsOf=""; this.loadOutcomes().catch(e=>this.error(e)); });
  for (const select of [staff,period]) select.addEventListener("change", () => apply().catch(e=>this.error(e)));
  const controls = el("div", {class:"outcome-filters"}, [el("label", {}, ["Period",period]),el("label", {}, ["Source",source]),el("label", {}, ["Librarian",staff]),more]);
  const query = new URLSearchParams({source:source.value,staff_id:staff.value});
  if (/^\d{4}$/.test(period.value)) { query.set("start",`${period.value}-07-01`);query.set("end",`${Number(period.value)+1}-07-01`); }
  else if (period.value === "custom") { if(start.value)query.set("start",start.value);if(end.value)query.set("end",charts.shiftDate(end.value,1)); }
  if(asof.value)query.set("as_of",`${asof.value}T00:00:00Z`);
  return charts.report(target, `/api/metrics/paired?${query}`, report => {
    const evidence = p => p.exclusion ? `Not enough evidence: ${p.exclusion}` : `${p.before} → ${p.after} · ${p.before_start || p.before_date} to ${p.before_date}; ${p.after_start || p.after_date} to ${p.after_date}`;
    const records = charts.disclosure("View student records");
    const recordBody = el("div");records.append(recordBody);
    const showRecords = (measure, direction) => {
      const rows = report.rows.filter(r => !measure || r[measure].direction === direction);
      const heading = el("div", { class: "section-heading" }, [el("h3",{text:measure ? `${measure}: ${direction}` : "All students in this cohort"})]);
      if (measure) heading.append(this.button("Show all students", () => showRecords()));
      recordBody.replaceChildren(heading, charts.table(["Student","First contact / librarian","Borrowing","English","Reading check","Later contacts"], rows.map(r => [
        this.button(this.name(r.student_id),async()=>{await selectStudent(r.student_id);switchTab("progress",true);}),
        `${r.contact_id} · ${this.local(r.contact_at)} · ${charts.staff(r.facilitator)}`, evidence(r.borrowing),evidence(r.english),evidence(r.reading),
        r.later_contacts.map(c=>`${this.local(c.at)} · ${charts.staff(c.staff_id)}`).join("; ") || "None",
      ])));
      if(measure){ records.open=true;records.scrollIntoView({block:"nearest"}); }
    };
    const measures = [["borrowing","Borrowing"],["english","English grades"],["reading","Reading checks"]];
    target.replaceChildren(navigation, controls, el("div", {class:"outcome-intro"}, [el("span",{class:"source-badge",text:source.value==="historical"?"Illustrative history · fictional contacts":"Live session"}),el("h2",{text:"What changed after we met?"}),el("p",{text:`${report.students} students with completed contacts · ${this.day(report.filter.start)} through ${charts.shiftDate(this.day(report.filter.end),-1)} · ${staff.options[staff.selectedIndex].text}`}),el("p",{class:"hint",text:`Evidence as of ${this.local(report.filter.as_of)}. These observations do not show that a conversation caused a change.`})]),
      el("div",{class:"chart-grid"},measures.map(([key,title])=>charts.distribution(title,report[key],direction=>showRecords(key,direction)))));
    const c=report.borrowing;
    target.append(el("p",{class:"takeaway",text:c.eligible?`${c.increased} of ${c.eligible} comparable students borrowed more. ${c.unchanged} were unchanged and ${c.decreased} borrowed less. ${c.excluded} need more evidence.`:"We don't have enough comparable borrowing records yet. Missing evidence is not a poor outcome."}));
    if(!report.available) target.append(el("p",{role:"status",text:report.note}));
    showRecords();target.append(records,charts.disclosure("How this is counted",el("p",{text:report.note}),el("p",{text:"The first completed contact sets attribution before the librarian filter. Borrowing pairs need fully covered 84-day windows within one year of contact. English and reading observations use ±180 days and compatible course/grade forms. Date-only evidence is available after its school day ends. Session records never inherit illustrative historical evidence."})));
  });
};
