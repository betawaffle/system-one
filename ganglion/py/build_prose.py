"""Build a prose sentence dataset from ganglion's utterances (design/11).

Usage: python -I build_prose.py [--data DIR] [--name prose-v1] [--cap-other N]

Reads utterances (index.db + blobs), keeps the kinds that are someone's own
prose, strips Markdown structure that isn't prose, splits into sentences and
filters them with simple, computable rules. The rules only reject what clearly
isn't a sentence; judging *quality* beyond that is a model's job, later.

Output in <data>/datasets/<name>/: train.jsonl, test.jsonl (one sentence per
line with provenance: utterance hash, speaker, kind, source, model), and
stats.json. Splits follow the utterance's sighting split (by session or
document); a sentence seen in train never appears in test.

Our own text (human, claude, subagent) is kept in full. Other models' and
third-party text is capped per speaker/kind (--cap-other) so it can't drown ours.
"""
import argparse
import collections
import json
import os
import random
import re
import sqlite3
import warnings

warnings.filterwarnings("ignore", category=SyntaxWarning)
import pysbd  # noqa: E402

PROSE_KINDS = {"typed", "pasted", "reply", "thinking", "summary", "document", "commit", "issue", "prompt"}
OURS = {"human", "claude", "subagent"}

FENCE = re.compile(r"^(```|~~~).*?^\1\s*$", re.S | re.M)
LINK = re.compile(r"\[([^\]]+)\]\([^)]+\)")
EMPH = re.compile(r"(\*\*|__)(.+?)\1")
LIST = re.compile(r"^\s*(?:[-*+]|\d+[.)]|[A-Z]\d+[a-z]?\.)\s+")
URL = re.compile(r"https?://\S+")


def prose_blocks(text, speaker):
    """Paragraph-ish blocks of prose from Markdown-ish text."""
    text = FENCE.sub("\n", text)
    out = []
    for para in re.split(r"\n\s*\n", text):
        lines = []
        for line in para.split("\n"):
            s = line.strip()
            if not s:
                continue
            if s.startswith(">"):
                # quoted text is someone else's words (often mine, quoted back to me)
                continue
            if s.startswith(("#", "|", "<", "$ ", "    ")) or line.startswith("    ") or line.startswith("\t"):
                continue
            s = LIST.sub("", s)
            lines.append(s)
        if lines:
            raw = [l for l in para.split("\n") if l.strip() and not l.strip().startswith(">")]
            if any(LIST.match(l) for l in raw):
                # list items are separate units; a wrapped item's continuation lines join it
                items = []
                for l in raw:
                    s = l.strip()
                    if s.startswith(("#", "|", "<", "$ ")):
                        continue
                    if LIST.match(l) or not items:
                        items.append(LIST.sub("", s))
                    else:
                        items[-1] += " " + s
                out.extend(items)
            else:
                out.append(" ".join(lines))
    return out


def clean(sent):
    s = LINK.sub(r"\1", sent)
    s = EMPH.sub(r"\2", s)
    return re.sub(r"\s+", " ", s).strip()


def is_sentence(s):
    if not (20 <= len(s) <= 300):
        return False
    words = s.split()
    if not (4 <= len(words) <= 80):
        return False
    if URL.search(s) and len(URL.sub("", s).split()) < 4:
        return False
    nonspace = [c for c in s if not c.isspace()]
    letters = sum(c.isalpha() for c in nonspace)
    if letters / len(nonspace) < 0.65:
        return False
    plain = [w for w in words if re.fullmatch(r"[A-Za-z][a-z']*[,.;:!?]?", w)]
    if len(plain) / len(words) < 0.5:  # mostly identifiers, paths or code
        return False
    if s.count("`") > 6 or s.count("=") > 2 or s.count("{") or s.count("(") > 3:
        return False
    if not re.search(r"[a-z]", s) or s[0] in "-*/\\|=;:,.)]}":
        return False
    if not re.search(r"[.!?][\"')\]`*]*$", s):  # a sentence ends like one
        return False
    return True


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--data", default=os.path.expanduser("~/.local/share/ganglion"))
    ap.add_argument("--name", default="prose-v1")
    ap.add_argument("--cap-other", type=int, default=150000, help="max sentences per other speaker/kind")
    ap.add_argument("--seed", type=int, default=1)
    a = ap.parse_args()
    rng = random.Random(a.seed)
    seg = pysbd.Segmenter(language="en", clean=False)
    db = sqlite3.connect(f"file:{a.data}/index.db?mode=ro", uri=True)
    blobs = sqlite3.connect(f"file:{a.data}/blobs.db?mode=ro", uri=True)

    def blob(h):
        r = blobs.execute("SELECT data FROM blobs WHERE hash = ?", (h,)).fetchone()
        if r:
            return r[0]
        return open(os.path.join(a.data, "blobs", h[:2], h), "rb").read()

    rows = db.execute("""SELECT u.hash, u.speaker, u.kind, COALESCE(u.model, ''),
            GROUP_CONCAT(DISTINCT s.split), MIN(s.source)
        FROM utterances u JOIN usources s ON s.utterance = u.id
        WHERE u.speaker != 'harness' GROUP BY u.id""").fetchall()
    by_group = collections.defaultdict(list)  # (speaker, kind) -> [(sentence, split, prov)]
    seen_utt = collections.Counter()
    for h, speaker, kind, model, splits, source in rows:
        if kind not in PROSE_KINDS:
            continue
        u = json.loads(blob(h))
        split = "train" if "train" in splits.split(",") else "test"
        n = 0
        for block in prose_blocks(u["text"], speaker):
            for raw in seg.segment(block):
                s = clean(raw)
                if is_sentence(s):
                    by_group[(speaker, kind)].append((s, split, {"utterance": h, "speaker": speaker, "kind": kind,
                                                                  "model": model or None, "source": source, "i": n}))
                    n += 1
        seen_utt[(speaker, kind)] += 1

    out_dir = os.path.join(a.data, "datasets", a.name)
    os.makedirs(out_dir, exist_ok=True)
    chosen = []
    stats = {"groups": {}, "rules": "see build_prose.py is_sentence/prose_blocks", "seed": a.seed, "cap_other": a.cap_other}
    for (speaker, kind), items in sorted(by_group.items()):
        total = len(items)
        if speaker not in OURS and total > a.cap_other:
            items = rng.sample(items, a.cap_other)
        chosen += items
        stats["groups"][f"{speaker}/{kind}"] = {"utterances": seen_utt[(speaker, kind)], "sentences": total, "kept": len(items)}

    # exact-duplicate sentences: keep one; train wins over test
    best = {}
    for s, split, prov in chosen:
        if s not in best or (split == "train" and best[s][0] == "test"):
            best[s] = (split, prov)
    counts = collections.Counter()
    with open(os.path.join(out_dir, "train.jsonl"), "w") as tr, open(os.path.join(out_dir, "test.jsonl"), "w") as te:
        for s, (split, prov) in best.items():
            (tr if split == "train" else te).write(json.dumps({"text": s, **prov}, ensure_ascii=False) + "\n")
            counts[(split, prov["speaker"])] += 1
    stats["written"] = {f"{k[0]}/{k[1]}": v for k, v in sorted(counts.items())}
    stats["duplicates_dropped"] = len(chosen) - len(best)
    json.dump(stats, open(os.path.join(out_dir, "stats.json"), "w"), indent=2)
    print(json.dumps(stats, indent=2))


if __name__ == "__main__":
    main()
