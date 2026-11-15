# Synthetic data: idea (draft 1, 2026-11-15)

Numbering continues the global series in `design/` (Q, D, T). Cross-references inside this folder are `sd/<file>/<ID>`; plain `<file>/<ID>` means the parent `design/` notes.

## Goal

G1. A process that takes data the user created or vetted and uses it to make more data: "self-replicating madlibs". (user)
G2. Build a huge prose-generation dataset: data that lets plain code generate more correct prose and prose templates. (user)
G3. Start with sentences. (user)
G4. Sentences have to mean something, and the meaning has to be computable from the code that generated them. (user)
G5. Tooling lives in the ganglion UI. (user)

## Principles

P1. Meaning first, text second: code builds a meaning, then realises it as text, so the meaning is known because it came first. Text → meaning would be semantic parsing, the hard direction. (user: "Yes, exactly this.")
P2. The user writes text for a given meaning, a few variations, whatever they think of. They never have to work out what a sentence means and turn it into structure. (user)
P3. Meaning → text is probably easier for models too: it's plain generation, and text → meaning is where they make quiet mistakes. (user's suggestion, agreed)
P4. Templates are the data. Code fills the variables, including with other templates of the right kind. (user)
P5. Measure distinct templates, not rows: a million rows from 200 templates reads like 200 sentences. (user: "Exactly this.")

## Prior work to borrow from

K1. Grammatical Framework (GF): abstract syntax (meaning) plus concrete syntax (English); its resource grammar library handles English morphology and agreement. The closest existing match.
K2. English Resource Grammar with the ACE processor: broad-coverage, generates from a meaning representation (MRS). Heavy; a possible checker or source of realisations.
K3. Synchronous grammars: one derivation produces meaning and text together. The formal name for G4.
K4. VerbNet, FrameNet, PropBank: verb frames, role names, selectional restrictions, to seed frames and types.
K5. AMR (Abstract Meaning Representation): widely used, with text↔AMR datasets to mine. Loose in places, not designed for computing with.
K6. SimpleNLG: a small surface realiser; shows how little code inflection and agreement need.
K7. bAbI, CLEVR: generation from a world state, so sentences are true or false of it.

## Risks

X1. Combinatorial blandness (see P5).
X2. Pragmatic oddness: true and well-typed but strange ("Mara gave Tom a book. Mara gave Tom a book."). A world model and discourse rules come later (T10).
X3. Vetting throughput: the user is the bottleneck, so they vet templates and types, not instances, and spot-check instances.

## Future topics

T10. A world model: code builds a small world state and generates sentences true (or deliberately false) of it. Gives truth, consistency across sentences, then pronouns and multi-sentence prose. Needed once sentences become paragraphs.

## Decided

D52. Meaning first, text second (P1). (user: "Yes, exactly this.")
D53. The user authors text from a given meaning, never meaning from text (P2). (user: "I'd rather go from the meaning and write text (maybe a few variations, whatever I can think of).")
D54. Templates are the dataset; size is counted in distinct templates (P4, P5). (user)
D55. Tooling goes in the ganglion UI (G5). (user: "We're gonna add the tooling for this in the ganglion UI.")

## Open questions

(none for this note; see sd/02 and sd/03)
