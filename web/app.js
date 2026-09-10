const studentSel = document.getElementById("student");
const searchEl = document.getElementById("search");
const recs = document.getElementById("recs");
const history = document.getElementById("history");
const statusEl = document.getElementById("status");
const goBtn = document.getElementById("go");

let students = [];
let recSeq = 0;
let histSeq = 0;

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
      else node.setAttribute(k, v === true ? "" : String(v));
    });
  }
  (children || []).forEach((c) => {
    if (c == null) return;
    node.appendChild(typeof c === "string" ? text(c) : c);
  });
  return node;
}

function setStatus(msg, isErr) {
  statusEl.textContent = msg || "";
  statusEl.className = isErr ? "status err" : "status";
}

function labelFor(s) {
  const mark = s.demo_role ? ` · ${s.demo_role}` : "";
  return `${s.first_name} ${s.last_initial}. (${s.student_id}, g${s.grade})${mark}`;
}

function renderOptions(filter) {
  const q = (filter || "").trim().toLowerCase();
  const rows = students.filter((s) => {
    if (!q) return true;
    const blob = `${s.first_name} ${s.last_initial} ${s.student_id} ${s.demo_role || ""} ${s.cluster || ""}`.toLowerCase();
    return blob.includes(q);
  });
  const prev = studentSel.value;
  studentSel.innerHTML = "";
  if (rows.length === 0) {
    studentSel.appendChild(el("option", { value: "", disabled: true, text: "No match" }));
    return;
  }
  rows.forEach((s) => {
    studentSel.appendChild(el("option", { value: s.student_id, text: labelFor(s) }));
  });
  if (rows.some((s) => s.student_id === prev)) studentSel.value = prev;
  else studentSel.value = rows[0].student_id;
}

async function loadStudents() {
  const res = await fetch("/api/students");
  if (!res.ok) throw new Error("Could not load students");
  students = await res.json();
  renderOptions("");
  studentSel.value = "S-406";
  await showStudent();
}

async function showStudent() {
  const id = studentSel.value;
  if (!id) {
    history.replaceChildren(el("div", { class: "muted", text: "Pick a student." }));
    return;
  }
  const seq = ++histSeq;
  history.replaceChildren(el("div", { class: "muted", text: "Loading history…" }));
  try {
    const res = await fetch(`/api/students/${encodeURIComponent(id)}`);
    if (seq !== histSeq) return;
    if (res.status === 404) {
      history.replaceChildren(el("div", { class: "muted", text: "Unknown student." }));
      return;
    }
    if (!res.ok) throw new Error("history failed");
    const data = await res.json();
    if (seq !== histSeq) return;
    const st = data.student || {};
    const evs = data.history || [];
    const nodes = [
      el("div", null, [
        `${st.first_name || ""} ${st.last_initial || ""}. · grade ${st.grade} · band ${st.reading_band || "—"} · cluster ${st.cluster || "—"}`,
      ]),
    ];
    if (st.anecdote) nodes.push(el("div", { class: "why" }, [st.anecdote]));
    if (evs.length === 0) {
      nodes.push(el("div", { class: "muted", text: "No checkouts. Cold start — popularity fallback unless you type a query." }));
    } else {
      evs.forEach((e) => {
        nodes.push(el("div", { class: "card" }, [
          el("strong", { text: e.title || e.book_id }),
          el("span", { class: "meta", text: `${e.checkout_date} → ${e.return_date || "out"}` }),
        ]));
      });
    }
    history.replaceChildren(...nodes);
  } catch (err) {
    if (seq !== histSeq) return;
    history.replaceChildren(el("div", { class: "muted", text: "Could not load history." }));
    setStatus(String(err.message || err), true);
  }
}

function talkingText(rec) {
  const lines = (rec.items || []).map((it) => {
    const tp = it.talking_point || (it.reasons || []).join("; ");
    return `${it.title} (${it.book_id}): ${tp}`;
  });
  return lines.join("\n");
}

async function copyTalking(rec) {
  const payload = talkingText(rec);
  try {
    await navigator.clipboard.writeText(payload);
    setStatus("Talking points copied. Speak them; they are drafts.");
  } catch (err) {
    setStatus("Clipboard blocked. Select the talking points instead.", true);
  }
}

function renderRecs(rec) {
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
    nodes.push(el("div", { class: "card" }, [
      el("strong", { text: it.title || it.book_id }),
      el("span", { class: "meta", text: `${it.author || ""} · ${it.cluster || ""} · ${it.pages}p · ${it.copies_available} copies` }),
      el("div", { class: "evidence" }, [`Evidence: ${(it.reasons || []).join("; ") || "none"}`]),
      el("div", { class: "talk" }, [`Draft: ${it.talking_point || "—"}`]),
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
  if (items.length) {
    const btn = el("button", { class: "copy", type: "button", text: "Copy talking points" });
    btn.addEventListener("click", () => copyTalking(rec));
    nodes.push(btn);
  }
  recs.replaceChildren(...nodes);
}

async function recommend() {
  const id = studentSel.value;
  if (!id) {
    setStatus("Pick a student first.", true);
    return;
  }
  const seq = ++recSeq;
  goBtn.disabled = true;
  recs.replaceChildren(el("div", { class: "muted", text: "Looking…" }));
  setStatus("Looking up the shelf…");
  const body = {
    student_id: id,
    staff_id: document.getElementById("staff").value,
    query: document.getElementById("query").value,
    stretch: document.getElementById("stretch").checked,
    limit: 5,
  };
  try {
    const res = await fetch("/api/recommend", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
    });
    const rec = await res.json().catch(() => ({}));
    if (seq !== recSeq) return;
    if (!res.ok) {
      recs.replaceChildren(el("div", { class: "muted", text: rec.error || "Lookup failed." }));
      setStatus(rec.error || `HTTP ${res.status}`, true);
      return;
    }
    if (rec.student_id && rec.student_id !== id) {
      recs.replaceChildren(el("div", { class: "muted", text: "Stale result ignored." }));
      return;
    }
    renderRecs(rec);
    setStatus(`Showing ${rec.student_id} · ${rec.explain_mode || "template"}`);
  } catch (err) {
    if (seq !== recSeq) return;
    recs.replaceChildren(el("div", { class: "muted", text: "API not running. go run ./cmd/shelfmate serve" }));
    setStatus(String(err.message || err), true);
  } finally {
    if (seq === recSeq) goBtn.disabled = false;
  }
}

function pickStudent(id) {
  searchEl.value = "";
  renderOptions("");
  studentSel.value = id;
  showStudent();
}

document.getElementById("go").addEventListener("click", recommend);
searchEl.addEventListener("input", () => renderOptions(searchEl.value));
searchEl.addEventListener("keydown", (ev) => {
  if (ev.key === "Enter") {
    ev.preventDefault();
    recommend();
  }
});
studentSel.addEventListener("change", showStudent);
studentSel.addEventListener("keydown", (ev) => {
  if (ev.key === "Enter") {
    ev.preventDefault();
    recommend();
  }
});
document.querySelectorAll("[data-student]").forEach((btn) => {
  btn.addEventListener("click", () => pickStudent(btn.getAttribute("data-student")));
});
loadStudents().catch((err) => {
  recs.replaceChildren(el("div", { class: "muted", text: "API not running. go run ./cmd/shelfmate serve" }));
  setStatus(String(err.message || err), true);
});
