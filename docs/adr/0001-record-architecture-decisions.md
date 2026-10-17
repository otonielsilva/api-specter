# 0001. Record architecture decisions

- Status: Accepted
- Date: 2026-10-13

## Context

specter's design will change quickly while the project is young. Knowing
*why* something is the way it is matters as much as the code, both for
changing it safely and for contributors. Commit messages and issue threads
scatter that reasoning.

## Decision

Record every decision that shapes the code as a short ADR in `docs/adr/`,
using the template in `0000-template.md`. An ADR is written when the decision
is made, not afterwards, and is never rewritten. A changed decision gets a new
ADR that supersedes the old one.

## Alternatives considered

- **Only commit messages** — reasoning is scattered and hard to find.
- **A single design doc** — gets rewritten, so the history of *why* is lost.

## Consequences

- Small, frequent docs commits alongside code.
- Superseded ADRs stay visible, which shows how the design evolved.
