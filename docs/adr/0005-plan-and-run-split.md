# 0005. Plan and run split

- Status: Accepted
- Date: 2026-10-19

## Context

Many API bugs only show up after other calls: a feeding needs an animal and
a feed, and a delete matters only once something depends on the resource.
To reach them, specter has to know which operation produces the values
another one consumes. OpenAPI rarely says so; `links` exist but few specs
use them.

Working that out every run hides the result. Nobody can review the
dependencies, fix a wrong one, or see in a diff that they changed. An API
owner who knows the dependencies has nowhere to write them down.

The first question for specter is whether following those dependencies
takes a run deeper into an API than stateless fuzzing does. The design has
to make that measurable with as little machinery as possible.

## Decision

Split the work into two commands joined by a plan file:

- `specter plan` is offline. It reads the OpenAPI document and writes
  `plan.json`: the dependencies between operations and an execution order.
  It never opens a network connection.
- `specter run` loads the plan **and** the spec, and walks the plan's
  order. It does not compute an order, but it checks the one it is given:
  a plan can be written by hand, so `run` refuses an order that is not
  topological or does not cover every operation in the plan.

The plan holds only what the spec does not say:

- `version`, `generator` (who produced it, for auditing), and `spec`
  (path and SHA-256 of the raw bytes, to detect a stale plan).
- `order`: operation ids in topological order.
- `operations`: a map keyed by id. Each entry has `enabled`, `tags` and
  `requires`. Every operation appears, with `"requires": []` when it has
  no dependencies, so an owner can see where to fill one in.
- A requirement names a parameter, where it goes (`path`, `query`,
  `header`, `body`) and its producers. All requirements must be met (AND);
  any producer will do (OR).
- A producer names an operation id and where the value is in its response
  (`response.body.id`).
- `seeds_needed` and `cycles` list what the generator could not resolve.

Methods, paths, parameters and schemas are not copied; `run` reads them
from the spec through `contract`.

`spec.path` points to the OpenAPI file. A relative path is resolved from
the directory of the plan file, not the working directory, so a plan and
its spec can move together. `--spec` overrides it. The spec may be JSON or
YAML; `contract` accepts both. The plan itself is always JSON.

Rules:

- An operation id is always `"METHOD /path"`, never the `operationId`.
- The order comes from Kahn's algorithm with the tie-break key
  `(is DELETE, id)`, so a `DELETE` only runs when nothing else is ready.
  Every `DELETE` gets the `destructive` tag.
- When the graph has a cycle, `plan` breaks it deterministically by
  dropping one edge, and records what it dropped in `cycles`. The owner can
  correct it by hand. Which edge is dropped is decided with the heuristic
  generator.
- The plan is written by a generator (heuristics now, an LLM later) **or**
  by hand. These are alternatives: there is no merging of sources and no
  partial regeneration.
- The same spec and generator produce byte-identical output. There is no
  timestamp.
- Only `contract` imports the OpenAPI library. `internal/plan` owns the
  types and the load and save code.

The order is stored, although computing it takes microseconds, so that it
is visible, reviewable in a diff, and keeps graph logic out of `run`.

## Alternatives considered

- **One command that infers dependencies on every run** — nothing to
  review, correct or diff, and no place for an owner's knowledge.
- **A self-contained plan with parameters and schemas copied in** — `run`
  would not need the spec, but the plan grows with the API and can drift
  from it.
- **A plan with concrete requests and sequences** — this is a test suite,
  not a fuzzer. Choosing values and sequences belongs in `run`.
- **Merging owner, heuristic and LLM sources with an origin per edge** —
  more expressive, but needs precedence rules and partial regeneration.
  Revisit if editing a generated plan by hand turns out to be common.
- **Computing the order in `run`** — cheap, but the order would be
  invisible and `run` would carry graph code.

## Consequences

- `run` needs both files. A hand-written plan is not usable without the
  spec it describes.
- Edges and order can disagree after a hand edit. `run` detects it and
  stops, which costs one pass over the graph (O(n+e)) per run. A future
  `plan reorder` can fix the order without regenerating the plan.
- The check that the order is valid lives in `internal/plan`, so `run`
  and a future `plan validate` share it.
- A hand-edited plan is lost when it is regenerated. The spec hash only
  warns that the spec changed.
- The plan format and its JSON shape are a public interface. Loading
  rejects unknown fields and unknown versions.
- `run --no-deps` ignores `requires` and gives the stateless baseline the
  experiment needs.
- `kin-openapi` must be added to the accepted dependencies in ADR 0003.
- `docs/architecture.md` must be updated: two commands, `internal/plan`,
  and no AI layer that asks the executor to send requests.
