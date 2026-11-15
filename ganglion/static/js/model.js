// The model's first pass over samples (design/synthetic-data/10/M1, D110):
// shown under each sample as a suggestion; Accept copies it to your verdict.

const cache = new Map(); // sample text -> {result} | {error} | "pending"
let status = null;

export async function loadStatus() {
  try { status = await (await fetch("/api/model/status")).json(); } catch (e) { status = { up: false, state: "down" }; }
  return status;
}

// judge asks for any samples not yet known; onDone re-renders.
export async function judge(meaning, line, samples, onDone) {
  const todo = samples.filter(s => !cache.has(s.text));
  if (!todo.length) return;
  if (status && !status.up) { todo.forEach(s => cache.set(s.text, { error: "not running" })); onDone(); return; }
  todo.forEach(s => cache.set(s.text, "pending"));
  onDone();
  try {
    const r = await fetch("/api/model/judge", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ meaning, line, samples: todo }) });
    if (!r.ok) throw new Error(await r.text());
    const j = await r.json();
    for (const x of j.results) cache.set(x.text, x.result ? { result: x.result } : { error: x.error || "no answer" });
  } catch (e) {
    todo.forEach(s => cache.set(s.text, { error: String(e.message || e).slice(0, 120) }));
  }
  onDone();
}

const icons = { ok: "✓", but: "◐", bad: "✕" };
// line renders "Model: …" under a sample; accept(patch) makes it your verdict.
export function line(h, text, accept) {
  const c = cache.get(text);
  if (!c) return null;
  if (c === "pending") return h("div", { class: "model-line muted small" }, "Model: ", status && status.state !== "loaded" ? "warming up…" : "thinking…");
  if (c.error) return h("div", { class: "model-line muted small", "data-tip": c.error }, "Model: ", c.error === "not running" ? "not running" : "no answer");
  const v = c.result;
  const what = v.verdict === "ok" ? "fine" : v.verdict === "but" ? (v.but_kind === "context" ? "needs context" : "implies") : v.reason;
  const flagged = v.verdict !== "ok" || v.confidence === "low";
  return h("div", { class: "model-line small" + (flagged ? " flagged" : "") },
    h("span", { class: "model-who" }, "Model"),
    h("span", {}, icons[v.verdict], " ", what, v.note ? ": " + v.note : ""),
    h("span", { class: "muted" }, "(", v.confidence, ")"),
    h("button", { class: "btn chip", "data-tip": "Make this your verdict (A)", onclick: () => accept(patchOf(v)) }, "Accept"));
}

export function patchOf(v) {
  return { v: v.verdict, but_kind: v.verdict === "but" ? v.but_kind : null, reason: v.verdict === "bad" ? v.reason : null, note: v.note || "", via: "model", call: v.call };
}
