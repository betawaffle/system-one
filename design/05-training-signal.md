# Training signal (draft 1, 2026-11-14)

Stance: "it's gonna be whatever we have, can make, or get our hands on." JEPA-style is a good rough direction when we can use it.

## Per part

P1. What each part learns, and fitting signals:
   - domain encoders: what's in an event → JEPA within event; alignment to a pretrained teacher
   - stream: how events relate, what comes next → JEPA across events, action-conditioned
   - selection heads: what to do → imitation, distillation, outcomes
   - value/confidence: how good, how sure → outcomes, teacher agreement
   - desire head: what it would do unrestricted → teacher labels ignoring legality (03/A8)
   - speech effector: render an intent → distillation from a generative teacher
P2. JEPA covers perception and prediction, not preference. Predicting the world well isn't knowing what to do.

## JEPA

J1. Within an event (I-JEPA-style). Context encoder (kept) sees the event with parts masked and outputs slots. Target encoder (EMA copy, no gradient) sees the whole event. A small predictor takes context slots + "what's missing here" markers and predicts the target's representation of the masked parts. Loss is distance in representation space.
   - The predictor works only from slots, so slots must carry what lets the rest be inferred.
   - Predicting representations, not tokens, doesn't penalise unpredictable surface detail.
J1a. EMA = exponential moving average: θ_target ← τ·θ_target + (1−τ)·θ_context, τ ≈ 0.996–0.9999. Stable target; prevents collapse.
J2. Across events: predict the next event's encoding from stream state + action taken.
J3. Efference copy: predict action-result from action-taken (04/L4). Probably the highest value.
J4. Staged freezing: train encoders, freeze, then the stream's targets are fixed cached vectors. No collapse possible, many epochs over precomputed encodings, cheap on the Framework.
   − the stream can't predict more than frozen encoders capture. Later unfreeze with an EMA target if that's the limit.
J5. Noisy outcomes: a deterministic predictor learns the average of futures (a blurry non-state). Remedies: latent variable z in the predictor; several hypotheses (winner-take-all or mixture).

## Masking

M1. Mask every part independently and combine gradients before one update: yes (data2vec 2.0 does this; full-view target computed once). No leak within a step: weights fixed, each view a separate forward pass, nothing carried between views.
M2. Where cheating can happen:
   - M2a. Across steps: memorisation of repeated events. Ordinary overfitting; check on held-out events; dedupe by content hash.
   - M2b. Local shortcuts: mask meaningful units (fields, lines, spans), not single tokens. Typed events make units computable.
   - M2c. Structural giveaways: the missing-here marker should say where, not too much about what.
   - M2d. Easy parts soaking up gradient: weight masks toward less predictable parts (computable heuristics).
M3. Views per event scale with the number of maskable units (event size) and inversely with data abundance. data2vec's 8–16 came from fixed-size samples; a starting point, not a rule. Software can set the count per event.

## Base rates

B0. Fields predictable without context: software knows the usual value. The model needs little training to predict them but must still represent them; the rare deviation is often the most informative moment. Fields predictable *from context* are the lesson and should be masked.
B1. Give the base rate as input and learn only the residual: logits = log p_base + learned correction; surprisal (−log p) as a feature. Gradient goes only where context matters.
B2. Where a target is needed, train against the known distribution (KL, software as teacher), not one sampled value.
B3. Initialise head biases from log base rates.
B4. Conditioning stays computable while the condition is software-known (type, name, entity); it becomes judgment once it needs interpretation.
B5. Base rates drift; software stats update live, weights don't. The weights learn how to use surprise, not what's currently usual.

## Sources

O1. Have: logged streams of happenings + responses (shell histories, chat/agent transcripts, git history: commits = actions, diffs = results, reviews = judgments).
O2. Make: converted public datasets (Decider-style); teacher labeling with distributions over actions, including "do nothing" and a legality-ignoring version, ideally from a local open teacher (real probabilities); simulated environments with computable consequences; counterfactuals by replay.
O3. Get: public agent trajectory datasets (coding, web, tool use), public classification/ranking data, open-weight teachers run locally.

## Biases

X1. Inaction is invisible in logs: include events that didn't lead to action, labelled halt. Teacher labeling can supply this in bulk.
X2. Imitation learns habit, not quality.
X3. Few negatives in logs; simulation and counterfactuals are the main source.
X4. Distillation copies the teacher's calibration and blind spots; agreement among teachers is a cheap check.

## Noise and replay

N1. First domain is agentic work, where consequences can be simulated or replayed (sandbox + snapshot/restore → branching).
N2. Nondeterminism (network, time, external services): record and play back as fixed inputs, or accept as noise. Both are appropriate sometimes; much is inherently noisy and the model should understand that.
   - fixed playback isolates the action's effect (credit assignment)
   - natural noise teaches calibration
N3. Repeated replays of one counterfactual measure inherent noise directly → aleatoric vs epistemic uncertainty learned separately (03/A6).

## Proposed order

K1. Encoders (JEPA within events + alignment to pretrained, then freeze) → stream (action-conditioned next-event JEPA over cached encodings) → heads (converted datasets, distillation, imitation with inaction) → outcomes (value and preferences from simulated environments with counterfactuals). Each stage's data is easier to get than the next's.

## Decided

D13. Training data is whatever we have, can make, or get; JEPA-style where usable. (user)
D14. First domain: agentic work; consequences can be simulated or replayed (N1). (user: "I think so, yes")
D15. Both fixed playback and accepted noise, as appropriate; the model should understand inherent noise (N2). (user)
D16. Views per event depend on data size (M3). (user: "Presumably it also depends on how big the data is"; agreed)

D18. Proposed order K1, answering Q6. (user: "I think so, yes.")
D19. Multi-view masking with meaningful units and the M2 defences as the encoder recipe, answering Q7. (user: "Yes.")
D20. Base-rate residual on surprisal (B1) by default, B2/B3 where a learned distribution remains, answering Q8. (user: "Yes", with a grain of salt: not sure they fully understand the implications.)

## Open questions

(none for this note)

## Future topics

T2. Bootstrapping from pretrained models: learned adapters later dispensed with (distill-then-discard via a projector), and reshaping (probing layers, PCA/SVD of activations on domain data, use the useful subspace as the target).
