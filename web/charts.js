/* Shared presentation helpers; metrics and cohorts remain server-owned. */
window.charts = {
  colors: ["#2a78d6", "#eb6834", "#1baf7a"],
  // 12-point trend sparkline: single series, de-emphasis hue, latest point in the accent.
  // Unknown points (-1) render as gaps, never as zero; a lone point renders as a dot.
  sparkline(values, options = {}) {
    const W = options.width || 96, H = options.height || 28;
    const pad = 3, dotR = 2;
    const points = (values || []).map(v => (v == null || v < 0 ? null : v));
    const known = points.filter(v => v != null);
    if (known.length === 0) return null;
    const lo = Math.min(...known), hi = Math.max(...known);
    const span = hi - lo || 1;
    const x = i => pad + (points.length === 1 ? W / 2 - pad : (W - 2 * pad) * (i / (points.length - 1)));
    const y = v => H - pad - ((v - lo) / span) * (H - 2 * pad);
    const seg = [];
    let open = false;
    const circles = [];
    points.forEach((v, i) => {
      if (v == null) { open = false; return; }
      const cx = x(i), cy = y(v);
      seg.push(`${open ? "L" : "M"}${cx.toFixed(1)},${cy.toFixed(1)}`);
      open = true;
      circles.push({ cx, cy, last: i === points.length - 1 });
    });
    const last = circles[circles.length - 1];
    const node = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    node.setAttribute("viewBox", `0 0 ${W} ${H}`);
    node.setAttribute("width", W); node.setAttribute("height", H);
    node.setAttribute("aria-hidden", "true"); node.classList.add("sparkline");
    if (seg.length > 1) {
      const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
      path.setAttribute("d", seg.join(" "));
      path.setAttribute("fill", "none");
      path.setAttribute("stroke", options.line || "#596b7d");
      path.setAttribute("stroke-width", "2");
      path.setAttribute("stroke-linecap", "round");
      path.setAttribute("stroke-linejoin", "round");
      node.append(path);
    }
    circles.forEach(c => {
      const r = c.last ? dotR + 1 : dotR;
      const dot = document.createElementNS("http://www.w3.org/2000/svg", "circle");
      dot.setAttribute("cx", c.cx); dot.setAttribute("cy", c.cy); dot.setAttribute("r", r);
      dot.setAttribute("fill", c.last ? (options.accent || "#087f80") : (options.line || "#596b7d"));
      if (c.last) dot.setAttribute("stroke", "#fff"); dot.setAttribute("stroke-width", "1.5");
      node.append(dot);
    });
    return node;
  },
  disclosure(label, ...children) { return el("details", { class: "evidence-disclosure" }, [el("summary", { text: label }), ...children]); },
  table(headers, rows) {
    return el("div", { class: "table-scroll", tabindex: "0" }, [el("table", { class: "engagement-table" }, [
      el("thead", {}, [el("tr", {}, headers.map(h => el("th", { scope: "col", text: h })))]),
      el("tbody", {}, rows.map(row => el("tr", {}, row.map(v => el("td", {}, [v == null ? "—" : v instanceof Node ? v : String(v)]))))),
    ])]);
  },
  stat(label, value, note) { return el("article", { class: "stat-tile", "data-stat": label }, [el("p", { text: label }), el("strong", { text: value }), el("small", { text: note })]); },
  card(title, note, ...children) { return el("section", { class: "viz-card" }, [el("h3", { text: title }), ...(note ? [el("p", { class: "hint", text: note })] : []), ...children]); },
  // Transparent button area is larger than the mark; focus/tap and hover share details.
  bars(title, rows, options = {}) {
    const max = options.max || Math.max(1, ...rows.map(r => r.value ?? 0));
    const plot = el("div", { class: "bar-plot", "aria-label": title });
    const readout = el("p", { class: "chart-readout", role: "status", text: "Focus or tap a bar for details." });
    rows.forEach(row => {
      const known = row.value != null;
      const button = el("button", { type: "button", class: "bar-row", "aria-label": `${row.label}: ${known ? row.value : "Unknown"}. ${row.detail || ""}`, "data-value": known ? row.value : "unknown" }, [
        el("span", { class: "bar-label", text: row.label }),
        el("span", { class: "bar-track", "aria-hidden": "true" }, [el("span", { class: "bar-fill", style: `width:${known ? Math.max(0, Math.min(100, row.value / max * 100)) : 0}%` })]),
        el("strong", { text: known ? row.value : "—" }),
      ]);
      const show = () => { readout.textContent = `${row.label}: ${known ? row.value : "Unknown"}${options.unit ? ` ${options.unit}` : ""}. ${row.detail || ""}`; };
      button.addEventListener("pointerenter", show); button.addEventListener("focus", show);
      button.addEventListener("click", () => { show(); options.onSelect?.(row); });
      plot.append(button);
      if (row.detail) plot.append(el("p", { class: "bar-context", text: row.detail }));
    });
    if (!rows.length) plot.append(el("p", { class: "empty-copy", text: "No observations in this selection." }));
    return this.card(title, options.note, plot, readout,
      this.disclosure("Chart data", this.table(["Category", options.unit || "Count", "Context"], rows.map(r => [r.label, r.value ?? "Unknown", r.detail || ""]))));
  },
  ratio(title, ratio, note) {
    const known = ratio.denominator > 0;
    const pct = known ? Math.round(100 * ratio.numerator / ratio.denominator) : null;
    const card = this.bars(title, [{ label: known ? `${pct}%` : "No data yet", value: known ? ratio.numerator : null, detail: `${ratio.numerator} of ${ratio.denominator} · ${note}` }], { max: Math.max(1, ratio.denominator) });
    card.classList.add("ratio-card");
    return card;
  },
  distribution(title, counts, onSelect) {
    const keys = ["increased", "unchanged", "decreased"], labels = ["Increased", "Unchanged", "Decreased"];
    const track = el("div", { class: "stack-track", "aria-label": `${title} distribution` });
    const legend = el("div", { class: "chart-legend" });
    const readout = el("p", { class: "chart-readout", role: "status", text: counts.eligible ? "Select a category to see the students." : "Not enough comparable evidence yet." });
    keys.forEach((key, i) => {
      const value = counts[key], pct = counts.eligible ? Math.round(100 * value / counts.eligible) : 0;
      const detail = `${labels[i]}: ${value} of ${counts.eligible} students (${pct}%).`;
      const show = () => { readout.textContent = detail; };
      if (value > 0) {
        const mark = el("button", { type: "button", class: `stack-fill series-${i}`, style: `flex:${value}`, "aria-label": detail });
        mark.addEventListener("pointerenter", show);
        mark.addEventListener("focus", show);
        mark.addEventListener("click", () => { show(); onSelect?.(key); });
        track.append(mark);
      }
      const button = el("button", { type: "button", class: "legend-item", "aria-label": detail }, [el("span", { class: `swatch series-${i}`, "aria-hidden": "true" }), `${labels[i]} `, el("strong", { text: value })]);
      button.addEventListener("pointerenter", show); button.addEventListener("focus", show); button.addEventListener("click", () => { show(); onSelect?.(key); });
      legend.append(button);
    });
    track.addEventListener("pointerenter", () => { readout.textContent = keys.map((k,i) => `${labels[i]} ${counts[k]}`).join(" · "); });
    return this.card(title, `${counts.eligible} comparable students · ${counts.excluded} without enough evidence`, track, legend, readout,
      this.disclosure("Why some students are excluded", this.table(["Reason", "Students"], Object.entries(counts.reasons || {}))));
  },
  shiftDate(value, days) { const d = new Date(`${value}T12:00:00Z`); d.setUTCDate(d.getUTCDate() + days); return d.toISOString().slice(0,10); },
  periodLabel(start) { return new Intl.DateTimeFormat("en-US", { month: "short", year: "numeric", timeZone: "UTC" }).format(new Date(`${start}T12:00:00Z`)); },
  staff(id) { return [...document.getElementById("staff").options].find(o => o.value === id)?.text.split(" · ")[0] || id; },
  lockReport(target, locked) {
    [...target.children].forEach(node => {
      const keep = node.classList.contains("report-status") || node.classList.contains("outcome-tabs") || node.classList.contains("outcome-filters");
      node.inert = locked && !keep;
    });
  },
  async report(target, url, render) {
    const owner = window.engagement;
    const seq = owner.outcomeRequest = (owner.outcomeRequest || 0) + 1;
    target.setAttribute("aria-busy", "true");
    this.lockReport(target, true);
    let status = target.querySelector(":scope > .report-status");
    if (!status) { status = el("p", { class: "report-status", role: "status" }); target.prepend(status); }
    status.textContent = "Updating this view…";
    try {
      const response = await fetch(url), data = await response.json();
      if (seq !== owner.outcomeRequest || owner.view !== "outcomes") return;
      if (!response.ok) throw new Error(data.error || "Could not load this view");
      render(data); target.removeAttribute("data-stale"); this.lockReport(target, false);
    } catch (e) {
      if (seq !== owner.outcomeRequest || owner.view !== "outcomes") return;
      status.textContent = `${e.message}. Previous results have not been updated.`;
      target.setAttribute("data-stale", "true");
      this.lockReport(target, true);
      status.append(owner.button("Try again", () => owner.loadOutcomes()));
    } finally { if (seq === owner.outcomeRequest) target.removeAttribute("aria-busy"); }
  },
};
