// Ettin playground (08/K1, D41): talks to the server over one compressed
// websocket; the server forwards to the model worker.
import { seq, bitsScale, paintRamp } from "./heat.js";
import { showTip, hideTip } from "./app.js";

const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];
const el = (tag, cls, text) => { const e = document.createElement(tag); if (cls) e.className = cls; if (text != null) e.textContent = text; return e; };

const root = $(".ettin");
const text = $("#text");
const results = $("#results");
const statusText = $("#status-text");
const wsDot = $("#ws-dot");

function load(k, d) { try { return localStorage.getItem(k) || d; } catch (e) { return d; } }
function save(k, v) { try { localStorage.setItem(k, v); } catch (e) {} }

let mode = root.dataset.mode || load("ettin.mode", "surprise");
let model = load("ettin.model", "17m");
const MODELS = { "17m": { label: "17M", series: 1 }, "32m": { label: "32M", series: 2 } };

// --- socket -----------------------------------------------------------------
let ws, nextId = 1, pending = new Map(), workerUp = false, indexJob = null;
function connect() {
  ws = new WebSocket((location.protocol === "https:" ? "wss://" : "ws://") + location.host + "/ws");
  ws.onopen = () => { wsDot.className = "dot live"; run(); };
  ws.onclose = () => {
    wsDot.className = "dot down"; statusText.textContent = "Disconnected; retrying…";
    for (const [, p] of pending) p.reject(new Error("disconnected"));
    pending.clear();
    setTimeout(connect, 1500);
  };
  ws.onmessage = e => {
    const m = JSON.parse(e.data);
    if (m.push === "worker") { onWorker(m); return; }
    const p = pending.get(m.id);
    if (!p) return;
    pending.delete(m.id);
    m.ok ? p.resolve(m) : p.reject(new Error(m.error || "failed"));
  };
}
function call(op, args) {
  return new Promise((resolve, reject) => {
    if (!ws || ws.readyState !== 1) return reject(new Error("not connected"));
    const id = nextId++;
    pending.set(id, { resolve, reject });
    ws.send(JSON.stringify({ id, op, ...args }));
  });
}
function onWorker(m) {
  workerUp = m.up;
  indexJob = m.index;
  const dot = document.getElementById("worker-dot");
  if (dot) dot.className = "dot " + (m.up ? "live" : "down");
  if (!m.up) { statusText.textContent = "Model worker not connected. Start it and this page will pick it up."; wsDot.className = "dot down"; }
  else { wsDot.className = "dot live"; if (statusText.textContent.startsWith("Model worker") || statusText.textContent.startsWith("Connecting")) { statusText.textContent = "Ready"; run(); } }
  renderIndexJob();
}

// --- controls -----------------------------------------------------------------
function setMode(m) {
  mode = m; save("ettin.mode", m);
  $$("#modes button").forEach(b => b.setAttribute("aria-selected", String(b.dataset.mode === m)));
  $("#index-panel").hidden = m !== "nearest";
  results.innerHTML = "";
  if (m === "nearest") refreshIndex();
  run();
}
function setModel(m) {
  model = m; save("ettin.model", m);
  $$("#model button").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.model === m)));
  if (mode === "nearest") refreshIndex();
  run();
}
$$("#modes button").forEach(b => b.addEventListener("click", () => setMode(b.dataset.mode)));
$$("#model button").forEach(b => b.addEventListener("click", () => setModel(b.dataset.model)));
$$("[data-sample]").forEach(b => b.addEventListener("click", () => {
  text.value = b.dataset.sample;
  if (b.dataset.sample.includes("[MASK]")) setMode("fill");
  else if (b.textContent.startsWith("Similarity")) setMode("similarity");
  else run();
}));
$("#insert-mask").addEventListener("click", () => {
  const [a, b] = [text.selectionStart, text.selectionEnd];
  text.setRangeText("[MASK]", a, b, "end");
  text.focus();
  if (mode !== "fill") setMode("fill"); else run();
});
let timer;
text.addEventListener("input", () => { clearTimeout(timer); timer = setTimeout(run, 260); });

const models = () => (model === "both" ? ["17m", "32m"] : [model]);

