# Encoders: getting started (draft 1, 2026-11-14)

Builds on 02/C1–C5 (encoding), 05/J1, J1a, J4, M1–M3 (JEPA and masking), D19 (encoder recipe).

## What's needed

G1. A first domain (D27).
G2. An event schema for it: fields and their types (text, number, enum, time, entity reference, nested list). Which fields are mechanical (02/N3a) is decided per field type.
G3. Field embedders per field type:
   - mechanical: numbers (value features), enums (table), time (features), entity refs (attribute embedding, 02/N4)
   - learned: text fields through a tokenizer + text encoder
G4. Event encoder: bidirectional transformer over field embeddings (+ field-name and type embeddings), then the slot head.
G5. Slot head: bank of K_max learned queries cross-attending over the event; nested truncation during training (02/C3).
G6. JEPA machinery: EMA target copy (05/J1a), a small predictor, a mask generator over meaningful units (fields, lines, spans; 05/M2b), multi-view per event (05/M1, M3).
G7. Data: a corpus of events in the schema; content-hash dedupe; held-out split by *source* (whole trajectories or sessions), not by event, so near-duplicates don't leak across the split.
G8. Evaluation before freezing (Q12). Freezing (05/J4) is only safe if we can tell an encoder is good.
G9. Stack: PyTorch on ROCm works on the Framework's GPU. Nothing exotic needed for encoders this size.

## Structure: two levels (proposal)

L1. Field level: one encoder per *field type*, shared across domains (a text field in command output and a text field in a message use the same text encoder).
L2. Event level: one small encoder per *domain*, composing field embeddings into slots. This is what 02/N1 "one encoder per domain" becomes.
   + text knowledge paid for once, not per domain (02/N5b, budget)
   + new domains are mostly a schema plus a small event layer
   − a shared text encoder is a shared failure point; per-domain quirks in text (terminal output vs prose) must be learnable at the event level or by domain embeddings
L3. I lean two levels. (Q11)
L4. All text goes through the one text encoder (user): field names (if needed), field values, action descriptions, etc. One consistent vector space for all text.
   - L4a. Field names: the schema is known, so a learned table would do. But text-encoding them is free in practice: encodings are cached by content hash (02/S3a), so each name is encoded once ever, and new schemas work zero-shot. I lean text-encoded.
   - L4b. Action descriptions in the same space supports zero-shot actions (03/A3).
   - L4c. Consequence: untrusted output saying "delete all files" and an action described as "delete all files" encode near-identically. That's by design, but it means trust can't come from the text encoder. See X1.
   - L4d. One encoder covers prose, terminal output and code. These are different distributions; Ettin's data includes code (E5), and fine-tuning covers the rest.

## Conflict

X1. 02/N2 (separate tokenizers per domain; control tokens only in trusted vocabularies) conflicts with L1/L4 (one shared text encoder and tokenizer).
   - Proposed resolution: the shared text vocabulary contains *no* control tokens at all. Structure and control live entirely outside text (mechanical and structural embeddings, 02/N3a). Domain/role/trust is an embedding added at the event level (L2). That keeps N2's goal: text can't spell control, because nothing in the text vocabulary means control.
   - Agreed (D24); 02/N2 amended by 06/D24.
X1a. Practical consequence: Ettin's tokenizer has special tokens ([CLS], [SEP], [MASK], [PAD]). Input text must never produce them: a literal "[MASK]" in untrusted text has to tokenize as plain text (Hugging Face tokenizers parse special-token strings in input by default; `split_special_tokens=True` turns that off). JEPA masking happens at the field level with learned mask embeddings at the event level, not with [MASK] text tokens. Slots replace [CLS] pooling.

## Ettin (checked 2026-11-14)

E1. Paired encoders and decoders from JHU CLSP and LightOn, same data, architecture and recipe for both; ModernBERT recipe. Paper arXiv 2507.11412 (ICLR 2026). Models MIT on Hugging Face (jhu-clsp/ettin-encoder-*).
E2. Encoder sizes (layers / hidden):
   - 17M: 7 / 256
   - 32M: 10 / 384
   - 68M: 19 / 512
   - 150M: 22 / 768
   - 400M: 28 / 1024
   - 1B: 28 / 1792
E3. Tokenizer: ModernBERT's, vocabulary 50,368. Context: 1024 in pretraining, extended to 8K.
E4. Embedding table dominates the small sizes: 50,368 × 256 ≈ 12.9M of 17M; × 384 ≈ 19.3M of 32M; × 512 ≈ 25.8M of 68M (my arithmetic). So for tiny sizes, vocabulary is most of the parameters, and pruning it to our text's needs would shrink them a lot.
E5. Data: about 2T tokens in three phases (1.7T pretraining, 250B extension, 100B decay), all published (jhu-clsp/ettin-pretraining-data, -extension-data, -decay-data). Mix includes DCLM, Dolma v1.7, scientific papers, code, curated sources.
E6. Auditability (02/N5a): the batch-level training order is published (jhu-clsp/ettin-data-order, input_ids per step), plus 250+ intermediate checkpoints. You can say exactly what it saw and when. Auditable isn't the same as clean: DCLM and Dolma are web data, so unwanted knowledge is present but findable. Dataset licences aren't stated on the model page.
E7. Claimed: encoders beat ModernBERT across tasks and sizes. Not checked by us.
E8. Encoders are MLM-trained. JEPA training on top (05/J1) is a change of objective; their own cross-objective results (50B tokens of the other objective) show switching works but trails native training. Our switch is smaller in kind (still bidirectional, still representation-shaped).
E9. Not on the pages read: attention layout and RoPE. ModernBERT uses alternating local/global attention with RoPE; Ettin likely matches (unverified).
E10. Fit: strong for I2. 17M or 32M as the starting text encoder, given 02/N5b.

