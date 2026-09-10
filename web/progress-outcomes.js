/* Portfolio summaries reuse student-view data, not librarian attribution. */
window.engagement.loadStudentProgress = async function (target, navigation) {
  const source=el("select",{},[el("option",{value:"scenario",text:"Illustrative history"}),el("option",{value:"extract",text:"Extract + live borrowing"})]);
  source.value=this.progressSource||"scenario";
  source.addEventListener("change",()=>{this.progressSource=source.value;this.progressPeriod=null;this.loadOutcomes().catch(e=>this.error(e));});
  return charts.report(target,`/api/metrics/progress?source=${source.value}`,report=>{
    const controls=el("div",{class:"outcome-filters"},[el("label",{},["Source",source])]);
    target.replaceChildren(navigation,controls,el("div",{class:"outcome-intro"},[el("span",{class:"source-badge",text:source.value==="scenario"?"Illustrative history · whole roster":"Observed extract + live borrowing · whole roster"}),el("h2",{text:"A wider view of our readers."}),el("p",{text:"Borrowing, English grades and reading checks from the student records. Not attributed to a librarian."})]));
    if(!report.periods.length){target.append(el("div",{class:"empty-state"},[el("h3",{text:"No student progress records yet."}),el("p",{text:"Recommendations and conversations are still available without academic data."})]));return;}
    const period=el("select",{},report.periods.map(p=>el("option",{value:p.id,text:`${charts.periodLabel(p.start)} – ${charts.periodLabel(p.end)}`})));
    period.value=this.progressPeriod||report.periods.at(-1).id;
    if(!period.value)period.value=report.periods.at(-1).id;
    const detail=el("div",{class:"period-detail"});
    const grades=["A+","A","A-","B+","B","B-","C+","C","C-","D+","D","D-","F"];
    const studentLink=id=>this.button(this.name(id),async()=>{await selectStudent(id);switchTab("progress",true);if(source.value==="extract"){const d=document.querySelector(".extract-coverage");if(d){d.open=true;d.scrollIntoView({block:"start"});}}});
    const render=()=>{
      this.progressPeriod=period.value;
      const selected=report.periods.find(p=>p.id===period.value);
      const rows=report.rows.filter(r=>r.period===selected.id);
      const reading=report.reading.filter(r=>r.date>=selected.start&&r.date<=selected.end);
      detail.replaceChildren(el("div",{class:"section-heading"},[el("h3",{text:"Look closer at one period"}),el("label",{},["Period",period])]),el("p",{class:"coverage-note",text:`${selected.start} to ${selected.end} · ${selected.covered} fully covered, ${selected.partial} partly covered, ${selected.missing} missing borrowing records. Coverage changes can change totals.`}));
      const gradeChart=charts.bars("English grade distribution",grades.filter(g=>selected.grades[g]).map(g=>({label:g,value:selected.grades[g]})),{unit:"students",note:`${selected.missing_grades} students have no posted grade. Letters are not averaged.`});
      const assessment=el("section",{class:"viz-card"},[el("h3",{text:"Reading checks"})]);
      const groups=[...new Set(reading.map(r=>`${r.instrument} / ${r.scale} / grade ${r.form}`))];
      if(!groups.length)assessment.append(el("p",{text:"No reading checks in this period."}));
      else {
        const group=el("select",{},groups.map(g=>el("option",{value:g,text:g})));const plot=el("div");
        const draw=()=>{const items=reading.filter(r=>`${r.instrument} / ${r.scale} / grade ${r.form}`===group.value);const values=[...new Set(items.filter(r=>r.result!=null).map(r=>r.result))].sort((a,b)=>a-b);plot.replaceChildren(charts.bars("Results for this form",values.map(v=>({label:String(v),value:items.filter(r=>r.result===v).length})),{unit:"observations",note:`${items.length} observations · ${items.filter(r=>r.result==null).length} missing results. Other forms are not combined.`}));};
        group.addEventListener("change",draw);assessment.append(el("label",{},["Comparable form",group]),plot);draw();
      }
      detail.append(el("div",{class:"chart-grid two"},[gradeChart,assessment]),charts.disclosure("View student records",charts.table(["Student","Coverage","Checkouts","Unique titles","English","Books borrowed"],rows.map(r=>[studentLink(r.student_id),r.coverage,r.coverage==="missing"&&!r.checkouts?"Unknown":r.checkouts,r.coverage==="missing"&&!r.unique_titles?"Unknown":r.unique_titles,r.english||"Not posted",(r.borrowed||[]).length?charts.disclosure(`${r.borrowed.length} events`,el("ul",{},r.borrowed.map(b=>el("li",{text:`${b.title} · ${b.checkout_date}${b.renewal_or_repeat?" · repeat/renewal":""}`})))):"No observed events"])),el("h3",{text:"Reading observations"}),charts.table(["Student","Date","Instrument / form","Result","Prior / change"],reading.map(r=>[studentLink(r.student_id),r.date,`${r.instrument} / ${r.scale} / ${r.form}`,r.result??"Missing",r.delta==null?"No comparable prior":`${r.prior_date}: ${r.prior} → ${r.result} (${r.delta>0?"+":""}${r.delta})`]))));
    };
    period.addEventListener("change",render);
    target.append(charts.bars("Observed borrowing across periods",report.periods.map(p=>({label:charts.periodLabel(p.start),value:p.checkouts,id:p.id,detail:`${p.covered} full · ${p.partial} partial · ${p.missing} missing student windows`})),{unit:"checkout events",note:"All periods · repeats included. A larger total is not proof of reading or learning gains.",onSelect:r=>{period.value=r.id;render();}}),detail,charts.disclosure("How this is counted",el("p",{text:report.note}),charts.table(["Period","Student–title pairs","District distinct titles"],report.periods.map(p=>[p.id,p.student_titles,p.distinct_titles]))));
    render();
  });
};
