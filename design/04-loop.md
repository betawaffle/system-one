# Loop, halt, wakefulness (draft 1, 2026-11-14)

## Loop

L1. Mostly a continuous loop over an event stream, "probably with caveats". Caveat as stated: like a CPU, it needs a halt so it doesn't waste energy when unnecessary.
L2. Core primitives: actions (one of which is self-input, 03/A9) and the absence of action. Everything else is arguments.
L3. Waiting on a result needs no "continue": completion is an event, so the model halts and wakes on it. Continuing while an action runs = self-input.
L4. Efference copy: actions taken come back as an action-taken event (immediate) and later an action-result event. Predicting result from taken is a JEPA objective (05/J3); the mismatch is a surprise signal and a reason to wake or think more.

## Halt

H1. Halt is whatever isn't an action: "do nothing yet". No explicit halt needed; every step without action ends in halt-until-any.
H2. Plain halt wakes on any non-ambient event (02/E4). Ambient types wake only if named.
H3. Explicit arguments only narrow: wake mask (event types), timer, maybe a condition. Software checks them.
H4. Masked events are queued, not dropped (pending interrupts). On wake: the backlog, or count + summary if large.
H5. Non-maskable event types always wake; the model can't make itself unreachable.
H6. Interrupt coalescing: events arriving during a step batch into the next step.
H7. Idle depth: shallow halt keeps stream state in fast memory; deep halt stores it. Model could pick depth.
H8. "No more actions this step" (03/F4) is the same decision as halt, inside a step.

## Tiers of wakefulness

W1. "Does this event deserve to wake the big part?" is a judgment when the mask is too coarse, so it's a model: tiny, always on, like a peripheral controller. Fits the family idea (02/N1).
W2. Training to halt needs compute cost in the objective; imitation only teaches idleness where traces show it (05/X1).

## Decided

D11. Halt = absence of action; plain halt wakes on any non-ambient event (H1, H2). (user)
D12. Time is a timer argument, not an event (H3, 02/D2). (user)

## Open questions

Q5. How compute cost enters the objective so the model learns to halt and not over-think (W2). (user: don't know.) Becomes important at the outcomes stage (05/K1), not before.
