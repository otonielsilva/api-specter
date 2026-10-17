# 0003. Standard library first

- Status: Accepted
- Date: 2026-10-13

## Context

A fuzzer's behaviour depends on details that frameworks tend to hide: how
connections are reused, how cancellation propagates, how load is
limited. specter needs direct control over those. It is also a security
tool, so every dependency is supply-chain surface to audit and keep
updated.

## Decision

Use the standard library unless there is a concrete reason not to:

| Need | Choice |
|---|---|
| HTTP client and server | `net/http` (with Go 1.22+ routing patterns) |
| CLI | `flag` with one `FlagSet` per subcommand |
| Logging | `log/slog` |
| Randomness | `math/rand/v2` |
| Testing | `testing`, `net/http/httptest`, native fuzzing |

Adding a dependency requires an ADR that says what it replaces and why
writing it ourselves is not worth it. Expected candidates: an OpenAPI
parser and a SQLite driver.

### Dependencies accepted

| Dependency | Replaces | Why |
|---|---|---|
| `github.com/joho/godotenv` | A hand-written `.env` parser | Handles quoting, escapes, comments and `export` prefixes, which a short parser gets subtly wrong. Used only in `internal/config`. |

## Alternatives considered

- **cobra/viper for the CLI** — more than a few subcommands and flags would
  justify it; revisit if the CLI grows.
- **gin/echo/chi for the zoo** — `net/http` routing covers it.
- **testify** — the standard `testing` package with table-driven tests is
  idiomatic and enough.

## Consequences

- More code written by hand, in exchange for full control over the
  behaviour that matters most for a fuzzer.
- `go.mod` stays short, and every line in it has a recorded reason. New
  dependencies are added to "Dependencies accepted" above rather than in a
  separate ADR while the list stays small.
