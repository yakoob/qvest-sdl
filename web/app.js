const studentSel = document.getElementById("student");
const recs = document.getElementById("recs");
const history = document.getElementById("history");

async function loadStudents() {
  const rows = await (await fetch("/api/students")).json();
  studentSel.innerHTML = rows.map((s) => {
    const mark = s.demo_role ? ` · ${s.demo_role}` : "";
    return `<option value="${s.student_id}">${s.first_name} ${s.last_initial}. (${s.student_id}, g${s.grade})${mark}</option>`;
  }).join("");
  studentSel.value = "S-406";
  await showStudent();
}

async function showStudent() {
  const id = studentSel.value;
  const data = await (await fetch(`/api/students/${id}`)).json();
  const st = data.student;
  const evs = (data.history || []).slice().reverse();
  history.innerHTML = `
    <div>${st.first_name} ${st.last_initial}. · band ${st.reading_band} · cluster ${st.cluster}</div>
    <div class="why">${st.anecdote || ""}</div>
    ${evs.map((e) => `<div class="card"><strong>${e.title}</strong><span class="meta">${e.checkout_date} → ${e.return_date || "out"}</span></div>`).join("") || "<div class='muted'>No checkouts. Cold start.</div>"}
  `;
}

async function recommend() {
  recs.textContent = "Looking…";
  const body = {
    student_id: studentSel.value,
    staff_id: document.getElementById("staff").value,
    query: document.getElementById("query").value,
    stretch: document.getElementById("stretch").checked,
    limit: 5,
  };
  const rec = await (await fetch("/api/recommend", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  })).json();
  recs.innerHTML = rec.items.map((it) => `
    <div class="card">
      <strong>${it.title}</strong>
      <span class="meta">${it.author} · ${it.cluster} · ${it.pages}p · ${it.copies_available} copies</span>
      <div class="why">${it.talking_point || (it.reasons || []).join(" · ")}</div>
    </div>
  `).join("") + (rec.dropped || []).slice(0, 3).map((d) => `<div class="drop">dropped ${d.title}: ${d.why}</div>`).join("");
}

document.getElementById("go").addEventListener("click", recommend);
studentSel.addEventListener("change", showStudent);
loadStudents().catch((err) => {
  recs.textContent = "API not running. go run ./cmd/shelfmate serve";
  console.error(err);
});
