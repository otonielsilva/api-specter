# 0002. Modular monolith and package layout

- Status: Accepted
- Date: 2026-10-13

## Context

specter is a CLI that runs a pipeline: contract → generator → executor →
oracle → report. It needs clear boundaries between those stages, but
nothing in the problem requires separate processes.

A common default is a layered layout (`models/`, `services/`,
`controllers/`, `utils/`). That doesn't fit Go well: packages
are the unit of encapsulation, and a package named after a layer ends up
importing everything.

## Decision

- One Go module, one binary (`cmd/specter`) plus the benchmark target
  (`cmd/zoo`).
- Packages are organised **by component** (what they do), not by layer:
  `internal/contract`, `internal/generator`, `internal/executor`, …
- Everything lives under `internal/`. specter is an application, not a
  library, so nothing is importable from outside. No `pkg/` directory.
- `main` packages are thin. They wire dependencies and translate errors into
  exit codes. Logic lives in `internal/cli` so it can be tested.
- A package is created when it gets its first real code, not in advance.
  Planned packages are listed in `docs/architecture.md`.
- No `utils`, `common` or `helpers` packages. A helper belongs to the
  package that uses it until a second package needs it.

## Alternatives considered

- **Layered layout** — fights Go's package model; produces import cycles or
  god packages.
- **Microservices / separate processes** — no requirement justifies the
  operational cost. A distributed mode can be explored later as an explicit
  experiment with its own ADR.
- **Public `pkg/` API** — there are no library users; it would freeze APIs
  that are still being discovered.

## Consequences

- Package boundaries are enforced by the compiler (`internal/`).
- Moving to a public API later requires deliberately promoting packages,
  which is the right moment to design them.
