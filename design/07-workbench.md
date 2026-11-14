# Workbench app (draft 1, 2026-11-14)

## Requirements (user)

R1. Built before any training starts.
R2. Standalone web app, reachable over Tailscale.
R3. See what's going on, and experiment with the models involved.
R4. Very snazzy, polished, somewhat minimalist. Consistent styling and colour palette across the whole app.
R5. Live interactions; websockets where appropriate.
R6. A place for data labeling and/or review as necessary.

## Likely areas (proposal)

A1. Data: browse events by domain/schema, including example command executions so the first domain can be seen (06/D27); dedupe and split stats (06/G7); per-field base rates (05/B1).
A2. Encoder playground: pick or paste an event, see its slots, masked-field predictions (06/W3), nearest neighbours (06/V5), the nested-slot curve for that event (06/V3).
A3. Training runs, live: losses, collapse metrics (06/V4), probe scores (06/V2), checkpoints; compare runs.
A4. Labeling and review: queues of items needing a human judgment (halt labels for inaction, 05/X1; teacher disagreements, 05/X4; probe failures), with keyboard-first flow.
A5. Later: stream and heads views (desire/choice, surprise, 03/A8, 04/L4), replay/counterfactual browser (05/N1).

## Shape (proposal)

S1. Model work lives in Python (PyTorch on ROCm). Training runs as separate processes, so the UI never takes a run down and vice versa.
S2. Options for the server:
   - S2a. Python server (websockets) next to the models.
      + one language; inference in-process
      − UI server shares fate with model code
   - S2b. Go server for UI, data and websockets; Python workers behind a process boundary.
      + matches the Go-core, Python-models split you've preferred; robust server
      − two languages; a protocol between them
   - S2c. I lean S2b.
S3. Frontend without npm: plain modules, or a single static esbuild binary if TypeScript is wanted.
S4. A small design system first (palette, type scale, components) so every page shares it (R4); light and dark.

## Decided

D25. Server shape S2b: Go server for UI, data and websockets; Python workers behind a process boundary. Answers Q17. (user: "S2c seems good"; read as agreeing with the S2c lean, i.e. S2b)
D31. Design A1 and A2 in detail now (08). Answers Q19. (user: "Yes.")
D26. Areas: A1 + A2 first, A3 when training starts, A4 when there's something to label. Answers Q18. (user: "Agreed.")

## Open questions

(none for this note; detailed design continues in 08)
