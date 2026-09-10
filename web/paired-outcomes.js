/* Served-cohort evidence stays separate from portfolio and operational metrics. */
window.engagement.loadPairedOutcomes = async function (target, navigation) {
  this.metricsSeq = (this.metricsSeq || 0) + 1;
  this.progressSeq = (this.progressSeq || 0) + 1;
  const seq = this.pairedSeq;
  const source = el("select", {}, [el("option", {value:"historical",text:"Illustrative historical contacts"}), el("option", {value:"session",text:"This server session"})]);
  source.value = this.pairedSource || "historical";
  const staff = el("select", {}, [el("option", {value:"",text:"All facilitators"}), ...[...document.getElementById("staff").options].map(o => el("option", {value:o.value,text:o.text}))]);
  staff.value = this.pairedStaff || "";
  const start = el("input", {type:"date", value:this.pairedStart || ""});
  const end = el("input", {type:"date", value:this.pairedEnd || ""});
  const asof = el("input", {type:"date", value:this.pairedAsOf || ""});
  const controls = el("div", {class:"agenda-controls"}, [el("label", {}, ["Contact source", source]), el("label", {}, ["Index-contact facilitator", staff]), el("label", {}, ["Contact period from (optional)", start]), el("label", {}, ["Contact period until (exclusive)", end]), el("label", {}, ["Evidence as of (UTC day start; blank: now)", asof]), this.button("Apply cohort filters", () => {
    this.pairedSource=source.value; this.pairedStaff=staff.value; this.pairedStart=start.value; this.pairedEnd=end.value; this.pairedAsOf=asof.value;
    return this.loadOutcomes();
  })]);
  source.addEventListener("change", () => { this.pairedSource=source.value; this.pairedStart=""; this.pairedEnd=""; this.pairedAsOf=""; this.loadOutcomes().catch(e=>this.error(e)); });
  target.replaceChildren(navigation, controls, el("p", {role:"status",text:"Loading paired observations…"}));
  const query = new URLSearchParams({source:source.value, staff_id:staff.value, start:start.value, end:end.value});
  if (asof.value) query.set("as_of", `${asof.value}T00:00:00Z`);
  const response = await fetch(`/api/metrics/paired?${query}`);
  const report = await response.json();
  if (seq !== this.pairedSeq || this.view !== "outcomes" || this.outcomeSection !== "paired") return;
  target.replaceChildren(navigation, controls);
  if (!response.ok) { target.append(el("p", {role:"alert",text:report.error || "Could not load paired outcomes"})); return; }
  const table = (headers,rows) => el("div", {class:"table-scroll",tabindex:"0"}, [el("table", {class:"engagement-table"}, [el("thead", {}, [el("tr", {}, headers.map(h=>el("th", {scope:"col",text:h})))]), el("tbody", {}, rows.map(row=>el("tr", {}, row.map(v=>el("td", {}, [v == null ? "—" : typeof v === "object" ? v : String(v)])))))])]);
  target.append(el("h2", {text:"Students served · observed changes"}), el("p", {class:"hint",text:`${source.options[source.selectedIndex].text} · ${this.local(report.filter.start)} to ${this.local(report.filter.end)} (exclusive) · as of ${this.local(report.filter.as_of)} · ${report.students} distinct students`}), el("p", {class:"hint",text:report.note}));
  target.append(table(["Measure", "Paired N", "Increased", "Unchanged", "Decreased", "Excluded / reasons"], [["Borrowing events",report.borrowing], ["English grade direction",report.english], ["Reading check (compatible form)",report.reading]].map(([name,c])=>[name,c.eligible,c.increased,c.unchanged,c.decreased,`${c.excluded}: ${Object.entries(c.reasons || {}).map(([reason,n])=>`${n} ${reason}`).join("; ") || "none"}`])));
  const evidence = p => p.exclusion ? `Excluded: ${p.exclusion}` : `${p.before} → ${p.after} (${p.direction}) · ${p.before_id}: ${p.before_start ? p.before_start+" – " : ""}${p.before_date}; ${p.after_id}: ${p.after_start ? p.after_start+" – " : ""}${p.after_date}`;
  target.append(el("h3", {text:"Student evidence behind every count"}), table(["Student", "Index contact / facilitator", "Borrowing", "English", "Reading", "Later shared contacts"], report.rows.map(row=>[
    this.button(this.name(row.student_id), async()=>{await selectStudent(row.student_id);switchTab("progress",true);}),
    `${row.contact_id} · ${this.local(row.contact_at)} · ${row.facilitator}`,
    evidence(row.borrowing), evidence(row.english), evidence(row.reading),
    row.later_contacts.map(c=>`${c.id} · ${this.local(c.at)} · ${c.staff_id}`).join("; ") || "None in period",
  ])));
  if (!report.rows.length) target.append(el("p", {text:"No completed contacts in this source and period. Scheduling alone does not count as service."}));
  target.append(el("p", {class:"hint",text:"Borrowing windows must be fully covered, 84 days long, strictly before/after contact and within one year. Academic pairs use ±180 days and compatible course/grade form. Date-only observations are available after that school day ends. Historical fixtures never change live inventory, recommendations or reservations."}));
};
