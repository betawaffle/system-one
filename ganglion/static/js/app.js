// Shared page behaviour: theme switch, tooltips, bar widths, clickable rows,
// lazy output lines. No inline styles are allowed by the CSP, so sizes and
// colours are set through the CSSOM here.

const $ = (s, el = document) => el.querySelector(s);
const $$ = (s, el = document) => [...el.querySelectorAll(s)];

function store(k, v) {
  try { v === undefined ? localStorage.removeItem(k) : localStorage.setItem(k, v); } catch (e) {}
}

// theme
function setTheme(t) {
  if (t === "auto") { document.documentElement.removeAttribute("data-theme"); store("theme"); }
  else { document.documentElement.setAttribute("data-theme", t); store("theme", t); }
  $$("#theme button").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.themeSet === t)));
  document.dispatchEvent(new CustomEvent("themechange"));
}
const current = document.documentElement.getAttribute("data-theme") || "auto";
$$("#theme button").forEach(b => {
  b.setAttribute("aria-pressed", String(b.dataset.themeSet === current));
  b.addEventListener("click", () => setTheme(b.dataset.themeSet));
});

// tooltips: any [data-tip]
const tip = $("#tip");
let tipFor = null;
export function showTip(el, html, x, y) {
  tip.innerHTML = "";
  if (html instanceof Node) tip.append(html); else tip.textContent = html;
  tip.classList.add("on");
  const r = tip.getBoundingClientRect();
  let left = Math.min(window.innerWidth - r.width - 8, Math.max(8, x - r.width / 2));
  let top = y - r.height - 12;
  if (top < 8) top = y + 18;
  tip.style.left = left + "px";
  tip.style.top = top + "px";
}
export function hideTip() { tip.classList.remove("on"); tipFor = null; }
document.addEventListener("pointerover", e => {
  const el = e.target.closest("[data-tip]");
  if (!el || el === tipFor) return;
  tipFor = el;
  const r = el.getBoundingClientRect();
  showTip(el, el.dataset.tip, r.left + r.width / 2, r.top);
});
document.addEventListener("pointerout", e => {
  const el = e.target.closest("[data-tip]");
  if (el && !el.contains(e.relatedTarget)) hideTip();
});

// bars: data-w is a 0..1 share
export function sizeBars(root = document) {
  $$("[data-w]", root).forEach(el => { el.style.width = (Math.max(0, Math.min(1, +el.dataset.w)) * 100).toFixed(2) + "%"; });
  $$("[data-h]", root).forEach(el => { el.style.height = (Math.max(0, Math.min(1, +el.dataset.h)) * 100).toFixed(2) + "%"; });
}
sizeBars();

// rows that link somewhere
document.addEventListener("click", e => {
  const tr = e.target.closest("tr[data-href]");
  if (!tr || e.target.closest("a, button, input, select")) return;
  if (e.metaKey || e.ctrlKey) window.open(tr.dataset.href, "_blank");
  else location.href = tr.dataset.href;
});

// lazy output: a button with data-more loads the next lines of a field (08/N4)
document.addEventListener("click", async e => {
  const btn = e.target.closest("[data-more]");
  if (!btn) return;
  const pre = document.getElementById(btn.dataset.target);
  const from = +btn.dataset.from, all = btn.dataset.all === "1";
  btn.disabled = true;
  try {
    const n = all ? 2000 : 400;
    const r = await fetch(`/api/events/${btn.dataset.more}/lines?f=${btn.dataset.field}&from=${from}&n=${n}`);
    const j = await r.json();
    const frag = document.createDocumentFragment();
    for (const line of j.lines) {
      const s = document.createElement("span");
      s.className = "ln";
      s.textContent = line + "\n";
      frag.append(s);
    }
    pre.append(frag);
    const next = j.from + j.lines.length;
    const wrap = btn.closest(".fold");
    if (next >= j.total) { wrap.remove(); return; }
    $$("[data-more]", wrap).forEach(b => { b.dataset.from = next; b.disabled = false; });
    const left = $(".left", wrap);
    if (left) left.textContent = (j.total - next).toLocaleString() + " more lines";
  } catch (err) {
    btn.disabled = false;
  }
});

// wrap toggle for outputs
document.addEventListener("click", e => {
  const b = e.target.closest("[data-wrap]");
  if (!b) return;
  const pre = document.getElementById(b.dataset.wrap);
  const on = pre.classList.toggle("wrap");
  b.setAttribute("aria-pressed", String(on));
});

// copy buttons
document.addEventListener("click", async e => {
  const b = e.target.closest("[data-copy]");
  if (!b) return;
  try {
    await navigator.clipboard.writeText(b.dataset.copy);
    const t = b.textContent; b.textContent = "Copied"; setTimeout(() => (b.textContent = t), 1200);
  } catch (err) {}
});

// keyboard: "/" focuses search
document.addEventListener("keydown", e => {
  if (e.key === "/" && !e.target.closest("input, textarea, select")) {
    const q = $("input[name=q]");
    if (q) { e.preventDefault(); q.focus(); q.select(); }
  }
});

// filter selects submit on change
$$("form[data-autosubmit] select, form[data-autosubmit] input[type=checkbox]").forEach(el =>
  el.addEventListener("change", () => el.form.requestSubmit()));

export { $, $$ };
