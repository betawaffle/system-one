"""ganglion model worker: serves Ettin over a Unix socket (08/P2, D32).

Protocol: newline-delimited JSON. The server sends {"op":"hello",...} on
connect. Requests are {"id": n, "op": ..., ...}; replies are {"id": n, "ok":
true, ...} or {"id": n, "ok": false, "error": ...}. Long jobs may send
{"id": n, "progress": [done, total]} lines before the reply. A reply with
"bin": k is followed by k raw bytes (unused by the current ops).

Every request and reply is appended to <data>/worker-log/YYYY-MM-DD.jsonl with
the model identity, per the capture requirements.

Usage: python worker.py --socket PATH --data DIR
"""
import argparse
import asyncio
import datetime
import glob
import hashlib
import json
import math
import os
import platform
import sys
import time

import numpy as np
import torch
import transformers
from huggingface_hub import snapshot_download
from transformers import AutoModelForMaskedLM, AutoTokenizer

MODELS = {"17m": "jhu-clsp/ettin-encoder-17m", "32m": "jhu-clsp/ettin-encoder-32m"}
MAX_TOKENS = 512
torch.set_num_threads(max(1, (os.cpu_count() or 4) // 2))


def sha256_file(path):
    h = hashlib.sha256()
    with open(path, "rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


class Model:
    def __init__(self, key):
        repo = MODELS[key]
        self.key = key
        # Ettin ships pytorch_model.bin, not safetensors; transformers loads it with
        # torch.load(weights_only=True), which refuses arbitrary pickled objects.
        self.path = snapshot_download(repo, allow_patterns=["*.json", "*.safetensors", "pytorch_model.bin", "*.txt"])
        self.tok = AutoTokenizer.from_pretrained(self.path)
        self.model = AutoModelForMaskedLM.from_pretrained(self.path).eval()
        files = {}
        for name in sorted(os.listdir(self.path)):
            p = os.path.join(self.path, name)
            if os.path.isfile(p) and (name.endswith(".safetensors") or name in ("pytorch_model.bin", "tokenizer.json", "config.json")):
                files[name] = "sha256:" + sha256_file(p)
        self.identity = {
            "key": key,
            "repo": repo,
            "revision": os.path.basename(self.path),
            "files": files,
            "params": sum(p.numel() for p in self.model.parameters()),
            "vocab": self.tok.vocab_size if hasattr(self.tok, "vocab_size") else None,
            "mask_token": self.tok.mask_token,
        }
        # Snapshot id used as a cache key (02/S3a).
        self.snapshot = hashlib.sha256(json.dumps(files, sort_keys=True).encode()).hexdigest()[:16]

    def encode_plain(self, text):
        # Our pipeline never parses special-token strings in text (06/X1a).
        return self.tok(text, add_special_tokens=True, truncation=True, max_length=MAX_TOKENS,
                        return_offsets_mapping=True, split_special_tokens=True)

    def pieces(self, enc, text):
        out = []
        for i, (tid, (a, b)) in enumerate(zip(enc["input_ids"], enc["offset_mapping"])):
            special = tid in self.tok.all_special_ids and a == b
            out.append({"id": tid, "text": text[a:b] if not special else self.tok.convert_ids_to_tokens(tid),
                        "start": a, "end": b, "special": special})
        return out

    @torch.inference_mode()
    def tokenize(self, text):
        enc = self.encode_plain(text)
        return {"tokens": self.pieces(enc, text), "count": len(enc["input_ids"])}

    @torch.inference_mode()
    def fill_mask(self, text, k=8):
        # Here [MASK] is deliberate (08/K1b): parse special tokens.
        enc = self.tok(text, return_tensors="pt", truncation=True, max_length=MAX_TOKENS, return_offsets_mapping=True)
        offsets = enc.pop("offset_mapping")[0].tolist()
        ids = enc["input_ids"][0]
        pos = (ids == self.tok.mask_token_id).nonzero().flatten().tolist()
        if not pos:
            return {"masks": [], "note": f"Put {self.tok.mask_token} where a word should be predicted."}
        logits = self.model(**enc).logits[0]
        masks = []
        for p in pos:
            probs = torch.softmax(logits[p].float(), -1)
            top = torch.topk(probs, k)
            ent = float(-(probs * torch.log2(probs.clamp_min(1e-12))).sum())
            masks.append({"pos": p, "start": offsets[p][0], "end": offsets[p][1], "entropy_bits": round(ent, 3),
                          "top": [{"id": int(i), "text": self.tok.decode([int(i)]), "p": round(float(v), 5)}
                                  for v, i in zip(top.values, top.indices)]})
        return {"masks": masks, "input_ids": ids.tolist()}

    @torch.inference_mode()
    def surprise(self, text, batch=64):
        enc = self.encode_plain(text)
        ids = torch.tensor(enc["input_ids"])
        n = len(ids)
        toks = self.pieces(enc, text)
        targets = [i for i, t in enumerate(toks) if not t["special"]]
        results = {}
        for s in range(0, len(targets), batch):
            chunk = targets[s:s + batch]
            x = ids.repeat(len(chunk), 1)
            for row, p in enumerate(chunk):
                x[row, p] = self.tok.mask_token_id
            logits = self.model(input_ids=x, attention_mask=torch.ones_like(x)).logits
            for row, p in enumerate(chunk):
                probs = torch.softmax(logits[row, p].float(), -1)
                pt = float(probs[ids[p]])
                best = int(torch.argmax(probs))
                results[p] = {"p": round(pt, 6), "bits": round(-math.log2(max(pt, 1e-12)), 3),
                              "best": self.tok.decode([best]), "best_p": round(float(probs[best]), 5)}
        for i, t in enumerate(toks):
            t.update(results.get(i, {}))
        bits = [results[p]["bits"] for p in targets]
        return {"tokens": toks, "count": n, "mean_bits": round(sum(bits) / len(bits), 3) if bits else 0,
                "input_ids": ids.tolist()}

    @torch.inference_mode()
    def embed(self, texts, batch=64):
        vecs = []
        for s in range(0, len(texts), batch):
            enc = self.tok(texts[s:s + batch], padding=True, truncation=True, max_length=128,
                           return_tensors="pt", split_special_tokens=True)
            h = self.model.base_model(**enc).last_hidden_state
            m = enc["attention_mask"].unsqueeze(-1).float()
            v = (h * m).sum(1) / m.sum(1).clamp_min(1)
            vecs.append(torch.nn.functional.normalize(v.float(), dim=-1))
        return torch.cat(vecs).numpy() if vecs else np.zeros((0, 1), dtype=np.float32)


class Inverter:
    """A trained embedding inverter (train/invert.py): projector + decoder with LoRA merged.
    Loads the newest finished run under <data>/runs/invert-command-*."""

    def __init__(self, run_dir):
        from safetensors.torch import load_file
        from transformers import AutoModelForCausalLM
        self.run = os.path.basename(run_dir)
        self.config = json.load(open(os.path.join(run_dir, "config.json")))
        self.metrics = json.load(open(os.path.join(run_dir, "metrics.json")))
        dec_path = os.path.join(run_dir, "decoder")
        self.tok = AutoTokenizer.from_pretrained(dec_path)
        self.dec = AutoModelForCausalLM.from_pretrained(dec_path).eval()
        st = load_file(os.path.join(run_dir, "projector.safetensors"))
        self.mean = st.pop("center_mean")
        k = self.config["args"]["k"]
        d_model = self.dec.config.hidden_size
        w = {n[len("proj."):]: v for n, v in st.items()}
        self.proj = torch.nn.ModuleDict({"net": torch.nn.Sequential(
            torch.nn.Linear(self.mean.shape[1], 2048), torch.nn.GELU(), torch.nn.Linear(2048, k * d_model)),
            "norm": torch.nn.LayerNorm(d_model)})
        self.proj.load_state_dict({n: v for n, v in w.items() if n != "scale"})
        self.scale, self.k, self.d_model = w["scale"], k, d_model
        self.max_new = self.config["args"]["max_tokens"]

    @torch.inference_mode()
    def __call__(self, e):
        """e: (1, d) embedding already centred. Returns greedy text."""
        prefix = self.proj["norm"](self.proj["net"](e).view(-1, self.k, self.d_model)) * self.scale
        ids = [self.tok.cls_token_id]
        emb = self.dec.get_input_embeddings()
        for _ in range(self.max_new):
            inp = torch.cat([prefix, emb(torch.tensor([ids]))], 1)
            nxt = int(self.dec(inputs_embeds=inp).logits[0, -1].argmax())
            if nxt == self.tok.sep_token_id:
                break
            ids.append(nxt)
        return self.tok.decode(ids[1:])


def latest_inverter_run(data):
    runs = sorted(d for d in glob.glob(os.path.join(data, "runs", "invert-command-*"))
                  if os.path.exists(os.path.join(d, "metrics.json")) and os.path.isdir(os.path.join(d, "decoder")))
    return runs[-1] if runs else None


class Geometry:
    """Corpus statistics for one model's embedding space: mean and whitening.

    Built from a sample of corpus texts. Spaces: raw (as the model gives them),
    centred (corpus mean removed, renormalised), whitened (centred, then every
    principal direction rescaled to unit variance, renormalised; BERT-whitening).
    """

    def __init__(self, vecs=None, path=None):
        if path is not None:
            z = np.load(path, allow_pickle=False)
            self.mean, self.W, self.eig, self.n = z["mean"], z["W"], z["eig"], int(z["n"])
            self.panel = json.loads(str(z["panel"]))
            return
        V = vecs.astype(np.float64)
        self.n = len(V)
        self.mean = V.mean(0, keepdims=True)
        X = V - self.mean
        C = X.T @ X / max(1, self.n - 1)
        eig, U = np.linalg.eigh(C)
        order = np.argsort(eig)[::-1]
        eig, U = np.clip(eig[order], 0, None), U[:, order]
        self.eig = eig
        self.W = U / np.sqrt(eig + 1e-6 * eig[0])[None, :]
        self.panel = self._panel(vecs)

    def to(self, v, space):
        v = v.astype(np.float64)
        if space == "raw":
            out = v
        elif space == "centred":
            out = v - self.mean
        elif space == "whitened":
            out = (v - self.mean) @ self.W
        else:
            raise ValueError(f"unknown space {space!r}")
        return (out / np.maximum(np.linalg.norm(out, axis=1, keepdims=True), 1e-12)).astype(np.float32)

    def _panel(self, vecs, pairs=20000, bins=40):
        rng = np.random.default_rng(0)
        i = rng.integers(0, len(vecs), pairs)
        j = rng.integers(0, len(vecs), pairs)
        keep = i != j
        i, j = i[keep], j[keep]
        hist = {}
        for space in ("raw", "centred", "whitened"):
            s = self.to(vecs, space)
            cos = (s[i] * s[j]).sum(1)
            h, _ = np.histogram(cos, bins=bins, range=(-1, 1))
            hist[space] = {"counts": h.tolist(), "mean": round(float(cos.mean()), 4),
                           "p05": round(float(np.quantile(cos, 0.05)), 4), "p95": round(float(np.quantile(cos, 0.95)), 4)}
        total = float(self.eig.sum())
        return {"n": self.n, "pairs": int(len(i)), "bins": bins, "hist": hist,
                "top_var": [round(float(x / total), 5) for x in self.eig[:10]],
                "participation": round(float(total ** 2 / (self.eig ** 2).sum()), 2), "dims": int(len(self.eig)),
                "mean_norm": round(float(np.linalg.norm(self.mean)), 4)}

    def save(self, path):
        np.savez(path, mean=self.mean, W=self.W, eig=self.eig, n=self.n, panel=json.dumps(self.panel))


def center(v, mean):
    v = v - mean
    return (v / np.maximum(np.linalg.norm(v, axis=1, keepdims=True), 1e-8)).astype(np.float32)


class Worker:
    def __init__(self, data):
        self.data = data
        self.models = {}
        self.indexes = {}  # (key, field) -> (hashes, texts, matrix, mean)
        self.geometries = {}  # (key, field) -> Geometry
        self.lock = asyncio.Lock()
        os.makedirs(os.path.join(data, "worker-log"), exist_ok=True)
        os.makedirs(os.path.join(data, "index"), exist_ok=True)

    def model(self, key):
        if key not in MODELS:
            raise ValueError(f"unknown model {key!r}")
        if key not in self.models:
            self.models[key] = Model(key)
        return self.models[key]

    def log(self, rec):
        rec["at"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        day = rec["at"][:10]
        with open(os.path.join(self.data, "worker-log", day + ".jsonl"), "a") as f:
            f.write(json.dumps(rec, ensure_ascii=False) + "\n")

    def hello(self):
        return {"op": "hello", "worker": "ganglion-worker/0.1", "models": list(MODELS),
                "loaded": {k: m.identity for k, m in self.models.items()},
                "engine": {"torch": torch.__version__, "transformers": transformers.__version__,
                           "python": platform.python_version(), "device": "cpu",
                           "threads": torch.get_num_threads(), "machine": platform.machine()}}

    def geometry_path(self, m, field):
        return os.path.join(self.data, "index", f"geom-{field}-{m.key}-{m.snapshot}.npz")

    def load_geometry(self, m, field):
        k = (m.key, field)
        if k not in self.geometries:
            p = self.geometry_path(m, field)
            if not os.path.exists(p):
                return None
            self.geometries[k] = Geometry(path=p)
        return self.geometries[k]

    def index_path(self, m, field):
        return os.path.join(self.data, "index", f"{field}-{m.key}-{m.snapshot}.npz")

    def load_index(self, m, field):
        k = (m.key, field)
        if k not in self.indexes:
            p = self.index_path(m, field)
            if not os.path.exists(p):
                return None
            z = np.load(p, allow_pickle=False)
            if "mean" not in z:
                return None  # built before centring; rebuild
            self.indexes[k] = (list(z["hashes"]), list(z["texts"]), z["vecs"], z["mean"])
        return self.indexes[k]

    async def handle(self, req, send):
        op = req.get("op")
        loop = asyncio.get_running_loop()
        run = lambda f, *a: loop.run_in_executor(None, f, *a)
        if op == "hello":
            return self.hello()
        if op == "load":
            m = await run(self.model, req["model"])
            return {"identity": m.identity, "snapshot": m.snapshot}
        m = await run(self.model, req.get("model", "17m"))
        base = {"model": m.identity["repo"], "revision": m.identity["revision"], "snapshot": m.snapshot}
        if op == "tokenize":
            return base | await run(m.tokenize, req["text"])
        if op == "fill_mask":
            return base | await run(m.fill_mask, req["text"], int(req.get("k", 8)))
        if op == "surprise":
            return base | await run(m.surprise, req["text"])
        if op == "similarity":
            texts = req["texts"][:8]
            space = req.get("space", "raw")
            v = await run(m.embed, texts)
            if space != "raw":
                g = self.load_geometry(m, req.get("field", "command"))
                if g is None:
                    return base | {"needs_stats": True, "space": space}
                v = g.to(v, space)
            sim = (v @ v.T).round(4).tolist()
            return base | {"sim": sim, "space": space}
        if op == "geometry":
            field = req.get("field", "command")
            g = None if req.get("rebuild") else self.load_geometry(m, field)
            if g is None:
                texts = req.get("texts") or []
                if len(texts) < 100:
                    return base | {"needs_texts": True}
                vecs = await run(m.embed, texts)
                g = await run(Geometry, vecs)
                g.save(self.geometry_path(m, field))
                self.geometries[(m.key, field)] = g
            return base | {"panel": g.panel}
        if op == "invert":
            run_dir = latest_inverter_run(self.data)
            if run_dir is None:
                return base | {"ready": False}
            if getattr(self, "inverter", None) is None or self.inverter.run != os.path.basename(run_dir):
                self.inverter = await run(Inverter, run_dir)
            inv = self.inverter
            enc = self.model(inv.config["encoder"]["repo"].split("-")[-1])  # the encoder it was trained on
            e = torch.from_numpy(await run(enc.embed, [req["text"]]))
            e = torch.nn.functional.normalize(e - inv.mean, dim=-1)
            hyp = await run(inv, e)
            r = torch.from_numpy(await run(enc.embed, [hyp])) if hyp else e
            r = torch.nn.functional.normalize(r - inv.mean, dim=-1)
            return base | {"ready": True, "hyp": hyp, "cos": round(float((r * e).sum()), 4), "run": inv.run,
                           "metrics": inv.metrics, "steps": inv.config.get("counts"), "encoder": inv.config["encoder"]["repo"]}
        if op == "index_status":
            idx = self.load_index(m, req["field"])
            return base | {"ready": idx is not None, "size": len(idx[0]) if idx else 0}
        if op == "index_build":
            hashes, texts = req["hashes"], req["texts"]
            vecs = []
            step = 512
            for s in range(0, len(texts), step):
                vecs.append(await run(m.embed, texts[s:s + step]))
                await send({"id": req["id"], "progress": [min(s + step, len(texts)), len(texts)]})
            mat = np.concatenate(vecs) if vecs else np.zeros((0, 1), np.float32)
            # Raw MLM embeddings share one dominant direction (every cosine is ~0.99),
            # so centre on the corpus mean and renormalise before comparing.
            mean = mat.mean(0, keepdims=True) if len(mat) else np.zeros((1, mat.shape[1]), np.float32)
            mat = center(mat, mean)
            np.savez(self.index_path(m, req["field"]), hashes=np.array(hashes), texts=np.array(texts), vecs=mat, mean=mean)
            self.indexes[(m.key, req["field"])] = (hashes, texts, mat, mean)
            return base | {"size": len(hashes)}
        if op == "nearest":
            idx = self.load_index(m, req["field"])
            if idx is None:
                return base | {"ready": False, "items": []}
            hashes, texts, mat, mean = idx
            q = center(await run(m.embed, [req["text"]]), mean)
            scores = mat @ q[0]
            k = min(int(req.get("k", 10)), len(scores))
            top = np.argpartition(-scores, k - 1)[:k] if k else []
            top = sorted(top, key=lambda i: -scores[i])
            return base | {"ready": True, "items": [{"hash": hashes[i], "text": texts[i], "score": round(float(scores[i]), 4)} for i in top]}
        raise ValueError(f"unknown op {op!r}")

    async def serve(self, reader, writer):
        wlock = asyncio.Lock()

        async def send(obj):
            async with wlock:
                writer.write((json.dumps(obj, ensure_ascii=False) + "\n").encode())
                await writer.drain()

        await send(self.hello())
        tasks = set()

        async def one(req):
            t0 = time.time()
            try:
                res = await self.handle(req, send)
                reply = {"id": req.get("id"), "ok": True} | res
            except Exception as e:  # report every failure to the caller
                reply = {"id": req.get("id"), "ok": False, "error": f"{type(e).__name__}: {e}"}
            reply["ms"] = round((time.time() - t0) * 1000, 1)
            logged = dict(req)
            if req.get("op") in ("index_build", "geometry"):
                logged = {k: v for k, v in req.items() if k not in ("texts", "hashes")} | {"items": len(req.get("texts", []))}
            self.log({"request": logged, "reply": reply, "engine": self.hello()["engine"]})
            await send(reply)

        while True:
            line = await reader.readline()
            if not line:
                break
            try:
                req = json.loads(line)
            except json.JSONDecodeError:
                continue
            t = asyncio.create_task(one(req))
            tasks.add(t)
            t.add_done_callback(tasks.discard)
        writer.close()


async def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--socket", required=True)
    ap.add_argument("--data", required=True)
    ap.add_argument("--preload", default="17m,32m")
    a = ap.parse_args()
    w = Worker(a.data)
    for k in filter(None, a.preload.split(",")):
        w.model(k)
        print(f"loaded {k}", file=sys.stderr, flush=True)
    if os.path.exists(a.socket):
        os.unlink(a.socket)
    server = await asyncio.start_unix_server(w.serve, path=a.socket, limit=1 << 28)
    os.chmod(a.socket, 0o600)
    print(f"worker listening on {a.socket}", file=sys.stderr, flush=True)
    async with server:
        await server.serve_forever()


if __name__ == "__main__":
    asyncio.run(main())
