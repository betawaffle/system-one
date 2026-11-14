"""Embedding inversion: can an Ettin decoder rebuild text from an Ettin encoder's embedding?

Target embeddings are exactly what ganglion's nearest index uses (08/U4): Ettin
encoder 17M, mean-pooled over tokens, L2-normalised, centred on the corpus mean,
renormalised. A small projector turns one embedding into K soft tokens that
prefix the Ettin 150M decoder (LoRA); it learns to write the original text.

Run inside the ganglion-gpu image (train/run.sh):
  train/run.sh python3 train/invert.py --field command

Outputs go to /store/runs/invert-<field>-<time>/ : config.json (data, model
identities and hashes, hyperparameters, git commit, image), log.jsonl,
projector.safetensors + decoder/ (LoRA merged in), eval.jsonl, metrics.json.
"""
import argparse
import datetime
import difflib
import hashlib
import json
import math
import os
import random
import sqlite3
import subprocess
import time

import torch
import torch.nn as nn
import torch.nn.functional as F
from peft import LoraConfig, get_peft_model
from safetensors.torch import save_file
from transformers import AutoModel, AutoModelForCausalLM, AutoTokenizer

ENCODER = "jhu-clsp/ettin-encoder-17m"
DECODER = "jhu-clsp/ettin-decoder-150m"


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return "sha256:" + h.hexdigest()


def identity(repo):
    from huggingface_hub import snapshot_download
    path = snapshot_download(repo, allow_patterns=["*.json", "*.safetensors", "pytorch_model.bin"])
    files = {n: sha256_file(os.path.join(path, n)) for n in sorted(os.listdir(path))
             if n in ("pytorch_model.bin", "tokenizer.json", "config.json") or n.endswith(".safetensors")}
    return {"repo": repo, "revision": os.path.basename(path), "files": files}, path


def load_texts(store, field, max_chars):
    """Distinct texts per split. An event goes to test only if every sighting is test."""
    db = sqlite3.connect(f"file:{store}/index.db?mode=ro", uri=True)
    if field != "command":
        raise SystemExit("only --field command for now")
    rows = db.execute("""
        SELECT e.cmd, GROUP_CONCAT(DISTINCT s.split)
        FROM events e JOIN sources s ON s.event = e.id
        WHERE length(e.cmd) BETWEEN 2 AND ?
        GROUP BY e.id""", (max_chars,)).fetchall()
    split = {"train": set(), "test": set(), "local": set()}
    for text, splits in rows:
        ss = set(splits.split(","))
        if ss == {"test"}:
            split["test"].add(text)
        elif ss == {"local"}:
            split["local"].add(text)
        elif "train" in ss:
            split["train"].add(text)
    # a text seen in train must not count as held out
    split["test"] -= split["train"]
    split["local"] -= split["train"]
    return {k: sorted(v) for k, v in split.items()}


class Embedder:
    """Mirror of the worker's Model.embed (py/worker.py), on GPU."""

    def __init__(self, path, device):
        self.tok = AutoTokenizer.from_pretrained(path)
        self.model = AutoModel.from_pretrained(path).to(device).eval()
        self.device = device

    @torch.inference_mode()
    def __call__(self, texts, batch=512):
        out = []
        for s in range(0, len(texts), batch):
            enc = self.tok(texts[s:s + batch], padding=True, truncation=True, max_length=128,
                           return_tensors="pt", split_special_tokens=True).to(self.device)
            h = self.model(**enc).last_hidden_state
            m = enc["attention_mask"].unsqueeze(-1).float()
            v = (h * m).sum(1) / m.sum(1).clamp_min(1)
            out.append(F.normalize(v.float(), dim=-1))
        return torch.cat(out)


def center(v, mean):
    return F.normalize(v - mean, dim=-1)


class Projector(nn.Module):
    def __init__(self, d_in, d_model, k):
        super().__init__()
        self.k, self.d_model = k, d_model
        self.net = nn.Sequential(nn.Linear(d_in, 2048), nn.GELU(), nn.Linear(2048, k * d_model))
        self.norm = nn.LayerNorm(d_model)
        self.scale = nn.Parameter(torch.tensor(1.0))

    def forward(self, e):
        return self.norm(self.net(e).view(-1, self.k, self.d_model)) * self.scale


