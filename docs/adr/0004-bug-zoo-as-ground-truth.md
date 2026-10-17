# 0004. Bug zoo as ground truth

- Status: Accepted
- Date: 2026-10-13

## Context

A fuzzer is only as good as the evidence that it finds bugs. Fuzzing a real
or public API gives no ground truth: you can't tell whether a run missed
something. It also isn't reproducible, and it can cause harm. Comparing
strategies (baseline vs. adaptive vs. AI) needs a target where every bug is
known.

## Decision

Keep a deliberately buggy API, the **bug zoo**, inside the repository:

- Code in `internal/zoo`, server in `cmd/zoo`, contract served at
  `/openapi.yaml`.
- Every bug has a stable ID (`BUG-NNN`), a row in `internal/zoo/BUGS.md`
  and a `// BUG-NNN:` comment where it is planted.
- Zoo tests assert that every bug **still reproduces**. A failing zoo test
  means the benchmark broke.
- The OpenAPI document describes the intended API, not the bugs, so the
  fuzzer has to find them.
- Bugs are added over time and cover different classes: boundary values,
  type confusion, state/sequence bugs, contract violations in responses.

External targets (e.g. OWASP crAPI) are secondary and only run locally.

## Alternatives considered

- **Public APIs** — no ground truth, not reproducible, not authorized.
- **Only third-party vulnerable apps** — useful later, but bugs aren't
  catalogued in a form a benchmark can score against.

## Consequences

- Benchmarks are reproducible offline and in CI.
- Results can be scored: bugs found, time to first find, false positives.
- Risk of overfitting the fuzzer to the zoo. Mitigation: write bugs
  modelled on real-world classes, and validate against external targets
  before claiming anything.
