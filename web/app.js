const searchEl = document.getElementById("search");
const listEl = document.getElementById("student-list");
const recsEl = document.getElementById("recs");
const loansEl = document.getElementById("loans-body");
const acadEl = document.getElementById("acad-body");
const headerEl = document.getElementById("student-header");
const statusEl = document.getElementById("status");
const goBtn = document.getElementById("go");
const confirmEl = document.getElementById("confirm");
const activityEl = document.getElementById("activity");
const sessionBanner = document.getElementById("session-banner");
const findForm = document.getElementById("find-form");

const state = {
  students: [],
  supportQueue: new Map(),
  sessionStarted: null,
  selectedId: "S-406",
  revision: 0,
  recSeq: 0,
  loadSeq: 0,
  busy: false,
  activeTab: "books",
  lastConfirm: null,
};

function text(s) {
  return document.createTextNode(s == null ? "" : String(s));
}

function el(tag, attrs, children) {
  const node = document.createElement(tag);
  if (attrs) {
    Object.entries(attrs).forEach(([k, v]) => {
      if (v == null || v === false) return;
      if (k === "class") node.className = v;
      else if (k === "text") node.textContent = v;
      else if (k === "on") return;
      else node.setAttribute(k, v === true ? "" : String(v));
    });
  }
  (children || []).forEach((c) => {
    if (c == null) return;
    node.appendChild(typeof c === "string" ? text(c) : c);
  });
  return node;
}

function setStatus(msg, kind) {
  statusEl.textContent = msg || "";
  statusEl.className = kind === "err" ? "status err" : kind === "ok" ? "status ok" : "status";
}

function staffId() {
  return document.getElementById("staff").value;
}

function noteRevision(rev) {
  if (typeof rev === "number" && rev > state.revision) state.revision = rev;
}

function newerThan(rev) {
  return typeof rev === "number" && state.revision > rev;
}

