# Template tree format (draft 1, 2026-11-15)

Trees all the way down (sd/03/D62). This note proposes what a template is, what its nodes are, how features flow, and how the tree carries its meaning.

## Template

N1. A template is a header plus one tree.
N1a. Header:
  - `id`
  - `kind`: what it produces and so which slots it can fill (N3)
  - `means`: the meaning pattern it realises, with variables (sd/02/L5)
  - `supports`: which feature values it handles (tense, polarity, mood; sd/02/L4)
  - provenance: author (the user or a model id), source sentence, created, vetting state
N1b. Code may use a template for a meaning only if the meaning matches `means` and its feature values are in `supports`.
N1c. Parts of `means` marked `?` are optional: matching ignores them when the meaning lacks them, and the tree wraps what realises them in `opt` (N2.5).

## Nodes

N2. Node types:
  1. `word`: fixed text ("as", "from"). Punctuation and capitals are code's job, never words.
  2. `lex`: a word code inflects: a lemma plus features ("give", tense from the event, agreeing with the subject). It can name a class of same-meaning lemmas instead of one (sd/03/A8b).
  3. `slot`: asks for a kind and a meaning variable (`REF(a)`). Code fills it with another template of that kind whose `means` matches what the variable is bound to. This is templates filling templates (sd/03/A9).
  4. `seq`: children in order. The default.
  5. `opt`: children included only if a meaning part is present (e.g. the occasion). Lets one template cover the meaning with and without a modifier.
N2a. Every node may carry a label (`subj`, `vg`, `obj`) used for agreement and clause handling (F2, C1).

## Kinds

N3. A starting set; more get added as needed:
  - `S`: a whole sentence (a clause plus capitals and end punctuation, added by code)
  - `CLAUSE`: an event with its roles
  - `REF`: a way to refer to an entity: name, pronoun, "a/the {noun}", "the {DESC} {noun}", "the {noun} {REL}"
  - `DESC`: a property phrase ("tall", "very old", "made of glass")
  - `REL`: a clause describing an entity, with one role as the gap ("who Tom met")
  - `MOD`: an event modifier ("on his birthday", "in the mail")

## Features

F1. Features flow down from the meaning: tense, polarity and mood from the event, and number, gender and person from the entity.
F2. Agreement is by label: `lex` in `vg` agrees with `subj` by default. A slot passes up the features of what fills it (a plural REF makes the verb plural).
F3. Case comes from position: the `subj` REF is nominative ("he"), others take object case ("him"). Possessives are a REF form ("his").
F4. Reference choice is code's, from context: first mention gets a name or "a {noun}", later mentions a pronoun or "the {noun}". A template can pin one (always a name, say). This is where "a" vs "the" comes from (sd/03/A8a).

## Clause handling

C1. Verb groups: `vg` is a clause-level node code realises from tense, aspect, polarity and mood. That covers auxiliaries ("did give", "has given"), negation ("didn't give") and question inversion ("Did Mara give…?"), the way SimpleNLG does it (sd/00/K6). One authored template then covers every combination in `supports`.
C1b. The copula ("be") negates and inverts without "do": "Mara isn't tall", "Is Mara tall?", never "Does Mara be tall?". Main-verb "have" varies by dialect ("Have you a pen?" vs "Do you have a pen?"); I'd use the "do" forms. Both are rules in the verb-group code, not template work (E3).
C1a. A template whose wording only works in some combinations (an idiom that can't be negated) lists only those in `supports`.
C2. Gaps: a REL is a CLAUSE with one role marked `gap`; code fronts "who/that/which" and drops that role ("the book [that] Mara gave Tom"). Q42 asks whether REL templates are written separately or derived from CLAUSE templates.

## Meaning on the tree

M1. Every node that isn't grammar carries a `realises` link to the meaning part it expresses. A slot realises its variable's entity, a `lex` realises the frame or a property, and a `word` like "as a gift" realises `purpose=gift`.
M2. The template's meaning is the union of its `realises` links, and a filled tree's meaning is the merge of all of them. This is what makes meaning computable from the generating code (sd/00/G4).
M3. Alignment (sd/03/A13) is filling in `realises` links: traced words get one from the start, and each untraced span resolved in A13d gets one too. A template with any non-grammar node lacking a link isn't finished.

## Examples

E1. "Mara gave Tom a book." (sd/03/E1)
```
template give-1
  kind: S
  means: e = give(giver: a, thing: t, recipient: r)
  supports: tense *, polarity *, mood statement|question
  tree:
    subj: REF(a)
    vg:   lex give          realises e
    obj:  REF(r)
    obj2: REF(t)
```
"Mara didn't give Tom a book." and "Did Mara give Tom a book?" come from the same template (C1).

E2. "Tom got <title> as a gift from Mara on his birthday." (sd/03/E2)
```
template give-2
  kind: S
  means: e = give(giver: a, thing: t, recipient: r), e.purpose = gift, e.occasion = o?
  supports: tense *, polarity *, mood statement|question
  tree:
    subj: REF(r)
    vg:   lex get            realises e (converse viewpoint)
    obj:  REF(t)
    word  "as a gift"        realises e.purpose = gift
    word  "from"
    REF(a)
    opt e.occasion:
      MOD(o)                 "on his birthday": "his" is REF(r) again, a later mention (F4)
```
The title comes from REF(t): a book REF template may use the title.

E3. "The book is old." From the first frame family (sd/03/D64).
```
template prop-1
  kind: S
  means: s = has-property(holder: x, property: p)
  supports: tense *, polarity *, mood statement|question
  tree:
    subj: REF(x)
    vg:   lex be             grammar (the copula carries no meaning of its own)
    pred: DESC(p)            realises s
```
"The book isn't old." and "Is the book old?" follow C1b. The same DESC templates fill "the {DESC} {noun}" REFs, so "the old book" reuses p.

## Identity and counting

I1. A template's identity is a hash of its normalised tree and `means` (ignoring ids and provenance), so exact duplicates are caught.
I2. Two templates that differ only in a lemma from the same "same meaning" class (sd/03/A8b) are one template with a `lex` class (N2.2), not two. That keeps the distinct-template count honest (sd/00/P5).

## Syntax

S1. Stored as JSON (the canonical form); shown and edited in a compact text form like E1 and E2.

## Decided

(none yet)

## Open questions

Q40. The node types (N2) and starting kinds (N3)? I lean yes as listed, adding kinds as frames need them.
Q41. Verb groups realised by code from features (C1), rather than templates spelling out auxiliaries? I lean code.
Q42. REL templates: written by the user, or derived from CLAUSE templates by marking a gap (C2)? I lean derived, with the user vetting the result, since otherwise every frame needs a second set.
Q43. Reference choice by code from context, with templates able to pin it (F4)? I lean yes.
Q44. JSON stored, compact text shown (S1)? I lean yes.
Q45. Same-meaning lemma classes count as one template (I2)? I lean yes.
