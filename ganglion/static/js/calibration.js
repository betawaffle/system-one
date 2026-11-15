import { showCall } from "./model.js";
// Calibration panel (design/synthetic-data/10/M2): how often the model's first
// pass agrees with your verdicts, per prompt version and reasoning effort.

const icons = { ok: "✓", but: "◐", bad: "✕" };
const names = { ok: "fine", but: "fine, but", bad: "wrong" };

export function mount(el, h) {
  let timer = null;
  const pct = (a, n) => n ? Math.round(100 * a / n) + "%" : "–";
  async function load() {
    let st, cal;
    try {
      [st, cal] = await Promise.all([fetch("/api/model/status").then(r => r.json()), fetch("/api/model/calibration").then(r => r.json())]);
    } catch (e) { el.replaceChildren(h("div", { class: "card-body muted small" }, "Model status unavailable.")); return; }
    const job = cal.job;
    const change = (d, patch) => fetch("/api/write/verdict", { method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ meaning: d.meaning, line: d.line, sample: d.sample, verdict: "", but_kind: "", reason: "", note: "", star: false, via: "", model_call: "", ...patch }) }).then(load);
    const run = effort => fetch("/api/model/calibrate", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify({ effort }) }).then(load);
    el.replaceChildren(
      h("div", { class: "card-head" },
        h("div", { class: "stack tight" },
          h("h3", {}, "Model: First Pass"),
          h("span", { class: "muted small" }, st.up ? `${st.model}, ${st.state}` : "not running", " · prompt ", h("span", { class: "mono" }, cal.prompt), " · examples from your verdicts ", cal.few_shot ? "on" : "off")),
        h("div", { class: "row" },
          h("button", { class: "btn", disabled: job.running || !st.up, onclick: () => run("low"), "data-tip": "Run the model over every sample you've judged, low reasoning effort" }, "Check Against Mine"),
          h("button", { class: "btn ghost", disabled: job.running || !st.up, onclick: () => run("medium"), "data-tip": "The same with medium reasoning effort, to compare" }, "Medium Effort"))),
      h("div", { class: "card-body stack tight" },
        h("div", { class: "muted small" }, `You've judged ${cal.judged} samples. The model only suggests; your verdicts are the ones that count.`),
        h("details", { class: "tree", ontoggle: async e => { if (e.target.open && !e.target.dataset.loaded) { const p = await fetch("/api/model/prompt").then(r => r.json()); e.target.dataset.loaded = 1; e.target.querySelector("pre").textContent = p.text; e.target.querySelector(".pfile").textContent = p.file; } } },
          h("summary", {}, "The Prompt (version ", cal.prompt, ")"),
          h("div", { class: "muted small gap-top" }, "The template the first pass fills in for each sample: the part before {{end}} is the instructions, the rest is the sample. File: ", h("span", { class: "mono pfile" }, "…"), ". Each edit is a new version, and calibration compares versions. Use Why? on any suggestion to see one filled in, with the model's reasoning."),
          h("pre", { class: "out wrap gap-top" }, "Loading…")),
        job.running ? h("div", { class: "row small" }, h("span", { class: "covbar" }, h("i", { "data-w": job.total ? job.done / job.total : 0 })), `Checking ${job.done} of ${job.total} (${job.effort} effort)…`) : null,
        (cal.stats || []).length ? h("div", { class: "table-wrap" }, h("table", { class: "table" },
          h("thead", {}, h("tr", {}, ["Prompt", "Effort", "Agrees", "Your ✓", "Your ◐", "Your ✕"].map(x => h("th", {}, x)))),
          h("tbody", {}, cal.stats.map(s => h("tr", {},
            h("td", { class: "mono small" }, s.prompt, s.current ? " (now)" : ""),
            h("td", {}, s.effort),
            h("td", {}, h("b", {}, pct(s.agree, s.n)), h("span", { class: "muted small" }, ` ${s.agree} of ${s.n}`)),
            ["ok", "but", "bad"].map(k => { const b = s.by_yours[k] || [0, 0]; return h("td", { "data-tip": `Of samples you marked ${names[k]}, the model agreed on ${b[0]} of ${b[1]}` }, b[1] ? `${b[0]}/${b[1]}` : "–"); })))))) :
          h("div", { class: "muted small" }, "No comparison yet. Check Against Mine runs the model over what you've judged."),
        (cal.stats || []).filter(s => s.current && (s.disagree || []).length).map(s => h("details", { class: "tree" },
          h("summary", {}, `Where it disagrees with you (${s.disagree.length}, ${s.effort} effort)`),
          h("div", { class: "stack tight gap-top" }, s.disagree.map(d => h("div", { class: "disagree" },
            h("div", { class: "row" }, h("span", { class: "spacer" }),
              h("button", { class: "btn chip", "data-tip": "Make the model's verdict yours", onclick: () => change(d, { verdict: d.model.verdict, but_kind: d.model.but_kind || "", reason: d.model.reason || "", note: d.model.note || "", via: "model", model_call: d.model.call }) }, "Use Model's"),
              h("button", { class: "btn chip", "data-tip": "Mark it fine", onclick: () => change(d, { verdict: "ok" }) }, "✓ Fine"),
              h("button", { class: "btn chip", "data-tip": "Clear your verdict; Coverage will offer this kind again", onclick: () => change(d, { verdict: "" }) }, "Clear Mine"),
              h("button", { class: "btn chip ghostchip", "data-tip": "See exactly what the model was asked, and its reasoning", onclick: () => showCall(h, d.model.call) }, "Why?")),
            h("div", {}, d.text),
            h("div", { class: "small" }, h("span", { class: "muted" }, "You: "), d.yours.replace(/^ok$/, "✓ fine").replace(/^but/, "◐").replace(/^bad/, "✕")),
            h("div", { class: "small" }, h("span", { class: "muted" }, "Model: "), icons[d.model.verdict], " ",
              d.model.verdict === "bad" ? d.model.reason : d.model.verdict === "but" ? d.model.but_kind : "fine", d.model.note ? ": " + d.model.note : "", ` (${d.model.confidence})`))))))));
    el.querySelectorAll("[data-w]").forEach(x => { x.style.width = (Math.max(0, Math.min(1, +x.dataset.w)) * 100).toFixed(1) + "%"; });
    clearTimeout(timer);
    if (job.running) timer = setTimeout(load, 2000);
  }
  load();
}
