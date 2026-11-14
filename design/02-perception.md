# Perception: events, encoding, stream (draft 1, 2026-11-14)

## Events

E1. Input is typed events, not turns or messages.
E2. Time is not an event. It's a feature on every event (absolute and relative) and a timer argument on halt (04/H3).
E3. Routing by event type is computable, so software routes. No learned router between modules; learned conditional routing, if any, only inside a module.
E4. Ambient/streaming event types (progress chunks, telemetry, heartbeats) are marked by software; they don't wake on a plain halt (04/H2).

## Domains and tokenization

N1. One encoder per domain, run only when an event of that domain arrives. "A family where parts are only involved when they need to be."
N2. Separate tokenizers / embedding tables per domain. Control-like tokens exist only in trusted domains' vocabularies, so untrusted text can't spell an instruction. Provenance lives in the embedding, not in text.
   - Amended by 06/D24: one shared text vocabulary with no special tokens, just text; trust/role as an event-level embedding.
N3. Numbers as values (sign + log-magnitude, or Fourier features), not digit strings.
   - N3a. General rule (user): anything trivially encodable in software (no learned values needed) is encoded mechanically. Numbers as values *when possible*; more than numbers is likely encodable this way.
   - N3b. Where English text comes in, some learned "mechanical number ↔ written-out number" mapping will remain.
N4. Entities (files, people, processes, messages) as slots whose embeddings come from attributes software supplies. Actions refer to them by pointer; the model never spells an identifier. (user: "Exactly.")
N5. Natural language keeps a pretrained tokenizer (inherit knowledge); a lean vocabulary helps at small sizes.
   - N5a. Concern (user): knowledge we *don't* want. Preference for a model whose training data is auditable in the future.
   - N5b. Smaller is better: it leaves more of the budget for the main model.
   - N5c. The tokenizer itself carries little knowledge; the concern bites mostly on borrowed encoder/backbone weights (05/T2). Families known for releasing training data (OLMo, SmolLM, Ettin) are candidates; unverified which fit.
N6. Base rates come from software (05/B1): surprisal of an observed value as a feature, computed from live counts.

## Encoding

C1. Bidirectional within an event.
C2. Encoding depends only on (type, content): no conditioning on stream state. Cacheable by hash, parallel, encoder trainable and swappable on its own.
   - Considered and rejected: conditioning on stream state at arrival. Costs: sequential encoder/stream dependency, RNN-style training, no content-addressed cache, retraining the stream invalidates all encodings.
   - Context comes in at read time (stream attends over slots from its current state) or by a deliberate re-encode-with-focus action (03/A9).
   - The bottleneck should teach the encoder to extract what is typically important.
C3. Output per event: nested slots (Matryoshka-style). Ordered bank of learned slot queries; training randomly truncates k so early slots hold the most important information. Software sets k from event size.
   - Aging falls out: an old event drops its later slots before folding (S2).
   - Slot collapse defences: nested ordering, slot-attention competition, diversity penalty.
C4. Chunk slots (one per region via local attention) are a possible later addition for one domain, if nested slots prove too lossy for big many-part events. The choice is per domain.
   - + addressability (where things are).
   − weaker abstraction; span pointing is covered by re-encode-with-focus anyway.
C5. "Vectors vs matrix": same numbers; differs in reading. Slot set = sharp, exact retrieval. Matrix-as-operator (read = M·q, linear-attention-style state) = fixed size, additive, blurry. Used for different ages (S2).

## Stream

S1. Causal over event slots (not tokens). Likely linear-attention or recurrent. "Bidirectional over a whole event, then causal over the results of that." Details to be figured out.
S2. Exactness has limited scope: recent events as slot sets, older ones drop slots, then fold into a matrix. Long horizons and exact data come back through actions (fetch, re-encode).
S3. Encodings are immutable once computed, so caching is trivial.
   - S3a. Cache key = (encoder snapshot, type, content hash). (user suggested the snapshot)
   - S3b. With nested slots, store the full slot bank and truncate on read, so k needn't be in the key.

## Decided

D1. Typed events (E1). (user, agreed)
D2. Time is a feature and a timer argument, not an event (E2). (user: "Agreed.")
D3. Software routes by type (E3). (user: "Correct.")
D4. Fully independent encoding (C2); context only later or by deliberate re-encode. (user)
D5. Nested slots by default (C3); hybrid only as a possible per-domain addition (C4). (user leaned nested, open to hybrid; I didn't push)
D6. Bidirectional within event, causal across (C1, S1). (user)
D21. Base-rate surprisal as a software feature (N6). (user: "Yes, tentatively.")
D22. Mechanical encoding wherever no learned values are needed (N3a). (user)

## Open questions

Q1. Where does trust live in the stream? Independent encoding makes the encode side of the one-way valve automatic, but the stream mixes everything. Is domain identity in the embedding enough, or does the stream need separate state lanes or limits on what untrusted slots can write? (user: too early to decide.) Becomes important before the stream architecture is fixed.
Q2. Aging rules: when slots drop, when events fold, how state persists across a deep halt. (user: too early.) Becomes important when sizing the stream state.

## Future topics

T1. Conditioning encoders on rarely-changing context (role/task vector), cache keyed by (content hash, context version). Not starting there.
