# Actions, heads, effectors (draft 1, 2026-11-14)

## Action space

A1. A policy, not a predictor: "which of the things I could do right now should I do, and with what parameters?"
A2. Software defines the legal action set and argument domains; the model scores.
A3. Actions are described entities: each available action is an input (description, argument schema), scored by a pointer head. New actions work zero-shot.
   - Rejected: fixed vocabulary of action types (retrain per new action).
   - A3a. Probably in between (user): some actions are inherent and can be built in to some extent. Most (if not all) need support from the harness and/or inference engine, which sets what's possible; those can be updated without retraining, which is partial motivation for flexibility about actions.
   - A3b. Built-in candidates: self-input, re-encode-with-focus, fetch (engine-level). Built-ins can still be entities in the same pointer head, with fixed learned embeddings instead of encoded descriptions, so one head covers both.
A4. "Do nothing" and "wait until X" are first-class. Something that acts in the world needs to be good at not acting.

## Heads

A5. An action is a short sequence of decisions (a few steps, not hundreds of tokens). Per argument kind:
   - which action: pointer over action entities, masked to legal ones
   - which object: pointer over input entities
   - piece of input text: span head (start/end), Decider-Locate-style
   - choice from a software-provided set: masked categorical
   - several of a set: multi-label
   - quantity/duration/priority: distribution head (quantiles or mixture)
A6. Value/confidence head. "Ask someone" becomes a learnable action when unsure.
   - Should separate aleatoric (world is noisy) from epistemic (model doesn't know); they call for different actions (05/N3).
A7. Head biases initialised from log base rates (05/B3).

## Wanted-but-illegal mass

A8. Masked-only training gives illegal logits no gradient, so their mass is noise. Proposed:
   - A8a. Two readouts from one state: desire (all shown actions, supervised ignoring legality) and choice (masked).
   - A8b. Kinds of illegal: not permitted, not possible now (both as flagged entities), doesn't exist (open-set "not in this list" slot).
   - A8c. Signals: desire on forbidden = possibly pushed off course (e.g. injection); desire on not-yet-possible = missing ability or permission; open-set mass = action space is missing something; desire/choice gap = settling for a compromise.

## Thinking

A9. Self-input action: puts a *self* event into the stream. System one becomes a system-two loop by iteration.
   - A9a. Thought content options: latent vector (expressive, not inspectable, hard to train); focus action ("look again at X", "re-read span Y with question Q", "fetch exact Z"; inspectable, supervisable); imagined outcome (predict the representation of A's result, score with value head; JEPA-style planning).
   - A9b. Long-term all three are useful. Start with focus actions plus a small latent carry; imagined outcomes later.

## Effectors and multiple actions

F1. Speech is an effector: arguments are an intent (latent or pointers) and settings; a separate small generative model renders it. The decision model never generates text.
F2. Parallel buckets for inherently independent channels (speak and walk). Each bucket is its own head with its own "nothing from this bucket"; all picked in one pass. Software declares buckets and reports channel state (busy speech channel → masked, or continue/interrupt/queue).
F3. Execution independence ≠ decision independence: independent sampling from an ambiguous state can mix plans ("say X, go right"). Fixes: argmax picks; or a shared intent pick first, then buckets in parallel conditioned on it.
F4. Within a bucket: sequential picks conditioned on earlier ones, with "no more" at each pick. Costs one head pass per extra action, not a stream step; parameters barely change.
F5. Conflicting sets (two actions on one entity) are computable: software rejects them or masks later picks.
F6. Mix: limited batching per step, buckets, plus self-input. "Just ideas, not specifying."
F7. Whether batching helps depends on how often independent actions occur; answerable only from traces. Training data must show concurrency or it goes unused (harmless).

## Decided

D7. Actions as described entities (A3). (user: "definitely what I'd want")
D8. Not acting is first-class (A4). (user: "Absolutely.")
D9. Thoughts: all three kinds long-term; start with focus actions plus small latent (A9b). (user: "your lean seems right")
D10. Buckets, intent pick, efference copy, mix of batching and self-input (F2, F3, F6, 04/L4) as direction, not spec. (user: "agreed on what you've said"; "just ideas, not specifying")

D17. Desire/choice readouts plus open-set slot (A8), answering Q3. (user: "Fine with me unless we run into something undesirable about it later.")

## Open questions

Q4. What an action entity contains (description, argument schema, bucket, cost) and how argument types bind to heads; what "zero-shot new action" really requires. (user: don't know.) Becomes important when the training data format is set (05/K1 heads stage).