function retryId(kind, studentId, extra) {
  return `${kind}-${studentId}-${extra}-${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

function labelFor(s) {
  return `${s.first_name} ${s.last_initial}.`;
}

function filteredStudents() {
  const q = (searchEl.value || "").trim().toLowerCase();
  return state.students.filter((s) => {
    const band = document.getElementById("support-filter").value;
    if (band && state.supportQueue.get(s.student_id)?.grade_status !== band) return false;
    if (!q) return true;
    const blob = `${s.first_name} ${s.last_initial} ${s.student_id} ${s.demo_role || ""} ${s.cluster || ""}`.toLowerCase();
    return blob.includes(q);
  });
}

function renderStudentList() {
  const rows = filteredStudents();
  document.getElementById("student-count").textContent = rows.length;
  listEl.replaceChildren();
  if (rows.length === 0) {
    listEl.appendChild(el("div", { class: "muted", text: "No match" }));
    return;
  }
  rows.forEach((s) => {
    const selected = s.student_id === state.selectedId;
    const btn = el("button", {
      class: "student",
      type: "button",
      "aria-pressed": selected ? "true" : "false",
      "data-id": s.student_id,
    }, [
      el("span", { class: "avatar", "aria-hidden": "true", text: `${s.first_name?.[0] || ""}${s.last_initial || ""}` }),
      el("span", { class: "student-info" }, [
        el("span", { class: "student-name", text: labelFor(s) }),
        el("span", { class: "student-meta", text: `Grade ${s.grade} · ${s.student_id}${s.open_loans ? ` · ${s.open_loans} out` : ""}` }),
        el("span", { class: `support-band band-${state.supportQueue.get(s.student_id)?.grade_status || state.supportQueue.get(s.student_id)?.band || "insufficient"}`, text: state.supportQueue.get(s.student_id)?.grade_status_label || state.supportQueue.get(s.student_id)?.label || "Loading context" }),
      ]),
    ]);
    btn.addEventListener("click", () => selectStudent(s.student_id));
    listEl.appendChild(btn);
  });
  document.querySelectorAll("[data-student]").forEach((chip) => {
    chip.setAttribute("aria-pressed", chip.getAttribute("data-student") === state.selectedId ? "true" : "false");
  });
}

async function parseJSON(res) {
  return res.json().catch(() => ({}));
}

async function loadStudents() {
  const res = await fetch("/api/students");
  if (!res.ok) throw new Error("Could not load students");
  const data = await res.json();
  if (Array.isArray(data)) {
    state.students = data;
  } else {
    state.students = data.students || [];
    if (state.sessionStarted !== data.session_started) {
      state.sessionStarted = data.session_started;
      state.revision = 0;
    }
    noteRevision(data.revision);
    if (data.session_note) sessionBanner.textContent = data.session_note;
  }
  renderStudentList();
}

function renderHeader(detail) {
  const st = detail.student || {};
  const loans = detail.loans || [];
  const band = state.supportQueue.get(st.student_id);
  headerEl.replaceChildren(
    el("span", { class: "avatar", "aria-hidden": "true", text: `${st.first_name?.[0] || ""}${st.last_initial || ""}` }),
    el("div", { class: "student-heading" }, [
      el("h2", { text: `${st.first_name || ""} ${st.last_initial || ""}.` }),
      el("div", { class: "meta-row" }, [
        el("span", { text: st.student_id }),
        el("span", { text: `Grade ${st.grade}` }),
        el("span", { text: `Homeroom ${st.homeroom_id || "—"}` }),
      ]),
    ]),
    el("div", { class: "header-badges" }, [
      el("span", { class: `support-band band-${band?.grade_status || band?.band || "insufficient"}`, text: band?.grade_status_label || band?.label || "Not enough information" }),
      el("span", { class: "loan-count", text: `${loans.length} current loan${loans.length === 1 ? "" : "s"}` }),
    ]),
  );
  state.deskNote = st.anecdote || "";
}

function renderLoans(detail) {
  const loans = detail.loans || [];
  const hist = detail.history || [];
  if (window.engagement) {
    window.engagement.bookTitles ||= {};
    [...loans, ...hist].forEach(l => { window.engagement.bookTitles[l.book_id] = l.title || l.book_id; });
  }
  const openNodes = loans.length
    ? loans.map((l) => loanCard(l, true))
    : [el("p", { class: "muted", text: "Nothing out right now." })];
  const histNodes = hist.length
    ? hist.slice(0, 12).map((l) => loanCard(l, false))
    : [el("p", { class: "muted", text: "No returned titles in this extract." })];
  loansEl.replaceChildren(
    el("div", { class: "loan-grid" }, [
      el("div", null, [el("h3", { text: "Out now" }), ...openNodes]),
      el("div", null, [el("h3", { text: "Returned (borrowed, not finished)" }), ...histNodes]),
    ]),
  );
}

function loanCard(l, canReturn) {
  const badge = el("span", { class: "badge", text: l.provenance === "session" ? "this session" : "source" });
  const node = el("div", { class: "loan-item" }, [
    el("strong", null, [l.title || l.book_id, badge]),
    el("div", { class: "meta", text: `${l.checkout_date || ""}${l.due_date ? " · due " + l.due_date : ""}${l.return_date ? " · back " + l.return_date : ""}` }),
  ]);
  if (canReturn) {
    const btn = el("button", { class: "ghost", type: "button", text: "Return" });
    btn.addEventListener("click", () => returnLoan(l));
    node.appendChild(btn);
  }
  return node;
}

function renderRecs(rec, firstName) {
  const nodes = [];
  const mode = rec.explain_mode || "template";
  nodes.push(el("div", { class: "mode" }, [
    `Explain: ${mode}. ${rec.explain_note || "Librarian-reviewed draft."}`,
  ]));
  const items = rec.items || [];
  if (items.length === 0) {
    nodes.push(el("div", { class: "muted", text: "No in-stock titles passed policy for this lookup." }));
  }
  items.forEach((it) => {
    if (window.engagement) { window.engagement.bookTitles ||= {}; window.engagement.bookTitles[it.book_id] = it.title || it.book_id; }
    const checkout = el("button", {
      type: "button",
      text: `Check out to ${firstName}`,
    });
    if (window.engagement?.active()) {
      checkout.textContent = "Choose together";
      checkout.addEventListener("click", () => window.engagement.choose(it).catch(e => window.engagement.error(e)));
    } else {
      checkout.addEventListener("click", () => checkoutBook(it, checkout));
    }
    const copy = el("button", { class: "secondary", type: "button", text: "Copy talking point" });
    copy.addEventListener("click", () => copyTalkingItem(it));
    nodes.push(el("div", { class: "card" }, [
      el("strong", { text: it.title || it.book_id }),
      el("span", { class: "meta", text: `${it.author || ""} · ${it.pages}p · ${it.copies_available} on shelf` }),
      el("div", { class: "evidence" }, [`Why this: ${(it.reasons || []).join("; ") || "none"}`]),
      el("details", { class: "talk-more" }, [
        el("summary", { text: "Talking point (draft)" }),
        el("div", { class: "talk" }, [it.talking_point || "—"]),
      ]),
      el("div", { class: "card-actions" }, [checkout, copy]),
    ]));
  });
  (rec.dropped || []).slice(0, 5).forEach((d) => {
    const unavailable = d.why === "copies_available=0";
    nodes.push(el("div", { class: "drop" }, [
      unavailable
        ? `Unavailable (not a spoken rec): ${d.title} — ${d.why}`
        : `Dropped ${d.title}: ${d.why}`,
    ]));
  });
  recsEl.replaceChildren(...nodes);
}

function gradeCell(v) {
  if (v == null || v === "") return el("span", { class: "missing", text: "not posted" });
  return text(v);
}

function renderAcademics(payload) {
  const acad = payload.academics || payload;
  const scenario = acad.scenario;
  const nodes = [];
  if (window.renderProgress) nodes.push(window.renderProgress(acad));
  if (!scenario) {
    nodes.push(el("p", { class: "hint" }, [
      `${acad.source_label || "synthetic_demo"} · ${acad.note || ""}`,
    ]));
    if (acad.circulation_coverage) nodes.push(el("p", { class: "hint", text: acad.circulation_coverage }));
    if (acad.disclaimer) nodes.push(el("p", { class: "hint", text: acad.disclaimer }));
  }
  const semesters = acad.semesters || [];
  if (!acad.loaded) {
    nodes.push(el("p", { class: "muted", text: acad.note || "No academic fixture loaded." }));
    acadEl.replaceChildren(...nodes);
    return;
  }
  const extractNodes = [];
  if (semesters.length === 0 && !scenario) {
    nodes.push(el("p", { class: "muted", text: acad.note || "No academic rows for this student. Missing is not a zero." }));
  } else if (semesters.length) {
    const table = el("table", { class: "sem" }, [
      el("thead", null, [
        el("tr", null, [
          el("th", { text: "Semester" }),
          el("th", { text: "English (letter_A_F)" }),
          el("th", { text: "Checkouts" }),
          el("th", { text: "Unique titles" }),
          el("th", { text: "Borrowed in window" }),
        ]),
      ]),
    ]);
    const tbody = el("tbody");
    semesters.forEach((s) => {
      const list = (s.borrowed || []).map((b) => {
        const extra = b.renewal_or_repeat ? " · repeat/renewal" : "";
        const back = b.return_date ? ` → ${b.return_date}` : " (out)";
        return el("li", { text: `${b.title} · ${b.checkout_date}${back}${extra}` });
      });
      tbody.appendChild(el("tr", null, [
        el("td", null, [
          el("strong", { text: s.label }),
          el("div", { class: "hint", text: `${s.start} – ${s.end}` }),
          s.missing_note ? el("div", { class: "hint", text: s.missing_note }) : null,
        ]),
        el("td", null, [gradeCell(s.english_grade), el("div", { class: "hint", text: s.status || "" })]),
        el("td", { text: String(s.checkout_count) }),
        el("td", { text: String(s.unique_titles) }),
        el("td", { class: "borrowed" }, [
          el("div", { class: "hint", text: s.borrowing_note || "" }),
          list.length
            ? el("details", null, [el("summary", { text: `${list.length} title event${list.length === 1 ? "" : "s"}` }), el("ul", null, list)])
            : el("span", { class: "missing", text: "none in extract" }),
        ]),
      ]));
    });
    table.appendChild(tbody);
    extractNodes.push(el("h3", { text: "English by semester (extract)" }), table);
  }

  const assesses = acad.assessments || [];
  if (assesses.length === 0 && !scenario) {
    extractNodes.push(el("h3", { text: "Willow Bend Reading Check (fictional)" }), el("p", { class: "muted", text: "No assessment rows. Missing is not a zero." }));
  } else if (assesses.length) {
    const wrap = el("div", { class: "assess" });
    assesses.forEach((a) => {
      const result = a.result == null ? "not posted" : `${a.result} / 4${a.band ? " · " + a.band : ""}`;
      wrap.appendChild(el("article", { class: "assess-card" }, [
        el("strong", { text: a.name }),
        el("div", { class: "meta", text: `${a.date} · grade ${a.grade} form · scale ${a.scale}` }),
        el("div", null, [`Result: ${result}`]),
        el("div", { class: "hint", text: a.compare_note || "" }),
        a.note ? el("div", { class: "hint", text: a.note }) : null,
      ]));
    });
    extractNodes.push(el("h3", { text: "Willow Bend Reading Check (extract)" }), wrap);
  }
  if (extractNodes.length) {
    if (scenario) {
      const details = el("details", { class: "extract-coverage" }, [
        el("summary", { text: "Incomplete extract coverage (not the matched-window story)" }),
        el("p", { class: "hint", text: acad.circulation_coverage || "Coverage is incomplete. Absence is not zero borrowing." }),
      ]);
      extractNodes.forEach((n) => details.appendChild(n));
      nodes.push(details);
    } else {
      nodes.push(...extractNodes);
    }
  }
  acadEl.replaceChildren(...nodes);
}

function renderActivity(items) {
  if (!items || items.length === 0) {
    activityEl.replaceChildren(el("p", { class: "muted", text: "No demo checkouts yet this session." }));
    return;
  }
  const nodes = items.map((a) => {
    const delta = a.copies_delta > 0 ? `+${a.copies_delta} copy` : `${a.copies_delta} copy`;
    return el("div", { class: "act" }, [
      el("strong", { text: `${a.action} · ${a.title || a.book_id}` }),
      el("div", { text: `${a.student_id} · ${delta} · now ${a.copies_available}/${a.copies_total} on shelf` }),
      el("div", { class: "when", text: a.ts ? String(a.ts).replace("T", " ").replace("Z", " UTC") : "" }),
    ]);
  });
  activityEl.replaceChildren(...nodes);
}

async function loadActivity() {
  const res = await fetch("/api/activity");
  if (!res.ok) return;
  const data = await parseJSON(res);
  noteRevision(data.revision);
  renderActivity(data.items || []);
}

function switchTab(name, focus = false) {
  if (!["books", "support", "progress"].includes(name)) return;
  state.activeTab = name;
  ["books", "support", "progress"].forEach(key => {
    const tab = document.getElementById(`tab-${key}`);
    const selected = key === name;
    tab.setAttribute("aria-selected", String(selected));
    tab.tabIndex = selected ? 0 : -1;
    document.getElementById(`panel-${key}`).hidden = !selected;
  });
  if (focus) document.getElementById(`tab-${name}`).focus();
}

function openDirectory() {
  const rail = document.getElementById("student-directory");
  if (window.matchMedia("(max-width: 800px)").matches) {
    rail.classList.add("directory-open");
    rail.setAttribute("role", "dialog");
    rail.setAttribute("aria-modal", "true");
    document.getElementById("main-workspace").inert = true;
    document.querySelector(".top").inert = true;
  }
  searchEl.focus();
}

function closeDirectory() {
  const rail = document.getElementById("student-directory");
  const wasOpen = rail.classList.contains("directory-open");
  rail.classList.remove("directory-open");
  rail.removeAttribute("role");
  rail.removeAttribute("aria-modal");
  document.getElementById("main-workspace").inert = false;
  document.querySelector(".top").inert = false;
  if (wasOpen) document.getElementById("open-students").focus();
}

async function selectStudent(id, opts) {
  closeDirectory();
  if (window.engagement) window.engagement.showView("students");
  if (state.selectedId !== id) {
    state.recSeq++;
    goBtn.disabled = false;
    document.getElementById("theme").value = "";
    document.getElementById("query").value = "";
    headerEl.replaceChildren();
    acadEl.replaceChildren();
    document.getElementById("support-body").replaceChildren();
    recsEl.replaceChildren();
  }
  state.selectedId = id;
  if (window.engagement) {
    window.engagement.offer = null;
    window.engagement.renderConversation();
  }
  renderStudentList();
  confirmEl.hidden = true;
  confirmEl.replaceChildren();
  await refreshStudent(opts);
  if (window.engagement) await window.engagement.refresh();
}

async function refreshStudent(opts) {
  const id = state.selectedId;
  if (!id) return;
  const seq = ++state.loadSeq;
  const keepRecs = opts && opts.keepRecs;
  loansEl.replaceChildren(el("div", { class: "muted", text: "Loading loans…" }));
  try {
    const [detailRes, acadRes, supportRes] = await Promise.all([
      fetch(`/api/students/${encodeURIComponent(id)}`),
      fetch(`/api/students/${encodeURIComponent(id)}/academics`),
      fetch(`/api/students/${encodeURIComponent(id)}/support`),
    ]);
    if (seq !== state.loadSeq) return;
    if (detailRes.status === 404) {
      headerEl.replaceChildren(el("p", { class: "muted", text: "Unknown student." }));
      return;
    }
    const detail = await parseJSON(detailRes);
    if (seq !== state.loadSeq) return;
    if (!detailRes.ok) throw new Error("Could not load selected student");
    if (detail.session_started !== state.sessionStarted) {
      state.sessionStarted = detail.session_started;
      state.revision = 0;
    }
    if (newerThan(detail.revision)) return;
    noteRevision(detail.revision);
    renderHeader(detail);
    renderLoans(detail);
    if (acadRes.ok) {
      const acad = await parseJSON(acadRes);
      if (seq !== state.loadSeq) return;
      noteRevision(acad.revision);
      renderAcademics(acad);
    } else if (seq === state.loadSeq) {
      acadEl.replaceChildren(el("p", { class: "hint", text: "Academic records are unavailable. Finding books and recording the conversation still work." }));
    }
    if (supportRes.ok) {
      const supportData = await parseJSON(supportRes);
      if (seq !== state.loadSeq || state.selectedId !== id) return;
      renderSupport(supportData);
    }
    if (!keepRecs) {
      recsEl.replaceChildren(el("div", { class: "muted", text: "Next book to see in-stock titles for this student." }));
    }
    await loadActivity();
    await loadStudents();
    renderStudentList();
  } catch (err) {
    if (seq !== state.loadSeq) return;
    setStatus(String(err.message || err), "err");
  }
}

function talkingText(rec) {
  return (rec.items || []).map((it) => {
    const tp = it.talking_point || (it.reasons || []).join("; ");
    return `${it.title} (${it.book_id}): ${tp}`;
  }).join("\n");
}

async function copyTalkingItem(it) {
  const payload = `${it.title} (${it.book_id}): ${it.talking_point || (it.reasons || []).join("; ")}`;
  try {
    await navigator.clipboard.writeText(payload);
    setStatus("Talking point copied. Speak it; it is a draft.", "ok");
  } catch (err) {
    setStatus("Clipboard blocked.", "err");
  }
}

async function recommend(ev) {
  if (ev) ev.preventDefault();
  const id = state.selectedId;
  if (!id) {
    setStatus("Pick a student first.", "err");
    return;
  }
  const seq = ++state.recSeq;
  const expectedRev = state.revision;
  goBtn.disabled = true;
  recsEl.replaceChildren(el("div", { class: "muted", text: "Looking…" }));
  setStatus("Looking up the shelf…");
  const body = {
    student_id: id,
    staff_id: staffId(),
    theme: document.getElementById("theme").value,
    query: document.getElementById("query").value,
    stretch: document.getElementById("stretch").checked,
    limit: 5,
  };
  try {
    const active = window.engagement?.active();
    let res, rec;
    if (active) {
      const result = await window.engagement.command({ action: "offer", interaction_id: active.id, query: `${body.query} ${body.theme}`.trim(), stretch: body.stretch });
      window.engagement.offer = { id: result.id, interaction: active.id };
      rec = { ...result.recommendation, revision: result.inventory_revision };
      res = { ok: true };
    } else {
      res = await fetch("/api/recommend", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      rec = await parseJSON(res);
    }
    if (seq !== state.recSeq) return;
    if (state.selectedId !== id) return;
    if (newerThan(rec.revision) || (typeof rec.revision === "number" && rec.revision < expectedRev)) {
      recsEl.replaceChildren(el("div", { class: "muted", text: "Shelf changed. Refreshing…" }));
      await refreshStudent({ keepRecs: false });
      return;
    }
    if (!res.ok) {
      recsEl.replaceChildren(el("div", { class: "muted", text: rec.error || "Lookup failed." }));
      setStatus(rec.error || `HTTP ${res.status}`, "err");
      return;
    }
    if (rec.student_id && rec.student_id !== id) {
      recsEl.replaceChildren(el("div", { class: "muted", text: "Stale result ignored." }));
      return;
    }
    noteRevision(rec.revision);
    const stu = state.students.find((s) => s.student_id === id);
    renderRecs(rec, stu ? stu.first_name : "student");
    setStatus(`Showing ${rec.student_id} · ${rec.explain_mode || "template"}`);
  } catch (err) {
    if (seq !== state.recSeq) return;
    recsEl.replaceChildren(el("div", { class: "muted", text: "API not running. go run ./cmd/shelfmate serve" }));
    setStatus(String(err.message || err), "err");
  } finally {
    if (seq === state.recSeq) goBtn.disabled = false;
  }
}

function showConfirm(data, firstName) {
  confirmEl.hidden = false;
  const copies = `${data.copies_available} of ${data.copies_total} left on the shelf`;
  confirmEl.className = "confirm";
  confirmEl.replaceChildren(
    el("strong", { text: `Checked out ${data.loan.title} to ${firstName}` }),
    el("div", { text: copies }),
    el("div", { class: "hint", text: data.note || "This session only. Restart restores the extract." }),
    el("div", { class: "next" }, [
      buttonAction("Find another book", () => {
        switchTab("books");
        confirmEl.hidden = true;
        document.getElementById("query").focus();
      }),
      buttonAction("Next student", openDirectory, true),
    ]),
  );
}

function buttonAction(label, fn, secondary) {
  const b = el("button", { class: secondary ? "secondary" : "", type: "button", text: label });
  b.addEventListener("click", fn);
  return b;
}

async function checkoutBook(it, btn) {
  const id = state.selectedId;
  if (!id || state.busy) return;
  state.busy = true;
  if (btn) btn.disabled = true;
  setStatus(`Checking out ${it.title}…`);
  try {
    const res = await fetch("/api/checkouts", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        student_id: id,
        book_id: it.book_id,
        staff_id: staffId(),
        retry_id: retryId("co", id, it.book_id),
      }),
    });
    const data = await parseJSON(res);
    if (state.selectedId !== id) return;
    if (!res.ok) {
      confirmEl.hidden = false;
      confirmEl.className = "conflict";
      confirmEl.replaceChildren(
        el("strong", { text: data.error || "Could not check out" }),
        el("div", { text: data.hint || "Refresh this student and try again." }),
        buttonAction("Refresh student", () => refreshStudent({ keepRecs: true }), true),
      );
      setStatus(data.error || "Checkout failed", "err");
      return;
    }
    noteRevision(data.revision);
    const stu = state.students.find((s) => s.student_id === id);
    showConfirm(data, stu ? stu.first_name : id);
    setStatus(`Checked out ${data.loan.title}. ${data.copies_available} left on the shelf.`, "ok");
    await refreshStudent({ keepRecs: true });
    await recommend();
  } catch (err) {
    setStatus(String(err.message || err), "err");
  } finally {
    state.busy = false;
    if (btn) btn.disabled = false;
  }
}

async function returnLoan(loan) {
  const id = state.selectedId;
  if (!id || state.busy) return;
  state.busy = true;
  setStatus(`Returning ${loan.title}…`);
  try {
    const res = await fetch("/api/returns", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        student_id: id,
        loan_id: loan.loan_id,
        staff_id: staffId(),
        retry_id: retryId("ret", id, loan.loan_id),
      }),
    });
    const data = await parseJSON(res);
    if (state.selectedId !== id) return;
    if (!res.ok) {
      setStatus(data.error || "Return failed", "err");
      confirmEl.hidden = false;
      confirmEl.className = "conflict";
      confirmEl.replaceChildren(
        el("strong", { text: data.error || "Could not return" }),
        el("div", { text: data.hint || "Refresh this student." }),
      );
      return;
    }
    noteRevision(data.revision);
    setStatus(`Returned ${data.loan.title}. Shelf is now ${data.copies_available}/${data.copies_total}.`, "ok");
    confirmEl.hidden = false;
    confirmEl.className = "confirm";
    confirmEl.replaceChildren(
      el("strong", { text: `Returned ${data.loan.title}` }),
      el("div", { text: `${data.copies_available} of ${data.copies_total} on the shelf` }),
    );
    await refreshStudent({ keepRecs: true });
    await recommend();
  } catch (err) {
    setStatus(String(err.message || err), "err");
  } finally {
    state.busy = false;
  }
}

searchEl.addEventListener("input", () => {
  renderStudentList();
});
searchEl.addEventListener("keydown", (ev) => {
  if (ev.key === "Enter") {
    ev.preventDefault();
    recommend(ev);
  }
});
findForm.addEventListener("submit", recommend);
document.querySelectorAll("[data-student]").forEach((btn) => {
  btn.addEventListener("click", () => {
    searchEl.value = "";
    const tab = btn.getAttribute("data-tab");
    if (tab) switchTab(tab);
    selectStudent(btn.getAttribute("data-student"));
  });
});
window.addEventListener("focus", () => {
  if (state.selectedId) refreshStudent({ keepRecs: true });
});

async function loadSupportQueue() {
  const response = await fetch("/api/support/queue");
  if (!response.ok) throw new Error("Could not load support queue");
  const data = await response.json();
  state.supportQueue = new Map((data.students || []).map(s => [s.student_id, s]));
}

function renderSupport(data) {
  const result = data.support;
  const guidance = data.reading_guidance || {};
  const now = new Date().toISOString().slice(0, 10);
  const notes = (guidance.teacher_notes || []).map(n => el("article", { class: "note-card" }, [
    el("strong", { text: `Teacher observation · ${n.date}` }),
    el("p", { text: n.text }),
    el("small", { text: `${n.source_id} · structured request: ${n.request}` }),
  ]));
  const shared = (guidance.guidance || []).map(g => {
    const current = g.approved_at <= now && now <= g.review_by;
    const buttons = (g.themes || []).map(theme => {
      const b = buttonAction(`Explore ${theme}`, () => {
        switchTab("books");
        document.getElementById("theme").value = theme;
        document.getElementById("find").scrollIntoView({ behavior: "smooth", block: "start" });
        recommend();
      }, true);
      b.disabled = !current;
      return b;
    });
    return el("article", { class: "note-card" }, [
      el("strong", { text: "Shared reading guidance · counselor approved" }),
      el("p", { text: g.summary }),
      el("p", { class: "hint", text: `${g.source_id} · approved ${g.approved_at} · review by ${g.review_by}${current ? "" : " · review needed before use"}` }),
      el("div", { class: "card-actions" }, buttons),
    ]);
  });
  const actions = [["check_in", "Record a check-in"], ["enjoyed", "Student reported enjoyment"], ["try_another", "Student wants another option"]].map(([action, label]) => {
    const b = buttonAction(label, () => recordFollowup(action, b), true);
    return b;
  });
  const labels = { check_in: "Check-in recorded", enjoyed: "Student reported enjoyment", try_another: "Student requested another option" };
  document.getElementById("support-body").replaceChildren(
    el("div", { class: "support-summary" }, [
      el("strong", { class: `support-band band-${result.band}`, text: result.label }),
      el("span", { text: result.coverage }),
    ]),
    el("p", { class: "hint", text: `${result.disclaimer} Evidence reviewed as of ${result.config.as_of} · ${result.config.id}.` }),
    el("details", { class: "support-evidence" }, [
      el("summary", { text: "Why this band? See evidence and missing inputs" }),
      ...result.evidence.map(e => el("p", null, [
        el("strong", { text: `${e.kind}: ${e.included ? e.value : "not used"}` }),
        ` — ${e.reason}${e.date ? ` (${e.date})` : ""}${e.rule ? ` · ${e.rule}` : ""}`,
      ])),
    ]),
    state.deskNote ? el("details", { class: "support-evidence" }, [el("summary", { text: "Existing librarian context" }), el("p", { text: state.deskNote })]) : null,
    el("p", { text: `Strengths & interests: ${(guidance.strengths || []).join(" · ") || "Ask the student what they enjoy."}` }),
    el("div", { class: "guidance-grid" }, [...notes, ...shared]),
    !notes.length && !shared.length ? el("p", { class: "hint", text: "No shared teacher or counselor guidance in this demo file. This does not mean no concerns." }) : null,
    el("h3", { text: "Check back with the student" }),
    el("p", { class: "hint", text: "Record only something that happened. These actions do not change academic results or support bands. Restart clears them." }),
    el("div", { class: "card-actions" }, actions),
    el("ul", { class: "followup-list" }, (data.followups || []).map(f => el("li", { text: `${labels[f.action] || f.action} · ${f.at} · ${f.staff_id}` }))),
  );
}

async function recordFollowup(action, button) {
  const id = state.selectedId;
  button.disabled = true;
  if (!button.dataset.retryId) button.dataset.retryId = retryId("followup", id, action);
  try {
    const res = await fetch("/api/support/followups", {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ student_id: id, staff_id: staffId(), action, retry_id: button.dataset.retryId }),
    });
    const out = await parseJSON(res);
    if (id !== state.selectedId) return;
    if (!res.ok) throw new Error(out.error || "Follow-up failed");
    setStatus(out.note, "ok");
    await refreshStudent({ keepRecs: true });
  } catch (err) {
    if (id === state.selectedId) setStatus(err.message, "err");
  } finally { button.disabled = false; }
}

const tabNames = ["books", "support", "progress"];
tabNames.forEach((name, index) => {
  const tab = document.getElementById(`tab-${name}`);
  tab.addEventListener("click", () => switchTab(name));
  tab.addEventListener("keydown", ev => {
    let next;
    if (ev.key === "ArrowRight") next = (index + 1) % tabNames.length;
    if (ev.key === "ArrowLeft") next = (index + tabNames.length - 1) % tabNames.length;
    if (ev.key === "Home") next = 0;
    if (ev.key === "End") next = tabNames.length - 1;
    if (next !== undefined) { ev.preventDefault(); switchTab(tabNames[next], true); }
  });
});
const activityDialog = document.getElementById("activity-dialog");
document.getElementById("open-activity").addEventListener("click", () => { activityDialog.showModal(); loadActivity(); });
document.getElementById("close-activity").addEventListener("click", () => activityDialog.close());
activityDialog.addEventListener("click", ev => {
  if (ev.target !== activityDialog) return;
  const rect = activityDialog.getBoundingClientRect();
  if (ev.clientX < rect.left || ev.clientX > rect.right || ev.clientY < rect.top || ev.clientY > rect.bottom) activityDialog.close();
});
document.getElementById("open-students").addEventListener("click", openDirectory);
document.getElementById("close-students").addEventListener("click", closeDirectory);
window.matchMedia("(max-width: 800px)").addEventListener("change", closeDirectory);
document.getElementById("student-directory").addEventListener("keydown", ev => {
  if (!ev.currentTarget.classList.contains("directory-open")) return;
  if (ev.key === "Escape") { ev.preventDefault(); closeDirectory(); }
  if (ev.key === "Tab") {
    const controls = [...ev.currentTarget.querySelectorAll("button, input, select")].filter(n => !n.disabled && n.getClientRects().length);
    const first = controls[0], last = controls[controls.length - 1];
    if (ev.shiftKey && document.activeElement === first) { ev.preventDefault(); last.focus(); }
    if (!ev.shiftKey && document.activeElement === last) { ev.preventDefault(); first.focus(); }
  }
});

document.getElementById("support-filter").addEventListener("change", renderStudentList);

Promise.all([loadSupportQueue(), loadStudents()])
  .then(async () => { await refreshStudent(); await window.engagement.showView("day", { boot: true }); })
  .catch((err) => {
    recsEl.replaceChildren(el("div", { class: "muted", text: "API not running. go run ./cmd/shelfmate serve" }));
    setStatus(String(err.message || err), "err");
  });
