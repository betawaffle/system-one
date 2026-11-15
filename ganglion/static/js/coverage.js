import * as calibration from "./calibration.js";
import * as model from "./model.js";
import * as fineBut from "./fine-but.js";

// The coverage view (design/synthetic-data/08/W9): which kinds of sample each
// saved template can make, which you've judged, and samples aimed at the rest.

const $ = (s, el = document) => el.querySelector(s);
function h(tag, attrs = {}, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(attrs)) {
    if (v === false || v == null) continue;
    if (k === "class") el.className = v;
    else if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else el.setAttribute(k, v === true ? "" : v);
  }
  for (const k of kids.flat()) if (k != null && k !== false) el.append(k instanceof Node ? k : String(k));
  return el;
}
const cap = s => s ? s.charAt(0).toUpperCase() + s.slice(1) : s;
async function api(path, body) {
  const r = await fetch(path, body === undefined ? {} : { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

const reasons = [
  ["Word Form", "A word is spelled or inflected wrong: \"gived\", \"a umbrella\".", "the word's lexicon entry"],
  ["Wrong Kind of Filler", "Grammatical, but that thing can't fill that role: \"The idea is tall.\"", "the role's type"],
  ["Ungrammatical", "Broken English: word order, agreement, a missing or extra word.", "the template: consider rewording it on Write"],
  ["Means Something Else", "Reads fine, but doesn't match its tags, or a word adds meaning the tags don't have.", "the template's meaning links"],
  ["Sounds Odd", "Correct, and means what the tags say, but nobody would say it like that without more context.", "the template: that tense now gets a time modifier (a new template version)"],
  ["Other", "Something else. Say what.", "a note"],
];

let items = [];
const shown = new Map();    // template id -> samples
const verdicts = new Map(); // id + sample text -> verdict this page view
const seeds = new Map();
const review = new Map();   // template id -> {samples, filter: [dim, value] | null}

// the samples you've judged, newest first, to look at again (08/W9e)
async function openReview(it, filter = null) {
  try {
    const j = await api("/api/coverage/judged?id=" + encodeURIComponent(it.id));
    review.set(it.id, { samples: j.samples || [], filter });
  } catch (e) { return alert(e.message); }
  render(); sizeBars();
}
function reviewBlock(it) {
  const r = review.get(it.id);
  const list = r.filter ? r.samples.filter(s => (s.combo || {})[r.filter[0]] === r.filter[1]) : r.samples;
  const label = r.filter ? `${r.filter[0] === "verb" ? "" : r.filter[0].split(":")[1] + ": "}${r.filter[1]}` : "all";
  return h("div", { class: "stack tight gap-top review" },
    h("div", { class: "row" },
      h("h4", {}, "Judged: ", label), h("span", { class: "muted small" }, list.length, " sample", list.length === 1 ? "" : "s", ", newest first. Change any verdict here."),
      h("span", { class: "spacer" }),
      r.filter ? h("button", { class: "btn chip", onclick: () => { r.filter = null; render(); sizeBars(); } }, "Show All") : null,
      h("button", { class: "btn chip", onclick: () => { review.delete(it.id); render(); sizeBars(); } }, "Close")),
    list.length ? list.map(s => sampleRow(it, s)) :
      h("div", { class: "muted small" }, r.filter ? "Only your own sentence covers this one: nothing judged by sample yet." : "No samples judged yet."));
}

async function load() {
  try { items = (await api("/api/coverage")).templates; } catch (e) { $("#templates").replaceChildren(h("div", { class: "card card-pad notice-fail" }, e.message)); return; }
  render();
}
function render() {
  if (!items.length) {
    $("#templates").replaceChildren(h("div", { class: "card" }, h("div", { class: "empty" }, h("h3", {}, "No Templates Yet"), h("p", {}, "Save a sentence on Write and it shows up here."))));
    return;
  }
  $("#templates").replaceChildren(...items.map(card));
}
function card(it) {
  const pct = it.total ? it.done / it.total : 0;
  return h("section", { class: "card", "data-id": it.id },
    h("div", { class: "card-head" },
      h("div", { class: "stack tight" },
        h("div", { class: "row" }, h("span", { class: "tag" }, it.frame), (it.frozen || []).length ? h("span", { class: "tag" }, "Frozen") : null, h("b", {}, it.text)),
        h("div", { class: "row small muted" },
          h("span", { class: "covbar" }, h("i", { "data-w": pct })),
          `${it.done} of ${it.total} kinds judged`,
          (it.needs || []).map(n => h("span", { class: "tag warn" }, `${cap(n.tense)}: adds a time`)))),
      h("div", { class: "row" },
        h("button", { class: "btn", onclick: () => review.has(it.id) ? (review.delete(it.id), render(), sizeBars()) : openReview(it), "aria-expanded": String(review.has(it.id)) }, "Judged"),
        h("button", { class: "btn primary", onclick: () => show(it) }, it.done < it.total ? "Show Me" : "Show More"))),
    h("div", { class: "card-body stack tight" },
      it.dims.map(d => h("div", { class: "dimrow" },
        h("span", { class: "label" }, d.title),
        h("div", { class: "row chips" }, d.values.map((v, i) => d.judged[i]
          ? h("button", { class: "tag ok chipbtn", "data-tip": "Judged: click to see the samples", onclick: () => openReview(it, [d.name, v]) }, "✓ ", v)
          : h("span", { class: "tag pending", "data-tip": "Not judged yet" }, "○ ", v))))),
      review.has(it.id) ? reviewBlock(it) : null,
      shown.has(it.id) ? h("div", { class: "stack tight gap-top" }, h("h4", {}, "Aimed at what's left"), shown.get(it.id).map(s => sampleRow(it, s))) : null));
}
async function show(it) {
  const seed = (seeds.get(it.id) || 0) + 1;
  seeds.set(it.id, seed);
  try {
    const j = await api("/api/coverage/show", { id: it.id, seed });
    shown.set(it.id, j.samples || []);
    const todo = (j.samples || []).filter(s => !s.prior);
    if (todo.length) model.judge(it.meaning, it.text, todo, () => { render(); sizeBars(); });
  } catch (e) { return alert(e.message); }
  render(); sizeBars();
}
function sampleRow(it, s) {
  const key = it.id + "\u0000" + s.text;
  const p = s.prior;
  const v = verdicts.get(key) || (p ? { v: p.verdict || null, but_kind: p.but_kind || null, reason: p.reason || null, note: p.note || "", star: !!p.star, prior: p.at } : {});
  const send = async patch => {
    const nv = { ...v, ...patch, prior: null };
    delete nv.via; delete nv.call;
    verdicts.set(key, nv);
    try {
      await api("/api/write/verdict", { meaning: it.meaning, line: it.text, sample: s, verdict: nv.v || "", but_kind: nv.v === "but" ? nv.but_kind || "" : "", reason: nv.reason || "", note: nv.note || "", star: !!nv.star, via: patch.via || "", model_call: patch.call || "" });
      if (nv.v === "but" && nv.note) await fineBut.loadNotes();
      if (patch.reason === "Sounds Odd") await api("/api/template/need", { id: it.id, tense: s.features.tense });
    } catch (e) { alert(e.message); }
    const fresh = (await api("/api/coverage")).templates;
    items = fresh;
    if (review.has(it.id)) {
      const j = await api("/api/coverage/judged?id=" + encodeURIComponent(it.id));
      review.get(it.id).samples = j.samples || [];
    }
    render(); sizeBars();
  };
  const judged = new Map(it.dims.map(d => [d.name, new Map(d.values.map((x, i) => [x, d.judged[i]]))]));
  const tags = Object.entries(s.combo || {}).map(([k, val]) => {
    const fresh = judged.get(k) && judged.get(k).get(val) === false;
    const name = k === "verb" ? val : `${k.split(":")[1]}: ${val}`;
    return h("span", { class: "tag mini" + (fresh ? " accent" : ""), "data-tip": fresh ? "Not judged yet: this is why it's shown" : "Already judged" }, name);
  });
  return h("div", { class: "sample-row" + (v.v ? " judged" : "") + (v.prior ? " prior" : "") },
    h("div", { class: "sample-main" },
      h("div", { class: "sample-text" }, s.text, v.prior ? h("span", { class: "tag mini", "data-tip": `Judged ${new Date(v.prior).toLocaleString()}` }, "Judged ", new Date(v.prior).toLocaleDateString()) : null),
      h("div", { class: "row sample-tags" }, tags, s.time ? h("span", { class: "tag mini" }, "time: ", s.time) : null,
        (s.implies || []).map(x => h("span", { class: "tag mini implies" }, "implies: ", x))),
      v.v ? null : model.line(h, s.text, p => send(p))),
    h("div", { class: "row" },
      h("div", { class: "seg rate", role: "group", "aria-label": "Verdict" },
        h("button", { "aria-pressed": String(v.v === "ok"), "data-rate": "good", onclick: () => send({ v: v.v === "ok" ? null : "ok", reason: null }), "data-tip": "Right" }, "✓"),
        h("button", { "aria-pressed": String(v.v === "but"), "data-rate": "but", onclick: () => send({ v: v.v === "but" ? null : "but", reason: null }), "data-tip": "Fine, but it needs context or implies more" }, "◐"),
        h("button", { "aria-pressed": String(v.v === "bad"), "data-rate": "bad", onclick: () => { verdicts.set(key, { ...v, v: v.v === "bad" ? null : "bad", prior: null }); render(); sizeBars(); }, "data-tip": "Wrong: then say why" }, "✕"),
        h("button", { "aria-pressed": String(!!v.star), onclick: () => send({ star: !v.star }), "data-tip": "Keep as a good example or boundary case" }, "★"))),
    v.v === "but" ? fineBut.block(h, v, send) : null,
    v.v === "bad" ? h("div", { class: "row reasons" },
      h("span", { class: "label" }, "What's wrong?"),
      reasons.map(([x, why]) => h("button", { class: "btn chip" + (v.reason === x ? " picked" : ""), "data-tip": why, onclick: () => send({ v: "bad", reason: x }) }, x)),
      v.reason ? (([x, why, goes]) => h("div", { class: "reason-why small" },
        h("div", {}, why),
        x === "Other" ? h("div", { class: "row" },
          h("input", { class: "input grow", placeholder: "What's wrong? Enter saves", value: v.note || "", "aria-label": "Note", onchange: e => send({ v: "bad", reason: "Other", note: e.target.value }) }),
          v.note ? h("span", { class: "tag ok" }, "✓ Note Saved") : null) : null,
        h("div", { class: "muted" }, "Goes to ", goes, ".")))(reasons.find(r => r[0] === v.reason)) : null) : null);
}
function sizeBars() {
  document.querySelectorAll("[data-w]").forEach(el => { el.style.width = (Math.max(0, Math.min(1, +el.dataset.w)) * 100).toFixed(1) + "%"; });
}
if (!$("#cov-error")) { fineBut.loadNotes(); model.loadStatus(); load().then(sizeBars); calibration.mount($("#model-cal"), h); }
