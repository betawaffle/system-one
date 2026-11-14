import { seq, div, bitsScale, paintRamp } from "./heat.js";
import { showTip, hideTip } from "./app.js";

const $$ = (s) => [...document.querySelectorAll(s)];

function paint() {
  const cs = getComputedStyle(document.documentElement);
  $$("[data-var]").forEach(el => { el.style.background = `var(${el.dataset.var})`; });
  $$("[data-hex]").forEach(el => { el.textContent = cs.getPropertyValue(el.dataset.hex).trim(); });
  $$("[data-ramp]").forEach(el => paintRamp(el, el.dataset.ramp));
  tokens();
}

// A fixed sample: a Python traceback with made-up surprise values.
const SAMPLE = [["Trace", 0.4], ["back", 0.3], [" (", 0.2], ["most", 0.1], [" recent", 0.1], [" call", 0.2], [" last", 0.3], ["):", 1.2], ["\n  ", 0.5],
  ["File", 0.9], [" \"", 1.1], ["/test", 6.5], ["bed", 3.2], ["/src", 4.4], ["/parser", 9.8], [".py", 1.4], ["\",", 0.6], [" line", 0.4], [" 88", 7.7], ["\n", 0.3],
  ["Key", 5.1], ["Error", 2.3], [":", 0.5], [" '", 2.2], ["encoding", 11.6], ["'", 0.9]];

function tokens() {
  const el = document.getElementById("demo-tokens");
  if (!el) return;
  el.innerHTML = "";
  for (const [t, b] of SAMPLE) {
    const s = document.createElement("span");
    s.className = "tok";
    s.textContent = t;
    const c = seq(bitsScale(b));
    s.style.background = c.bg;
    s.style.color = c.ink;
    s.dataset.bits = b;
    el.append(s);
  }
}
document.addEventListener("pointerover", e => {
  const t = e.target.closest(".tok[data-bits]");
  if (!t) return;
  const r = t.getBoundingClientRect();
  showTip(t, `${JSON.stringify(t.textContent)} · ${(+t.dataset.bits).toFixed(1)} bits`, r.left + r.width / 2, r.top);
});
document.addEventListener("pointerout", e => { if (e.target.closest(".tok")) hideTip(); });

function slots() {
  const el = document.getElementById("demo-slots");
  if (!el) return;
  let x = 7;
  const rnd = () => ((x = (x * 16807) % 2147483647) / 2147483647);
  for (let r = 0; r < 8; r++) {
    for (let c = 0; c < 32; c++) {
      const v = Math.tanh((rnd() + rnd() + rnd() - 1.5) * 1.6 * (1 - r / 10));
      const i = document.createElement("i");
      i.style.background = div(v);
      i.dataset.tip = `slot ${r + 1}, dim ${c + 1}: ${v.toFixed(2)}`;
      el.append(i);
    }
  }
}

paint();
slots();
document.addEventListener("themechange", paint);
matchMedia("(prefers-color-scheme: dark)").addEventListener("change", paint);
