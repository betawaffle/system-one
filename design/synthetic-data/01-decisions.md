# Synthetic data: decisions (draft 3, 2026-11-15)

Index; detail lives in the topic notes. Numbers continue the global series in `design/`.

## By topic

- sd/00 idea: D52 meaning first, D53 user writes text from meaning, D54 templates are the dataset (count distinct templates), D55 tooling in ganglion.
- sd/02 meaning language: D56 our own small typed language, D108 implication rules (habit, refuses), D57 UI explains roles, D59 plain role names plus generic names with explainers, D60 grow frames from scratch, D61 humour and boundary tests on purpose.
- sd/03 authoring: D58 word-by-word helper, D62 trees all the way down, D63 flag extra meaning and ask, D64 states and properties first, then transfer, D65 the extra-meaning mechanism (A13), D72 live fast samples while editing (A16), D73 generator in Go inside ganglion.
- sd/04 template trees: D66 node types and kinds, D67 code-realised verb groups, D68 derived REL templates, D69 reference choice by code, D70 JSON stored / text shown, D71 lemma classes count once but every lemma generates, D103 determiners in the reference; possessives are owners.

- sd/05 inflection: D74 rules propose, look up at generation, vet as needed, D75 WordNet + CMUdict, clean licence (no share-alike), D76 explicit pronoun sets (amends sd/02/L2), D77 style features, US default, D78 first verb-group scope, D79 retroactive fixes and doubt flags, no pre-vetting, D80 unsupported verb groups saved frozen and usable.

- sd/06 first frames: D81 frames F1–F6, D82 properties as attribute + value, D83 "not old" ≠ "new", D84 adjective order by code, D85 starting types and attributes, D109 rods have length, not height, D100 age comparisons present-only until the world model, D102 age only increases.
- sd/07 storage: D86 authoring events and model calls logged per the capture requirements, D87 content-addressed versions (id outside the hash), heads, snapshots, D88 datasets as recipes plus derivations, D89 authored sentences as utterances, D90 one logged model-call path, D91 append-only source of truth plus a snapshot step in the backup (done).

- sd/08 writing screen: D92 one page, D93 Next from coverage gaps, D94 align while typing, D95 suggestions automatic / helper on request, D96 keys as K1, D97 mock-up first, D98 Sounds Odd makes the generator add a modifier, D99 "!" is extra meaning, D101 Save holds back broken lines, D104 verdicts carry over by sentence, D105 Coverage page, D106 Judged list, D107 Fine, But… verdict.

- sd/09 data shapes: (none yet)

- sd/10 model: D110 model first pass, user reviews, D111 llm-server with gpt-oss-120b, D112 one call per sample, D113 few-shot off to start, prompts versioned, D114 accept one at a time, D115 llm-server at boot, D116 low effort first, D117 try narrow checks, keep only if they beat v2, D118 Jev is the first pass, never asked the same thing twice.

## Open questions (all)

- Q71 data shapes for the first build (sd/09). Comment on anything off; cheap to change for now.
## Future topics (all)

T10 world model (sd/00). T11 modals, passive, more aspects (sd/05). T12 coordination agreement (sd/05). T13 superlatives, T14 generics, T15 existentials, T16 numbers and quantities, T18 editing types and entities in the UI (sd/06). T17 publishing the grammar (sd/07).
