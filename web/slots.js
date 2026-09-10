/* A selected slot is a suggestion until the server atomically books it. */
window.createSlotPicker = function (options) {
  const owner = window.engagement;
  const root = el("div", { class: "slot-picker" });
  const staff = el("select", {}, [...document.getElementById("staff").options].map(o => el("option", { value: o.value, text: o.text })));
  staff.value = options.staff || staffId();
  staff.disabled = !!options.lockStaff;
  const date = el("input", { type: "date", required: true, value: options.date || owner.day() });
  const duration = el("input", { type: "number", min: 5, max: 60, required: true, value: options.duration || 10 });
  const slots = el("div", { class: "slot-grid", role: "group", "aria-label": "Available times" });
  const message = el("p", { role: "status", "aria-live": "polite" });
  const selection = el("p", { class: "slot-selection", text: "Select an available time." });
  const confirmation = el("input", { type: "checkbox", disabled: true });
  let chosen = null, sequence = 0, disposed = false, controller;
  function notify() { options.onChange?.(chosen && confirmation.checked ? value() : null); }
  function value() {
    if (!chosen || !confirmation.checked) return null;
    const clock = new Intl.DateTimeFormat("en-GB", { timeZone: owner.timezone, hour: "2-digit", minute: "2-digit", hourCycle: "h23" }).format(new Date(chosen.start));
    return { staff_id: staff.value, start: `${owner.day(chosen.start)}T${clock}`, duration: Number(duration.value), confirmed: true };
  }
  function clear() {
    chosen = null; confirmation.checked = false; confirmation.disabled = true;
    selection.textContent = "Select an available time.";
    slots.replaceChildren(); notify();
  }
  async function reload() {
    const seq = ++sequence;
    controller?.abort(); controller = new AbortController(); clear();
    if (!date.value || !duration.checkValidity()) { message.textContent = "Choose a date and a duration of 5–60 minutes."; return; }
    message.textContent = "Finding available times…";
    const query = new URLSearchParams({ student_id: options.student, staff_id: staff.value, date: date.value, duration: duration.value, exclude: options.exclude || "" });
    try {
      const response = await fetch(`/api/availability?${query}`, { signal: controller.signal });
      const data = await response.json();
      if (disposed || seq !== sequence) return;
      if (!response.ok) throw new Error(data.error || "Could not load available times");
      message.textContent = data.slots.length ? `${data.slots.length} suggested times · ${data.timezone}` : "No available slots on this date. Try another day or librarian.";
      for (const slot of data.slots) {
        const button = el("button", { type: "button", class: "secondary", "aria-pressed": "false", text: `${owner.local(slot.start)} – ${new Intl.DateTimeFormat("en-US", {timeZone: data.timezone, hour: "numeric", minute: "2-digit"}).format(new Date(slot.end))}` });
        button.addEventListener("click", () => {
          if (disposed || seq !== sequence) return;
          chosen = slot; confirmation.checked = false; confirmation.disabled = false;
          slots.querySelectorAll("button").forEach(b => b.setAttribute("aria-pressed", String(b === button)));
          selection.textContent = `Selected: ${button.textContent}`; notify();
        });
        slots.append(button);
      }
    } catch (error) {
      if (disposed || seq !== sequence || error.name === "AbortError") return;
      message.textContent = error.message;
    }
  }
  function moveDay(delta) {
    const value = date.value || owner.day();
    const next = new Date(`${value}T12:00:00Z`); next.setUTCDate(next.getUTCDate() + delta);
    date.value = next.toISOString().slice(0, 10); reload();
  }
  for (const input of [staff, date, duration]) input.addEventListener("change", reload);
  // Invalidate immediately while editing; change performs the next lookup.
  for (const input of [date, duration]) input.addEventListener("input", () => { ++sequence; controller?.abort(); clear(); });
  confirmation.addEventListener("change", notify);
  root.append(el("p", { class: "hint", text: `${owner.name(options.student)} · ${owner.timezone}. Slots exclude declared conflicts. Confirm student availability and any duties not in the structured calendar.` }), el("div", {class: "agenda-controls"}, [el("label", {}, ["Assigned librarian", staff]), el("label", {}, ["Date", date]), el("label", {}, ["Minutes", duration])]), el("div", {class: "engagement-actions"}, [owner.button("Previous day", () => moveDay(-1)), owner.button("Next day", () => moveDay(1)), owner.button("Refresh available times", reload)]), message, slots, selection, el("label", {class: "check"}, [confirmation, "I confirmed staff and student availability"]));
  return { element: root, value, reload, dispose() { disposed = true; ++sequence; controller?.abort(); } };
};
