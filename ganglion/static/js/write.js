// The writing screen mock-up (design/synthetic-data/08, D97). Alignment,
// suggestions, the word helper and samples are faked here in the browser with
// the data in write-data.js; nothing is sent to the server or saved.
import { entities, verbs, adjectives, grammar, frames, meanings, spanHints, choices, helper } from "./write-data.js";

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
function rng(seed) { // mulberry32
  return () => { seed |= 0; seed = (seed + 0x6d2b79f5) | 0; let t = Math.imul(seed ^ (seed >>> 15), 1 | seed); t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t; return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
}
const pick = (r, xs) => xs[Math.floor(r() * xs.length)];
const cap = s => s.charAt(0).toUpperCase() + s.slice(1);

// ---------- state ----------
let mi = 0;
let meaning = clone(meanings[0]);
let seed = 42;
let lines = [];           // aligned lines
const memory = new Map(); // span text -> {choice, confirmed, suggested, detail}
const pending = new Set();
const saved = [];
let helperFor = null;     // {word, entry}
let curSample = -1;
const verdicts = new Map(); // sample key -> {v, reason, star}

function clone(x) { return JSON.parse(JSON.stringify(x)); }
const frame = () => frames[meaning.frame];
const roleNames = () => Object.keys(frame().roles);
const roleIndex = r => roleNames().indexOf(r) + 1;

// ---------- meaning card (W2) ----------
function fillerText(role, v) {
  if (typeof v === "object") return `${v.attribute} = ${v.value}`;
  const e = entities[v];
  const p = e.pron ? `${e.pron.subj}/${e.pron.obj}` : "";
  if (e.coerced) return `${e.phrase} · ${e.type}, treated as ${e.coerced}`;
  return e.name ? `${e.name} · ${e.type} · ${p}` : `${e.noun} · ${e.type}`;
}
function renderMeaning() {
  const f = frame(), el = $("#meaning");
  el.replaceChildren(
    h("div", { class: "card-head" },
      h("div", { class: "stack tight" },
        h("div", { class: "row" }, h("h3", {}, f.title), meaning.joke ? h("span", { class: "tag warn", "data-tip": meaning.joke }, "Joke On Purpose") : null),
        h("span", { class: "muted small" }, f.explainer)),
      h("div", { class: "row" },
        h("button", { class: "btn primary", onclick: next, "data-tip": "Alt+N" }, "Next"),
        h("button", { class: "btn", onclick: newFillers, "data-tip": "Alt+F" }, "New Fillers"),
        h("button", { class: "btn ghost", onclick: toggleChange, "aria-expanded": String(!!changing) }, "Change"))),
    h("div", { class: "card-body stack tight" },
      h("table", { class: "table roles" }, h("tbody", {},
        roleNames().map(r => h("tr", {},
          h("td", { class: "nowrap" }, h("span", { class: `series role-bg-${roleIndex(r)}` }), h("b", {}, r),
            h("span", { class: "info", tabindex: "0", "aria-label": "About this role", "data-tip": `${f.roles[r].explainer} Generic name: ${f.roles[r].generic}.` }, "i")),
          h("td", {}, h("span", { class: "tag" }, fillerText(r, meaning.roles[r]))))))),
      h("div", { class: "row" },
        ["tense", "polarity", "mood"].map(k => h("span", { class: "tag accent" }, cap(meaning.features[k]))),
        Object.entries(meaning.modifiers || {}).map(([k, v]) => h("span", { class: "tag" }, `${cap(k)}: ${v}`))),
      h("div", { class: "muted small" }, "Why this one: ", meaning.gap),
      changing ? changePanel() : null));
}
let changing = false;
function toggleChange() { changing = !changing; renderMeaning(); }
function changePanel() {
  const seg = (k, vals) => h("div", { class: "seg", role: "group", "aria-label": cap(k) },
    vals.map(v => h("button", { "aria-pressed": String(meaning.features[k] === v), onclick: () => { meaning.features[k] = v; renderMeaning(); realign(); } }, cap(v))));
  return h("div", { class: "row change" },
    h("select", { class: "input", "aria-label": "Meaning", onchange: e => setMeaning(+e.target.value) },
      meanings.map((m, i) => h("option", { value: i, selected: i === mi }, `${frames[m.frame].title}: ${Object.values(m.roles).map(v => typeof v === "object" ? v.word : (entities[v].name || entities[v].noun)).join(", ")}`))),
    seg("tense", ["past", "present", "future"]), seg("polarity", ["positive", "negative"]), seg("mood", ["statement", "question"]));
}
function setMeaning(i) {
  mi = (i + meanings.length) % meanings.length;
  meaning = clone(meanings[mi]);
  $("#text").value = "";
  helperFor = null;
  renderMeaning(); realign();
}
function next() { setMeaning(mi + 1); }
function newFillers() {
  const r = rng(Date.now() & 0xffff), used = new Set();
  for (const [role, v] of Object.entries(meaning.roles)) {
    if (typeof v !== "string" || entities[v].coerced) { used.add(v); continue; }
    const pool = Object.keys(entities).filter(k => entities[k].type === entities[v].type && !used.has(k) && k !== v && !entities[k].coerced);
    meaning.roles[role] = pool.length ? pick(r, pool) : v;
    used.add(meaning.roles[role]);
  }
  renderMeaning(); realign();
}

// ---------- alignment (W3a, sd/03/A13) ----------
const modals = new Set(["can", "could", "may", "might", "must", "shall", "should", "would"]);
const copula = new Set(["is", "are", "was", "were", "am", "be", "been", "being"]);
const auxes = new Set(["did", "does", "do", "will", "has", "have", "had", "not", "n't"]);
const extraGrammar = new Set(["from", "by"]);

function tokenize(text) {
  const out = [];
  const re = /[A-Za-z]+(?:'[A-Za-z]+)?|[0-9]+|[^\sA-Za-z0-9]/g;
  let m;
  while ((m = re.exec(text))) out.push({ text: m[0], at: m.index });
  return out;
}
function negBase(w) {
  if (w === "won't") return "will";
  if (w === "can't") return "can";
  if (w.endsWith("n't")) return w.slice(0, -3);
  return null;
}
function classify(toks) {
  const f = frame();
  const roleEnts = Object.entries(meaning.roles).filter(([, v]) => typeof v === "string");
  const lw = toks.map(t => t.text.toLowerCase());
  for (let i = 0; i < toks.length; i++) {
    const t = toks[i];
    if (t.cls) continue;
    const w = lw[i], neg = negBase(w), base = neg || w;
    if (!/[a-z0-9]/i.test(w)) { t.cls = "punct"; continue; }
    // multiword phrases (the coerced doom)
    let matched = false;
    for (const [role, k] of roleEnts) {
      const e = entities[k];
      const words = (e.noun || "").split(" ");
      if (words.length > 1 && words.every((x, j) => lw[i + j] === x)) {
        for (let j = 0; j < words.length; j++) Object.assign(toks[i + j], { cls: "role", role, ent: k, form: "noun", part: j });
        matched = true; break;
      }
    }
    if (matched) continue;
    for (const [role, k] of roleEnts) {
      const e = entities[k];
      if (e.name && w === e.name.toLowerCase()) { Object.assign(t, { cls: "role", role, ent: k, form: "name" }); break; }
      if (e.name && w === e.name.toLowerCase() + "'s") { Object.assign(t, { cls: "role", role, ent: k, form: "possname" }); break; }
      if (e.noun && w === e.noun) { Object.assign(t, { cls: "role", role, ent: k, form: "noun" }); break; }
      if (e.plural && w === e.plural) { Object.assign(t, { cls: "role", role, ent: k, form: "plural" }); break; }
      const pc = e.pron && Object.entries(e.pron).find(([c, p]) => c !== "plural" && p === w);
      if (pc) { Object.assign(t, { cls: "role", role, ent: k, form: "pron", pcase: pc[0] }); break; }
    }
    if (t.cls) continue;
    for (const [role, v] of Object.entries(meaning.roles)) {
      if (typeof v !== "object") continue;
      const family = Object.values(adjectives).find(fs => fs.includes(v.word));
      const conv = v.word === "taller" ? ["shorter"] : [];
      if ((family && family.includes(w)) || conv.includes(w)) { Object.assign(t, { cls: "role", role, prop: true, converse: conv.includes(w) }); break; }
    }
    if (t.cls) continue;
    const lemmaOf = ws => Object.entries(verbs).find(([l, v]) => ws.includes(l) && v.forms.includes(base));
    const fv = lemmaOf(f.verbs) || (f.converse && lemmaOf(f.converse));
    if (fv && !(fv[0] === "be" && !f.verbs.includes("be"))) {
      Object.assign(t, { cls: "frame", lemma: fv[0], fi: verbs[fv[0]].forms.indexOf(base), neg: !!neg, converse: !!(f.converse || []).includes(fv[0]) });
      if (copula.has(w) || copula.has(base)) t.form = base;
      continue;
    }
    if (copula.has(base) && f.verbs.includes("be")) { Object.assign(t, { cls: "frame", lemma: "be", form: base, neg: !!neg }); continue; }
    if (modals.has(base)) { t.cls = "untraced"; t.modal = true; continue; }
    if (grammar.has(w) || grammar.has(base) || extraGrammar.has(w) || copula.has(base) || auxes.has(base)) { t.cls = "grammar"; t.neg = !!neg; continue; }
    t.cls = "untraced";
  }
  return toks;
}
function findSpans(toks) {
  const spans = [];
  let run = [];
  const flush = () => {
    while (run.length && run[run.length - 1].cls !== "untraced") run.pop();
    while (run.length && ["a", "an", "the"].includes(run[0].text.toLowerCase())) run.shift();
    if (run.some(t => t.cls === "untraced")) spans.push(run.slice());
    run = [];
  };
  // a grammar word joins a span when the next non-grammar word is untraced ("as a gift")
  const leadsIn = i => { let j = i + 1; while (toks[j] && toks[j].cls === "grammar") j++; return toks[j] && toks[j].cls === "untraced"; };
  toks.forEach((t, i) => {
    const next = toks[i + 1];
    const possInside = t.cls === "role" && (t.pcase === "det" || t.form === "possname") && run.length && next && next.cls === "untraced";
    if (t.cls === "untraced" || (t.cls === "grammar" && (run.length || leadsIn(i))) || possInside) run.push(t);
    else flush();
  });
  flush();
  return spans.map(ts => {
    const text = ts.map(t => t.text).join(" ").replace(/ ([.,!?;:])/g, "$1");
    ts.forEach(t => (t.span = text.toLowerCase()));
    return { key: text.toLowerCase(), text, toks: ts };
  });
}
function waitsOn(toks) {
  const w = toks.map(t => t.text.toLowerCase());
  const out = [];
  if (toks.some(t => t.modal)) out.push("Modals");
  const fi = toks.findIndex(t => t.cls === "frame" && t.lemma !== "be" && t.fi === 3);
  if (fi > 0 && copula.has(w[fi - 1])) out.push("Passive");
  if (/,\s*(isn't|aren't|wasn't|didn't|doesn't|won't|is|was|did|does)\s+(she|he|they|it|i|you)\s*\?\s*$/i.test(toks.map(t => t.text).join(" "))) out.push("Tag Questions");
  return out;
}

function realign() {
  const raw = $("#text").value.split("\n");
  lines = raw.map((text, n) => {
    if (!text.trim()) return null;
    const toks = classify(tokenize(text));
    const spans = findSpans(toks);
    const waits = waitsOn(toks);
    for (const s of spans) suggest(s);
    return { n, text, toks, spans, waits };
  }).filter(Boolean);
  renderLines(); renderReview(); renderSamples(); renderSaved();
}

// ---------- suggestions (W4): a fake model call with a delay ----------
function suggest(span) {
  if (memory.has(span.key) || pending.has(span.key)) return;
  pending.add(span.key);
  setTimeout(() => {
    pending.delete(span.key);
    const k = span.key, modal = span.toks.some(t => t.modal);
    let s = modal ? [3, "possibility"] : null;
    for (const [re, c, d] of spanHints) if (!s && re.test(k)) s = [c, d];
    if (!s) s = [1, ""];
    if (!memory.has(k)) memory.set(k, { choice: s[0], suggested: true, confirmed: false, detail: s[1] });
    renderLines(); renderReview(); renderSamples();
  }, 600 + Math.random() * 500);
}
const resolved = s => memory.get(s.key)?.confirmed;
const fullyTraced = l => l.spans.every(resolved);

// ---------- traced view under the textarea (W3a) ----------
function renderLines() {
  const box = $("#lines");
  box.replaceChildren(...lines.map(l => {
    const segs = [];
    let last = 0;
    for (const t of l.toks) {
      if (t.at > last) segs.push(l.text.slice(last, t.at));
      const cls = ["tok"];
      if (t.cls === "role") cls.push(`role-${roleIndex(t.role)}`);
      else if (t.cls === "frame") cls.push("frame");
      else if (t.cls === "untraced" || t.span) cls.push(memory.get(t.span)?.confirmed ? "resolved" : t.cls === "untraced" ? "untraced" : "inspan");
      const tip = t.cls === "role" ? `${t.role}${t.form === "pron" ? ` (${t.pcase} pronoun)` : ""}` : t.cls === "frame" ? `${t.lemma}${t.converse ? ", converse viewpoint" : ""}` : t.cls === "untraced" ? (memory.get(t.span)?.confirmed ? "Resolved" : "Not in the meaning yet") : t.cls === "grammar" ? "Grammar" : null;
      segs.push(h("span", { class: cls.join(" "), "data-tip": tip, tabindex: t.cls === "punct" ? null : "-1", onclick: () => openHelper(t) }, t.text));
      last = t.at + t.text.length;
    }
    if (last < l.text.length) segs.push(l.text.slice(last));
    const open = l.spans.filter(s => !resolved(s)).length;
    return h("div", { class: "traced" },
      h("div", { class: "traced-text" }, segs),
      h("div", { class: "row" },
        open ? h("span", { class: "tag warn" }, "? ", `${open} to resolve`) : h("span", { class: "tag ok" }, "✓ Fully Traced"),
        l.waits.map(w => h("span", { class: "tag", "data-tip": "Saved frozen: usable now, tense and mood fixed (sd/05/D80)" }, "Frozen · Waits On ", w))));
  }));
}

// ---------- review: spans (W4) and word helper (W5) ----------
const modifierKinds = ["occasion", "purpose", "time", "means", "place"];
function renderReview() {
  const box = $("#review");
  const cards = [];
  if (helperFor) cards.push(helperCard());
  lines.forEach(l => {
    if (!l.spans.length) return;
    cards.push(h("div", { class: "card" },
      h("div", { class: "card-head" }, h("h3", {}, "Line ", l.n + 1, ": What Do These Add?"), h("span", { class: "muted label" }, "Tab between spans · 1–5 picks")),
      h("div", { class: "card-body stack tight" }, l.spans.map(s => spanRow(s)))));
  });
  box.replaceChildren(...cards);
}
function spanRow(s) {
  const m = memory.get(s.key);
  const setChoice = c => { memory.set(s.key, { choice: c, suggested: false, confirmed: true, detail: m && m.choice === c ? m.detail : "" }); rerenderKeepFocus(s.key); };
  const detail = () => {
    if (!m) return h("span", { class: "muted small" }, "Waiting for a suggestion…");
    const upd = v => { m.detail = v; renderSaved(); };
    switch (choices[m.choice]) {
      case "Modifier": return h("select", { class: "input", "aria-label": "Which modifier", onchange: e => upd(e.target.value) }, modifierKinds.map(k => h("option", { selected: m.detail === k }, k)));
      case "New Modifier": return h("span", { class: "row" },
        h("input", { class: "input", placeholder: "Name", value: m.detail || "", "aria-label": "Modifier name", oninput: e => upd(e.target.value) }),
        h("select", { class: "input", "aria-label": "Type" }, ["event detail", "entity detail"].map(k => h("option", {}, k))),
        h("input", { class: "input grow", placeholder: "One-line explainer", "aria-label": "Explainer" }));
      case "Event": return h("select", { class: "input", "aria-label": "Which event", onchange: e => upd(e.target.value) }, ["send", "arrive", "receive", "(new frame…)"].map(k => h("option", { selected: m.detail === k }, k)));
      case "Attitude": return h("input", { class: "input", value: m.detail || "", placeholder: "e.g. expected and late", "aria-label": "Attitude", oninput: e => upd(e.target.value) });
      case "Drop": return h("span", { class: "muted small" }, "Cut from the template.");
    }
  };
  return h("div", { class: "span-row" + (m?.confirmed ? " done" : ""), tabindex: "0", "data-span": s.key },
    h("span", { class: "span-text" }, "“", s.text, "”"),
    h("div", { class: "seg", role: "group", "aria-label": "What this span is" },
      choices.map((c, i) => h("button", { tabindex: "-1", "aria-pressed": String(!!m && m.choice === i), onclick: () => setChoice(i), "data-tip": `${i + 1}` }, c))),
    m?.suggested ? h("span", { class: "tag accent", "data-tip": "A model's guess (mocked). Pick it, or another, to confirm." }, "Suggested") : null,
    m?.confirmed ? h("span", { class: "tag ok" }, "✓") : null,
    detail());
}
function rerenderKeepFocus(key) {
  renderLines(); renderReview(); renderSamples(); renderSaved();
  const el = $(`[data-span="${CSS.escape(key)}"]`);
  if (el) el.focus();
}
function openHelper(t) {
  if (t.cls === "punct") return;
  const w = t.text.toLowerCase();
  helperFor = { word: t.text, entry: null, loading: true, tok: t };
  renderReview();
  setTimeout(() => {
    if (!helperFor || helperFor.word !== t.text) return;
    helperFor.loading = false;
    helperFor.entry = helper[w] || helper[negBase(w)] || null;
    renderReview();
  }, 450);
}
function formsFor(t) {
  const w = t.text.toLowerCase();
  if (t.lemma) return [t.lemma, verbs[t.lemma].forms];
  const a = Object.entries(adjectives).find(([, fs]) => fs.includes(w));
  if (a) return a;
  if (t.ent && entities[t.ent].noun) return [entities[t.ent].noun, [entities[t.ent].noun, entities[t.ent].plural]];
  return null;
}
function helperCard() {
  const e = helperFor.entry, forms = formsFor(helperFor.tok);
  const cell = v => h("td", { contenteditable: "true", spellcheck: "false" }, v || "");
  return h("div", { class: "card" },
    h("div", { class: "card-head" },
      h("div", { class: "row" }, h("h3", {}, "Word Helper: ", helperFor.word), h("span", { class: "tag accent", "data-tip": "Made by a model (mocked here). Edit any cell; edits are vetting data." }, "Suggested")),
      h("button", { class: "btn ghost", onclick: () => { helperFor = null; renderReview(); } }, "Close")),
    h("div", { class: "card-body stack tight" },
      helperFor.loading ? h("div", { class: "muted small" }, "Asking the model…") :
        e ? h("div", { class: "table-wrap" }, h("table", { class: "table helper" },
          h("thead", {}, h("tr", {}, ["Role", "Same Meaning", "Different Meaning", "Breaks It"].map(x => h("th", {}, x)))),
          h("tbody", {}, h("tr", {}, cell(e.role), cell(e.same), cell(e.diff), cell(e.breaks))))) :
          h("div", { class: "muted small" }, "No mock table for this word."),
      forms ? h("details", { class: "forms" }, h("summary", {}, "Hey, Wait a Minute: Forms of ", forms[0]),
        h("div", { class: "row gap-top" }, forms[1].map(f => h("input", { class: "input form", value: f, "aria-label": "Form" }))),
        h("div", { class: "muted small" }, "In the real thing, a fix here makes a new lexicon version and regenerates affected rows (sd/05/D79).")) : null));
}

// ---------- samples (W6): a tiny fake generator ----------
const tenses = ["past", "present", "future"];
function plan(l) {
  // the verb group: the frame verb, plus adjacent auxiliaries before it
  const vi = l.toks.findIndex(t => t.cls === "frame");
  if (vi < 0) return null;
  let start = vi;
  while (start > 0 && l.toks[start - 1].cls === "grammar" && auxes.has(negBase(l.toks[start - 1].text.toLowerCase()) || l.toks[start - 1].text.toLowerCase())) start--;
  const subj = l.toks.slice(0, start).find(t => t.cls === "role");
  const simple = l.toks.slice(0, start).every(t => t.cls === "role" || ["a", "an", "the"].includes(t.text.toLowerCase()));
  return { vi, start, subj, simple };
}
function verbGroup(lemma, f, plural, question) {
  const be = lemma === "be", forms = verbs[lemma].forms;
  const neg = f.polarity === "negative";
  if (be) {
    const fin = { past: plural ? "were" : "was", present: plural ? "are" : "is", future: "will" }[f.tense];
    const finN = neg ? (f.tense === "future" ? "won't" : fin + "n't") : fin;
    const rest = f.tense === "future" ? ["be"] : [];
    return question ? { aux: cap(finN), main: rest } : { aux: null, main: [finN, ...rest] };
  }
  if (f.tense === "future") return question ? { aux: neg ? "Won't" : "Will", main: [forms[0]] } : { aux: null, main: [neg ? "won't" : "will", forms[0]] };
  if (neg || question) {
    const d = f.tense === "past" ? "did" : plural ? "do" : "does";
    const dN = neg ? d + "n't" : d;
    return question ? { aux: cap(dN), main: [forms[0]] } : { aux: null, main: [dN, forms[0]] };
  }
  return { aux: null, main: [f.tense === "past" ? forms[2] : plural ? forms[0] : forms[1]] };
}
function realizeTok(t, fill) {
  if (t.cls !== "role" || t.prop) return t.text;
  const e = entities[fill[t.role]];
  if (t.form === "name") return e.name || "the " + e.noun;
  if (t.form === "possname") return (e.name || e.noun) + "'s";
  if (t.form === "pron") return e.pron[t.pcase];
  if (t.form === "noun") return e.noun.split(" ")[t.part || 0] ?? "";
  if (t.form === "plural") return e.plural;
  return t.text;
}
function makeSample(l, p, f, fill, question) {
  if (l.waits.length) { // frozen (sd/05/D80): the verb words stay as written; only fillers vary
    const s = l.toks.map(t => realizeTok(t, fill)).join(" ").replace(/ ([.,!?;:])/g, "$1").replace(/\b(a|an) (\w)/gi, (m, d, c) => (/[aeiou]/i.test(c) ? (d[0] === "A" ? "An" : "an") : (d[0] === "A" ? "A" : "a")) + " " + c);
    return cap(s);
  }
  const words = [];
  const subjEnt = p.subj ? entities[fill[p.subj.role]] : null;
  const plural = !!(p.subj && ((p.subj.form === "pron" && subjEnt.pron.plural) || p.subj.form === "plural"));
  const vt = l.toks[p.vi];
  const vg = verbGroup(vt.lemma, f, plural, question);
  const pre = l.toks.slice(0, p.start).map(t => realizeTok(t, fill));
  const post = l.toks.slice(p.vi + 1).filter(t => !(t.cls === "grammar" && (t.text.toLowerCase() === "not" || t.text.toLowerCase() === "n't"))).map(t => realizeTok(t, fill));
  if (question) { words.push(vg.aux, ...pre.map((w, i) => (i === 0 && !p.subj?.form?.match(/name/) ? w.toLowerCase() : w)), ...vg.main, ...post); }
  else words.push(...pre, ...vg.main, ...post);
  let s = words.filter(w => w !== "").join(" ").replace(/ ([.,!?;:])/g, "$1").replace(/\b(a|an) (\w)/gi, (m, d, c) => (/[aeiou]/i.test(c) ? (d[0] === "A" ? "An" : "an") : (d[0] === "A" ? "A" : "a")) + " " + c);
  s = s.replace(/[.!?]?\s*$/, question ? "?" : ".");
  return cap(s);
}
function samplesFor(l, li) {
  const p = plan(l);
  if (!p) return [];
  const r = rng(seed * 7919 + li * 104729);
  const frozen = l.waits.length > 0;
  const out = [], seen = new Set();
  const roles = Object.entries(meaning.roles).filter(([, v]) => typeof v === "string");
  for (let i = 0; out.length < 8 && i < 40; i++) {
    const fill = {}, used = new Set();
    for (const [role, k] of roles) {
      const pool = Object.keys(entities).filter(x => entities[x].type === entities[k].type && !used.has(x) && !(entities[x].coerced && x !== k));
      fill[role] = i === 0 ? k : pick(r, pool.length ? pool : [k]);
      used.add(fill[role]);
    }
    const f = frozen || i === 0 ? { ...meaning.features } : {
      tense: tenses[i % 3],
      polarity: i % 4 === 3 ? "negative" : "positive",
      mood: p.simple && i % 5 === 4 ? "question" : "statement",
    };
    let text;
    try { text = makeSample(l, p, f, fill, f.mood === "question"); } catch (e) { continue; }
    if (seen.has(text)) continue;
    seen.add(text);
    out.push({ key: `${li}:${text}`, text, f, fill });
  }
  return out;
}
function sampleTags(s) {
  const tags = [];
  for (const k of ["tense", "polarity", "mood"]) tags.push([cap(s.f[k]), s.f[k] !== meaning.features[k]]);
  for (const [role, k] of Object.entries(s.fill)) tags.push([`${role}: ${entities[k].name || entities[k].noun}`, k !== meaning.roles[role]]);
  return tags;
}
const reasons = ["Word Form", "Wrong Kind of Filler", "Ungrammatical", "Means Something Else", "Other"];
const reasonGoes = { "Word Form": "the lexicon entry", "Wrong Kind of Filler": "the slot's type", "Ungrammatical": "the template", "Means Something Else": "the span resolution", "Other": "a note" };
function renderSamples() {
  const box = $("#samples");
  const ready = lines.map((l, i) => [l, i]).filter(([l]) => fullyTraced(l));
  const head = h("div", { class: "card-head" },
    h("div", { class: "row" }, h("h3", {}, "Samples"), h("span", { class: "tag", "data-tip": "Faked in the browser. The real generator is Go in ganglion (sd/03/D73)." }, "Mock")),
    h("div", { class: "row" }, h("span", { class: "muted small" }, "seed ", h("span", { class: "mono" }, seed)), h("button", { class: "btn", onclick: freshDraw, "data-tip": "R" }, "Fresh Draw")));
  if (!ready.length) {
    box.replaceChildren(head, h("div", { class: "card-body muted small" }, lines.length ? "Samples appear once a line is fully traced." : "Write a sentence above to see samples of it."));
    return;
  }
  const groups = ready.map(([l, i]) => {
    const ss = samplesFor(l, i);
    return h("div", { class: "stack tight" },
      h("div", { class: "label" }, "From line ", l.n + 1, l.waits.length ? " · frozen: fillers vary, tense and mood don't" : ""),
      ss.map(s => sampleRow(s)));
  });
  box.replaceChildren(head, h("div", { class: "card-body stack" }, groups));
  const rows = $$(".sample-row", box);
  if (curSample >= rows.length) curSample = rows.length - 1;
  rows.forEach((r, i) => r.classList.toggle("current", i === curSample));
}
function sampleRow(s) {
  const v = verdicts.get(s.key) || {};
  const set = patch => { verdicts.set(s.key, { ...v, ...patch }); renderSamples(); };
  return h("div", { class: "sample-row" + (v.v ? " judged" : ""), "data-key": s.key },
    h("div", { class: "sample-main" },
      h("div", { class: "sample-text" }, s.text),
      h("div", { class: "row sample-tags" }, sampleTags(s).map(([t, changed]) => h("span", { class: "tag mini" + (changed ? " accent" : "") }, t)))),
    h("div", { class: "row" },
      h("div", { class: "seg rate", role: "group", "aria-label": "Verdict" },
        h("button", { "aria-pressed": String(v.v === "ok"), "data-rate": "good", onclick: () => set({ v: v.v === "ok" ? null : "ok", reason: null }), "data-tip": "Right (Y)" }, "✓"),
        h("button", { "aria-pressed": String(v.v === "bad"), "data-rate": "bad", onclick: () => set({ v: v.v === "bad" ? null : "bad" }), "data-tip": "Wrong (X)" }, "✕"),
        h("button", { "aria-pressed": String(!!v.star), onclick: () => set({ star: !v.star }), "data-tip": "Keep as a good example or boundary case" }, "★"))),
    v.v === "bad" ? h("div", { class: "row reasons" },
      h("span", { class: "label" }, "What's wrong?"),
      reasons.map(x => h("button", { class: "btn chip" + (v.reason === x ? " picked" : ""), onclick: () => set({ reason: x }) }, x)),
      v.reason ? h("span", { class: "muted small" }, "Goes to ", reasonGoes[v.reason], " (mock: not recorded).") : null) : null);
}
function freshDraw() { seed = Math.floor(Math.random() * 100000); renderSamples(); }
function moveSample(d) {
  const rows = $$(".sample-row");
  if (!rows.length) return;
  curSample = Math.max(0, Math.min(rows.length - 1, curSample + d));
  rows.forEach((r, i) => r.classList.toggle("current", i === curSample));
  rows[curSample].scrollIntoView({ block: "nearest", behavior: "smooth" });
}
function judge(v) {
  const rows = $$(".sample-row");
  if (curSample < 0) moveSample(1);
  const row = rows[curSample];
  if (!row) return;
  const key = row.dataset.key, old = verdicts.get(key) || {};
  verdicts.set(key, { ...old, v, reason: v === "ok" ? null : old.reason });
  renderSamples();
  if (v === "ok") moveSample(1);
}

// ---------- save (W7) ----------
function compact(l) {
  const out = [];
  const v = {}; // a short variable per role: giver -> g, thing -> t
  for (const r of roleNames()) { const x = r[0]; v[r] = Object.values(v).includes(x) ? r : x; }
  const ent = Object.entries(meaning.roles).map(([r, val]) => `${r}: ${typeof val === "object" ? `${val.attribute}=${val.value}` : v[r]}`).join(", ");
  const addOf = (s, m) => {
    const c = choices[m.choice];
    if (c === "Modifier" || c === "New Modifier") return `e.${m.detail || "?"}`;
    if (c === "Attitude") return `e.attitude = ${m.detail || "?"}`;
    if (c === "Event") return `e2 = ${m.detail || "?"}(…), linked to e`;
    return null;
  };
  const adds = l.spans.map(s => [s, memory.get(s.key)]).filter(([, m]) => m && choices[m.choice] !== "Drop").map(([s, m]) => addOf(s, m)).filter(Boolean);
  out.push(`template ${meaning.frame}-${saved.length + 1}   (mock)`, `  kind: S`, `  means: e = ${meaning.frame}(${ent})${adds.length ? ", " + adds.join(", ") : ""}`);
  out.push(l.waits.length ? `  supports: ${meaning.features.tense} only, ${meaning.features.mood} only   (frozen: waits on ${l.waits.join(", ")})` : `  supports: tense *, polarity *, mood statement|question`, `  tree:`);
  const p = plan(l);
  const refOf = t => `REF(${v[t.role]})${t.form === "pron" ? ` ${t.pcase === "det" ? "possessive" : t.pcase} pronoun` : t.form === "possname" ? " possessive" : ""}`;
  l.toks.forEach((t, i) => {
    if (t.cls === "punct") return;
    if (t.span) { // the whole span once, at its first token; roles inside it stay slots
      const s = l.spans.find(x => x.key === t.span), m = memory.get(t.span);
      if (s.toks[0] !== t || !m || choices[m.choice] === "Drop") return;
      const body = s.toks.map(x => (x.cls === "role" ? `{${refOf(x)}}` : x.text)).join(" ");
      out.push(`    word "${body}"          realises ${addOf(s, m)}`);
      return;
    }
    if (t.cls === "role" && !t.prop) { if (t.part) return; out.push(`    ${p && i < p.start ? "subj: " : ""}${refOf(t)}`); }
    else if (t.cls === "role") out.push(`    DESC(${t.role})          realises ${t.role}`);
    else if (t.cls === "frame") out.push(`    vg: lex ${t.lemma}${l.waits.length ? " (frozen words)" : ""}          ${t.lemma === "be" ? "grammar" : "realises e"}${t.converse ? " (converse)" : ""}`);
    else if (t.cls === "grammar" && !(p && i >= p.start && i < p.vi)) out.push(`    word "${t.text}"`);
  });
  return out.join("\n");
}
function save(draft) {
  const ready = lines.filter(l => draft ? !fullyTraced(l) : fullyTraced(l));
  for (const l of ready) saved.push({ text: l.text, draft, frame: frame().title, tree: draft ? null : compact(l), waits: l.waits });
  renderSaved();
}
function renderSaved() {
  const box = $("#saved");
  const canSave = lines.filter(fullyTraced).length, drafts = lines.length - canSave;
  box.replaceChildren(
    h("div", { class: "card-head" }, h("h3", {}, "Save"),
      h("div", { class: "row" },
        h("button", { class: "btn", disabled: !drafts, onclick: () => save(true) }, "Save as Draft"),
        h("button", { class: "btn primary", disabled: !canSave, onclick: () => save(false), "data-tip": "Ctrl+Enter" }, "Save Template", canSave > 1 ? `s (${canSave})` : ""))),
    h("div", { class: "card-body stack tight" },
      saved.length ? saved.map(s => h("div", { class: "saved-row" },
        h("div", { class: "row" }, h("span", { class: s.draft ? "tag warn" : "tag ok" }, s.draft ? "Draft" : "✓ Template"), h("span", { class: "tag" }, s.frame),
          s.waits.length ? h("span", { class: "tag" }, "Frozen") : null, h("span", {}, s.text)),
        s.tree ? h("details", { class: "tree" }, h("summary", {}, "Tree"), h("pre", { class: "out" }, s.tree)) : null)) :
        h("div", { class: "muted small" }, "Nothing saved yet. In this mock-up, saving only lists it here for this page view.")));
}

// ---------- keys (K1) ----------
$("#text").addEventListener("input", () => { clearTimeout(window.__t); window.__t = setTimeout(realign, 150); });
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

renderMeaning();
realign();