// --- running ------------------------------------------------------------------
let seqNo = 0;
async function run() {
  if (!ws || ws.readyState !== 1 || !workerUp) return;
  const t = text.value;
  const my = ++seqNo;
  if (!t.trim()) { results.innerHTML = ""; statusText.textContent = "Ready"; return; }
  statusText.textContent = "Running…";
  const t0 = performance.now();
  try {
    const ms = models();
    let outs;
    if (mode === "surprise") outs = await Promise.all(ms.map(m => call("surprise", { model: m, text: t })));
    else if (mode === "tokens") outs = await Promise.all(ms.map(m => call("tokenize", { model: m, text: t })));
    else if (mode === "fill") outs = await Promise.all(ms.map(m => call("fill_mask", { model: m, text: t, k: 8 })));
    else if (mode === "similarity") {
      const texts = t.split("\n").map(s => s.trim()).filter(Boolean).slice(0, 8);
      outs = await Promise.all(ms.map(m => call("similarity", { model: m, texts })));
      outs.texts = texts;
    } else if (mode === "nearest") outs = await Promise.all(ms.map(m => call("nearest", { model: m, field: "command", text: t, k: 12 })));
    if (my !== seqNo) return;
    render(ms, outs);
    const worker = outs.map(o => o.ms).filter(Boolean);
    statusText.textContent = `${Math.round(performance.now() - t0)} ms round trip · ${worker.map(x => Math.round(x) + " ms").join(" / ")} in the model · ` +
      outs.map(o => `${o.model.split("/")[1]} @ ${o.revision.slice(0, 7)}`).join(", ");
  } catch (e) {
    if (my === seqNo) statusText.textContent = e.message;
  }
}

function pane(m, outs, i) {
  const p = el("section", "card pane");
  const head = el("div", "card-head");
  const name = el("h3");
  const sw = el("span", "series s" + MODELS[m].series);
  name.append(sw, document.createTextNode(" Ettin " + MODELS[m].label));
  head.append(name);
  p.append(head);
  const body = el("div", "card-body stack");
  p.append(body);
  return [p, head, body];
}

function render(ms, outs) {
  results.innerHTML = "";
  results.classList.toggle("split", ms.length > 1);
  ms.forEach((m, i) => {
    const [p, head, body] = pane(m, outs, i);
    const o = outs[i];
    if (mode === "surprise") renderSurprise(o, head, body);
    else if (mode === "tokens") renderTokens(o, head, body);
    else if (mode === "fill") renderFill(o, head, body, m);
    else if (mode === "similarity") renderSim(o, head, body, outs.texts);
    else if (mode === "nearest") renderNearest(o, head, body);
    results.append(p);
  });
}

function tokSpan(t) {
  const s = el("span", "tok", t.text);
  if (t.text.trim() === "") s.classList.add("sp");
  return s;
}

function renderSurprise(o, head, body) {
  head.append(el("span", "muted label num", `${o.count} tokens · mean ${o.mean_bits.toFixed(2)} bits`));
  const box = el("div", "tokens");
  for (const t of o.tokens) {
    if (t.special) continue;
    const s = tokSpan(t);
    const c = seq(bitsScale(t.bits));
    s.style.background = c.bg; s.style.color = c.ink;
    s.dataset.info = JSON.stringify({ t: t.text, bits: t.bits, p: t.p, best: t.best, best_p: t.best_p });
    box.append(s);
  }
  const legend = el("div", "legend row");
  const ramp = el("div", "ramp grow");
  paintRamp(ramp);
  legend.append(el("span", "muted small", "0 bits"), ramp, el("span", "muted small", "12+ bits"));
  const top = [...o.tokens].filter(t => !t.special).sort((a, b) => b.bits - a.bits).slice(0, 6);
  const list = el("div", "row");
  list.append(el("span", "label", "Most Surprising"));
  for (const t of top) {
    const tag = el("span", "tag mono", `${JSON.stringify(t.text)} ${t.bits.toFixed(1)}`);
    tag.dataset.tip = `Ettin expected ${JSON.stringify(t.best)} (${(t.best_p * 100).toFixed(1)}%)`;
    list.append(tag);
  }
  body.append(box, legend, list);
}

function renderTokens(o, head, body) {
  const chars = text.value.length;
  head.append(el("span", "muted label num", `${o.count} tokens · ${(chars / Math.max(1, o.count - 2)).toFixed(2)} chars per token`));
  const box = el("div", "tokens chips");
  o.tokens.forEach((t, i) => {
    const s = tokSpan(t);
    s.classList.add(i % 2 ? "odd" : "even");
    if (t.special) s.classList.add("special");
    s.dataset.tip = `id ${t.id}${t.special ? " · special" : ""} · ${JSON.stringify(t.text)}`;
    box.append(s);
  });
  body.append(box);
}

function renderFill(o, head, body, m) {
  if (!o.masks.length) { body.append(el("p", "muted", o.note || "No [MASK] in the text.")); return; }
  const grid = el("div", "grid fills");
  o.masks.forEach((mk, n) => {
    const c = el("div", "stack");
    c.append(el("div", "label", `Mask ${n + 1} · ${mk.entropy_bits.toFixed(2)} bits of uncertainty`));
    const bars = el("div", "bars");
    const maxp = mk.top[0].p;
    for (const t of mk.top) {
      const b = el("div", "bar");
      b.dataset.tip = `${JSON.stringify(t.text)} · ${(t.p * 100).toFixed(2)}% · id ${t.id}`;
      const name = el("span", "name", t.text);
      const track = el("span", "track");
      const fill = el("span", "fill s" + MODELS[m].series);
      fill.style.width = (t.p / maxp * 100).toFixed(1) + "%";
      track.append(fill);
      b.append(name, track, el("span", "val", (t.p * 100).toFixed(1) + "%"));
      bars.append(b);
    }
    c.append(bars);
    grid.append(c);
  });
  body.append(grid);
}

