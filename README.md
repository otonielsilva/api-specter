<p align="center">
  <img src="docs/assets/logo.svg" alt="specter: contract-driven HTTP API fuzzer" width="440">
</p>

<p align="center">
  <a href="https://github.com/otonielsilva/api-specter/actions/workflows/ci.yml"><img src="https://github.com/otonielsilva/api-specter/actions/workflows/ci.yml/badge.svg" alt="CI"></a>
  <a href="https://github.com/otonielsilva/api-specter/actions/workflows/security.yml"><img src="https://github.com/otonielsilva/api-specter/actions/workflows/security.yml/badge.svg" alt="Security"></a>
  <a href="https://goreportcard.com/report/github.com/otonielsilva/api-specter"><img src="https://goreportcard.com/badge/github.com/otonielsilva/api-specter" alt="Go Report Card"></a>
  <a href="go.mod"><img src="https://img.shields.io/github/go-mod/go-version/otonielsilva/api-specter" alt="Go version"></a>
</p>

---

A contract-driven HTTP API fuzzer written in Go. It reads an OpenAPI
document, generates valid, boundary and invalid inputs, sends them under
controlled concurrency, and records reproducible failures.

> **Status:** early development. Not usable yet.

## Why

Hand-written tests cover the cases developers thought of. APIs still break
on boundary values, unexpected combinations, deeply nested inputs and
specific sequences of calls. specter explores those systematically,
starting from the contract the API already publishes.

## Goals

- **Reproducible.** Every run has a seed, and every failure can be replayed
  exactly.
- **Safe by default.** Only local targets unless explicitly allowed, with
  request budgets and rate limits.
- **Measured.** Strategies are compared against a benchmark API with known
  bugs, and the results are published, including negative ones.
- **Adaptive.** Later runs use what earlier runs found to decide what to
  try next. An optional AI layer suggests cases; it never sends requests
  itself.

## Roadmap

- [ ] End-to-end run against the bug zoo
- [ ] Input generation from schemas (valid, boundary, invalid)
- [ ] Concurrent executor with cancellation and rate limiting
- [ ] Contract-violation oracles and deterministic replay
- [ ] Persistent corpus and adaptive scheduling
- [ ] Optional AI layer: retrieval over past runs, suggested cases
- [ ] v0.1.0

## Layout

```
cmd/specter/        CLI entry point
cmd/zoo/            benchmark target server
internal/cli/       subcommands and flags
internal/contract/  OpenAPI → internal model
internal/generator/ input generation
internal/executor/  HTTP, concurrency, scope guard
internal/oracle/    failure detection
internal/zoo/       deliberately buggy API (see BUGS.md)
docs/architecture.md
docs/adr/           architecture decision records
docs/experiments/   experiment reports
```

See [docs/architecture.md](docs/architecture.md) for the pipeline and the
dependency rules, and [docs/adr/](docs/adr/) for the reasoning behind them.

## Development

Requires Go 1.27+ and [golangci-lint](https://golangci-lint.run/) v2.

```sh
make check   # vet, tests with -race, lint
make build   # bin/specter and bin/zoo
make zoo     # run the bug zoo on 127.0.0.1:8080
```

## Safety

specter sends large volumes of malformed requests. By default it only
targets loopback addresses.
Only fuzz systems you own or are explicitly authorized to test.

## License

TBD.
