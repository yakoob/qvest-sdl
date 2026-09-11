/* Engagement shares student selection and inventory refresh with app.js. */
window.engagement = {
  view: "students", source: "session", snapshot: {}, revision: 0, timezone: "America/Los_Angeles", busy: false, offer: null,
  button(label, fn) { const b = el("button", { type: "button", class: "secondary", text: label }); b.addEventListener("click", () => Promise.resolve().then(fn).catch(e => this.error(e))); return b; },
  error(e) { document.getElementById("engagement-status").textContent = e.message || String(e); setStatus(e.message || String(e), "err"); },
  name(id) { const s = state.students.find(s => s.student_id === id); return s ? labelFor(s) : id; },
  primary(label, fn) { const b = this.button(label, fn); b.classList.add("primary"); return b; },
  time(t) { return new Intl.DateTimeFormat("en-US", { timeZone: this.timezone, hour: "numeric", minute: "2-digit" }).format(new Date(t)); },
  book(id) { return this.bookTitles?.[id] || id; },
  local(t) { return new Intl.DateTimeFormat("en-US", { timeZone: this.timezone, dateStyle: "medium", timeStyle: "short" }).format(new Date(t)); },
  day(t = new Date()) { const p = new Intl.DateTimeFormat("en-CA", { timeZone: this.timezone, year: "numeric", month: "2-digit", day: "2-digit" }).formatToParts(new Date(t)); return ["year", "month", "day"].map(k => p.find(x => x.type === k).value).join("-"); },
  async refresh() {
    const seq = this.refreshSeq = (this.refreshSeq || 0) + 1;
    const r = await fetch("/api/agenda");
    if (!r.ok) throw new Error("Could not load agenda");
    const d = await r.json();
    if (seq !== this.refreshSeq) return d;
    const restarted = this.sessionStarted && this.sessionStarted !== d.session_started;
    if (!restarted && d.state.revision < this.revision) return d;
    if (restarted) { this.pending = null; this.offer = null; }
    this.sessionStarted = d.session_started;
    const previous = this.active()?.id;
    this.snapshot = d.state; this.revision = d.state.revision; this.timezone = d.timezone; this.now = d.now;
    if (previous !== this.active()?.id) {
      this.offer = null;
      state.recSeq++;
      goBtn.disabled = false;
      recsEl.replaceChildren(el("p", { text: "Conversation changed. Find available books again." }));
    }
    this.renderConversation();
    return d;
  },
  async command(c) {
    if (this.busy) throw new Error("Wait for the current action to finish");
    this.busy = true;
    try {
      if (this.pending) throw new Error("Retry the interrupted action before starting another action");
      const body = { ...c, request_id: crypto.randomUUID(), expected_revision: this.revision };
      // Keep the exact body for an explicit retry after transport failure.
      this.pending = body;
      const result = await this.send(body); this.pending = null; this.renderConversation(); return result;
    } catch (e) { this.renderConversation(); throw e; } finally { this.busy = false; }
  },
  async send(body) {
    const res = await fetch("/api/engagement", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
    const out = await res.json();
    if (!res.ok) { this.pending = null; await this.refresh(); throw new Error(out.error || "Action failed"); }
    await this.refresh();
    if (out.checkout) { noteRevision(out.checkout.revision); await refreshStudent({ keepRecs: false }); }
    if (this.view === "day") await this.loadAgenda();
    return out;
  },
  active() { return (this.snapshot.interactions || []).find(i => i.student_id === state.selectedId && !i.completed_at); },
  retryButton(onDone) {
    return this.button("Retry interrupted action", async () => {
      if (this.busy || !this.pending) return;
      this.busy = true;
      try {
        await this.send(this.pending);
        this.pending = null;
        this.renderConversation();
        onDone?.();
      } finally { this.busy = false; }
    });
  },
  pendingNotice(container, onDone) {
    if (!this.pending) return;
    if (container.querySelector?.("[data-retry-notice]")) return;
    container.append(el("div", { class: "empty-state", "data-retry-notice": "true" }, [
      el("p", { text: "An action is awaiting confirmation. Retry safely before continuing." }),
      this.retryButton(onDone),
    ]));
  },
  async showView(view, opts = {}) {
    if (!["day", "students", "outcomes"].includes(view)) return;
    if (opts.boot && this.choseView) return;
    if (!opts.boot) this.choseView = true;
    this.view = view;
    for (const key of ["day", "students", "outcomes"]) { document.getElementById(`${key}-view`).hidden = key !== view; const b = document.querySelector(`[data-view="${key}"]`); if (key === view) b.setAttribute("aria-current", "page"); else b.removeAttribute("aria-current"); }
    try { if (view === "day") await this.loadAgenda(); if (view === "outcomes") await this.loadOutcomes(); } catch (e) { this.error(e); }
  },
  async loadAgenda() {
    const seq = this.agendaSeq = (this.agendaSeq || 0) + 1;
    const target = document.getElementById("agenda-body");
    await this.refresh();
    const res = await fetch("/api/support/queue");
    if (!res.ok) throw new Error("Could not load students to check in with");
    const data = await res.json();
    if (seq !== this.agendaSeq || this.view !== "day") return;
    const date = el("input", { type: "date", value: this.agendaDate || this.day() });
    const mode = el("select", {}, [["upcoming","Today's meetings"],["overdue","Needs attention"],["completed","Past activity"]].map(([value,text]) => el("option", {value,text})));
    mode.value = this.agendaMode || "upcoming";
    const refresh = () => { this.agendaDate = date.value; this.agendaMode = mode.value; return this.loadAgenda(); };
    for (const control of [date,mode]) control.addEventListener("change", () => refresh().catch(e=>this.error(e)));
    const move = n => { date.value = charts.shiftDate(date.value,n); return refresh(); };
    const controls = el("div", {class:"agenda-controls"}, [this.button("Today",()=>{date.value=this.day();return refresh();}),this.button("Previous day",()=>move(-1)),el("label",{},["Date",date]),this.button("Next day",()=>move(1)),el("label",{},["Show",mode])]);
    const agenda = el("section", {class:"task"}, [el("div",{class:"section-heading"},[el("div",{},[el("h2",{text:"Your day, one reader at a time."}),el("p",{class:"hint",text:charts.staff(staffId())+" · "+this.timezone})]),this.primary("Find a student",()=>this.showView("students"))]),controls]);
    const appointments = this.snapshot.appointments || [];
    const interactions = this.snapshot.interactions || [];
    const follows = this.snapshot.followups || [];
    const now = new Date(this.now);
    let count = 0;
    const add = (time, title, note, action, more) => {
      agenda.append(el("div",{class:"agenda-item"},[el("time",{text:time}),el("div",{},[el("strong",{text:title}),el("p",{class:"hint",text:note})]),el("div",{},[action,more])])); count++;
    };
    for (const ap of [...appointments].sort((a,b)=>a.start.localeCompare(b.start))) {
      const follow = follows.find(f=>f.appointment_id===ap.id);
      const origin = follow && interactions.find(i=>i.id===follow.interaction_id);
      if (ap.staff_id!==staffId() && origin?.facilitator!==staffId()) continue;
      const past = new Date(ap.end)<now;
      const rebook = follow && !follow.completed_at && ["cancelled","no_show"].includes(ap.status);
      if (mode.value==="upcoming" && (this.day(ap.start)!==date.value || !["scheduled","in_progress"].includes(ap.status))) continue;
      if (mode.value==="overdue" && !(rebook || (past && ap.status==="scheduled"))) continue;
      if (mode.value==="completed" && (this.day(ap.start)!==date.value || !["completed","cancelled","no_show"].includes(ap.status))) continue;
      const ready = new Date(ap.start)<=now;
      let action = this.button("Open student",()=>selectStudent(ap.student_id));
      if (rebook) action = this.primary("Book a time",()=>this.schedule(ap.student_id,null,null,follow));
      else if (ap.status==="in_progress") action=this.primary("Continue",()=>selectStudent(ap.student_id));
      else if (ap.status==="scheduled") {
        action=this.primary("Start now",async()=>{await this.command({action:"start",appointment_id:ap.id,staff_id:staffId()});await selectStudent(ap.student_id);});
        action.title=ready?"Start this conversation":"Start this booked conversation now";
      }
      let more;
      if(ap.status==="scheduled") {
        const noShow=this.button("Mark no-show",()=>{if(window.confirm("Mark this meeting as not attended?"))return this.command({action:"no_show",id:ap.id});});noShow.disabled=!ready;
        more=charts.disclosure("More actions",this.button("Reschedule",()=>this.schedule(ap.student_id,ap)),this.button("Cancel meeting",()=>{if(window.confirm("Cancel this meeting?"))return this.command({action:"cancel",id:ap.id});}),noShow);
      }
      const label = rebook?"Needs rebooking":ap.status==="in_progress"?"In progress":ap.status==="scheduled"?(past?"Overdue":ready?"Ready to start":"Upcoming · start anytime"):ap.status==="completed"?"Finished":ap.status==="no_show"?"Not attended":"Cancelled";
      add(this.time(ap.start),this.name(ap.student_id),(follow?"Follow-up · ":"")+label+" · "+charts.staff(ap.staff_id)+(ap.staff_id!==staffId()?" (assigned)":""),action,more);
    }
    if(mode.value!=="completed") {
      for(const i of interactions) if(!i.completed_at && !i.appointment_id && i.facilitator===staffId()) add("Now",this.name(i.student_id),"Walk-in · In progress",this.primary("Continue",()=>selectStudent(i.student_id)));
      for(const f of follows) {
        const i=interactions.find(i=>i.id===f.interaction_id);
        if(f.completed_at || i?.facilitator!==staffId()) continue;
        if(!f.appointment_id) add("To book",this.name(i.student_id),"Follow-up · Choose an available time",this.primary("Book a time",()=>this.schedule(i.student_id,null,null,f)));
      }
    }
    if(!count) agenda.append(el("div",{class:"empty-state"},[el("h3",{text:mode.value==="overdue"?"Nothing waiting for attention.":"No meetings in this view."}),el("p",{text:"Find a reader to start a conversation or book time together."})]));
    const attention=el("section",{class:"task"},[el("h2",{text:"Students to check in with"}),el("p",{class:"hint",text:"Below-grade readers first, from English grades and reading checks—not a diagnosis. Students already in a conversation, booked, or due for a follow-up stay on My day instead, where you can start now."})]);
    for(const row of data.students || []) {
      if(!row.waiting) continue;
      const status = row.grade_status_label || row.label;
      const detail = [`Grade ${row.grade}`, row.english ? `English ${row.english}` : null, row.reading ? `Reading ${row.reading}` : null].filter(Boolean).join(" · ");
      attention.append(el("div",{class:"visit-summary agenda-row"},[el("div",{},[el("strong",{text:this.name(row.student_id)}),el("p",{class:`hint support-band band-${row.grade_status || row.band}`,text:status}),el("p",{class:"hint",text:detail})]),this.button("Open student",()=>selectStudent(row.student_id))]));
    }
    if(!(data.students || []).some(row => row.waiting)) attention.append(el("p",{class:"hint",text:"No one is waiting for a first check-in right now."}));
    target.replaceChildren(agenda,attention);
  },
  async schedule(student, existing, completion, followup) {
    const previous = document.activeElement;
    await this.refresh();
    const title = completion ? "Complete and book follow-up" : followup ? "Book follow-up" : existing ? "Reschedule conversation" : "Schedule conversation";
    const dialog = el("dialog", { class: "engagement-dialog", "aria-label": title });
    const form = el("form");
    const save = el("button", { type: "submit", text: title, disabled: true });
    const message = el("p", { role: "status" });
    const place = el("input", { value: existing?.place || "Library", maxlength: 100 });
    const picker = createSlotPicker({ student, staff: existing?.staff_id || staffId(), lockStaff: !!existing,
      date: existing ? this.day(existing.start) : this.day(), exclude: existing?.id,
      duration: existing ? Math.round((new Date(existing.end) - new Date(existing.start)) / 60000) : 10,
      onChange: value => { save.disabled = !value; },
    });
    form.append(el("h2", { text: title }));
    this.pendingNotice(form, () => dialog.close());
    if (this.pending) save.disabled = true;
    form.append(picker.element, charts.disclosure("Place", el("label", {}, ["Place (no confidential notes)", place])), message, save, this.button("Close", () => dialog.close()));
    form.addEventListener("submit", async e => {
      e.preventDefault();
      const slot = picker.value();
      if (!slot || this.busy) return;
      save.disabled = true;
      try {
        await this.command({ ...slot, place: place.value, student_id: student,
          action: completion ? "complete" : followup ? "book_followup" : existing ? "reschedule" : "schedule",
          ...(completion ? { interaction_id: completion.id } : {}),
          ...(followup || existing ? { id: (followup || existing).id } : {}),
        });
        dialog.close();
      } catch (error) {
        message.textContent = error.message;
        this.pendingNotice(form, () => dialog.close());
        await picker.reload();
        save.disabled = !picker.value() || !!this.pending;
      }
    });
    dialog.append(form); document.body.append(dialog);
    dialog.addEventListener("close", () => { picker.dispose(); dialog.remove(); if (previous?.isConnected) previous.focus(); else document.querySelector(`[data-view="${this.view}"]`).focus(); });
    dialog.showModal(); await picker.reload();
  },
  showDialog(title, build) {
    const previous = document.activeElement;
    const dialog = el("dialog", { class: "engagement-dialog", "aria-label": title });
    const message = el("p", { role: "status" });
    dialog.append(el("h2", { text: title }));
    this.pendingNotice(dialog, () => dialog.close());
    build(dialog, message);
    dialog.append(message, this.button("Close", () => dialog.close()));
    document.body.append(dialog);
    dialog.addEventListener("close", () => { dialog.remove(); if (previous?.isConnected) previous.focus(); else document.getElementById("tab-books").focus(); });
    dialog.showModal();
    return dialog;
  },
  wrapUp(interaction) {
    this.showDialog("Finish conversation", (dialog, message) => {
      dialog.append(el("p", { text: "Finish this visit now, or choose an available time to check in again." }),
        this.primary("Finish now", async () => {
          if (this.busy) return;
          try { await this.command({action:"complete",interaction_id:interaction.id}); dialog.close(); }
          catch (error) { message.textContent=error.message; this.pendingNotice(dialog, () => dialog.close()); }
        }),
        this.button("Book a follow-up", () => { dialog.close(); return this.schedule(interaction.student_id,null,interaction); }));
    });
  },
  addFeedback(interaction, choice) {
    this.showDialog("Book feedback", (dialog, message) => {
      const select = options => el("select", {}, options.map(([value,text]) => el("option",{value,text})));
      const reading=select([["unknown","Not sure yet"],["not_started","Not started"],["reading","Still reading"],["stopped","Stopped reading"],["finished","Finished"]]);
      const enjoyment=select([["unknown","Not asked yet"],["yes","Enjoyed it"],["no","Did not enjoy it"],["neutral","Mixed / neutral"]]);
      const source=select([["student_reported","The student told me"],["staff_observed","Staff observation"]]);
      dialog.append(el("p",{text:this.name(interaction.student_id)+" · "+this.book(choice.book_id)}),
        el("label",{},["Reading status",reading]), el("label",{},["Enjoyment",enjoyment]), el("label",{},["Report source",source]),
        this.primary("Save feedback",async()=>{
          if (this.busy) return;
          try { await this.command({action:"feedback",interaction_id:interaction.id,book_id:choice.book_id,reading:reading.value,enjoyment:enjoyment.value,source:source.value,staff_id:staffId()});dialog.close(); }
          catch(error){message.textContent=error.message; this.pendingNotice(dialog, () => dialog.close());}
        }));
    });
  },
  renderConversation() {
    const target=document.getElementById("conversation-body");if(!target)return;
    const id=state.selectedId, active=this.active();
    const nodes=[];
    this.pendingNotice({ append(...items) { nodes.push(...items); } });
    const scheduled=(this.snapshot.appointments||[]).find(a=>a.student_id===id && a.status==="scheduled");
    const follow=(this.snapshot.followups||[]).find(f=>{
      if(f.completed_at) return false;
      const origin=(this.snapshot.interactions||[]).find(i=>i.id===f.interaction_id);
      return origin?.student_id===id;
    });
    if(!active){
      const actions=el("div",{class:"engagement-actions"});
      if(scheduled){
        nodes.push(el("div",{class:"visit-summary"},[el("div",{},[el("h3",{text:"Booked conversation"}),el("p",{class:"hint",text:(follow?"Follow-up · ":"")+this.local(scheduled.start)+" · "+charts.staff(scheduled.staff_id)+". Start now, or keep the booked time."})]),el("div",{class:"engagement-actions"},[this.primary("Start now",()=>this.command({action:"start",appointment_id:scheduled.id,staff_id:staffId()})),this.button("Reschedule",()=>this.schedule(id,scheduled))])]));
      } else {
        actions.append(this.primary("Start conversation",()=>this.command({action:"start",student_id:id,staff_id:staffId()})),this.button("Book a time",()=>this.schedule(id)));
        if(follow && !follow.appointment_id) actions.append(this.button("Book follow-up",()=>this.schedule(id,null,null,follow)));
        nodes.push(el("div",{class:"visit-summary"},[el("div",{},[el("h3",{text:"A little conversation. A better next book."}),el("p",{class:"hint",text:follow && !follow.appointment_id?"This reader has an unbooked follow-up. Start now, or choose a time.":"Start a visit, or browse books below for a quick lookup."})]),actions]));
      }
    }
    else {
      const choice=(this.snapshot.choices||[]).find(c=>c.interaction_id===active.id);
      const heading=!choice?"What would they like to read?":choice.book_id&&!choice.loan_id?"Ready to take this book home?":"Ready to wrap up?";
      const note=!choice?"Ask what they enjoyed, then find available books together.":choice.book_id?this.book(choice.book_id)+(choice.loan_id?" · Checked out":" · Chosen, not checked out"):"No book today. The conversation still counts.";
      const actions=el("div",{class:"engagement-actions"});
      if(!choice){
        actions.append(this.primary("Find books together",()=>{switchTab("books");document.getElementById("find").scrollIntoView({block:"start"});return recommend();}),this.button("None today",()=>this.command({action:"choose",interaction_id:active.id,source:"none"})));
      } else if(choice.book_id&&!choice.loan_id)actions.append(this.primary("Check out chosen book",()=>this.command({action:"checkout",id:choice.id,staff_id:staffId()})));
      actions.append(this.button("Finish conversation",()=>this.wrapUp(active)));
      nodes.push(el("div",{class:"visit-summary"},[el("div",{},[el("span",{class:"source-badge",text:"Conversation with "+charts.staff(active.facilitator)}),el("h3",{text:heading}),el("p",{text:note})]),actions]));
    }
    const completed=(this.snapshot.interactions||[]).filter(i=>i.student_id===id&&i.completed_at).slice().reverse();
    if(completed.length){
      const history=charts.disclosure("Past conversations ("+completed.length+")");
      for(const interaction of completed){
        const choice=(this.snapshot.choices||[]).find(c=>c.interaction_id===interaction.id);
        const row=el("div",{class:"agenda-row"},[el("strong",{text:this.local(interaction.completed_at)+" · "+charts.staff(interaction.facilitator)}),el("p",{text:choice?.book_id?this.book(choice.book_id):"No book chosen"})]);
        if(choice?.book_id){
          row.append(this.button("Add book feedback",()=>this.addFeedback(interaction,choice)));
          if(!choice.loan_id)row.append(this.button("Check out chosen book",()=>this.command({action:"checkout",id:choice.id,staff_id:staffId()})));
        }
        const reports=(this.snapshot.feedback||[]).filter(f=>f.interaction_id===interaction.id);
        if(reports.length)row.append(charts.disclosure("Feedback history",charts.table(["Recorded","Reading","Enjoyment","Source"],reports.map(f=>[this.local(f.at),f.reading.replaceAll("_"," "),f.enjoyment,f.source==="student_reported"?"Student report":"Staff observation"]))));
        history.append(row);
      }
      nodes.push(history);
    }
    target.replaceChildren(...nodes);
  },
  async choose(item) { const active = this.active(); if (!active || !this.offer || this.offer.interaction !== active.id) throw new Error("Refresh candidate books for this conversation first"); await this.command({ action: "choose", interaction_id: active.id, offer_id: this.offer.id, book_id: item.book_id, source: "offered" }); },
};
document.querySelectorAll("[data-view]").forEach(b => b.addEventListener("click", () => window.engagement.showView(b.dataset.view)));
document.getElementById("staff").addEventListener("change", () => { if (window.engagement.view === "day") window.engagement.loadAgenda().catch(e => window.engagement.error(e)); });
