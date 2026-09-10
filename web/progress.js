/* Separate observed measures; no inferred academic impact or missing-value interpolation. */
(function () {
  const LETTERS = ["F", "D-", "D", "D+", "C-", "C", "C+", "B-", "B", "B+", "A-", "A", "A+"];
  const SERIES = "#2a78d6";
  const SURFACE = "#fcfcfb";
  const GRID = "#e1e0d9";
  const INK = "#0b0b0b";
  const MUTED = "#52514e";

  function isoDate(value) {
    if (value == null) return "";
    const s = String(value).trim();
    return s.length >= 10 ? s.slice(0, 10) : s;
  }

  window.renderProgress = function (acad) {
    const root = document.createElement("div");
    root.className = "progress-grid";
    const scenario = acad && acad.scenario;
    const isPrimary = scenario && scenario.id === "improving_engagement_illustrative";
    if (scenario && scenario.label) {
      const banner = document.createElement("aside");
      banner.className = isPrimary ? "illustrative-banner" : "illustrative-banner comparator-banner";
      banner.setAttribute("role", "note");
      const title = document.createElement("strong");
      title.textContent = scenario.label;
      const body = document.createElement("p");
      body.textContent = isPrimary
        ? "Checkouts 2 → 4 → 6 → 9 · English D+ → C- → C → B+. " + (scenario.caveat || "Illustrative only — co-timing is not causal proof.")
        : (scenario.caveat || "Illustrative only — co-timing is not causal proof.");
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
      const height = Math.max(240, 28 + levels.length * 18);
      const svg = svgNode("svg", { viewBox: `0 0 640 ${height}`, role: "group", "aria-label": title });
      const left = 72, right = 600, top = 24, bottom = height - 48;
      const y = i => bottom - i * (bottom - top) / Math.max(1, levels.length - 1);
      levels.forEach((level, i) => {
        svg.append(svgNode("line", { x1: left, x2: right, y1: y(i), y2: y(i), stroke: GRID, "stroke-width": 1 }));
        const label = svgNode("text", { x: left - 10, y: y(i) + 4, "text-anchor": "end", fill: MUTED, "font-size": 11, "font-variant-numeric": "tabular-nums" });
        label.textContent = String(level);
        svg.append(label);
      });
      const caption = make("p", "Hover or focus a mark for details. All values are also in the table.", "chart-detail");
      caption.setAttribute("aria-live", "polite");
      rows.forEach((row, index) => {
        const x = left + (index + 0.5) * (right - left) / rows.length;
        const pos = row.value == null ? -1 : levels.indexOf(row.value);
        const bottomLabel = svgNode("text", { x, y: height - 20, "text-anchor": "middle", fill: MUTED, "font-size": 11 });
        bottomLabel.textContent = isoDate(row.date);
        svg.append(bottomLabel);
        if (pos < 0) {
          const absent = svgNode("text", { x, y: bottom - 8, "text-anchor": "middle", fill: MUTED, "font-size": 12 });
          absent.textContent = "No result";
          svg.append(absent);
          return;
        }
        const group = svgNode("g", { tabindex: 0, role: "img", "aria-label": row.detail });
        const tip = svgNode("title", {}); tip.textContent = row.detail; group.append(tip);
        if (bar) {
          const h = Math.max(0, bottom - y(pos));
          group.append(svgNode("rect", { x: x - 9, y: y(pos), width: 18, height: h, rx: 4, fill: SERIES }));
        }
        if (row.label != null && !bar) {
          const direct = svgNode("text", { x: x + 10, y: y(pos) + 4, fill: INK, "font-size": 12, "font-weight": 600 });
          direct.textContent = String(row.label);
          group.append(direct);
        }
        group.append(svgNode("circle", { cx: x, cy: y(pos), r: 5, fill: SERIES, stroke: SURFACE, "stroke-width": 2 }));
        group.append(svgNode("circle", { cx: x, cy: y(pos), r: 14, fill: "transparent" }));
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
        [isoDate(row.date), row.label == null ? "Not available" : String(row.label), row.detail].forEach(v => tr.append(make("td", v)));
        tbody.append(tr);
      });
      table.append(tbody); details.append(table); card.append(details); root.append(card);
    }

    if (scenario && (scenario.windows || []).length) {
      const windows = scenario.windows.slice().sort((a, b) => isoDate(a.start).localeCompare(isoDate(b.start)));
      const events = [];
      windows.forEach(w => {
        if (w.english_status === "final") {
          events.push({ date: isoDate(w.end), kind: "English", detail: `${w.academic_year} ${w.semester} · grade ${w.school_grade}: ${w.english_grade ?? "not posted"}` });
        }
        if (w.reading_check) {
          const r = w.reading_check;
          events.push({ date: isoDate(r.date), kind: "Assessment", detail: `Willow Bend Reading Check grade ${r.grade_form} form: ${r.result == null ? "not posted" : r.result + " / 4"}` });
        }
        (w.borrowed || []).forEach(b => {
          events.push({ date: isoDate(b.checkout_date), kind: "Borrowed", detail: `${b.title}${b.renewal_or_repeat ? " · repeat/renewal" : ""}. Borrowed does not mean finished.` });
        });
      });
      events.sort((a, b) => a.date.localeCompare(b.date) || a.kind.localeCompare(b.kind));
      const card = make("section", null, "progress-card timeline-card");
      card.append(make("h3", "Matched observation windows"), make("p", `${scenario.matched_window_days || 84}-day windows · isolated synthetic timeline · not operational loans`, "hint"));
      const controls = make("div", null, "timeline-controls");
      const label = make("label", "Observation window");
      const select = make("select");
      select.setAttribute("aria-label", "Timeline observation month");
      const all = make("option", "All recorded dates"); all.value = ""; select.append(all);
      [...new Set(events.map(e => e.date.slice(0, 7)).filter(Boolean))].forEach(month => {
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

      plot(
        "English · exact letter grades",
        windows.map(w => {
          const valid = w.english_status === "final" && LETTERS.includes(w.english_grade);
          return {
            date: isoDate(w.end),
            value: valid ? w.english_grade : null,
            label: valid ? w.english_grade : null,
            detail: `${w.academic_year} ${w.semester} · grade ${w.school_grade}: ${valid ? w.english_grade : "not posted"}.`,
          };
        }),
        LETTERS,
        "Plus and minus are distinct steps. Ordinal categories, not a numeric growth scale.",
      );

      const byForm = new Map();
      windows.forEach(w => {
        const r = w.reading_check;
        if (!r) return;
        const key = `Willow Bend Reading Check · grade ${r.grade_form}`;
        if (!byForm.has(key)) byForm.set(key, []);
        byForm.get(key).push({
          date: isoDate(r.date),
          value: Number.isInteger(r.result) ? r.result : null,
          label: r.result == null ? null : r.result,
          detail: `${isoDate(r.date)}: ${r.result == null ? "not posted" : r.result + " / 4"} · grade ${r.grade_form} form.`,
        });
      });
      byForm.forEach((rows, title) => {
        plot(title, rows.sort((a, b) => a.date.localeCompare(b.date)), [1, 2, 3, 4], "Fictional reading check. Different grade forms are separate charts.");
      });

      const borrowing = windows.map(w => ({
        date: isoDate(w.end),
        value: w.borrowing_known ? w.checkout_count : null,
        label: w.borrowing_known ? w.checkout_count : null,
        detail: `${w.academic_year} ${w.semester}: ${w.borrowing_known ? w.checkout_count + " checkouts, " + w.unique_titles + " unique titles" : "not a covered window"} (${w.inclusive_days} days).`,
      }));
      const max = Math.max(1, ...borrowing.map(b => b.value || 0));
      plot("Borrowing · matched windows", borrowing, Array.from({ length: max + 1 }, (_, i) => i), "Equal-length synthetic windows. Not operational circulation and not books finished.", true);
      return root;
    }

    const semesters = (acad.semesters || []).slice().sort((a, b) => isoDate(a.start).localeCompare(isoDate(b.start)) || isoDate(a.end).localeCompare(isoDate(b.end)));
    const events = [];
    const seenLoans = new Set();
    semesters.forEach(s => {
      if (s.status === "final") events.push({ date: isoDate(s.end), kind: "English", detail: `${s.label}: ${s.english_grade ?? "not posted"} (${s.scale})` });
      (s.borrowed || []).forEach(b => {
        const key = `${b.book_id}|${b.checkout_date}`;
        if (seenLoans.has(key)) return;
        seenLoans.add(key);
        events.push({ date: isoDate(b.checkout_date), kind: "Borrowed", detail: `${b.title}${b.renewal_or_repeat ? " · repeat/renewal" : ""}. Borrowed does not mean finished.` });
      });
    });
    (acad.assessments || []).forEach(a => events.push({ date: isoDate(a.date), kind: "Assessment", detail: `${a.name}: ${a.result ?? "not posted"} / 4 · grade ${a.grade} form. ${a.compare_note || ""}` }));
    events.sort((a, b) => a.date.localeCompare(b.date) || a.kind.localeCompare(b.kind));
    if (events.length) {
      const card = make("section", null, "progress-card timeline-card");
      card.append(make("h3", "Books & learning · extract timeline"), make("p", "Incomplete extract coverage. Absence is not zero borrowing.", "hint"));
      const controls = make("div", null, "timeline-controls");
      const label = make("label", "Observation window");
      const select = make("select");
      select.setAttribute("aria-label", "Timeline observation month");
      const all = make("option", "All recorded dates"); all.value = ""; select.append(all);
      [...new Set(events.map(e => e.date.slice(0, 7)).filter(Boolean))].forEach(month => {
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
      const valid = s.status === "final" && s.scale === "letter_A_F" && LETTERS.includes(s.english_grade || "");
      return { date: isoDate(s.end), value: valid ? s.english_grade : null, label: valid ? s.english_grade : null,
        detail: `${s.label}: ${valid ? s.english_grade : "not posted or incomparable"}. ${s.status}. ${s.missing_note || ""}` };
    }), LETTERS, "Ordinal categories with plus/minus as distinct steps. Incomplete extract coverage.");
    const groups = new Map();
    (acad.assessments || []).forEach(a => {
      const key = `${a.name} · grade ${a.grade} · ${a.scale}`;
      if (!groups.has(key)) groups.set(key, []);
      groups.get(key).push(a);
    });
    groups.forEach((assessments, title) => {
      plot(title, assessments.sort((a, b) => isoDate(a.date).localeCompare(isoDate(b.date))).map(a => ({
        date: isoDate(a.date), value: a.scale === "willow_bend_reading_1_4" && Number.isInteger(a.result) ? a.result : null,
        label: a.result, detail: `${isoDate(a.date)}: ${a.result == null ? "not posted" : a.result + " / 4"}. ${a.compare_note || ""} ${a.note || ""}`,
      })), [1, 2, 3, 4], "Fictional reading check. Different grade forms are separate; no cross-form growth is calculated.");
    });
    const borrowing = semesters.map(s => ({
      date: isoDate(s.end), value: s.borrowing_known ? s.checkout_count : null, label: s.borrowing_known ? s.checkout_count : null,
      detail: `${s.label}: ${s.borrowing_known ? s.checkout_count + " observed checkout events, " + s.unique_titles + " unique titles" : "no covered borrowing window"}. ${s.borrowing_note || ""} Partial export; not a count of books read.`,
    }));
    const max = Math.max(1, ...borrowing.map(b => b.value || 0));
    plot("Borrowing · observed checkout events", borrowing, Array.from({ length: max + 1 }, (_, i) => i), "Partial export plus this process. Periods have different coverage; counts are not comparable reading rates.", true);
    return root;
  };
})();
