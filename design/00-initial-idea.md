# Initial idea (draft 1, 2026-11-14)

Started from Perplexity's Decider v1.3 write-up (https://www.perplexity.ai/hub/blog/perplexity-decider), itself inspired by TypeSafe's Jev ("System One Model"): a classifier on an LLM backbone, one forward pass, readout heads at marker tokens, probabilities instead of text.

- Custom local model design. Topics of interest: structure, specialized tokenization, domain separation between inputs, output heads that aren't generative.
- Oriented around *actions* rather than output. Output (in the chat sense) is just one of many potential actions.
- Analogy: humans aren't output based. They act (muscle control) to do things like speaking and typing.

## Ground rules

R1. A model is for judgements, not things that are trivially computable with a single right answer.
R2. Software is for things that are computable.
R3. Whatever we make has to be trainable and runnable on the Framework.

## Reference points

P1. Decider attention patterns: everything-sees-everything (v1.2, answers depend on what else is asked); isolated questions (v1.3 default: state sees all questions, each question sees state + itself); fully cacheable (state doesn't see questions; reusable KV, but "letting the state see the questions matters a lot for several tasks").
P2. Decider head: `<|marker|>` token, linear head initialised from the LM head's rows for option labels; KL distillation from the teacher; Flash is a 4B open-weight model.
P3. JEPA: predict representations, not tokens; action-conditioned world models (V-JEPA 2-AC, JEPA world models for planning, arXiv 2512.24497). "A good rough direction when we can use it."
P4. data2vec 2.0: one full-view target per sample, several masked student views, gradients averaged.
