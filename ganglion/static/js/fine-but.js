// "Fine, but…" (design/synthetic-data/08/D107): a sample that's right but needs
// context, or implies more than its tags. Notes written before can be picked
// again, so repeats become visible and can turn into rules (02/L9).

export const kinds = [
  ["context", "Needs Context", "Fine, but only after something else: a scene where you can point, or an earlier sentence.", "What could come before it?"],
  ["implies", "Implies Something", "Fine, but a reader infers more than the tags say.", "What does it imply?"],
];

let notes = { context: [], implies: [] };
export async function loadNotes() {
  try {
    const r = await fetch("/api/write/notes");
    if (r.ok) notes = await r.json();
  } catch (e) {}
  return notes;
}

// block renders the choices under a sample marked ◐. send(patch) saves.
export function block(h, v, send) {
  const kind = kinds.find(k => k[0] === v.but_kind);
  const past = kind ? (notes[kind[0]] || []).slice(0, 12) : [];
  return h("div", { class: "row reasons" },
    h("span", { class: "label" }, "Fine, but…"),
    kinds.map(([k, label, why]) => h("button", { class: "btn chip" + (v.but_kind === k ? " picked" : ""), "data-tip": why, onclick: () => send({ v: "but", but_kind: k, note: v.but_kind === k ? v.note : "" }) }, label)),
    kind ? h("div", { class: "reason-why small" },
      h("div", {}, kind[2]),
      past.length ? h("div", { class: "row pastnotes" },
        h("span", { class: "muted" }, "Said before:"),
        past.map(n => h("button", { class: "btn chip" + (v.note === n.note ? " picked" : ""), "data-tip": `Used ${n.count} time${n.count === 1 ? "" : "s"}`, onclick: () => send({ v: "but", but_kind: kind[0], note: n.note }) }, n.note))) : null,
      h("div", { class: "row" },
        h("input", { class: "input grow", placeholder: kind[3] + " Enter saves", value: v.note || "", "aria-label": kind[3], onchange: e => send({ v: "but", but_kind: kind[0], note: e.target.value }) }),
        v.note ? h("span", { class: "tag ok" }, "✓ Saved") : null)) : null);
}
