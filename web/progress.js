/* Separate observed measures; no inferred academic impact or missing-value interpolation. */
window.renderProgress = function (acad) {
  const root = document.createElement("div");
  root.className = "progress-grid";
  if (acad && acad.demo_case === "improving_same_form_illustrative") {
    const banner = document.createElement("aside");
    banner.className = "illustrative-banner";
    banner.setAttribute("role", "note");
    const title = document.createElement("strong");
    title.textContent = "Synthetic illustrative trajectory";
    const body = document.createElement("p");
    body.textContent = acad.demo_note || "Same-form reading check 2 then 3 with a historical English B leading to the existing B+. Borrowed is not finished. Co-timing is not causal proof.";
    banner.append(title, body);
    root.append(banner);
  }
  const make = (tag, value, cls) => {
    const n = document.createElement(tag);
    if (value != null) n.textContent = value;
    if (cls) n.className = cls;
    return n;
  };
  const svgNode = (tag, attrs) => {
    const n = document.createElementNS("http://www.w3.org/2000/svg", tag);
    Object.entries(attrs).forEach(([k, v]) => n.setAttribute(k, v));
    return n;
  };
  function plot(title, rows, levels, note, bar) {
    if (!rows.length) return;
    const card = make("section", null, "progress-card");
    card.append(make("h3", title), make("p", note, "hint"));
    const svg = svgNode("svg", { viewBox: "0 0 600 240", role: "group", "aria-label": title });
    const left = 68, right = 570, top = 24, bottom = 180;
    const y = i => bottom - i * (bottom - top) / Math.max(1, levels.length - 1);
    levels.forEach((level, i) => {
      svg.append(svgNode("line", { x1: left, x2: right, y1: y(i), y2: y(i), stroke: "#ddd4c4", "stroke-width": 1 }));
      const label = svgNode("text", { x: left - 10, y: y(i) + 4, "text-anchor": "end", fill: "#3d3830", "font-size": 12 });
      label.textContent = level;
      svg.append(label);
    });
    const caption = make("p", "Hover or focus a mark for details. All values are also in the table.", "chart-detail");
    caption.setAttribute("aria-live", "polite");
    rows.forEach((row, index) => {
      const x = left + (index + 0.5) * (right - left) / rows.length;
      const pos = row.value == null ? -1 : levels.indexOf(row.value);
      const bottomLabel = svgNode("text", { x, y: 204, "text-anchor": "middle", fill: "#3d3830", "font-size": 11 });
      bottomLabel.textContent = row.date;
      svg.append(bottomLabel);
      if (pos < 0) {
        const absent = svgNode("text", { x, y: bottom - 8, "text-anchor": "middle", fill: "#6b6458", "font-size": 12 });
        absent.textContent = "No result";
        svg.append(absent);
        return;
      }
      const group = svgNode("g", { tabindex: 0, role: "img", "aria-label": row.detail });
      const tip = svgNode("title", {}); tip.textContent = row.detail; group.append(tip);
      if (bar) group.append(svgNode("rect", { x: x - 9, y: y(pos), width: 18, height: Math.max(0, bottom - y(pos)), rx: 4, fill: "#3779B5" }));
      group.append(svgNode("circle", { cx: x, cy: y(pos), r: 5, fill: "#3779B5", stroke: "#fcfcfb", "stroke-width": 2 }));
      group.append(svgNode("circle", { cx: x, cy: y(pos), r: 15, fill: "transparent" }));
      group.addEventListener("mouseenter", () => { caption.textContent = row.detail; });
      group.addEventListener("focus", () => { caption.textContent = row.detail; });
      svg.append(group);
    });
    card.append(svg, caption);
    const details = make("details");
    details.append(make("summary", "View chart data"));
    const table = make("table", null, "sem");
    const thead = make("thead"), head = make("tr");
    ["Date", "Value", "Context"].forEach(v => head.append(make("th", v)));
    thead.append(head); table.append(thead);
    const tbody = make("tbody");
    rows.forEach(row => {
      const tr = make("tr");
      [row.date, row.label == null ? "Not available" : row.label, row.detail].forEach(v => tr.append(make("td", v)));
      tbody.append(tr);
    });
    table.append(tbody); details.append(table); card.append(details); root.append(card);
  }
  const semesters = (acad.semesters || []).slice().sort((a, b) => a.end.localeCompare(b.end));
  const events = [];
  const seenLoans = new Set();
  semesters.forEach(s => {
    if (s.status === "final") events.push({ date: s.end, kind: "English", detail: `${s.label}: ${s.english_grade ?? "not posted"} (${s.scale})` });
    (s.borrowed || []).forEach(b => {
      const key = `${b.book_id}|${b.checkout_date}`;
      if (seenLoans.has(key)) return;
      seenLoans.add(key);
      events.push({ date: b.checkout_date, kind: "Borrowed", detail: `${b.title}${b.renewal_or_repeat ? " · repeat/renewal" : ""}. Borrowed does not mean finished.` });
    });
  });
  (acad.assessments || []).forEach(a => events.push({ date: a.date, kind: "Assessment", detail: `${a.name}: ${a.result ?? "not posted"} / 4 · grade ${a.grade} form. ${a.compare_note || ""}` }));
  events.sort((a, b) => a.date.localeCompare(b.date) || a.kind.localeCompare(b.kind));
  if (events.length) {
    const card = make("section", null, "progress-card timeline-card");
    card.append(make("h3", "Books & learning · one timeline"), make("p", "Select a month to see books borrowed and academic observations together. Fictional context—not proof that books caused a result. Borrowing is not completion.", "hint"));
    const controls = make("div", null, "timeline-controls");
    const label = make("label", "Observation window");
    const select = make("select");
    select.setAttribute("aria-label", "Timeline observation month");
    const all = make("option", "All recorded dates"); all.value = ""; select.append(all);
    [...new Set(events.map(e => e.date.slice(0, 7)))].forEach(month => {
      const option = make("option", month); option.value = month; select.append(option);
    });
    label.append(select); controls.append(label); card.append(controls);
    const list = make("ol", null, "learning-timeline");
    const draw = () => {
      list.replaceChildren();
      events.filter(e => !select.value || e.date.startsWith(select.value)).forEach(e => {
        const row = make("li");
        row.append(make("time", e.date), make("strong", e.kind), make("span", e.detail));
        list.append(row);
      });
    };
    select.addEventListener("change", draw); draw(); card.append(list); root.append(card);
  }
  plot("English · reported letter grades", semesters.map(s => {
    const valid = s.status === "final" && s.scale === "letter_A_F" && /^[ABCD][+-]?$|^F$/.test(s.english_grade || "");
    return { date: s.end, value: valid ? s.english_grade[0] : null, label: valid ? s.english_grade : null,
      detail: `${s.label}: ${valid ? s.english_grade : "not posted or incomparable"}. ${s.status}. ${s.missing_note || ""}` };
  }), ["F", "D", "C", "B", "A"], "Ordinal categories, not a numeric growth scale. Plus/minus retained in details; no lines connect grades.");
  const groups = new Map();
  (acad.assessments || []).forEach(a => {
    const key = `${a.name} · grade ${a.grade} · ${a.scale}`;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key).push(a);
  });
  groups.forEach((assessments, title) => {
    plot(title, assessments.sort((a, b) => a.date.localeCompare(b.date)).map(a => ({
      date: a.date, value: a.scale === "willow_bend_reading_1_4" && Number.isInteger(a.result) ? a.result : null,
      label: a.result, detail: `${a.date}: ${a.result == null ? "not posted" : a.result + " / 4"}. ${a.compare_note || ""} ${a.note || ""}`,
    })), [1, 2, 3, 4], "Fictional reading check. Different grade forms are separate; no cross-form growth is calculated.");
  });
  const borrowing = semesters.map(s => ({
    date: s.end, value: s.borrowing_known ? s.checkout_count : null, label: s.borrowing_known ? s.checkout_count : null,
    detail: `${s.label}: ${s.borrowing_known ? s.checkout_count + " observed checkout events, " + s.unique_titles + " unique titles" : "no covered borrowing window"}. ${s.borrowing_note || ""} Partial export; not a count of books read.`,
  }));
  const max = Math.max(1, ...borrowing.map(b => b.value || 0));
  plot("Borrowing · observed checkout events", borrowing, Array.from({ length: max + 1 }, (_, i) => i), "Partial export plus this process. Periods have different coverage; counts are not comparable reading rates. Source and session events are identified in Current loans & history.", true);
  return root;
};
