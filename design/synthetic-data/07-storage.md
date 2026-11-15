# Storage in ganglion (draft 1, 2026-11-15)

How the grammar (types, frames, lexicon, templates), the authoring history and generated data are stored. Built on what ganglion already does (08/D33): content-addressed blobs in `blobs.db`, an SQLite index in `index.db`, append-only JSONL beside them (`labels.jsonl`, `blacklist.jsonl`), datasets as directories under `datasets/`. Observed in `ganglion/internal/store`: events and utterances are already hashed as sha256 of their canonical JSON.

## Grammar objects

O1. Five object kinds: type, attribute, frame, lexicon entry, template.
O2. Each object has a stable id (`template:give-1`, `lex:give.v`) and a series of versions.

## Versions

V1. A version is the object's canonical JSON, stored once as a blob under its sha256, like utterances today. Versions never change.
V2. Each version records its parent version, who made it (user, model id, import), when, and why (edit, fix from a sample, import).
V3. An index table maps id → current version (the head). An edit writes a new version and moves the head. Retiring an object is a version too (state retired). Nothing is deleted, matching the capture rules' append-only requirement (CAPTURE-REQUIREMENTS 2).
V4. History per object is the parent chain, so the UI can show what changed and roll back by writing an older content as a new version.

## Snapshots

N1. A snapshot is the sorted list of (id, head version) at one moment, stored as a blob under its own hash. It names one exact state of the whole grammar.
N2. Generation always runs against a snapshot, never against "whatever's current", so results are reproducible.
N3. Cheap: a list of hashes. One is taken per generation run; previews (sd/03/A16) use the live heads and need none.

## Generated data

G1. Generation is deterministic from (snapshot, generator code commit, settings, seed). So a dataset is first a **recipe**: `recipe.json` with those four.
G2. The rows are written beside it as compressed JSONL in `datasets/<name>/` (as prose datasets are, never in git). Each row has text, meaning, and its derivation: the template and lexicon versions used at each node, plus the style features (sd/05/D77).
G3. A fix (sd/05/D79) is: new version, new snapshot, regenerate. Which rows changed is found by scanning derivations for the superseded version. No per-row database index needed, since that would be millions of rows × dozens of dependencies.
G4. "No dataset leaves ganglion in a form that can't be regenerated" (sd/05/D79) is met by exporting the recipe with the rows. Exported rows keep their derivations, so a consumer can also filter out rows that used a version known to be wrong.
G5. Previews aren't stored, except a sample the user gives a verdict on (E1): then its derivation is stored with the verdict.

## Authoring events

E1. An append-only `synthetic.jsonl` records each authoring step as a typed event with a schema version:
  - meaning shown to the user
  - sentence written: verbatim, with how it got there (typed, pasted, an accepted model suggestion, an edited model draft: then both the draft and the final, CAPTURE-REQUIREMENTS 13)
  - alignment proposed, spans resolved (sd/03/A13)
  - template saved (links to its version)
  - sample verdict, "hey wait a minute" fix (sd/05/D74)
  - word-helper tables proposed and corrections (sd/03/A8c)
E2. Every event has who (a name claim; no logins, 08/D34), when (UTC), and which client sent it (`ganglion/authoring`, as labels have `ganglion/samples`).
E3. Versions (V2) and events point at each other, so any template can be traced back to the sentence and the meaning it came from.

## Model calls

M1. Models propose realisations, word-helper tables and span choices (sd/03/A5, A8c, A13d). Each call is logged raw per CAPTURE-REQUIREMENTS: full request and response, model and weights details, sampling settings and seed. Events reference the call by id.
M2. I'd route every model call in ganglion through one place in the Go server that does this logging, so no feature can forget it, and the planned provenance integration (CAPTURE-REQUIREMENTS 31) has one place to hook in. (Q62)

## Seed data

S1. WordNet and CMUdict are kept as downloaded under `raw/`, with their licence files and hashes (sd/05/D75).
S2. Lexicon versions made by import name the source file and its hash (V2), so the licence of every entry is traceable.

## Authored sentences as prose

U1. The user's sentences are their own prose, written to a meaning. They could also enter the utterance store (speaker human, a new kind `authored`), so prose datasets can include them, marked so they can be left out. (Q61)

## Backup

B1. All of this lives in ganglion's data directory, which is backed up with everything else. Not in git: templates are data, and prose stays out of the repo (system-one data rules).

## Future topics

T17. Publishing the grammar (templates, frames, lexicon) as an open dataset. Clean-licence sources (sd/05/D75) keep that possible.

## Decided

D86. Authoring events and model calls are logged append-only per CAPTURE-REQUIREMENTS (E1–E3, M1). Not a new decision: this applies the user's standing capture requirements.

## Open questions

Q59. Content-addressed versions with heads and snapshots (V1–V4, N1–N3)? I lean yes.
Q60. Datasets as recipes plus rows with per-row derivations; regenerate rather than patch (G1–G4)? I lean yes.
Q61. Authored sentences also go into the utterance store as kind `authored` (U1)? I lean yes.
Q62. One model-call path in the Go server that logs everything (M2)? I lean yes.
