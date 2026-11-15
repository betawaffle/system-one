"""Build a prose sentence dataset from ganglion's utterances (design/11).

Usage: python -I build_prose.py [--data DIR] [--name prose-v2] [--cap-other N] [--jobs N]

Reads utterances (index.db + blobs), keeps the kinds that are someone's own
prose, strips Markdown structure that isn't prose, splits into sentences and
filters them with simple, computable rules. The rules only reject what clearly
isn't a sentence; judging *quality* beyond that is a model's job, later.

Who a sentence belongs to (11/Q1–Q4):
  - Blacklisted utterances (<data>/blacklist.jsonl; Claude's own output only)
    contribute nothing.
  - A sentence of ours (human, claude, subagent) that also appears, normalised,
    in another source is a quote, not ours: dropped from ours. Between the two of
    us, the earlier speaker keeps it.
  - A sentence of ours that shares most of its 4-word shingles with one sentence
    from another source is a paraphrased quote: dropped from ours too.
  Dropped sentences go to quotes.jsonl with what they matched, for review.

Output in <data>/datasets/<name>/: train.jsonl, test.jsonl (one sentence per
line, with provenance), quotes.jsonl, stats.json. Splits follow the utterance's
sighting split; a sentence seen in train never appears in test. Our own text is
kept in full; other speakers are capped per speaker/kind (--cap-other).

Sentences are cached per utterance hash and rules version in
<data>/datasets/.cache/sentences-<RULES>.sqlite, so rebuilds only segment new text.
"""
import argparse
import collections
import hashlib
import json
import multiprocessing as mp
import os
import random
import re
import sqlite3
import time
import warnings

warnings.filterwarnings("ignore", category=SyntaxWarning)

RULES = "r3"  # bump when prose_blocks / clean / is_sentence change (r3: reject missing-space joins)
PROSE_KINDS = {"typed", "pasted", "reply", "thinking", "summary", "document", "commit", "issue", "prompt"}
OURS = {"human", "claude", "subagent"}

FENCE = re.compile(r"^(```|~~~).*?^\1\s*$", re.S | re.M)
LINK = re.compile(r"\[([^\]]+)\]\([^)]+\)")
EMPH = re.compile(r"(\*\*|__)(.+?)\1")
LIST = re.compile(r"^\s*(?:[-*+]|\d+[.)]|[A-Z]\d+[a-z]?\.)\s+")
URL = re.compile(r"https?://\S+")
JOINED = re.compile(r"[a-z]{2}[.!?][A-Z][a-z]")


def prose_blocks(text):
    """Paragraph-ish blocks of prose from Markdown-ish text."""
    text = FENCE.sub("\n", text)
    out = []
    for para in re.split(r"\n\s*\n", text):
        raw = [l for l in para.split("\n") if l.strip() and not l.strip().startswith(">")]
        keep = lambda l: not (l.strip().startswith(("#", "|", "<", "$ ")) or l.startswith(("    ", "\t")))
        raw = [l for l in raw if keep(l)]
        if not raw:
            continue
        if any(LIST.match(l) for l in raw):
            # list items are separate units; a wrapped item's continuation lines join it
            items = []
            for l in raw:
                if LIST.match(l) or not items:
                    items.append(LIST.sub("", l.strip()))
                else:
                    items[-1] += " " + l.strip()
            out.extend(items)
        else:
            out.append(" ".join(l.strip() for l in raw))
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
    if sum(c.isalpha() for c in nonspace) / len(nonspace) < 0.65:
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
    if JOINED.search(s):  # "word.Next": two sentences run together without a space (user, 11/R2)
        return False
    return True


_seg = None


def sentences_of(text):
    global _seg
    if _seg is None:
        import pysbd
        _seg = pysbd.Segmenter(language="en", clean=False)
    out = []
    for block in prose_blocks(text):
        for raw in _seg.segment(block):
            s = clean(raw)
            if is_sentence(s):
                out.append(s)
    return out


def _work(item):
    h, text = item
    return h, sentences_of(text)


def norm(s):
    return " ".join(re.sub(r"[^a-z0-9]+", " ", s.lower()).split())


