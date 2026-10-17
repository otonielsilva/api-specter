# Architecture

specter is a single Go binary organised as a modular monolith (ADR 0002).
A run is a pipeline:

```
            OpenAPI document
                   │
             ┌─────▼─────┐
             │ contract  │  parse → internal model
             └─────┬─────┘
             ┌─────▼─────┐
             │ generator │  model + seed → request inputs
             └─────┬─────┘
             ┌─────▼─────┐
             │ executor  │  inputs → HTTP exchanges (concurrency, limits, scope guard)
             └─────┬─────┘
             ┌─────▼─────┐
             │  oracle   │  exchanges → failures
             └─────┬─────┘
                   ▼
                report
```

`cmd/specter` wires the pipeline through `internal/cli`.
`cmd/zoo` serves the benchmark target from `internal/zoo`.

## Packages

| Package | Responsibility |
|---|---|
| `internal/cli` | Subcommands, flags, output |
| `internal/contract` | OpenAPI → internal model |
| `internal/generator` | Valid, boundary, invalid and mutated inputs |
| `internal/executor` | HTTP, concurrency, rate limits, cancellation, scope guard |
| `internal/oracle` | 5xx, contract violations, explicit rules |
| `internal/zoo` | Deliberately buggy benchmark API |

Planned packages, in expected order. They are created only when they get
their first real code (ADR 0002):

| Package | Responsibility |
|---|---|
| `internal/report` | Run summaries, export formats |
| `internal/replay` | Deterministic reproduction of a failure |
| `internal/corpus` | Persisted runs, seeds and failures |
| `internal/scheduler` | Strategy selection from feedback (bandit) |
| `internal/ai` | Retrieval over past runs, LLM suggestions, agent loop |

## Dependency rules

- Arrows only point down the pipeline. `generator` knows `contract`;
  `contract` knows nobody.
- Only `contract` imports the OpenAPI library.
- Only `executor` sends network requests. Anything else that wants a request
  sent, including the future AI layer, asks the executor.
- Interfaces are declared by the package that *consumes* them, and only when
  a second implementation or a test actually needs one.

## Conventions

- Errors are wrapped with `%w` and context (`fmt.Errorf("load spec %s: %w", path, err)`);
  sentinel errors are matched with `errors.Is`/`errors.As`.
- Structured logging with `log/slog`. Libraries take a `*slog.Logger`; they
  never use the global logger.
- Every blocking or I/O function takes a `context.Context` as its first
  argument.
- No global mutable state, including the global `math/rand` source.
- Tests are table-driven and live next to the code. Benchmarks use
  `testing.B.Loop` and are compared with `benchstat`.

## Open questions

- Which OpenAPI library to use. Decide when `contract` gets code.
- Persistence: SQLite (pure-Go driver) is the likely choice for `corpus`.
  Decide with an ADR when `corpus` gets code.
- Vector search for the AI layer: brute force in Go first, measure, then
  decide whether an index is needed.
