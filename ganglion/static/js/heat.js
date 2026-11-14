// Colour scales for heatmaps (09-look). Sequential amber for magnitude
// (surprise in bits), diverging blue <-> amber for signed values.

export const SEQ_STEPS = 9;

// seq returns the CSS colour and matching ink for a value in [0, 1].
export function seq(t) {
  const i = Math.max(0, Math.min(SEQ_STEPS - 1, Math.round(t * (SEQ_STEPS - 1))));
  return { bg: `var(--seq-${i})`, ink: seqInk(i), step: i };
}

// In dark mode the ramp runs the other way in lightness, so ink flips too.
export function seqInk(step) {
  const dark = document.documentElement.getAttribute("data-theme") === "dark" ||
    (!document.documentElement.hasAttribute("data-theme") && matchMedia("(prefers-color-scheme: dark)").matches);
  if (dark) return step >= 6 ? "#1f1b17" : "var(--ink)";
  return step >= 5 ? "#fbf8f3" : "var(--ink)";
}

const DIV = ["--div-n4", "--div-n3", "--div-n2", "--div-n1", "--div-0", "--div-p1", "--div-p2", "--div-p3", "--div-p4"];
// div maps [-1, 1] to the diverging ramp.
export function div(v) {
  const i = Math.max(0, Math.min(8, Math.round((v + 1) * 4)));
  return `var(${DIV[i]})`;
}

// bitsScale maps surprisal bits to [0, 1]; 12 bits and above is the top step.
export const bitsScale = b => Math.min(1, Math.max(0, b / 12));

// paintRamp fills a .ramp element with the sequential steps (a legend).
export function paintRamp(el, kind = "seq") {
  el.innerHTML = "";
  const n = kind === "seq" ? SEQ_STEPS : 9;
  for (let i = 0; i < n; i++) {
    const s = document.createElement("span");
    s.style.background = kind === "seq" ? `var(--seq-${i})` : `var(${DIV[i]})`;
    el.append(s);
  }
}