def shingles(n_text, k=4):
    w = n_text.split()
    return {hash(" ".join(w[i:i + k])) for i in range(len(w) - k + 1)}


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--data", default=os.path.expanduser("~/.local/share/ganglion"))
    ap.add_argument("--name", default="prose-v2")
    ap.add_argument("--cap-other", type=int, default=150000, help="max sentences per other speaker/kind")
    ap.add_argument("--near", type=float, default=0.75, help="shingle share that makes a paraphrased quote")
    ap.add_argument("--min-near-words", type=int, default=10, help="shorter sentences are never treated as paraphrases")
    ap.add_argument("--min-quote-words", type=int, default=8, help="shorter sentences are never treated as quotes")
    ap.add_argument("--jobs", type=int, default=max(1, (os.cpu_count() or 4) - 4))
    ap.add_argument("--seed", type=int, default=1)
    ap.add_argument("--ignore-blacklist", action="store_true", help="for testing what quote detection alone catches")
    a = ap.parse_args()
    t0 = time.time()
    rng = random.Random(a.seed)
    db = sqlite3.connect(f"file:{a.data}/index.db?mode=ro", uri=True)
    # read from the pool's feeder thread too while segmenting; read-only, one reader at a time
    blobs = sqlite3.connect(f"file:{a.data}/blobs.db?mode=ro", uri=True, check_same_thread=False)

    def blob(h):
        r = blobs.execute("SELECT data FROM blobs WHERE hash = ?", (h,)).fetchone()
        return r[0] if r else open(os.path.join(a.data, "blobs", h[:2], h), "rb").read()

    black = {}
    bl_path = os.path.join(a.data, "blacklist.jsonl")
    if os.path.exists(bl_path) and not a.ignore_blacklist:
        for line in open(bl_path):
            e = json.loads(line)
            black[e["hash"]] = e

    rows = db.execute("""SELECT u.hash, u.speaker, u.kind, COALESCE(u.model, ''),
            GROUP_CONCAT(DISTINCT s.split), MIN(s.source), COALESCE(MIN(s.at), ''),
            (SELECT unit FROM usources f WHERE f.utterance = u.id ORDER BY f.rowid LIMIT 1),
            (SELECT turn FROM usources f WHERE f.utterance = u.id ORDER BY f.rowid LIMIT 1)
        FROM utterances u JOIN usources s ON s.utterance = u.id
        WHERE u.speaker != 'harness' GROUP BY u.id""").fetchall()
    rows = [r for r in rows if r[2] in PROSE_KINDS]
    blacklisted = [r for r in rows if r[0] in black]
    rows = [r for r in rows if r[0] not in black]

    # sentences, cached by utterance hash
    cache_dir = os.path.join(a.data, "datasets", ".cache")
    os.makedirs(cache_dir, exist_ok=True)
    cache = sqlite3.connect(os.path.join(cache_dir, f"sentences-{RULES}.sqlite"))
    cache.execute("CREATE TABLE IF NOT EXISTS s (hash TEXT PRIMARY KEY, sents TEXT NOT NULL)")
    have = {h for (h,) in cache.execute("SELECT hash FROM s")}
    todo = [r[0] for r in rows if r[0] not in have]
    print(json.dumps({"utterances": len(rows), "cached": len(rows) - len(todo), "to_segment": len(todo), "jobs": a.jobs}), flush=True)
    if todo:
        def items():
            for h in todo:
                yield h, json.loads(blob(h))["text"]
        with mp.Pool(a.jobs) as pool:
            for n, (h, sents) in enumerate(pool.imap_unordered(_work, items(), chunksize=16)):
                cache.execute("INSERT OR REPLACE INTO s VALUES (?, ?)", (h, json.dumps(sents, ensure_ascii=False)))
                if n % 5000 == 0:
                    cache.commit()
        cache.commit()
    sents = {h: json.loads(j) for h, j in cache.execute("SELECT hash, sents FROM s")}

    # every sentence, before caps: (text, speaker, kind, model, split, source, at, utterance, i, unit, turn)
    allsent = []
    for h, speaker, kind, model, splits, source, at, unit, turn in rows:
        split = "train" if "train" in splits.split(",") else "test"
        for i, s in enumerate(sents.get(h, [])):
            allsent.append((s, speaker, kind, model or None, split, source, at, h, i, unit, turn))

    def quoted_back(x, y):
        """True when y (outside) quotes x (ours): same conversation, x said first."""
        return x[5] == y[5] and x[9] == y[9] and (x[10], x[8]) < (y[10], y[8])

    # exact quotes: normalised text -> earliest occurrence per speaker
    first = {}
    for k, x in enumerate(allsent):
        key = norm(x[0])
        d = first.setdefault(key, {})
        if x[1] not in d or (x[6] and x[6] < allsent[d[x[1]]][6]):
            d[x[1]] = k
    quotes = []
    drop = set()
    for k, x in enumerate(allsent):
        if x[1] not in OURS or len(x[0].split()) < a.min_quote_words:
            continue  # short common sentences ("Am I missing something?") match by coincidence
        d = first[norm(x[0])]
        others = [sp for sp in d if sp != x[1]]
        if not others:
            continue
        outside = [sp for sp in others if sp not in OURS]
        for sp in list(outside):
            if quoted_back(x, allsent[d[sp]]):  # the other speaker quoted us: theirs is the quote
                drop.add(d[sp])
                outside.remove(sp)
        earlier = [sp for sp in others if sp in OURS and allsent[d[sp]][6] and x[6] and allsent[d[sp]][6] < x[6]]
        src = outside[0] if outside else (earlier[0] if earlier else None)
        if src:
            drop.add(k)
            m = allsent[d[src]]
            quotes.append({"text": x[0], "speaker": x[1], "kind": x[2], "utterance": x[7], "match": "exact",
                           "matched_text": m[0], "matched_speaker": m[1], "matched_source": m[5], "matched_utterance": m[7]})

    # paraphrased quotes: ours vs everything outside, by shared 4-word shingles
    ours_sh = {}
    index = collections.defaultdict(list)
    for k, x in enumerate(allsent):
        if x[1] in OURS and k not in drop and len(x[0].split()) >= a.min_near_words:
            sh = shingles(norm(x[0]))
            if len(sh) >= 3:
                ours_sh[k] = sh
                for g in sh:
                    index[g].append(k)
    best = {}
    for j, y in enumerate(allsent):
        if y[1] in OURS:
            continue
        hits = collections.Counter()
        for g in shingles(norm(y[0])):
            for k in index.get(g, ()):
                hits[k] += 1
        for k, c in hits.items():
            share = c / len(ours_sh[k])
            if share < a.near:
                continue
            if quoted_back(allsent[k], y):  # they quoted us: drop their sentence, keep ours
                drop.add(j)
                continue
            if share > best.get(k, (0,))[0]:
                best[k] = (share, j)
    for k, (share, j) in best.items():
        x, m = allsent[k], allsent[j]
        drop.add(k)
        quotes.append({"text": x[0], "speaker": x[1], "kind": x[2], "utterance": x[7], "match": f"near {share:.2f}",
                       "matched_text": m[0], "matched_speaker": m[1], "matched_source": m[5], "matched_utterance": m[7]})

    # caps, duplicates, write
    groups = collections.defaultdict(list)
    for k, x in enumerate(allsent):
        if k not in drop:
            groups[(x[1], x[2])].append(x)
    out_dir = os.path.join(a.data, "datasets", a.name)
    os.makedirs(out_dir, exist_ok=True)
    stats = {"rules": RULES, "seed": a.seed, "cap_other": a.cap_other, "near": a.near, "min_quote_words": a.min_quote_words, "min_near_words": a.min_near_words, "groups": {},
             "blacklisted_utterances": len(blacklisted),
             "quotes_dropped": collections.Counter(q["match"].split()[0] + "/" + q["speaker"] for q in quotes)}
    chosen = []
    for (speaker, kind), items in sorted(groups.items()):
        total = len(items)
        if speaker not in OURS and total > a.cap_other:
            items = rng.sample(items, a.cap_other)
        chosen += items
        stats["groups"][f"{speaker}/{kind}"] = {"sentences": total, "kept": len(items)}
    keep = {}
    for x in chosen:
        if x[0] not in keep or (x[4] == "train" and keep[x[0]][4] == "test"):
            keep[x[0]] = x
    counts = collections.Counter()
    with open(os.path.join(out_dir, "train.jsonl"), "w") as tr, open(os.path.join(out_dir, "test.jsonl"), "w") as te:
        for s, x in keep.items():
            rec = {"text": s, "utterance": x[7], "speaker": x[1], "kind": x[2], "model": x[3], "source": x[5], "i": x[8]}
            (tr if x[4] == "train" else te).write(json.dumps(rec, ensure_ascii=False) + "\n")
            counts[f"{x[4]}/{x[1]}"] += 1
    with open(os.path.join(out_dir, "quotes.jsonl"), "w") as qf:
        for q in quotes:
            qf.write(json.dumps(q, ensure_ascii=False) + "\n")
    stats["written"] = dict(sorted(counts.items()))
    stats["duplicates_dropped"] = len(chosen) - len(keep)
    stats["seconds"] = round(time.time() - t0)
    json.dump(stats, open(os.path.join(out_dir, "stats.json"), "w"), indent=2)
    print(json.dumps(stats), flush=True)


if __name__ == "__main__":
    main()