def batches(xs, size, shuffle):
    idx = list(range(len(xs)))
    if shuffle:
        random.shuffle(idx)
    for s in range(0, len(idx), size):
        yield idx[s:s + size]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--field", default="command")
    ap.add_argument("--store", default="/store")
    ap.add_argument("--k", type=int, default=16, help="soft prefix tokens")
    ap.add_argument("--max-chars", type=int, default=300)
    ap.add_argument("--max-tokens", type=int, default=96)
    ap.add_argument("--epochs", type=float, default=3)
    ap.add_argument("--batch", type=int, default=128)
    ap.add_argument("--lr-proj", type=float, default=1e-3)
    ap.add_argument("--lr-lora", type=float, default=3e-4)
    ap.add_argument("--lora-r", type=int, default=32)
    ap.add_argument("--eval-n", type=int, default=1000)
    ap.add_argument("--seed", type=int, default=1)
    a = ap.parse_args()
    random.seed(a.seed)
    torch.manual_seed(a.seed)
    dev = "cuda"

    stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%d-%H%M%S")
    out = os.path.join(a.store, "runs", f"invert-{a.field}-{stamp}")
    os.makedirs(out, exist_ok=True)
    log = open(os.path.join(out, "log.jsonl"), "a")

    def note(**kw):
        kw["t"] = round(time.time() - t0, 1)
        log.write(json.dumps(kw) + "\n")
        log.flush()
        print(json.dumps(kw), flush=True)

    t0 = time.time()
    enc_id, enc_path = identity(ENCODER)
    dec_id, dec_path = identity(DECODER)
    data = load_texts(a.store, a.field, a.max_chars)
    try:
        commit = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True, cwd="/work").stdout.strip()
        dirty = bool(subprocess.run(["git", "status", "--porcelain"], capture_output=True, text=True, cwd="/work").stdout.strip())
    except FileNotFoundError:
        commit, dirty = None, None
    config = {"args": vars(a), "encoder": enc_id, "decoder": dec_id, "counts": {k: len(v) for k, v in data.items()},
              "git": {"commit": commit, "dirty": dirty}, "image": os.environ.get("GANGLION_IMAGE"),
              "torch": torch.__version__, "device": torch.cuda.get_device_name(0), "started": stamp,
              "embedding": "encoder mean-pooled, L2-normalised, centred on train mean, renormalised"}
    json.dump(config, open(os.path.join(out, "config.json"), "w"), indent=2)
    note(event="data", **config["counts"])

    # embeddings, once
    emb = Embedder(enc_path, dev)
    E = {k: emb(v) for k, v in data.items()}
    mean = E["train"].mean(0, keepdim=True)
    E = {k: center(v, mean) for k, v in E.items()}
    note(event="embedded")

    tok = AutoTokenizer.from_pretrained(dec_path)
    dec = AutoModelForCausalLM.from_pretrained(dec_path).to(dev)
    d_model = dec.config.hidden_size
    dec = get_peft_model(dec, LoraConfig(r=a.lora_r, lora_alpha=2 * a.lora_r, lora_dropout=0.05,
                                         target_modules="all-linear", task_type="CAUSAL_LM"))
    proj = Projector(E["train"].shape[1], d_model, a.k).to(dev)
    embed_tokens = dec.get_input_embeddings()

    def encode_texts(texts):
        ids = [[tok.cls_token_id] + tok(t, add_special_tokens=False, split_special_tokens=True)["input_ids"][:a.max_tokens] + [tok.sep_token_id] for t in texts]
        n = max(map(len, ids))
        x = torch.full((len(ids), n), tok.pad_token_id)
        m = torch.zeros((len(ids), n), dtype=torch.long)
        for i, r in enumerate(ids):
            x[i, :len(r)] = torch.tensor(r)
            m[i, :len(r)] = 1
        return x.to(dev), m.to(dev)

    def forward(e, x, m):
        prefix = proj(e)
        inp = torch.cat([prefix, embed_tokens(x)], 1)
        mask = torch.cat([torch.ones(x.shape[0], a.k, dtype=m.dtype, device=dev), m], 1)
        logits = dec(inputs_embeds=inp, attention_mask=mask).logits
        # predict x[1:] from positions k .. k+len-2 (after [CLS])
        pred = logits[:, a.k:-1]
        tgt = x[:, 1:].masked_fill(m[:, 1:] == 0, -100)
        return F.cross_entropy(pred.reshape(-1, pred.shape[-1]).float(), tgt.reshape(-1), ignore_index=-100)

    params = [{"params": proj.parameters(), "lr": a.lr_proj},
              {"params": [p for p in dec.parameters() if p.requires_grad], "lr": a.lr_lora}]
    opt = torch.optim.AdamW(params, weight_decay=0.01)
    train = data["train"]
    steps = int(math.ceil(len(train) / a.batch) * a.epochs)
    warm = min(200, steps // 10)
    sched = torch.optim.lr_scheduler.LambdaLR(opt, lambda s: min(1, (s + 1) / warm) * 0.5 * (1 + math.cos(math.pi * min(1, s / steps))))
    note(event="train_start", steps=steps, trainable=sum(p.numel() for g in params for p in g["params"]))

    step, run_loss = 0, 0.0
    dec.train()
    while step < steps:
        for idx in batches(train, a.batch, True):
            if step >= steps:
                break
            x, m = encode_texts([train[i] for i in idx])
            with torch.autocast("cuda", dtype=torch.bfloat16):
                loss = forward(E["train"][idx], x, m)
            opt.zero_grad(set_to_none=True)
            loss.backward()
            torch.nn.utils.clip_grad_norm_([p for g in params for p in g["params"]], 1.0)
            opt.step()
            sched.step()
            step += 1
            run_loss += loss.item()
            if step % 50 == 0:
                note(event="step", step=step, loss=round(run_loss / 50, 4), lr=sched.get_last_lr()[0])
                run_loss = 0.0

    # save: projector + decoder with LoRA merged, loadable without peft
    dec.eval()
    merged = dec.merge_and_unload()
    merged.save_pretrained(os.path.join(out, "decoder"), safe_serialization=True)
    tok.save_pretrained(os.path.join(out, "decoder"))
    save_file({**{f"proj.{k}": v.detach().cpu().contiguous() for k, v in proj.state_dict().items()},
               "center_mean": mean.cpu().contiguous()}, os.path.join(out, "projector.safetensors"))
    note(event="saved")

    @torch.inference_mode()
    def generate(e, max_new=a.max_tokens):
        prefix = proj(e)
        cur = torch.full((e.shape[0], 1), tok.cls_token_id, device=dev)
        done = torch.zeros(e.shape[0], dtype=torch.bool, device=dev)
        for _ in range(max_new):
            inp = torch.cat([prefix, merged.get_input_embeddings()(cur)], 1)
            with torch.autocast("cuda", dtype=torch.bfloat16):
                nxt = merged(inputs_embeds=inp).logits[:, -1].argmax(-1)
            nxt = torch.where(done, torch.full_like(nxt, tok.pad_token_id), nxt)
            cur = torch.cat([cur, nxt[:, None]], 1)
            done |= nxt == tok.sep_token_id
            if done.all():
                break
        outs = []
        for row in cur[:, 1:].tolist():
            if tok.sep_token_id in row:
                row = row[:row.index(tok.sep_token_id)]
            outs.append(tok.decode([t for t in row if t != tok.pad_token_id]))
        return outs

    def f1(a_, b_):
        x, y = a_.split(), b_.split()
        if not x or not y:
            return float(x == y)
        common = sum((min(x.count(w), y.count(w)) for w in set(x)))
        if common == 0:
            return 0.0
        p, r = common / len(y), common / len(x)
        return 2 * p * r / (p + r)

    metrics = {}
    ev = open(os.path.join(out, "eval.jsonl"), "w")
    train_E = E["train"]
    for split in ("test", "local"):
        texts = data[split]
        if not texts:
            continue
        rng = random.Random(a.seed)
        pick = sorted(rng.sample(range(len(texts)), min(a.eval_n, len(texts))))
        texts = [texts[i] for i in pick]
        e = E[split][pick]
        hyps = []
        for s in range(0, len(texts), 128):
            hyps += generate(e[s:s + 128])
        re = center(emb(hyps), mean) if hyps else e
        cos = (re * e).sum(-1).tolist()
        # baseline: the nearest training text by embedding (retrieval, not generation)
        nn_idx = (e @ train_E.T).argmax(-1).tolist()
        nn_txt = [train[i] for i in nn_idx]
        rows = []
        for t, h, c, n in zip(texts, hyps, cos, nn_txt):
            rows.append({"text": t, "hyp": h, "exact": t == h, "f1": f1(t, h), "char": difflib.SequenceMatcher(None, t, h).ratio(),
                         "cos": c, "nearest_train": n, "nn_exact": t == n, "nn_f1": f1(t, n), "nn_char": difflib.SequenceMatcher(None, t, n).ratio()})
        for r in rows:
            ev.write(json.dumps({"split": split, **r}, ensure_ascii=False) + "\n")
        avg = lambda k: round(sum(r[k] for r in rows) / len(rows), 4)
        metrics[split] = {"n": len(rows), "exact": avg("exact"), "token_f1": avg("f1"), "char_ratio": avg("char"), "reembed_cos": avg("cos"),
                          "nn_exact": avg("nn_exact"), "nn_token_f1": avg("nn_f1"), "nn_char_ratio": avg("nn_char")}
        note(event="eval", split=split, **metrics[split])
    json.dump(metrics, open(os.path.join(out, "metrics.json"), "w"), indent=2)
    note(event="done", out=out)


if __name__ == "__main__":
    main()
