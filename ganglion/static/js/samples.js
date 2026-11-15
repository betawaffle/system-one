// Rating samples (design/11 R1): buttons and keys save a label, then move on.
const cards = [...document.querySelectorAll(".sample[data-target]")];
let cur = -1;

function show(card) {
  const r = card.dataset.rating || "";
  card.querySelectorAll("[data-rate]").forEach(b => b.setAttribute("aria-pressed", String(b.dataset.rate === r)));
  card.classList.toggle("rated", !!r);
  card.dataset.ratedAs = r;
}

function focus(i) {
  if (!cards.length) return;
  cur = Math.max(0, Math.min(cards.length - 1, i));
  cards.forEach((c, j) => c.classList.toggle("current", j === cur));
  cards[cur].focus({ preventScroll: true });
  cards[cur].scrollIntoView({ block: "nearest", behavior: "smooth" });
}

async function rate(card, rating) {
  const prev = card.dataset.rating || "";
  if (rating === prev) rating = ""; // pressing the current rating again clears it
  card.dataset.rating = rating;
  show(card);
  try {
    const r = await fetch("/api/labels", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ target: card.dataset.target, rating, set: card.dataset.set, seed: +card.dataset.seed, context: JSON.parse(card.dataset.context || "{}") }),
    });
    if (!r.ok) throw new Error(await r.text());
    const j = await r.json();
    for (const k of ["good", "bad", "not-prose"]) {
      const el = document.getElementById("count-" + k);
      if (el) el.textContent = j.counts[k] || 0;
    }
  } catch (e) {
    card.dataset.rating = prev;
    show(card);
    card.classList.add("failed");
    setTimeout(() => card.classList.remove("failed"), 1500);
  }
}

cards.forEach((card, i) => {
  show(card);
  card.querySelectorAll("[data-rate]").forEach(b => b.addEventListener("click", () => { focus(i); rate(card, b.dataset.rate); }));
  card.addEventListener("click", e => { if (!e.target.closest("a, button")) focus(i); });
});

const keys = { "1": "good", "2": "bad", "3": "not-prose", "0": "" };
document.addEventListener("keydown", e => {
  if (e.metaKey || e.ctrlKey || e.altKey || e.target.closest("input, textarea, select")) return;
  if (e.key === "j") { e.preventDefault(); focus(cur + 1); }
  else if (e.key === "k") { e.preventDefault(); focus(cur - 1); }
  else if (e.key in keys) {
    if (cur < 0) focus(0);
    const card = cards[cur];
    if (keys[e.key] === "") { if (card.dataset.rating) rate(card, card.dataset.rating); }
    else rate(card, keys[e.key]);
    if (keys[e.key] !== "") setTimeout(() => focus(cur + 1), 120);
  }
});
if (cards.length) focus(0);
