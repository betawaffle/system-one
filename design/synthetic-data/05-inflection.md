# Inflection and agreement (draft 2, 2026-11-15)

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
P1c. Vetting is as needed, not up front: start by assuming the rules are correct, and give the user a way to say "hey wait a minute". (user) Any word shown anywhere (a sample, a template, a generated row) can be clicked to open its entry and fix a form.
P1d. Why pre-vetting might be wanted (the user asked if there's a reason they're missing): a wrong form ("childs") isn't one bad sentence. It's in every row that uses the word, and bulk generation hides it, because nobody reads most rows. Two things make as-needed safe anyway:
  1. Retroactive fixes: every generated row records which lexicon entry versions it used (provenance), so fixing a form finds every affected row and regenerates or drops it. Nothing is lost by finding the mistake late.
  2. Doubt flags: the rules know when they're guessing (word not in the seed data, a/an with no pronunciation, doubling that depends on stress like "visit/visited" vs "commit/committed", "-ie" and "-c" endings). Those forms go in a small "maybe check" list instead of being silently accepted. Confident forms need no look at all.
P1e. With P1d, pre-vetting buys nothing that a later fix doesn't, as long as no dataset leaves ganglion before its rows can be regenerated. (Q52)
P1b. The verb group (H6) and agreement (H7) stay as rules in Go, since they're combinations, not word lists. SimpleNLG and GF serve as references to check those rules against, not as dependencies.

## Data sources

S1. Candidates for exceptions and pronunciation:
  - WordNet's exception lists (irregular nouns, verbs, adjectives): permissive licence.
  - CMU Pronouncing Dictionary for "a/an" (first sound): permissive licence.
  - UniMorph English, Wiktionary: full tables, but CC BY-SA (share-alike).
  - NIH SPECIALIST Lexicon (SimpleNLG's default): large, with inflections; licence to check before use.
S1a. I lean WordNet and CMUdict as seeds, with UniMorph or Wiktionary only as a cross-check that flags disagreements for the user. Lexicon data lives in ganglion's data directory, not git (as for prose datasets), but its licence still carries into generated data, so each entry records its source (provenance, sd/04/N1a).
S1b. Keep the licence clean (user). Reading: permissive sources only. WordNet (Princeton's own licence, BSD-like) and CMUdict (BSD-style) are in. Share-alike sources (UniMorph, Wiktionary) stay out, not even as a cross-check, since adopting a form they flagged would be copying from them. SPECIALIST only if its terms turn out permissive. Our rules, the user's corrections and output from Apache-2.0 models are fine.
S1c. The whole lexicon is bulk-imported from WordNet (about 150k lemmas with parts of speech), with forms from the rules plus exception lists. Words the user writes that aren't in it (names, new words) are added automatically by alignment (sd/03/A13). Adding a word by hand should be rare. (user: "Hopefully I won't need to do this often...")

## Lexicon entries

L1. An entry: lemma, part of speech, features (countable, plural-only, comparison style), all inflected forms, an "a/an" override, source, vetting state.
L2. Proper names are entries too, with their pronoun set given explicitly (Mara: she/her, Tom: he/him, Sam: they/them). Pronouns are never inferred from a name. This replaces "female" and "male" in sd/02/L2 (flagged in Q49).
L3. Adding a word in the UI shows its proposed forms in one row ("give: gives, gave, given, giving"). The user accepts or fixes them, and that's the only time they see them. Amended by L3a.
L3a. Words are mostly added automatically (S1c) and their forms are trusted until the user says otherwise (P1c). An entry's forms show only when the user opens it, or when they're in the doubt list (P1d.2). (user's reaction to L3: *nervous laughter*)

## Verb group scope

V1. First version: tense (past, present, future) × aspect (simple, progressive, perfect) × polarity × mood (statement, question, command). That's "gives / is giving / has given / will have given / didn't give / Did … give? / Give …!".
V2. Later: modals (can, must, might), passive voice, perfect progressive, tag questions. (T11)
V2a. Is V2 harder? (user) The word forms are barely harder: a few more rules each. The hard part is elsewhere:
  - perfect progressive ("has been giving"): trivial, just one more aspect
  - tag questions ("…, didn't she?"): mechanical, flip the polarity and copy the auxiliary, with a few quirks ("I'm right, aren't I?")
  - modals: the forms are easy (modal + bare verb, no "do"; "can't", "mustn't"). The meaning is harder: possibility, necessity, permission and ability are different things, and "could" is sometimes past ability and sometimes possibility. The meaning language needs a modality feature first.
  - passive ("The book was given to Tom"): the forms are easy (be + past participle). The template side is harder: the "thing" becomes the subject, the giver becomes optional ("by Mara"), and not every verb allows it ("A book was had"). It's a derivation, like REL templates (sd/04/D68).
V2b. So V2 waits on meaning and template work, not on inflection.

## Register

R1. Some choices are style, not meaning: contractions ("didn't"), US vs UK spelling ("color/colour"), collective agreement ("the team is/are"). I'd make these generation features. They vary in samples and bulk output like tense does, but they don't count as meaning.

## Tests

X1. Table-driven Go tests: each vetted lexicon entry and each verb-group combination for a few verbs becomes a golden case. Samples the user marks wrong (sd/03/A16c) that trace to inflection or agreement become new cases.

## Future topics

T11. Modals, passive, perfect progressive, tag questions (V2).
T12. Coordination agreement: "Mara and Tom are", "Mara or the boys are" (nearest wins), needed once REF templates coordinate.

## Decided

D74. Rules propose forms, entries store them, generation only looks up; vetting is as needed, with "hey wait a minute" anywhere a word shows (P1a, P1c). Answers Q47. (user: "I like P1a, except that the vetting can be as-needed. Like start by assuming the rules are correct, but give me the ability to go 'hey wait a minute'.")
D75. Seed from WordNet and CMUdict, and keep the licence clean (S1b). Answers Q48. (user: "S1a is fine, but let's do our best to keep our license clean.") Read as: no share-alike data at all, cross-check included; see S1b.
D76. Entities and names carry an explicit pronoun set (L2). Answers Q49. (user: "Fine with me.") Amends sd/02/L2.
D77. Contractions, spelling dialect and collective agreement are style features, not meaning (R1); the default dialect is US. Answers Q50. (user: "R1: Agreed." "Q50: US.")
D78. First verb-group scope as in V1. Answers Q51. (user: "Fine.")

## Open questions

Q52. Retroactive fixes through provenance plus doubt flags, instead of pre-vetting (P1d, P1e)? I lean yes. The one condition: no dataset leaves ganglion in a form that can't be regenerated.

## Superseded

Q47 (answered by D74). Rules propose, user vets once, generation looks up?
Q48 (answered by D75). Seed from WordNet and CMUdict, cross-check UniMorph/Wiktionary?
Q49 (answered by D76). Explicit pronoun sets on entities?
Q50 (answered by D77). Style features and default dialect?
Q51 (answered by D78). First verb-group scope?