## Initialisation

I1. From scratch: cleanest provenance (02/N5a), but text fields need language knowledge, which is expensive to learn.
I2. Pretrained small encoder for the text field level: Ettin (17M to 1B, MIT) is a candidate family; training data reportedly released (unverified). Mechanical embedders and the event level are new either way.
I3. Distill-then-discard (05/T2): later, if borrowed weights are a concern.
I4. I lean I2 for text, new for everything else. (D28)

## Evaluating an encoder (proposal)

V1. Held-out JEPA loss (by source, G7).
V2. Linear probes from slots: recover field values and properties software already knows (exit code, error vs not, command family). Cheap, labels free (05/B0 fields are free probe targets).
V3. Nested-slot curve: probe and JEPA quality vs k. Should degrade gracefully, not fall off a cliff.
V4. Collapse checks: per-dimension variance, effective rank of slot embeddings, similarity between slots of one event.
V5. Retrieval sanity: nearest neighbours of an event should look related to a human.
V6. Downstream (later): does the stream/heads stage do better with this encoder than with a baseline (e.g. mean-pooled pretrained embeddings)?

## Sizes (provisional)

Z1. Text field encoder: tens of millions of parameters (Ettin 17M–68M range) to start.
Z2. Event level: a few layers, single-digit millions to low tens of millions per domain.
Z3. K_max: 8–16 slots, width matched to the stream. Placeholder until 05/K1 stream stage.

## First domain walkthrough: command executions (D27)

W1. One event, roughly:
   - command: argv as a list of text items (program name possibly also an entity)
   - cwd: entity ref (path)
   - started_at, duration: mechanical (time, number)
   - exit_code: mechanical (number), with base-rate surprisal (05/B1)
   - stdout, stderr: text, possibly long (T6)
   - truncated: mechanical (bool)
W2. Output of the encoder: K slots holding, in vector form, something like "a pytest run on the API tests, failed, one assertion error in test_x about a 500 status, took 4 s". That's what a stream model needs to judge the next action.
W3. What within-event JEPA teaches, by what's masked (05/J1):
   - exit code + stderr, from command + stdout: what failure looks like
   - stdout, from command: what a command typically produces (as a representation, not text)
   - command, from output: recognising what was run from its effects
   - duration, from command + output: cost signatures (slow suites, hangs)
   - lines or spans of stdout, from the rest: the structure of outputs (tracebacks, test summaries, compiler errors)
W4. Free probe labels (V2) from software parsers: non-zero exit, program family, error type, test counts.
W5. What it does *not* learn: anything about the next event (stream, 05/J2) or whether running the command was a good idea (heads).
W6. Data: public agent trajectories have many command/output pairs; sandboxes can produce more by running commands (and later counterfactuals, 05/N1).
W7. Risks: huge outputs (T6), boilerplate soaking up gradient (05/M2d), outputs that depend on hidden state (files not shown) so masked predictions are inherently noisy (05/J5).
W8. Alternatives: plain prose messages (simplest, exercises no structure); file contents (huge, code-heavy, little structure). Command executions are the richest small step.

## Decided

D23. Two levels: field-type encoders shared across domains, small event-level encoder per domain (L1, L2), answering Q11. (user: "Agreed." to both)
D27. First domain: command executions (W1–W8). Answers Q9. (user: "I think that's fine, but I'll want to see some examples in the UI, I think?" → examples in the data browser, 07/A1.)
D28. Text encoder: Ettin 17M by default, 32M if 17M falls short at the evaluation gate (V1–V5). Answers Q10. (user: fine with either, "17M if we can get away with it".)
   - Non-embedding capacity is the real difference: about 4M (17M) vs about 13M (32M), 7 vs 10 layers (from E2/E4 arithmetic). Terminal output and code may need more than 4M; the gate tells us.
   - Both are cheap enough to run side by side when the time comes.
D29. The event level sees each text field as a few vectors, not per token; re-encode-with-focus for detail. Answers Q15. (user: "A few vectors per field.")
D30. V1–V5 is the gate before freezing an encoder. Answers Q12. (user: "Yes.")
D24. Text vocabulary: no special tokens, just text. Trust/role is an embedding added at the event level. Answers Q14; amends 02/N2. (user)

## Open questions

Q16. Prune Ettin's vocabulary for small sizes (E4)? Not important yet.
Q13. Sizes and K_max (Z1–Z3). Provisional numbers are fine for now; not important yet.

## Future topics

T6. Very long text fields (big outputs): truncation, chunking, or chunk slots (02/C4) for this domain.
