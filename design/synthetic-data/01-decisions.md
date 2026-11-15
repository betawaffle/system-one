# Synthetic data: decisions (draft 3, 2026-11-15)

Index; detail lives in the topic notes. Numbers continue the global series in `design/`.

## By topic

- sd/00 idea: D52 meaning first, D53 user writes text from meaning, D54 templates are the dataset (count distinct templates), D55 tooling in ganglion.
- sd/02 meaning language: D56 our own small typed language, D57 UI explains roles, D59 plain role names plus generic names with explainers, D60 grow frames from scratch, D61 humour and boundary tests on purpose.
- sd/03 authoring: D58 word-by-word helper, D62 trees all the way down, D63 flag extra meaning and ask, D64 states and properties first, then transfer, D65 the extra-meaning mechanism (A13), D72 live fast samples while editing (A16), D73 generator in Go inside ganglion.
- sd/04 template trees: D66 node types and kinds, D67 code-realised verb groups, D68 derived REL templates, D69 reference choice by code, D70 JSON stored / text shown, D71 lemma classes count once but every lemma generates.

- sd/05 inflection: D74 rules propose, look up at generation, vet as needed, D75 WordNet + CMUdict, clean licence (no share-alike), D76 explicit pronoun sets (amends sd/02/L2), D77 style features, US default, D78 first verb-group scope, D79 retroactive fixes and doubt flags, no pre-vetting, D80 unsupported verb groups saved frozen and usable.

- sd/06 first frames: D81 frames F1–F6, D82 properties as attribute + value, D83 "not old" ≠ "new", D84 adjective order by code, D85 starting types and attributes.
- sd/07 storage: D86 authoring events and model calls logged per the capture requirements.

## Open questions (all)

- Q59 content-addressed versions, heads, snapshots (sd/07/V, N). Lean yes.
- Q60 datasets as recipes plus rows with derivations; regenerate, don't patch (sd/07/G). Lean yes.
- Q61 authored sentences into the utterance store as kind `authored` (sd/07/U1). Lean yes.
- Q62 one logged model-call path in the Go server (sd/07/M2). Lean yes.

## Future topics (all)

T10 world model (sd/00). T11 modals, passive, more aspects (sd/05). T12 coordination agreement (sd/05). T13 superlatives, T14 generics, T15 existentials, T16 numbers and quantities (sd/06). T17 publishing the grammar (sd/07).
