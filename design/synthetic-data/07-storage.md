# Storage in ganglion (draft 2, 2026-11-15)

How the grammar (types, frames, lexicon, templates), the authoring history and generated data are stored. Built on what ganglion already does (08/D33): content-addressed blobs in `blobs.db`, an SQLite index in `index.db`, append-only JSONL beside them (`labels.jsonl`, `blacklist.jsonl`), datasets as directories under `datasets/`. Observed in `ganglion/internal/store`: events and utterances are already hashed as sha256 of their canonical JSON.

## Grammar objects

O1. Five object kinds: type, attribute, frame, lexicon entry, template.
O2. Each object has a stable id (`template:give-1`, `lex:give.v`) and a series of versions.
O2a. The stable id is not part of the hashed content (user: "this stable id isn't part of the hashed object, right?"). Content is hashed alone, so identical content has one hash wherever it appears, and the hash is also the template identity used for counting (sd/04/I1).
O2b. References between objects inside content use stable ids (a template's `lex give` names `lex:give.v`, not a version). A snapshot (N1) resolves them to versions, so fixing a word doesn't force a new version of every template that uses it.

## Versions

V1. A version's content is the object's canonical JSON, without id or metadata, stored once as a blob under its sha256, like utterances today. Content never changes.
V2. A version record ties it together: stable id, content hash, parent version, who made it (user, model id, import), when, and why (edit, fix from a sample, import). Records are append-only (JSONL as the source of truth, indexed in SQLite; B1d.2), separate from the content blob, so two edits that arrive at the same content share the blob but keep their own records.
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
B1a. Checked 2026-11-15 (user asked): the nightly backup timer is active and its last runs succeeded, and its path list includes `~/.local/share`, so ganglion's data directory is covered.
B1b. Gap found: the backup copies SQLite files with rsync while ganglion may be writing them, which can leave an inconsistent copy of `index.db` or `blobs.db`. The append-only JSONL files are safe. Fix: ganglion writes a consistent copy (SQLite's `VACUUM INTO`, or the backup API) into its data directory before the backup window, and that copy is what a restore uses. (Q63) Amended by B1d.
B1c. Would WAL mode help? (user) Ganglion already opens both databases in WAL mode (`internal/store/store.go`). WAL keeps readers and the writer apart inside SQLite, but a file copy isn't a SQLite reader: rsync copies the main file and the `-wal` file at different moments, and a checkpoint in between leaves a pair that don't match. So it doesn't fix this, and it adds a second file that has to be copied in step.
B1d. The user's concern with B1b: it's fragile, because the backup time could change without ganglion knowing. Two ways out that don't depend on timing:
  1. **The backup takes the copy.** The backup service runs a pre-step that asks ganglion for a consistent copy (SQLite's online backup API) right before rsync starts. The backup owns the timing, so moving the timer moves the copy with it.
  2. **Nothing precious lives only in SQLite.** For the synthetic data, the source of truth is append-only files: version records and authoring events as JSONL, content as blob files. SQLite is only an index that can be rebuilt from them (as 08/D33 already frames it). A broken index copy then costs a rebuild, not data.
B1e. I lean both: 2 for everything new here, so correctness never depends on the backup, and 1 for the existing `index.db` and `blobs.db`, whose contents I haven't checked are fully rebuildable from `raw/`. (Q63, revised)
B1f. Done 2026-11-15 (user: "Let's do it, thanks."): `~/projects` added to the backup path list. A dry run lists it without errors (about 115k files, 9.8 GB in all).

## Future topics

T17. Publishing the grammar (templates, frames, lexicon) as an open dataset. Clean-licence sources (sd/05/D75) keep that possible.

## Decided

D86. Authoring events and model calls are logged append-only per CAPTURE-REQUIREMENTS (E1–E3, M1). Not a new decision: this applies the user's standing capture requirements.
D87. Content-addressed versions with heads and snapshots (V1–V4, N1–N3), with the stable id outside the hashed content (O2a). Answers Q59. (user: "Yes.")
D88. Datasets as recipes plus rows with per-row derivations; regenerate rather than patch (G1–G4). Answers Q60. (user: "Yes.")
D89. Authored sentences also go into the utterance store as kind `authored` (U1). Answers Q61. (user: "Yes.")
D90. One model-call path in the Go server that logs everything (M2). Answers Q62. (user: "Yes.")

## Open questions

Q63. Revised: source of truth in append-only files with SQLite as a rebuildable index, plus a pre-backup step in the backup service for the existing databases (B1d, B1e)? I lean both. (user on the original B1b: "I'm fine with Q63, but it feels fragile, since the backup time could change without ganglion knowing.")

## Superseded

Q59 (answered by D87). Content-addressed versions with heads and snapshots?
Q60 (answered by D88). Datasets as recipes plus rows with derivations?
Q61 (answered by D89). Authored sentences as utterances?
Q62 (answered by D90). One logged model-call path?
