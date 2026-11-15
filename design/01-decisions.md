# Decisions (draft 3, 2026-11-14)

Index of decisions; detail lives in the topic notes.

## Working rules

W1. Keep decisions open until they need to be decided, unless the answer is obvious. Call out when a question becomes important to decide. (user)

## Ground rules (00)

R1. Models are for judgements; R2. software for what's computable; R3. trainable and runnable on the Framework (00/R1–R3).
R4. "The model" ideally under 1B parameters.
R5. Action-oriented: output is one action among many (00).

## By topic

- 02 perception: D1 typed events, D2 time as feature, D3 software routes, D4 independent encoding, D5 nested slots, D6 bidirectional within event / causal across, D21 base-rate surprisal feature (tentative), D22 mechanical encoding where possible.
- 03 actions: D7 actions as entities, D8 not acting first-class, D9 thought kinds, D10 buckets/intent/efference/batching mix as direction, D17 desire/choice readouts (unless something undesirable shows up).
- 04 loop: D11 halt = absence of action, D12 time as timer argument.
- 06 encoders: D23 two levels (field-type encoders shared, event level per domain), D24 no special tokens in text; trust/role as event-level embedding (amends 02/N2), D27 command executions first, D28 Ettin 17M (32M fallback), D29 a few vectors per text field, D30 V1–V5 gate before freezing.
- 07 workbench: requirements R1–R6 (built before any training), D25 Go server + Python workers, D26 data browser and encoder playground first, D31 design them now (08).
- 08 workbench design: D32 NDJSON worker protocol, D33 blobs + SQLite index, D34 no login on tailnet, D35 light and dark, D36 public + local data, D37 name ganglion on 7171, D38 Ettin playground, D39 minimise wire data, D40 starting data sources, D41 Ettin playground parts.
- 09 look: D42 design system (palette, type, components) as built.
- 10 inversion: D43 GPU work in the ganglion-gpu image built on gpu-run's; D44 Ettin decoder 150M for inversion.
- 11 prose dataset: D45 include my thinking; D46 include tack and Pi; D47 any other known prose, high quality and realistic; D48 fineweb-edu usable for training, marked, quality judged by the user; D49 Samples page; D50 quote detection and a blacklist for Claude's output; D51 ratings saved as labels.
- synthetic-data/ (own index in synthetic-data/01): D52–D99, Q71 open, T10–T17.
- 05 training: D13 whatever data we can get, JEPA-style where usable; D14 agentic work first; D15 noise both ways; D16 views scale with data; D18 training order; D19 encoder masking recipe; D20 base-rate residual (grain of salt).

## Open questions (all)

- Q1 trust in the stream (02): too early; decide before the stream architecture is fixed.
- Q2 aging rules (02): too early; decide when sizing stream state.
- Q4 action entity contents (03): unknown; decide when the heads' training data format is set.
- Q5 compute cost for halting (04): unknown; decide at the outcomes stage.
- Q13 sizes, K_max (06): not yet.
- Q16 Ettin vocabulary pruning (06): not yet.
- Q17 is "text" one field type or several (06): decide when a second domain arrives.
- Q18 first judgement (2026-11-15): which judgement does the user actually want made, that they can label and check? Important now: D27 (commands first) reopened because it fails this test. Pick the judgement first, then the smallest domain and architecture that serve it.
- Q19 who extracts decisions from free-text replies (00/G9): the user by hand, a large model as teacher with the user confirming, or both. Decide when building the first decision dataset.

## Future topics (all)

T1 encoder conditioning on stable context (02); T2 bootstrapping from pretrained models (05).
T3. Evaluation: calibration, robustness to rewording/reordering, what the desire/choice gap and surprise signals show.
T4. Parameter budget split under 1B: encoders vs stream vs heads vs effectors.
T5. Learning from outcomes / credit assignment for multi-action steps.
T6. Very long text fields (06).
T7. vec2text-style iterative correction (10). T8. Inverting output chunks and our own slots (10).

## Decided

(see topic notes)

## Open questions

(see list above)
