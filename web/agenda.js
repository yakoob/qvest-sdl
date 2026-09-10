/* Engagement shares student selection and inventory refresh with app.js. */
window.engagement = {
  view: "students", source: "session", snapshot: {}, revision: 0, timezone: "America/Los_Angeles", busy: false, offer: null,
  button(label, fn) { const b = el("button", { type: "button", class: "secondary", text: label }); b.addEventListener("click", () => Promise.resolve().then(fn).catch(e => this.error(e))); return b; },
  error(e) { document.getElementById("engagement-status").textContent = e.message || String(e); setStatus(e.message || String(e), "err"); },
  name(id) { const s = state.students.find(s => s.student_id === id); return s ? `${labelFor(s)} (${id})` : id; },
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
  async showView(view) {
    if (!["day", "students", "outcomes"].includes(view)) return;
    this.view = view;
    for (const key of ["day", "students", "outcomes"]) { document.getElementById(`${key}-view`).hidden = key !== view; const b = document.querySelector(`[data-view="${key}"]`); if (key === view) b.setAttribute("aria-current", "page"); else b.removeAttribute("aria-current"); }
    try { if (view === "day") await this.loadAgenda(); if (view === "outcomes") await this.loadOutcomes(); } catch (e) { this.error(e); }
  },
  async loadAgenda() {
    const target = document.getElementById("agenda-body");
    await this.refresh(); const res = await fetch("/api/support/queue"); if (!res.ok) throw new Error("Could not load attention queue"); const data = await res.json(); if (this.view !== "day") return;
    const date = el("input", { type: "date", value: this.agendaDate || this.day() });
    const mode = el("select", {}, [el("option", { value: "upcoming", text: "Upcoming / open" }), el("option", { value: "overdue", text: "Overdue" }), el("option", { value: "completed", text: "Completed / cancelled / no-show" })]); mode.value = this.agendaMode || "upcoming";
    date.addEventListener("change", () => { this.agendaDate = date.value; this.loadAgenda().catch(e => this.error(e)); }); mode.addEventListener("change", () => { this.agendaMode = mode.value; this.loadAgenda().catch(e => this.error(e)); });
    const agenda = el("section", { class: "task" }, [el("h2", { text: `Agenda · ${staffId()} · ${this.timezone}` }), el("div", { class: "agenda-controls" }, [el("label", {}, ["Date", date]), el("label", {}, ["Show", mode])])]);
    for (const a of this.snapshot.appointments || []) {
      if (a.staff_id !== staffId()) continue;
      const past = new Date(a.end) < new Date(this.now);
      if (mode.value === "overdue" ? !(a.status === "scheduled" && past) : mode.value === "completed" ? !["completed", "cancelled", "no_show"].includes(a.status) || this.day(a.start) !== date.value : !["scheduled", "in_progress"].includes(a.status) || this.day(a.start) !== date.value) continue;
      const row = el("div", { class: "agenda-row" }, [el("strong", { text: `${this.name(a.student_id)} · ${a.status}` }), el("p", { class: "hint", text: `${this.local(a.start)} – ${this.local(a.end)} · ${a.place || "Library"}` }), this.button("Open student", () => selectStudent(a.student_id))]);
      if (a.status === "scheduled") row.append(this.button("Start conversation", async () => { await this.command({ action: "start", appointment_id: a.id, staff_id: staffId() }); await selectStudent(a.student_id); }), this.button("Reschedule", () => this.schedule(a.student_id, a)), this.button("Cancel", () => this.command({ action: "cancel", id: a.id })), this.button("No-show", () => this.command({ action: "no_show", id: a.id })));
      agenda.append(row);
    }
    for (const i of this.snapshot.interactions || []) if (!i.completed_at && i.facilitator === staffId() && mode.value === "upcoming") agenda.append(el("div", { class: "agenda-row" }, [el("strong", { text: `Open conversation · ${this.name(i.student_id)}` }), this.button("Continue", () => selectStudent(i.student_id))]));
    const follow = el("section", { class: "task" }, [el("h2", { text: "Follow-ups" })]);
    for (const f of this.snapshot.followups || []) {
      const i = this.snapshot.interactions.find(i => i.id === f.interaction_id);
      const appointment = (this.snapshot.appointments || []).find(a => a.id === f.appointment_id);
      if (!i || (i.facilitator !== staffId() && appointment?.staff_id !== staffId()) || f.completed_at) continue;
      const rebook = !appointment || ["cancelled", "no_show"].includes(appointment.status);
      const label = !appointment ? "Unscheduled follow-up" : rebook ? "Needs rebooking" : appointment.status === "in_progress" ? "Contact in progress" : new Date(f.due) <= new Date(this.now) ? "Due / overdue" : "Booked";
      const row = el("div", { class: "agenda-row" }, [el("strong", { text: `${this.name(i.student_id)} · ${label}` }), el("p", { text: `${this.local(f.due)} · Original facilitator ${i.facilitator}${appointment ? ` · Assigned ${appointment.staff_id}` : ""}` }), this.button("Open book feedback", () => selectStudent(i.student_id))]);
      if (rebook) row.append(this.button("Book a slot", () => this.schedule(i.student_id, null, null, f)));
      else if (appointment.status === "scheduled") row.append(this.button("Start follow-up conversation", async () => { await this.command({ action: "start", appointment_id: appointment.id, staff_id: staffId() }); await selectStudent(i.student_id); }), this.button("Reschedule follow-up", () => this.schedule(i.student_id, appointment)));
      else if (appointment.status === "in_progress") row.append(this.button("Continue follow-up", () => selectStudent(i.student_id)));
      follow.append(row);
    }
    if (follow.children.length === 1) follow.append(el("p", { text: "No open follow-ups for this facilitator." }));
    const attention = el("section", { class: "task" }, [el("h2", { text: "Needs attention" }), el("p", { class: "hint", text: "Explainable, unvalidated bands—not diagnosis. Missing academic data never hides a student. All readers remain in Students." })]);
    for (const row of data.students || []) { if (row.band === "none") continue; const contacts = (this.snapshot.interactions || []).filter(i => i.student_id === row.student_id && i.completed_at); const next = (this.snapshot.appointments || []).filter(a => a.student_id === row.student_id && a.status === "scheduled").sort((a,b) => a.start.localeCompare(b.start))[0]; attention.append(el("div", { class: "agenda-row" }, [el("strong", { text: `${this.name(row.student_id)} · ${row.label}` }), el("p", { class: "hint", text: `${row.coverage} · ${(row.reason_codes || []).join(", ")}` }), el("p", { class: "hint", text: `Last contact: ${contacts.length ? this.local(contacts[contacts.length-1].completed_at) : "none this session"}. Next: ${next ? this.local(next.start) : "not scheduled"}.` }), this.button("Open student", () => selectStudent(row.student_id)), this.button("Schedule", () => this.schedule(row.student_id))])); }
    target.replaceChildren(agenda, follow, attention);
  },
  async schedule(student, existing, completion, followup) {
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
    form.append(el("h2", { text: title }), picker.element, el("label", {}, ["Place (no confidential notes)", place]), message, save, this.button("Close", () => dialog.close()));
    form.addEventListener("submit", async e => {
      e.preventDefault();
      const slot = picker.value();
      if (!slot) return;
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
        await picker.reload();
      }
    });
    dialog.append(form); document.body.append(dialog);
    dialog.addEventListener("close", () => { picker.dispose(); dialog.remove(); });
    dialog.showModal(); await picker.reload();
  },
  renderConversation() {
    const target = document.getElementById("conversation-body"); if (!target) return;
    const id = state.selectedId, active = this.active();
    const nodes = [el("h2", { text: active ? "Reading conversation in progress" : "Reading conversations" }), el("p", { class: "hint", text: "Ask what they enjoyed and what they want next. Book choice, checkout and conversation completion are separate actions." })];
    if (this.pending) nodes.push(this.button("Retry last interrupted action", async () => { const result = await this.send(this.pending); this.pending = null; return result; }));
    if (!active) nodes.push(this.button("Start walk-in conversation", () => this.command({ action: "start", student_id: id, staff_id: staffId() })), this.button("Schedule conversation", () => this.schedule(id)));
    if (active) {
      const choice = (this.snapshot.choices || []).find(c => c.interaction_id === active.id);
      nodes.push(el("p", { text: `${active.id} · Facilitator ${active.facilitator}` }));
      if (!choice) nodes.push(this.button("None today", () => this.command({ action: "choose", interaction_id: active.id, source: "none" })));
      if (choice) nodes.push(el("p", { text: choice.book_id ? `Chosen ${choice.book_id} · ${choice.loan_id || "not checked out"}` : "No book chosen today" }));
      if (choice?.book_id && !choice.loan_id) nodes.push(this.button("Check out chosen book", () => this.command({ action: "checkout", id: choice.id, staff_id: staffId() })));
      nodes.push(this.button("Complete without follow-up", () => this.command({ action: "complete", interaction_id: active.id })), this.button("Complete and book follow-up", () => this.schedule(id, null, active)));
    }
    for (const i of (this.snapshot.interactions || []).filter(i => i.student_id === id && i.completed_at)) {
      const c = (this.snapshot.choices || []).find(c => c.interaction_id === i.id);
      const row = el("div", { class: "agenda-row" }, [el("strong", { text: `Completed ${this.local(i.completed_at)} · ${i.facilitator}` })]);
      if (c?.book_id) {
        row.append(el("p", { text: `${c.book_id} · ${c.loan_id || "not checked out"}` }));
        if (!c.loan_id) row.append(this.button("Check out chosen book", () => this.command({ action: "checkout", id: c.id, staff_id: staffId() })));
        const select = (options) => el("select", {}, options.map(v => el("option", { value: v, text: v.replaceAll("_", " ") })));
        const reading = select(["unknown", "not_started", "reading", "stopped", "finished"]), enjoyment = select(["unknown", "yes", "no", "neutral"]), source = select(["student_reported", "staff_observed"]);
        row.append(el("div", { class: "agenda-controls" }, [el("label", {}, ["Reading", reading]), el("label", {}, ["Enjoyment", enjoyment]), el("label", {}, ["Source", source])]), this.button("Save book feedback", () => this.command({ action: "feedback", interaction_id: i.id, book_id: c.book_id, reading: reading.value, enjoyment: enjoyment.value, source: source.value, staff_id: staffId() })));
        for (const f of (this.snapshot.feedback || []).filter(f => f.interaction_id === i.id)) row.append(el("p", { class: "hint", text: `${this.local(f.at)} · ${f.source}: ${f.reading}, enjoyment ${f.enjoyment} · recorder ${f.staff_id}` }));
      } else row.append(el("p", { text: "No accepted book choice." }));
      nodes.push(row);
    }
    target.replaceChildren(...nodes);
  },
  async choose(item) { const active = this.active(); if (!active || !this.offer || this.offer.interaction !== active.id) throw new Error("Refresh candidate books for this conversation first"); await this.command({ action: "choose", interaction_id: active.id, offer_id: this.offer.id, book_id: item.book_id, source: "offered" }); },
};
document.querySelectorAll("[data-view]").forEach(b => b.addEventListener("click", () => window.engagement.showView(b.dataset.view)));
document.getElementById("staff").addEventListener("change", () => { if (window.engagement.view === "day") window.engagement.loadAgenda().catch(e => window.engagement.error(e)); });
