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

  function dateValue(value) {
    const s = isoDate(value);
    if (!/^\d{4}-\d{2}-\d{2}$/.test(s)) return NaN;
    const time = Date.parse(s + "T00:00:00Z");
    return Number.isFinite(time) && new Date(time).toISOString().slice(0, 10) === s ? time : NaN;
  }

  function byUTC(a, b) {
    const da = dateValue(a), db = dateValue(b);
    if (Number.isFinite(da) && Number.isFinite(db) && da !== db) return da - db;
    return isoDate(a).localeCompare(isoDate(b));
  }

  function sortedWindows(scenario) {
    return (scenario.windows || []).slice().sort((a, b) => byUTC(a.start, b.start) || String(a.id || "").localeCompare(String(b.id || "")));
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
      const ordered = sortedWindows(scenario);
      body.textContent = (ordered.length ? `Checkouts ${ordered.map(w => w.borrowing_known ? w.checkout_count : "—").join(" → ")} · English ${ordered.map(w => w.english_status === "final" ? w.english_grade ?? "—" : "—").join(" → ")}. ` : "")
        + (scenario.caveat || "Illustrative only — co-timing is not causal proof.");
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

    function linkedChart(windows) {
      const card = make("section", null, "progress-card coordinated-chart");
      card.append(make("h3", "Borrowing, reading checks & English"), make("p", "One semester timeline, three separate measures. Hover or focus a semester to compare all values. Reading-check forms are labeled; no cross-form growth is inferred.", "hint"));
      const width = Math.max(800, windows.length * 130 + 170);
      const left = 140, right = width - 65;
      const times = windows.map(w => dateValue(w.end));
      if (times.some(t => !Number.isFinite(t))) {
        card.append(make("p", "Cannot plot an invalid observation date."));
        root.insertBefore(card, root.querySelector(".timeline-card"));
        return;
      }
      const start = Math.min(...times), span = Math.max(1, Math.max(...times) - start);
      const x = i => times.length === 1 ? (left + right) / 2 : left + (times[i] - start) / span * (right - left);
      const svg = svgNode("svg", { viewBox: `0 0 ${width} 680`, role: "group", "aria-label": "Semester-aligned borrowing, reading checks and English grades" });
      const label = (text, xx, yy, anchor = "start") => {
        const n = svgNode("text", { x: xx, y: yy, fill: INK, "font-size": 12, "text-anchor": anchor });
        n.textContent = text; svg.append(n);
      };
      const maxBorrowing = Math.max(1, ...windows.map(w => w.borrowing_known && Number.isFinite(w.checkout_count) ? w.checkout_count : 0));
      const lanes = [
        { name: "Borrowing", unit: "checkout events", top: 45, bottom: 175, max: maxBorrowing },
        { name: "Reading check", unit: "1–4 · form-specific", top: 265, bottom: 370, max: 3 },
        { name: "English", unit: "letter categories", top: 460, bottom: 590, max: LETTERS.length - 1 },
      ];
      const y = (lane, value) => lane.bottom - value / lane.max * (lane.bottom - lane.top);
      lanes.forEach((lane, index) => {
        label(lane.name, 8, lane.top + 8); label(lane.unit, 8, lane.top + 27);
        const ticks = index === 0 ? [0, maxBorrowing] : index === 1 ? [0, 1, 2, 3] : [0, 2, 5, 8, 11];
        ticks.forEach(t => {
          const yy = y(lane, t);
          svg.append(svgNode("line", { x1: left - 25, x2: right + 25, y1: yy, y2: yy, stroke: GRID }));
          label(index === 1 ? t + 1 : index === 2 ? LETTERS[t] : t, left - 30, yy + 4, "end");
        });
      });
      const cursor = svgNode("line", { x1: x(0), x2: x(0), y1: 25, y2: 605, stroke: SERIES, "stroke-width": 1, opacity: .45 });
      svg.append(cursor);
      const detail = make("p", null, "chart-detail linked-detail");
      detail.setAttribute("aria-live", "polite");
      const controls = make("div", null, "semester-controls");
      controls.setAttribute("role", "group"); controls.setAttribute("aria-label", "Inspect a semester");
      const rows = windows.map(w => {
        const r = w.reading_check;
        const borrowing = w.borrowing_known && Number.isInteger(w.checkout_count) && w.checkout_count >= 0 ? w.checkout_count : null;
        const reading = r && Number.isInteger(r.result) && r.result >= 1 && r.result <= 4 ? r.result : null;
        const english = w.english_status === "final" && LETTERS.includes(w.english_grade) ? w.english_grade : null;
        return { w, borrowing, reading, english,
          readingText: reading == null ? "Not posted" : `${reading}/4 · grade ${r.grade_form} form · ${isoDate(r.date)}` };
      });
      const show = index => {
        const { w, borrowing, readingText, english } = rows[index];
        cursor.setAttribute("x1", x(index)); cursor.setAttribute("x2", x(index));
        detail.textContent = `${w.academic_year} ${w.semester} · ${isoDate(w.start)}–${isoDate(w.end)} (${w.inclusive_days} days) · Borrowing: ${borrowing ?? "not available"} · Reading check: ${readingText} · English: ${english ?? "not posted"}`;
        [...controls.children].forEach((b, i) => b.setAttribute("aria-pressed", String(i === index)));
      };
      rows.forEach(({ w, borrowing, reading, readingText, english }, i) => {
        const xx = x(i);
        const values = [borrowing, reading == null ? null : reading - 1, english == null ? null : LETTERS.indexOf(english)];
        values.forEach((value, j) => {
          const lane = lanes[j];
          if (value == null) { label("Not posted", xx, lane.bottom - 8, "middle"); return; }
          const yy = y(lane, value);
          if (j === 0) svg.append(svgNode("rect", { x: xx - 10, y: yy, width: 20, height: lane.bottom - yy, rx: 4, fill: SERIES }));
          else svg.append(svgNode("circle", { cx: xx, cy: yy, r: 5, fill: SERIES, stroke: SURFACE, "stroke-width": 2 }));
          label(j === 0 ? borrowing : j === 1 ? reading : english, xx, yy - 10, "middle");
        });
        label(w.reading_check ? `Grade ${w.reading_check.grade_form} form` : "No assessment", xx, 398, "middle");
        label(w.academic_year, xx, 630, "middle"); label(w.semester, xx, 650, "middle");
        const hit = svgNode("rect", { x: xx - 28, y: 20, width: 56, height: 585, fill: "transparent", tabindex: 0, role: "img", "aria-label": `${w.academic_year} ${w.semester}: ${borrowing ?? "unknown"} checkouts, reading ${readingText}, English ${english ?? "not posted"}` });
        hit.addEventListener("mouseenter", () => show(i)); hit.addEventListener("focus", () => show(i)); hit.addEventListener("click", () => show(i));
        svg.append(hit);
        const button = make("button", `${w.academic_year} ${w.semester}`, "secondary"); button.type = "button";
        button.addEventListener("click", () => show(i)); button.addEventListener("focus", () => show(i)); controls.append(button);
      });
      const scroll = make("div", null, "coordinated-scroll"); scroll.append(svg);
      card.append(controls, scroll, detail);
      const tableDetails = make("details"); tableDetails.append(make("summary", "View all three measures as a table"));
      const table = make("table", null, "sem"), head = make("thead"), tr = make("tr");
      ["Year / semester", "Window (UTC)", "Checkouts", "Reading check", "English"].forEach(v => { const th = make("th", v); th.scope = "col"; tr.append(th); });
      head.append(tr); table.append(head);
      const body = make("tbody");
      rows.forEach(({ w, borrowing, readingText, english }) => {
        const row = make("tr");
        [`${w.academic_year} ${w.semester}`, `${isoDate(w.start)}–${isoDate(w.end)}`, borrowing ?? "Not available", readingText, english ?? "Not posted"].forEach(v => row.append(make("td", v)));
        body.append(row);
      });
      table.append(body); tableDetails.append(table); card.append(tableDetails);
      root.insertBefore(card, root.querySelector(".timeline-card"));
      show(rows.length - 1);
    }

    if (scenario && (scenario.windows || []).length) {
      const windows = sortedWindows(scenario);
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
      events.sort((a, b) => byUTC(a.date, b.date) || a.kind.localeCompare(b.kind));
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

      linkedChart(windows);
      return root;
    }

    const semesters = (acad.semesters || []).slice().sort((a, b) => byUTC(a.start, b.start) || byUTC(a.end, b.end) || String(a.id || "").localeCompare(String(b.id || "")));
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
      plot(title, assessments.sort((a, b) => byUTC(a.date, b.date)).map(a => ({
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
