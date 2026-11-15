# Inflection and agreement (draft 1, 2026-11-15)

What the Go generator (sd/03/D73) needs in order to turn lemmas and features into correct English words, and where the data comes from.

## What has to be covered

H1. Nouns: plural (regular "-s/-es/-ies"; irregular child→children, mouse→mice; same-form sheep; uncountable rice, advice; plural-only scissors), possessive ("Tom's", "the boys'").
H2. Verbs: third-person present ("gives", "goes", "tries"), past, past participle, "-ing" form. Spelling rules: doubling (stop→stopping), "-ie"→"-ying" (lie→lying), "-c"→"-cking" (panic→panicking). About 200 common irregular verbs (give/gave/given).
H3. Adjectives: comparative and superlative ("-er/-est" or "more/most"; irregular good/better/best; doubling big→bigger; happy→happier).
H4. Pronouns: a table by person, number, case and pronoun set: subject (she), object (her), possessive determiner (her), possessive pronoun (hers), reflexive (herself). Singular "they" takes the plural verb form ("they are").
H5. "a" vs "an" goes by sound, not letter: "an hour", "a university", "an MBA".
H6. The verb group (sd/04/C1): tense × aspect × polarity × mood, with auxiliaries, "do" insertion, "not" placement, inversion, and the copula exceptions (sd/04/C1b).
H7. Agreement:
  - subject and verb: number and person (sd/04/F2)
  - determiner and noun: this/these, a + singular countable, much/many
  - pronoun and its referent: the referent's pronoun set and number (sd/04/F4)
H8. Surface: capitals at the sentence start, end punctuation, spacing, and contractions ("didn't" vs "did not").

## Approach

P1. Options:
  1. Write the rules in Go, plus exception tables as data.
     + small, readable, fast, no dependencies
     − exceptions have to come from somewhere (P2)
  2. Port SimpleNLG's morphology (Java) or Grammatical Framework's English resource grammar.
     + proven rules
     − a port either way, and both bring their own lexicon formats
  3. Precompute full inflection tables from an existing lexicon and only look them up.
     + no rules at generation time
     − unknown words have no forms; licences follow the data
P1a. I lean a mix of 1 and 3: Go rules propose the forms **when a word is added to the lexicon**, exception data overrides them, the user vets the forms once, and the entry stores them explicitly. Generation is then pure table lookup: fast, deterministic, and never guessing. It's the same pattern as everything else here: code proposes, the user vets once, code reuses.
P1b. The verb group (H6) and agreement (H7) stay as rules in Go, since they're combinations, not word lists. SimpleNLG and GF serve as references to check those rules against, not as dependencies.

## Data sources

S1. Candidates for exceptions and pronunciation:
  - WordNet's exception lists (irregular nouns, verbs, adjectives): permissive licence.
  - CMU Pronouncing Dictionary for "a/an" (first sound): permissive licence.
  - UniMorph English, Wiktionary: full tables, but CC BY-SA (share-alike).
  - NIH SPECIALIST Lexicon (SimpleNLG's default): large, with inflections; licence to check before use.
S1a. I lean WordNet and CMUdict as seeds, with UniMorph or Wiktionary only as a cross-check that flags disagreements for the user. Lexicon data lives in ganglion's data directory, not git (as for prose datasets), but its licence still carries into generated data, so each entry records its source (provenance, sd/04/N1a).

## Lexicon entries

L1. An entry: lemma, part of speech, features (countable, plural-only, comparison style), all inflected forms, an "a/an" override, source, vetting state.
L2. Proper names are entries too, with their pronoun set given explicitly (Mara: she/her, Tom: he/him, Sam: they/them). Pronouns are never inferred from a name. This replaces "female" and "male" in sd/02/L2 (flagged in Q49).
L3. Adding a word in the UI shows its proposed forms in one row ("give: gives, gave, given, giving"). The user accepts or fixes them, and that's the only time they see them.

## Verb group scope

V1. First version: tense (past, present, future) × aspect (simple, progressive, perfect) × polarity × mood (statement, question, command). That's "gives / is giving / has given / will have given / didn't give / Did … give? / Give …!".
V2. Later: modals (can, must, might), passive voice, perfect progressive, tag questions. (T11)

## Register

R1. Some choices are style, not meaning: contractions ("didn't"), US vs UK spelling ("color/colour"), collective agreement ("the team is/are"). I'd make these generation features. They vary in samples and bulk output like tense does, but they don't count as meaning.

## Tests

X1. Table-driven Go tests: each vetted lexicon entry and each verb-group combination for a few verbs becomes a golden case. Samples the user marks wrong (sd/03/A16c) that trace to inflection or agreement become new cases.

## Future topics

T11. Modals, passive, perfect progressive, tag questions (V2).
T12. Coordination agreement: "Mara and Tom are", "Mara or the boys are" (nearest wins), needed once REF templates coordinate.

## Decided

(none yet)

## Open questions

Q47. Rules propose forms when a word is added, the user vets once, and generation only looks up (P1a)? I lean yes.
Q48. Seed exceptions from WordNet and CMUdict, cross-checking against UniMorph or Wiktionary (S1a)? I lean yes.
Q49. Entities and names carry an explicit pronoun set, replacing female/male in sd/02/L2 (L2)? I lean yes. Amends sd/02/L2 if accepted.
Q50. Contractions, spelling dialect and collective agreement as style features that vary but don't count as meaning (R1)? Which dialect is the default? I lean US.
Q51. First verb-group scope as in V1? I lean yes.
