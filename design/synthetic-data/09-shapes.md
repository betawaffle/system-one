# Data shapes (draft 1, 2026-11-15)

The exact on-disk shapes for the first build (sd/07/D87–D91), written while building step 1 so the user can look them over. Field names are JSON; `?` marks optional fields.

## Layout

Z1. Everything lives in `<data>/grammar/`, files only (sd/07/D91: no SQLite as the source of truth here):
```
grammar/
  objects/ab/abcd….json   content blobs, named by sha256 of their bytes
  versions.jsonl          append-only version records (Z2)
  events.jsonl            append-only authoring events (sd/07/E1)
  lexicon/base-<hash>.jsonl   the imported lexicon (Z6), one file per import
```
Z1a. At start, ganglion reads `versions.jsonl` into memory: the latest record per id is the head. The grammar is small enough for this; the lexicon base is loaded once per import.

## Version records

Z2. One line per version:
```json
{"schema":1, "id":"frame:give", "kind":"frame", "hash":"…", "parent":"…", "by":"user", "at":"2026-11-15T…Z", "why":"edit", "client":"ganglion/write", "event":"…"}
```
  - `hash`: sha256 of the content blob; the content has no id or metadata in it (sd/07/O2a)
  - `parent`: the previous version's hash, empty for the first
  - `why`: `create`, `edit`, `fix`, `retire`, `import`
  - `event`: the authoring event that caused it, if any (sd/07/E3)
Z2a. Retiring writes a record with `why: "retire"` and the last content's hash; the id then has no head.

## Content shapes

Z3. Canonical JSON: Go's encoder over fixed structs (fields in declared order, map keys sorted), no indentation, no trailing newline. The hash is over exactly those bytes.

Z4. Kinds and their content:
```json
type:      {"name":"artifact", "parent":"physical", "explainer":"A made thing."}
attribute: {"name":"age", "applies_to":["physical"], "scale":["young","old"],
            "words":{"young":{"person":"young","artifact":"new"}, "old":{"*":"old"}}}
frame:     {"name":"give", "title":"Give", "explainer":"…",
            "roles":[{"name":"giver","type":"person","generic":"agent","explainer":"…","optional":false}],
            "verbs":["give"], "converse":["get","receive"]}
entity:    {"type":"person", "name":"Mara", "pronouns":"she"}        (fillers for meanings)
entity:    {"type":"artifact", "noun":"book"}
lex:       {"lemma":"give", "pos":"verb", "forms":{"pres3":"gives","past":"gave","pastp":"given","ing":"giving"}}
template:  (Z5)
snapshot:  {"base":"<lexicon base hash>", "heads":[["frame:give","…"], …]}   (sorted by id)
```
Z4a. Pronoun sets are named (`she`, `he`, `they`, `it`), and the table of forms lives in code (sd/05/H4).
Z4b. A `lex` object is an override of the base lexicon (sd/05/D74): it exists only once a form is fixed by hand or a word isn't in the base.

## Templates

Z5. Template content:
```json
{"kind":"S",
 "means":{"frame":"give", "vars":{"giver":"g","thing":"t","recipient":"r"},
          "adds":[{"key":"purpose","value":"gift","optional":false}]},
 "supports":{"tense":["past","present","future"], "polarity":["positive","negative"], "mood":["statement","question"]},
 "needs":[{"when":{"tense":"future"}, "modifier":"time"}],
 "frozen":["modals"],
 "tree":{"t":"seq","kids":[…]},
 "source":{"text":"Tom got the book as a gift from Mara.", "event":"…"}}
```
  - `needs`: from Sounds Odd (sd/08/D98)
  - `frozen`: what a frozen template waits on (sd/05/D80); empty when not frozen
Z5a. Nodes (sd/04/N2): every node has `t` and may have `label` and `realises`.
```json
{"t":"word", "text":"as a gift", "realises":"e.purpose"}
{"t":"lex",  "lemma":"get", "label":"vg", "realises":"e", "converse":true}
{"t":"slot", "kind":"REF", "var":"r", "label":"subj", "form":"name|pron|noun|poss"}
{"t":"opt",  "if":"e.occasion", "kids":[…]}
{"t":"seq",  "kids":[…]}
```

## Lexicon base

Z6. `lexicon/base-<hash>.jsonl`, one entry per line, built by `ganglion lexicon import` from the raw downloads (sd/05/S1):
```json
{"lemma":"child", "pos":"noun", "forms":{"plural":"children"}, "src":"wordnet:noun.exc", "doubt":false}
{"lemma":"hour", "pos":"noun", "an":true, "src":"cmudict"}
```
Z6a. Rules fill regular forms; the exception lists override them; `doubt` marks forms the rules guessed (sd/05/P1d.2). The file's hash goes into each snapshot, so a dataset records exactly which lexicon it used.

## Decided

(none yet)

## Open questions

Q71. These shapes for the first build? Not urgent: they're cheap to change until real data exists. Comment on anything that looks off.