function renderSim(o, head, body, texts) {
  if (texts.length < 2) { body.append(el("p", "muted", "Put two or more texts on separate lines.")); return; }
  const n = texts.length;
  const tbl = el("table", "simtable");
  const thead = el("tr");
  thead.append(el("th"));
  texts.forEach((t, j) => { const th = el("th", "mono small", String(j + 1)); th.dataset.tip = t; thead.append(th); });
  tbl.append(thead);
  // scale off-diagonal values so differences show; mean-pooled embeddings are all fairly similar
  const off = [];
  for (let i = 0; i < n; i++) for (let j = 0; j < n; j++) if (i !== j) off.push(o.sim[i][j]);
  const lo = Math.min(...off), hi = Math.max(...off);
  texts.forEach((t, i) => {
    const tr = el("tr");
    const th = el("th", "mono small rowhead", `${i + 1}  ${t.length > 48 ? t.slice(0, 47) + "…" : t}`);
    th.dataset.tip = t;
    tr.append(th);
    for (let j = 0; j < n; j++) {
      const v = o.sim[i][j];
      const td = el("td", "num", v.toFixed(2));
      if (i !== j) {
        const c = seq(hi > lo ? (v - lo) / (hi - lo) : 0.5);
        td.style.background = c.bg; td.style.color = c.ink;
      } else td.className = "num diag";
      td.dataset.tip = `${texts[i]}  ↔  ${texts[j]}: ${v.toFixed(4)}`;
      tr.append(td);
    }
    tbl.append(tr);
  });
  head.append(el("span", "muted label", `colour spans ${lo.toFixed(2)}–${hi.toFixed(2)}`));
  const wrap = el("div", "table-wrap");
  wrap.append(tbl);
  body.append(wrap);
}

function renderNearest(o, head, body) {
  if (!o.ready) { body.append(el("p", "muted", "No index for this model yet. Build one above.")); return; }
  const list = el("div", "nearest");
  for (const it of o.items) {
    const a = el("a", "near");
    a.href = "/events/" + it.hash;
    const sc = el("span", "score num", it.score.toFixed(3));
    const fill = el("span", "fill");
    fill.style.width = (Math.max(0, it.score) * 100).toFixed(1) + "%";
    const track = el("span", "track"); track.append(fill);
    a.append(sc, track, el("span", "cmd one", it.text));
    list.append(a);
  }
  body.append(list);
}

// token tooltips (surprise)
document.addEventListener("pointerover", e => {
  const t = e.target.closest(".tok[data-info]");
  if (!t) return;
  const d = JSON.parse(t.dataset.info);
  const box = el("div");
  box.append(el("div", "m", JSON.stringify(d.t)), el("div", null, `${d.bits.toFixed(2)} bits · p ${(d.p * 100).toFixed(2)}%`),
    el("div", null, `Ettin's guess: ${JSON.stringify(d.best)} (${(d.best_p * 100).toFixed(1)}%)`));
  const r = t.getBoundingClientRect();
  showTip(t, box, r.left + r.width / 2, r.top);
});
document.addEventListener("pointerout", e => { if (e.target.closest(".tok[data-info]")) hideTip(); });

// --- index --------------------------------------------------------------------
const idxModel = () => (model === "both" ? "17m" : model);
async function refreshIndex() {
  try {
    const r = await call("index_status", { model: idxModel(), field: "command" });
    $("#index-info").textContent = r.ready ? `${r.size.toLocaleString()} commands indexed with ${idxModel().toUpperCase()}` : `Not built for ${idxModel().toUpperCase()}`;
    $("#index-build").textContent = r.ready ? "Rebuild" : "Build Index";
  } catch (e) { $("#index-info").textContent = e.message; }
}
$("#index-build").addEventListener("click", async () => {
  $("#index-build").disabled = true;
  try { await call("index_build", { model: idxModel() }); } catch (e) { $("#index-info").textContent = e.message; }
  $("#index-build").disabled = false;
  refreshIndex();
  run();
});
function renderIndexJob() {
  const bar = $("#index-progress");
  if (!bar) return;
  if (indexJob && indexJob.Total) {
    bar.hidden = false;
    $("i", bar).style.width = (100 * indexJob.Done / indexJob.Total).toFixed(1) + "%";
    $("#index-info").textContent = `Building with ${indexJob.Model.toUpperCase()}: ${indexJob.Done.toLocaleString()} of ${indexJob.Total.toLocaleString()}`;
    $("#index-build").disabled = true;
  } else { bar.hidden = true; $("#index-build").disabled = false; }
}

// --- start --------------------------------------------------------------------
$$("#modes button").forEach(b => b.setAttribute("aria-selected", String(b.dataset.mode === mode)));
$$("#model button").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.model === model)));
$("#index-panel").hidden = mode !== "nearest";
document.addEventListener("themechange", run);
connect();
