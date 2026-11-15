// The writing screen (design/synthetic-data/08). Alignment, templates and
// samples come from the server (plain Go, no model). Two model features aren't
// connected yet and say so: span suggestions are keyword guesses, and the word
// helper shows the lexicon only.

const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
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

// ---------- state ----------
let n = 0;            // how many times Next was pressed
let card = null;      // the meaning card from the server
let seed = 42;
let lines = [];       // preview results per line
const memory = new Map();  // span key -> {choice, detail, suggested, confirmed}
const pending = new Set();
const needs = new Map();   // line text -> [{tense, modifier}]
const verdicts = new Map();
const pasted = new Set();  // line texts that came from a paste
let helperFor = null;
let curSample = -1;
let changing = false;

const choices = ["Modifier", "New Modifier", "Event", "Attitude", "Drop"];
const modifierKinds = ["owner", "occasion", "purpose", "time", "means", "place"];
// Not a model: keyword guesses until the model call path exists (07/D90).
const spanHints = [
  [/birthday|christmas|anniversary/, 0, "occasion"], [/gift|present\b/, 0, "purpose"],
  [/finally|at last|eventually/, 3, "expected and late"], [/mail|post|sent|send|ship/, 2, "send"],
  [/yesterday|today|monday|morning|night|ago/, 0, "time"], [/^!$/, 3, "emphatic"],
  [/^(her|his|their|its|my|your|our|\w+'s)$/, 0, "owner"], // a possessive says who owns it
];

// ---------- meaning card (W2) ----------
function renderMeaning() {
  const el = $("#meaning");
  if (!card) { el.replaceChildren(h("div", { class: "card-body muted" }, "Loading…")); return; }
  const m = card.meaning;
  el.replaceChildren(
    h("div", { class: "card-head" },
      h("div", { class: "stack tight" },
        h("div", { class: "row" }, h("h3", {}, card.title), m.joke ? h("span", { class: "tag warn", "data-tip": m.joke }, "Joke On Purpose") : null),
        h("span", { class: "muted small" }, card.explainer)),
      h("div", { class: "row" },
        h("button", { class: "btn primary", onclick: next, "data-tip": "Alt+N" }, "Next"),
        h("button", { class: "btn", onclick: newFillers, "data-tip": "Alt+F" }, "New Fillers"),
        h("button", { class: "btn ghost", onclick: () => { changing = !changing; renderMeaning(); }, "aria-expanded": String(changing) }, "Change"))),
    h("div", { class: "card-body stack tight" },
      h("table", { class: "table roles" }, h("tbody", {},
        card.roles.map(r => h("tr", {},
          h("td", { class: "nowrap" }, h("span", { class: `series role-bg-${r.Series}` }), h("b", {}, r.Name),
            h("span", { class: "info", tabindex: "0", "aria-label": "About this role", "data-tip": `${r.Explainer} Generic name: ${r.Generic}.` }, "i")),
          h("td", {}, h("span", { class: "tag" }, r.Fill)))))),
      h("div", { class: "row" },
        ["tense", "polarity", "mood"].map(k => h("span", { class: "tag accent" }, cap(m.features[k]))),
        Object.entries(m.modifiers || {}).map(([k, v]) => h("span", { class: "tag" }, `${cap(k)}: ${v}`))),
      m.why ? h("div", { class: "muted small" }, "Why this one: ", m.why) : null,
      changing ? changePanel() : null));
}
function changePanel() {
  const m = card.meaning;
  const seg = (k, vals) => h("div", { class: "seg", role: "group", "aria-label": cap(k) },
    vals.map(v => h("button", { "aria-pressed": String(m.features[k] === v), onclick: () => { m.features[k] = v; renderMeaning(); refresh(); } }, cap(v))));
  return h("div", { class: "row change" },
    h("select", { class: "input", "aria-label": "Frame", onchange: e => load({ frame: e.target.value }) },
      card.frames.map(([id, title]) => h("option", { value: id, selected: id === m.frame }, title))),
    seg("tense", ["past", "present", "future"]), seg("polarity", ["positive", "negative"]), seg("mood", ["statement", "question"]));
}
async function load(q = {}) {
  const p = new URLSearchParams({ n, ...q });
  try {
    card = await api("/api/write/next?" + p);
  } catch (e) { return fail(e); }
  $("#text").value = "";
  helperFor = null;
  renderMeaning(); refresh();
}
function next() { n++; load(); }
async function newFillers() {
  try { card = await api("/api/write/fillers", { meaning: card.meaning, seed: Math.floor(Math.random() * 1e6) }); } catch (e) { return fail(e); }
  renderMeaning(); refresh();
}
function fail(e) {
  $("#lines").replaceChildren(h("div", { class: "notice-fail card-pad" }, String(e.message || e)));
}

// ---------- preview: alignment and samples from the server ----------
let inflight = 0;
async function refresh() {
  if (!card) return;
  const texts = $("#text").value.split("\n").filter(t => t.trim());
  const req = {
    meaning: card.meaning, seed,
    lines: texts.map(t => ({ text: t, res: resOf(t), needs: needs.get(t) || [] })),
  };
  const my = ++inflight;
  let out;
  try { out = await api("/api/write/preview", req); } catch (e) { return fail(e); }
  if (my !== inflight) return; // a newer request is on its way
  lines = out.lines.map((l, i) => ({ ...l, n: i }));
  for (const l of lines) for (const s of l.spans) suggest(s);
  renderLines(); renderReview(); renderSamples(); renderSave();
}
function resOf(text) {
  const out = {};
  // spans are keyed by their text; send confirmed answers for spans in this line
  for (const [k, m] of memory) if (m.confirmed && text.toLowerCase().includes(k)) out[k] = { choice: choices[m.choice], detail: m.detail || "" };
  return out;
}
function suggest(span) {
  if (memory.has(span.key) || pending.has(span.key)) return;
  pending.add(span.key);
  setTimeout(() => {
    pending.delete(span.key);
    let s = span.modal ? [3, "possibility"] : null;
    for (const [re, c, d] of spanHints) if (!s && re.test(span.key)) s = [c, d];
    if (!s) s = [1, ""];
    if (!memory.has(span.key)) memory.set(span.key, { choice: s[0], detail: s[1], suggested: true, confirmed: false });
    renderLines(); renderReview();
  }, 250);
}
const resolved = key => memory.get(key)?.confirmed;
const roleSeries = role => (card.roles.find(r => r.Name === role) || {}).Series || 1;

// ---------- traced view (W3a) ----------
function renderLines() {
  $("#lines").replaceChildren(...lines.map(l => {
    const segs = [];
    let last = 0;
    for (const t of l.tokens) {
      if (t.at > last) segs.push(l.text.slice(last, t.at));
      const cls = ["tok"];
      if (t.cls === "role") cls.push(`role-${roleSeries(t.role)}`);
      else if (t.cls === "frame") cls.push("frame");
      else if (t.span) cls.push(resolved(t.span) ? "resolved" : t.cls === "untraced" ? "untraced" : "inspan");
      const tip = t.cls === "role" ? `${t.role}${t.case ? ` (${t.case} pronoun)` : ""}` : t.cls === "frame" ? `${t.lemma}${t.converse ? ", converse viewpoint" : ""}` : t.span ? (resolved(t.span) ? "Resolved" : "Not in the meaning yet") : t.cls === "grammar" ? "Grammar" : null;
      segs.push(h("span", { class: cls.join(" "), "data-tip": tip, tabindex: t.cls === "punct" ? null : "-1", onclick: () => openHelper(t) }, t.text));
      last = t.at + t.text.length;
    }
    if (last < l.text.length) segs.push(l.text.slice(last));
    const open = l.spans.filter(s => !resolved(s.key)).length;
    return h("div", { class: "traced" },
      h("div", { class: "traced-text" }, segs),
      h("div", { class: "row" },
        l.traced && !savable(l) ? h("span", { class: "tag warn", "data-tip": blockers(l).join(" ") }, "Held Back") : null,
        l.traced ? h("span", { class: "tag ok" }, "✓ Fully Traced") : open ? h("span", { class: "tag warn" }, "? ", `${open} to resolve`) : h("span", { class: "tag warn" }, "? No verb found"),
        l.waits.map(w => h("span", { class: "tag", "data-tip": "Saved frozen: usable now, tense and mood fixed (sd/05/D80)" }, "Frozen · Waits On ", cap(w))),
        l.warnings.map(w => h("span", { class: "tag warn", "data-tip": "What the sentence says differs from the meaning card. Fix the sentence or change the meaning." }, "⚠ ", w))));
  }));
}

// ---------- review (W4, W5) ----------
function renderReview() {
  const cards = [];
  if (helperFor) cards.push(helperCard());
  for (const l of lines) {
    if (!l.spans.length) continue;
    cards.push(h("div", { class: "card" },
      h("div", { class: "card-head" }, h("h3", {}, "Line ", l.n + 1, ": What Do These Add?"), h("span", { class: "muted label" }, "Tab between spans · 1–5 picks")),
      h("div", { class: "card-body stack tight" }, l.spans.map(spanRow))));
  }
  $("#review").replaceChildren(...cards);
}
function spanRow(s) {
  const m = memory.get(s.key);
  const setChoice = c => {
    memory.set(s.key, { choice: c, suggested: false, confirmed: true, detail: m && m.choice === c ? m.detail : "" });
    rerenderKeepFocus(s.key);
  };
  const upd = v => { m.detail = v; clearTimeout(window.__d); window.__d = setTimeout(refresh, 300); };
  const detail = () => {
    if (!m) return h("span", { class: "muted small" }, "Waiting for a suggestion…");
    switch (choices[m.choice]) {
      case "Modifier": return h("select", { class: "input", "aria-label": "Which modifier", onchange: e => upd(e.target.value) }, modifierKinds.map(k => h("option", { selected: m.detail === k }, k)));
      case "New Modifier": return h("span", { class: "row" },
        h("input", { class: "input", placeholder: "Name", value: m.detail || "", "aria-label": "Modifier name", oninput: e => upd(e.target.value) }),
        h("input", { class: "input grow", placeholder: "One-line explainer", "aria-label": "Explainer" }));
      case "Event": return h("select", { class: "input", "aria-label": "Which event", onchange: e => upd(e.target.value) }, ["send", "arrive", "receive"].map(k => h("option", { selected: m.detail === k }, k)));
      case "Attitude": return h("input", { class: "input", value: m.detail || "", placeholder: "e.g. expected and late", "aria-label": "Attitude", oninput: e => upd(e.target.value) });
      case "Drop": return h("span", { class: "muted small" }, "Cut from the template.");
    }
  };
  return h("div", { class: "span-row" + (m?.confirmed ? " done" : ""), tabindex: "0", "data-span": s.key },
    h("span", { class: "span-text" }, "“", s.text, "”"),
    h("div", { class: "seg", role: "group", "aria-label": "What this span is" },
      choices.map((c, i) => h("button", { tabindex: "-1", "aria-pressed": String(!!m && m.choice === i), onclick: () => setChoice(i), "data-tip": `${i + 1}` }, c))),
    m?.suggested ? h("span", { class: "tag accent", "data-tip": "A keyword guess, not a model: no model is connected yet. Pick it, or another, to confirm." }, "Guess") : null,
    m?.confirmed ? h("span", { class: "tag ok" }, "✓") : null,
    detail());
}
function rerenderKeepFocus(key) {
  const focus = () => { const el = $(`[data-span="${CSS.escape(key)}"]`); if (el) el.focus(); };
  renderLines(); renderReview(); focus();
  refresh().then(focus);
}
async function openHelper(t) {
  if (t.cls === "punct") return;
  helperFor = { word: t.text, entries: null };
  renderReview();
  try {
    const j = await api("/api/lexicon?word=" + encodeURIComponent(t.text.replace(/n't$/i, "")));
    if (helperFor && helperFor.word === t.text) { helperFor.entries = j.entries || []; renderReview(); }
  } catch (e) { if (helperFor) { helperFor.entries = []; renderReview(); } }
}
function helperCard() {
  const es = helperFor.entries;
  return h("div", { class: "card" },
    h("div", { class: "card-head" }, h("h3", {}, "Word Helper: ", helperFor.word), h("button", { class: "btn ghost", onclick: () => { helperFor = null; renderReview(); } }, "Close")),
    h("div", { class: "card-body stack tight" },
      h("div", { class: "muted small" }, "The Same Meaning / Different Meaning / Breaks It table needs a model, and none is connected yet. What the lexicon says:"),
      es === null ? h("div", { class: "muted small" }, "Looking up…") :
        !es.length ? h("div", { class: "muted small" }, "Not in the lexicon.") :
          h("div", { class: "table-wrap" }, h("table", { class: "table helper" },
            h("thead", {}, h("tr", {}, ["Lemma", "Part", "Forms", "Guessed", "From"].map(x => h("th", {}, x)))),
            h("tbody", {}, es.map(e => h("tr", {},
              h("td", {}, e.lemma), h("td", {}, e.pos),
              h("td", { class: "mono small" }, Object.entries(e.forms || {}).map(([k, v]) => `${k} ${v}`).join(" · ") + (e.more ? " · more/most" : "") + (e.an ? " · an" : "")),
              h("td", {}, (e.doubt || []).join(", ") || "–"),
              h("td", { class: "muted small" }, (e.src || []).join(", "))))))),
      h("div", { class: "muted small" }, "Fixing a form here (hey, wait a minute) comes next: it will make a lexicon override and regenerate affected rows (sd/05/D79).")));
}

// ---------- samples (W6) ----------
const reasons = [
  ["Word Form", "A word is spelled or inflected wrong: \"gived\", \"a umbrella\".", "the word's lexicon entry"],
  ["Wrong Kind of Filler", "Grammatical, but that thing can't fill that role: \"The idea is tall.\"", "the role's type"],
  ["Ungrammatical", "Broken English: word order, agreement, a missing or extra word.", "the template"],
  ["Means Something Else", "Reads fine, but doesn't match its tags, or a word adds meaning the tags don't have.", "the template's meaning links"],
  ["Sounds Odd", "Correct, and means what the tags say, but nobody would say it like that without more context: \"This book will be old.\"", "the template's supported features: that combination now gets a time modifier"],
  ["Other", "Something else. Say what.", "a note"],
];
function renderSamples() {
  const box = $("#samples");
  const head = h("div", { class: "card-head" },
    h("h3", {}, "Samples"),
    h("div", { class: "row" }, h("span", { class: "muted small" }, "seed ", h("span", { class: "mono" }, seed)), h("button", { class: "btn", onclick: freshDraw, "data-tip": "R" }, "Fresh Draw")));
  const ready = lines.filter(l => l.traced);
  if (!ready.length) {
    box.replaceChildren(head, h("div", { class: "card-body muted small" }, lines.length ? "Samples appear once a line is fully traced." : "Write a sentence above to see samples of it."));
    return;
  }
  box.replaceChildren(head, h("div", { class: "card-body stack" }, ready.map(l => {
    const marked = needs.get(l.text) || [];
    return h("div", { class: "stack tight" },
      h("div", { class: "label" }, "From line ", l.n + 1, l.waits.length ? " · frozen: fillers vary, tense and mood don't" : ""),
      marked.map(nd => h("div", { class: "row needs" },
        h("span", { class: "tag warn" }, "Needs Context"),
        h("span", { class: "small" }, cap(nd.tense), " tense sounded odd, so the generator adds a time modifier to it (sd/08/D98)."),
        h("button", { class: "btn chip", onclick: () => { needs.set(l.text, marked.filter(x => x !== nd)); refresh(); } }, "Undo"))),
      l.error ? h("div", { class: "notice-fail small" }, l.error) : null,
      l.samples.map(s => sampleRow(l, s)),
      l.compact ? h("details", { class: "tree" }, h("summary", {}, "Template"), h("pre", { class: "out" }, l.compact)) : null);
  })));
  const rows = $$(".sample-row", box);
  if (curSample >= rows.length) curSample = rows.length - 1;
  rows.forEach((r, i) => r.classList.toggle("current", i === curSample));
}
function sampleRow(l, s) {
  const key = l.text + "\u0000" + s.text;
  const v = verdicts.get(key) || {};
  const send = patch => {
    const nv = { ...v, ...patch };
    verdicts.set(key, nv);
    api("/api/write/verdict", { meaning: card.meaning, line: l.text, sample: s, verdict: nv.v || "", reason: nv.reason || "", note: nv.note || "", star: !!nv.star }).catch(fail);
    if (patch.reason === "Sounds Odd") {
      const cur = needs.get(l.text) || [];
      if (!cur.some(x => x.tense === s.features.tense)) needs.set(l.text, [...cur, { tense: s.features.tense, modifier: "time" }]);
      refresh();
      return;
    }
    renderSamples(); renderSave();
  };
  const tags = [
    ...["tense", "polarity", "mood"].map(k => [cap(s.features[k]), s.changed.includes(k)]),
    ...Object.entries(s.fillers).map(([r, f]) => [`${r}: ${f}`, s.changed.includes(r)]),
    ...(s.time ? [[`time: ${s.time}`, true]] : []),
  ];
  return h("div", { class: "sample-row" + (v.v ? " judged" : ""), "data-key": key },
    h("div", { class: "sample-main" },
      h("div", { class: "sample-text" }, s.text),
      h("div", { class: "row sample-tags" }, tags.map(([t, c]) => h("span", { class: "tag mini" + (c ? " accent" : "") }, t)))),
    h("div", { class: "row" },
      h("div", { class: "seg rate", role: "group", "aria-label": "Verdict" },
        h("button", { "aria-pressed": String(v.v === "ok"), "data-rate": "good", onclick: () => send({ v: v.v === "ok" ? null : "ok", reason: null }), "data-tip": "Right (Y)" }, "✓"),
        h("button", { "aria-pressed": String(v.v === "bad"), "data-rate": "bad", onclick: () => send({ v: v.v === "bad" ? null : "bad" }), "data-tip": "Wrong (X)" }, "✕"),
        h("button", { "aria-pressed": String(!!v.star), onclick: () => send({ star: !v.star }), "data-tip": "Keep as a good example or boundary case" }, "★"))),
    v.v === "bad" ? h("div", { class: "row reasons" },
      h("span", { class: "label" }, "What's wrong?"),
      reasons.map(([x, why]) => h("button", { class: "btn chip" + (v.reason === x ? " picked" : ""), "data-tip": why, onclick: () => send({ reason: x }) }, x)),
      v.reason ? (([x, why, goes]) => h("div", { class: "reason-why small" },
        h("div", {}, why),
        x === "Other" ? h("div", { class: "row" },
          h("input", { class: "input grow", placeholder: "What's wrong? Enter saves", value: v.note || "", "aria-label": "Note", onchange: e => send({ note: e.target.value }) }),
          v.note ? h("span", { class: "tag ok", "data-tip": "Saved in the grammar's event log with this sample" }, "✓ Note Saved") : null) : null,
        h("div", { class: "muted" }, "Goes to ", goes, ".")))(reasons.find(r => r[0] === v.reason)) : null) : null);
}
function freshDraw() { seed = Math.floor(Math.random() * 1e6); refresh(); }
function moveSample(d) {
  const rows = $$(".sample-row");
  if (!rows.length) return;
  curSample = Math.max(0, Math.min(rows.length - 1, curSample + d));
  rows.forEach((r, i) => r.classList.toggle("current", i === curSample));
  rows[curSample].scrollIntoView({ block: "nearest", behavior: "smooth" });
}
function judge(v) {
  if (curSample < 0) moveSample(1);
  const row = $$(".sample-row")[curSample];
  if (!row) return;
  row.querySelector(v === "ok" ? '[data-rate="good"]' : '[data-rate="bad"]').click();
  if (v === "ok") setTimeout(() => moveSample(1), 50);
}

// ---------- save (W7) ----------
let savedList = [];
async function loadSaved() {
  try { savedList = (await api("/api/write/saved")).templates || []; } catch (e) { savedList = []; }
  renderSave();
}
// Save holds back lines that would make a broken template (08/D101): ones whose
// sentence disagrees with the meaning (⚠), or with a sample marked Ungrammatical
// or Means Something Else. Save as Draft still keeps them.
const blocking = new Set(["Ungrammatical", "Means Something Else"]);
function blockers(l) {
  const out = l.warnings.map(w => "⚠ " + w);
  const seen = new Set();
  for (const [k, v] of verdicts) {
    if (k.startsWith(l.text + "\u0000") && v.v === "bad" && blocking.has(v.reason) && !seen.has(v.reason)) {
      seen.add(v.reason);
      out.push(`A sample is marked ${v.reason}: reword the sentence, or clear that ✕ if it was a slip.`);
    }
  }
  return out;
}
const savable = l => l.traced && !blockers(l).length;
async function save(draft) {
  if (!card) return;
  const want = new Set(lines.filter(l => draft ? !savable(l) : savable(l)).map(l => l.text));
  const texts = $("#text").value.split("\n").filter(t => t.trim() && want.has(t));
  if (!texts.length) return;
  try {
    await api("/api/write/save", { meaning: card.meaning, draft, lines: texts.map(t => ({ text: t, res: resOf(t), needs: needs.get(t) || [], how: pasted.has(t) ? "pasted" : "typed" })) });
  } catch (e) { return fail(e); }
  loadSaved();
}
function renderSave() {
  const ok = lines.filter(savable).length, drafts = lines.length - ok;
  const held = lines.filter(l => l.traced && !savable(l));
  $("#saved").replaceChildren(
    h("div", { class: "card-head" }, h("h3", {}, "Save"),
      h("div", { class: "row" },
        h("button", { class: "btn", disabled: !drafts || !card, onclick: () => save(true) }, "Save as Draft"),
        h("button", { class: "btn primary", disabled: !ok || !card, onclick: () => save(false), "data-tip": "Ctrl+Enter" }, ok > 1 ? `Save Templates (${ok})` : "Save Template"))),
    h("div", { class: "card-body stack tight" },
      held.map(l => h("div", { class: "held" },
        h("div", { class: "row" }, h("span", { class: "tag warn" }, "Held Back"), h("span", {}, "Line ", l.n + 1, ": ", l.text)),
        h("ul", { class: "small" }, blockers(l).map(b => h("li", {}, b))))),
      h("div", { class: "muted small" }, savedList.length, " template", savedList.length === 1 ? "" : "s", " saved in the grammar."),
      savedList.slice(0, 20).map(s => h("div", { class: "saved-row" },
        h("div", { class: "row" }, h("span", { class: "tag ok" }, "✓ Template"), h("span", { class: "tag" }, s.Frame),
          (s.Frozen || []).length ? h("span", { class: "tag" }, "Frozen") : null, h("span", {}, s.Text)),
        h("details", { class: "tree" }, h("summary", {}, "Tree"), h("pre", { class: "out" }, s.Compact))))));
}

// ---------- keys (K1) ----------
const text = $("#text");
text.addEventListener("input", () => { clearTimeout(window.__t); window.__t = setTimeout(refresh, 120); });
text.addEventListener("paste", e => {
  const t = (e.clipboardData || window.clipboardData).getData("text");
  t.split("\n").filter(x => x.trim()).forEach(x => pasted.add(x));
});
document.addEventListener("keydown", e => {
  const inField = e.target.closest("textarea, input, select, [contenteditable]");
  if ((e.ctrlKey || e.metaKey) && e.key === "Enter") { e.preventDefault(); save(false); return; }
  if (e.altKey && e.code === "KeyN") { e.preventDefault(); next(); return; }
  if (e.altKey && e.code === "KeyF") { e.preventDefault(); newFillers(); return; }
  if (inField || e.ctrlKey || e.metaKey || e.altKey) return;
  const span = e.target.closest("[data-span]");
  if (span && /^[1-5]$/.test(e.key)) { e.preventDefault(); span.querySelectorAll(".seg button")[+e.key - 1].click(); return; }
  if (e.key === "j") { e.preventDefault(); moveSample(1); }
  else if (e.key === "k") { e.preventDefault(); moveSample(-1); }
  else if (e.key === "y") { e.preventDefault(); judge("ok"); }
  else if (e.key === "x") { e.preventDefault(); judge("bad"); }
  else if (e.key === "r") { e.preventDefault(); freshDraw(); }
  else if (e.key === "?") { e.preventDefault(); $("#keys").showModal(); }
});
$("#keys [data-close]").addEventListener("click", () => $("#keys").close());

if (!$("#write-error")) { renderMeaning(); load(); loadSaved(); }
