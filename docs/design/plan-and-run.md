# specter: design of `plan` and `run` (v0)

Decisions from 18–19 October 2026, revision 2. The architecture decision is
in [ADR 0005](../adr/0005-plan-and-run-split.md); this document is the v0
specification.

Goal of v0: answer quickly, with the zoo as the first lab, whether
**inferring dependencies between endpoints takes `run` deeper than
stateless fuzzing**. Anything that does not help measure that is left out.

## 1. Core idea

- **`plan`** reads the OpenAPI document offline and does the preprocessing
  that is not worth repeating on every run: **dependencies between
  endpoints and the execution order**. It writes a small JSON file that
  references the spec without copying it.
- **`run`** loads the plan **and** the spec (through `contract`) and walks
  the plan's `order`. It does not compute a graph or an order, but it
  checks that the `order` is topological and covers every operation,
  because a plan can be written by hand.
- The plan is also a **template the API owner can fill in by hand**. Who
  produces it is a choice: **either** the owner writes it, **or** a
  generator does (heuristics now, an LLM later). These are alternatives,
  not combined layers: no merging and no partial regeneration.
- Only the `contract` package imports the OpenAPI library. `plan` and `run`
  use `contract`, which is the single source of the operation model
  (parameters, request schema, response schemas).
- The plan holds only what the spec does **not** say.
- Why the order lives in the plan: not for speed (Kahn's algorithm runs in
  microseconds even with 537 operations), but because the order is then
  visible, versioned and reviewable in a diff, and `run` carries no graph
  logic. The cost is keeping edges and order in sync, which the check in
  `run` (and later `reorder`) covers.

## 2. Related tools (research)

- **RESTler (Microsoft)**: the closest. Its `compile` mode writes
  `grammar.json`/`grammar.py` with inferred producer–consumer
  dependencies. `grammar.json` can be edited and recompiled, and a
  dictionary and annotations fix dependencies it missed.
- **Schemathesis**: infers links from response schemas and path
  parameters, and accepts manual OpenAPI links. Without links it skips the
  stateful phase. Inference happens in memory, with no editable artifact.
- **EvoMaster**: handles dependencies between resources inside sampling and
  mutation, with no artifact for the user.
- **LLM-based**: AutoRestTest (semantic graph + multi-agent RL + LLM),
  LlamaRestTest (Llama3-8B fine-tuned for inter-parameter dependencies),
  RESTGPT (enriches the spec). They run embedded, at execution time.
- **What specter does differently**: a simple, versioned plan designed for
  hand editing, with an order check, and a swappable generator behind the
  same format.
- Not verified: based on summaries and docs, not on the exact
  `grammar.json` format. Open a real one before comparing in public.

## 3. Plan format (v1)

```json
{
  "version": 1,
  "generator": "heuristic/0.1",
  "spec": { "path": "openapi.yaml", "hash": "sha256:…" },
  "order": ["POST /animals", "POST /feeds", "GET /animals/{id}", "DELETE /animals/{id}"],
  "operations": {
    "GET /animals/{id}": {
      "enabled": true,
      "tags": ["animals"],
      "requires": [
        {
          "param": "id",
          "in": "path",
          "producers": [{ "op": "POST /animals", "from": "response.body.id" }]
        }
      ]
    }
  },
  "seeds_needed": [],
  "cycles": []
}
```

No timestamp, so the file is deterministic. The plan holds nodes, edges
and the ready-made order, with no `sequences`.

### Fields

Top level:
- `version`: format version; `run` and `validate` reject unknown versions.
- `generator`: who produced it (`heuristic/0.1`, `empty/0.1`, `manual`,
  later `llm/...`). For auditing only.
- `spec.path`: path to the spec. A relative path is resolved from the
  directory of the **plan**, not the working directory; `run` accepts
  `--spec` to override it. The spec may be JSON or YAML; the plan is always
  JSON.
- `spec.hash`: SHA-256 of the raw spec bytes; detects a stale plan.
- `order`: ids in topological order (see the `DELETE` rule in section 4).
  Operations with `enabled: false` stay in the list; `run` skips them.
- `operations`: a map by id, holding only what the spec does not say.
  Every operation appears, including those with no dependency
  (`"requires": []` written out, so the owner sees where to fill in).
- `seeds_needed`: requirements with no producer, which need a value from
  the user. `run` skips those operations instead of finding out halfway
  through.
- `cycles`: cycles found and what the tool did to break them.

Items of `seeds_needed` name the requirement that has no producer:

```json
{ "op": "GET /x/{id}", "param": "id", "in": "path" }
```

Items of `cycles` list the operations in the cycle and the edge that was
dropped to break it:

```json
{ "ops": ["A", "B"], "dropped": { "from": "B", "to": "A", "param": "id" } }
```

Per operation (key = id, always `"METHOD /path"`, never the
`operationId`):
- `enabled`: whether it may be tested.
- `tags`: inherited from OpenAPI and editable; every `DELETE` gets
  `destructive`.
- `requires`: requirements, all mandatory (AND).

Per requirement: `param` (the parameter name, or the field name when `in`
is `body`), `in` (`path|query|header|body`) and `producers` (any one will
do, OR; `run` uses the first one with a value available).

Per producer: `op` (id of the operation that creates the value) and `from`
(where the value is in the response, e.g. `response.body.id`).

Not in the plan, taken from the spec by id: `method`, `path`, `params`,
`request_schema`, `responses`.

### Optional extension: groups (Gitea phase)

When the same set of parameters comes from a single producer (e.g.
`owner`+`repo`), the plan defines a **named group** once and operations
reference it with `uses`:

```json
"groups": {
  "repo": {
    "params": ["owner", "repo"],
    "producers": [{
      "op": "POST /user/repos",
      "from": { "owner": "response.body.owner.login", "repo": "response.body.name" }
    }]
  }
},
"operations": {
  "GET /repos/{owner}/{repo}/issues/{index}": {
    "enabled": true, "tags": ["issue"], "uses": ["repo"],
    "requires": [{
      "param": "index", "in": "path",
      "producers": [{ "op": "POST /repos/{owner}/{repo}/issues", "from": "response.body.number" }]
    }]
  }
}
```

Lineage rule: if a consumer uses group G and needs a value produced by an
operation that also uses G, both run in the **same instance** of the group
(the issue `index` comes from the same repo). Each instance is an
independent sandbox, so chains on different repos can become parallel
goroutines.

`groups` and `uses` are optional and the zoo does not use them, so the
zoo golden file does not change. This part is a proposal; it has not been
implemented or tested.

### Consistency between plan and spec

- An operation in the plan that does not exist in the spec: **error**.
- An operation in the spec that is not in the plan: **warning**; it runs
  with no dependencies and goes at the end of the order.
- A different `spec.hash`: warning or error, depending on a flag.
- `order` must be a valid topological order of the current edges and
  cover every operation in the plan.

## 4. Graph model

- An operation is not a simple node: a consumer can need **several**
  parameters (AND), and each can have **several** producers (OR).
- **A producer is not always a `POST`**: `GET /user` produces the `owner`
  in Gitea. The rule "only `POST` produces" needs exceptions (a short list
  or an explicit field in the plan).
- With several producers, prefer the one with the **shortest dependency
  chain** (in Gitea, `POST /user/repos` rather than
  `POST /orgs/{org}/repos`).
- Self-loops are ignored. Cycles: `plan` detects them, breaks them
  deterministically and records them in `cycles`. The exact rule for which
  edge to drop is decided in the heuristic step (e.g. the edge whose
  producer is not a `POST`, with ties broken by id).
- **Correction:** Kahn's algorithm alone does **not** put `DELETE` last
  (`DELETE /animals/{id}` and `GET /animals/{id}` become ready together).
  The tie-break is the key `(is DELETE, id)` in the priority queue: a
  `DELETE` is only taken when no other operation is ready.
- Free test case: use the id after the `DELETE` and expect 404, not 500.
- Cycles are tested with hand-built graphs in unit tests, not in the zoo.
- For future concurrency, a `level` per operation can be derived from
  `order` and the edges. Later.

## 5. The zoo as the first lab

Facts from the spec (`internal/zoo/openapi.yaml`, OpenAPI 3.0.3, 15
operations, no `oneOf`/`allOf`):

- The path parameter is always `{id}`, never `{animalId}`. The link comes
  from the hierarchy: the `{id}` in `/animals/{id}` belongs to the
  `/animals` collection, produced by `POST /animals`.
- The most important edge is a **body** edge: `POST /animals/{id}/feedings`
  needs `feedId`, which comes from the `id` of `POST /feeds`. The
  name-based rule works here (`feedId` → `feed` → `/feeds`).
- `Id` is declared at path level (`$ref` to `components/parameters`);
  `start`/`end` are `$ref`s too. `contract.Load` must merge path-level and
  operation-level parameters.
- The spec has `servers: http://127.0.0.1:8080`, useful as the default
  `--base-url` against `cmd/zoo`.

Expected graph (golden, `testdata/zoo.plan.json`): 5 operations with no
dependency (`GET /healthz`, `GET /animals`, `POST /animals`, `GET /feeds`,
`POST /feeds`), 10 with a dependency, **11 edges**, no seeds and no
cycles.

| Consumers | Requirement | Producer |
|---|---|---|
| `GET`, `PUT`, `PATCH`, `DELETE /animals/{id}`, `GET` and `POST /animals/{id}/feedings` | `id` (path) | `POST /animals` |
| `GET`, `PUT`, `DELETE /feeds/{id}`, `POST /feeds/{id}/restock` | `id` (path) | `POST /feeds` |
| `POST /animals/{id}/feedings` | `feedId` (body) | `POST /feeds` |

Order produced by Kahn with the `(is DELETE, id)` tie-break:

```
GET /animals, GET /feeds, GET /healthz, POST /animals,
GET /animals/{id}, GET /animals/{id}/feedings, PATCH /animals/{id},
POST /feeds, GET /feeds/{id}, POST /animals/{id}/feedings,
POST /feeds/{id}/restock, PUT /animals/{id}, PUT /feeds/{id},
DELETE /animals/{id}, DELETE /feeds/{id}
```

Other tie-breaks are valid. The most robust test compares the edges for
equality and the `order` by property (topological, `DELETE` after the
others).

Note: `DELETE /feeds/{id}` runs after `POST /animals/{id}/feedings`, so the
feed already has a feeding and the delete should return 409. That is
documented in the spec and is **not a finding**; the report must not count
it as a failure.

### Zoo bugs and what v0 reaches

`BUGS.md` is the ground truth (9 planted bugs). With the minimal oracle
(5xx and undocumented status) and minimal values, **none shows up**: bugs
002, 005 and 007 return `200`/`201`. Estimate, not yet measured:

| Bug | Needs | In v0? |
|---|---|---|
| 002 (`PATCH` with `age` ≥ 128 turns negative) | response checked against `minimum: 1`, boundary values 127/128, an existing animal | yes, with the extras below |
| 005 (`name` > 64 bytes is cut) | long string and an echo oracle (response equals what was sent) | yes, with the extras |
| 007 (`restock` goes past 1,000,000) | response checked against `maximum`, boundary value, an existing feed | maybe, depends on the values combined |
| 006 (repeated `diet` with an invalid value) | generated invalid value, 400 expected | no (`standard`) |
| 001, 008 | concurrency | no |
| 003, 004, 009 | differential oracles and several instances in order | no |

Extras that enter v0 because of this: validation of the response body
(type, required, enum, min/max, which `contract` already loads), a
boundary value generator (schema min/max; 127/128 for integers with no
maximum; long strings) and an echo oracle for `POST`/`PUT`.

Testable hypothesis: without `--no-deps`, the `PATCH` gets a real animal
and can find 002 (and maybe 005 and 007); with `--no-deps` it gets 404.

Limit of the lab: the zoo validates the mechanics, but it has only 11
edges, so it does not measure the heuristic's coverage or precision. That
needs a second target (Gitea).

### How to decide on a pivot

Measure, on the zoo and later on a real target:

1. **Edge coverage**: of the operations with a path parameter, how many got
   a producer, and the precision on a manual sample of ~20 edges.
2. **Depth**: operations that end in 2xx with dependencies versus
   `--no-deps`.
3. **Findings**: how many of the zoo's 9 bugs, plus any real 5xx or
   divergence on a real target.

Starting rules (guesses, to adjust): coverage above ~50% with good
precision and a visible gain in depth, move on to `standard`. Small gain
over the baseline: the value is in the oracle, and the natural pivot is a
contract-drift report, which does not depend on the heuristic. Low
coverage but hand editing works: the product is the editable plan.

## 6. Gitea (second phase)

Measured on Gitea's `swagger.v1.json` (throwaway prototype; an **upper
bound**, does not measure precision):

- Swagger 2.0, `basePath: /api/v1`, 342 paths, 537 operations (261 GET,
  119 POST, 38 PATCH, 27 PUT, 92 DELETE), 225 definitions with no
  `allOf`/`oneOf`/`anyOf`.
- 462 operations have a path parameter, 75 do not. Most used: `owner`
  (304 operations), `repo` (298), `id` (127), `org` (70), `index` (67),
  `username` (44).
- 140 parameters are `in: body` (Swagger 2.0 has no `requestBody`):
  `contract` must normalise them. kin-openapi seems to have a conversion
  package (`openapi2conv`); **confirm**.

Coverage of 1089 path requirements:

| Rule | Requirements | % |
|---|---|---|
| `POST` on the parent collection (the zoo rule) | 272 | 25% |
| `owner`/`repo` pair ← `POST /user/repos` (`owner.login`, `name`) | 602 | 55% |
| `username` ← `GET /user` (`login`) | 34 | 3% |
| No producer | 181 | 17% |

With the three rules, 300 of the 462 operations with a path parameter
(65%) are fully resolvable. There is no `POST /repos`; repos are created
by `POST /user/repos`, `/orgs/{org}/repos`, `/org/{org}/repos` and
`/admin/users/{username}/repos`.

What is left without a producer:
- **Catalogues and system resources** (`notifications/threads/{id}`,
  `gitignore/templates/{name}`): a seed or a listing `GET`.
- **Names chosen by the client** (`{secretname}`, `{variablename}`),
  created by `PUT`: the value comes from the request itself.
- **Git refs** (`sha`, `filepath`, `branch`): come from a `GET`.
- **Runs, runners and workflows**: created by the server.
- An in-between case: `/issues/comments/{id}` has no `POST` on its
  collection, but the real producer is
  `POST .../issues/{index}/comments`; matching on the last segment
  (`comments`) would solve it. `{index}` is the issue's `number` field
  (the name does not lead to the field; a case for a manual plan or an
  LLM).

Safety: 33 operations under `/admin` and 92 `DELETE`s (including
`DELETE /repos/{owner}/{repo}`). Disable `/admin/*` by default and only
delete what the run itself created. Run against a throwaway local Gitea.
Almost everything needs a token, so `--auth` has to work early.

## 7. v0 scope

**In**
- `plan`: types and canonical `Marshal`, `contract.Load` (OpenAPI 3.0 in
  JSON or YAML, `$ref` resolved), minimal heuristic, `order` by Kahn,
  deterministic cycle breaking recorded in `cycles`, the `specter plan`
  CLI.
- `run` (smoke profile, serial): load plan and spec, check the `order`
  (topological and complete), execute in `order`, value pool, request
  building, oracle, report, `DELETE` skipped by default
  (`--include-destructive` enables it).
- `--no-deps`: ignores `requires` and uses generated values. It is the
  stateless baseline and costs almost nothing.
- Minimal heuristic: path parameter → `POST` on the parent collection with
  `id` in the response, and `xxxId` fields in the body → collection
  `xxx`. No OpenAPI links, no nested hierarchy, no groups.

### Minimal heuristic rules

The producer is always a `POST` and the value is read from
`response.body.id`. An operation never depends on itself. Every output is
ordered by the contract (operations) or by name (body fields), never by map
iteration, so the plan is the same on every run.

**Path rule.** Each `{x}` segment belongs to the collection made of the
segments before it: in `/animals/{id}/feedings` the collection of `{id}` is
`/animals`. If a `POST` exists on exactly that path, the operation gets
`Requirement{param: x, in: path}` with that `POST` as producer. If not, the
parameter goes to `seeds_needed`. Every `{x}` of a path is evaluated on its
own, left to right, whatever the method of the operation. The producer path
is always shorter than the consumer path, so this rule alone never creates a
cycle.

**Body rule** (best effort). Applies to the top-level properties of the
request body, in every media type, visited in name order.

1. A property is an id reference when its name ends in `Id`, `ID` or `_id`
   and something is left once the suffix is cut. A bare `id` is the
   resource's own id and is ignored.
2. The candidates are the collections with a `POST` whose last segment
   matches the stem, ignoring case, as is, with `s` or `es` added, or with
   `y` turned into `ies` (`feedId` → `/feeds`, `categoryId` →
   `/categories`).
3. Only a single candidate counts. With several, the rule stays silent
   instead of guessing.
4. The candidate may not be the operation itself.
5. If the same producer is already required by a path parameter, no second
   requirement is added.
6. The requirement is `{param: <field name>, in: body}`.

Names that do not follow these patterns are not guessed. They are left to
the LLM generator or to whoever edits the plan, and the result is checked
by `Sort` and `Validate` like any other plan.

**Out**
The `plan validate` and `plan reorder` commands, the `empty` generator,
JSON Schema, `--include/--exclude` filters, profiles, concurrency,
`--changed-since`, `groups`/`uses`. The plan is regenerated in v0, but
`run` checks the `order` because a plan can be written by hand.

## 8. Implementation

Packages. Already there: `internal/cli` (with `plan` and `run` stubs),
`internal/contract` (only `doc.go`), `internal/generator`,
`internal/executor` and `internal/oracle`. Per ADR 0002, a new package is
only created with real code:
- `internal/plan`: types, canonical `Marshal`, `Load`, order check,
  `Reorder`.
- `internal/plan/gen`: `Generator`, `empty` and `heuristic` (when they
  arrive).
- `internal/run`: the orchestrator, owner of the loop.
- `internal/report`: the report.

Order (one commit per step):
1. **Types, `Load` and canonical `Marshal`** in `internal/plan`. Test:
   `Load` → `Marshal` → `Load` → `Marshal` gives identical bytes.
2. **Kahn and the order check** with the `(is DELETE, id)` tie-break.
   Tests: reproduces the golden `order`, idempotence, a cycle (the message
   names the operations), a missing producer, an `order` that is out of
   order or incomplete.
   - *Optional:* **`specter plan graph`**. Reads a plan and writes a
     Markdown file with a Mermaid diagram of the graph and a table of the
     `order`, so [zoo-graph.md](zoo-graph.md) is generated instead of
     written by hand and stays in sync as endpoints are added. Uses
     `strings.Builder` or `text/template`. Test: the zoo fixture produces
     `zoo-graph.md` as a golden file.
3. **`contract.Load`**: kin-openapi with `$ref` resolved on load, merging
   path and operation parameters, accepting JSON and YAML. Test: the zoo
   returns the 15 golden ids, in both formats. `oneOf`/`allOf` are an
   "unsupported" error. Record the library in ADR 0003.
4. **`plan.Build`**: `spec.hash`, `tags`, `destructive` on `DELETE`s,
   every operation `enabled: true`.
5. **Heuristic**, followed by order, cycles and seeds.
6. **`plan` CLI**. Determinism test (two runs, same bytes) and acceptance
   against the zoo golden file.
7. **`run`**: executor, pool, request building, loop, oracle, report.
8. **`run` CLI** and an end-to-end test with an `httptest.Server`
   simulating the zoo; then run against the real `cmd/zoo` and collect
   the three metrics.

Golden file and serialisation details:
- Generate the canonical golden bytes once with an `-update` flag and
  review the diff, instead of formatting by hand (`json.MarshalIndent`
  expands arrays and objects).
- A `nil` slice becomes `null` in JSON; initialise `Requires`, `Tags`,
  `SeedsNeeded` and `Cycles` as empty slices.
- `Decoder.DisallowUnknownFields()` in `Load`, so a typo by the plan's
  owner becomes an error; `SetEscapeHTML(false)` on the encoder.
- The CLI stub message says the plan will have "seed inputs and
  strategies". In the current design those belong to `run`/`generator`,
  not the plan; fix the text when implementing.

Tests to have:
- Determinism of `plan` on the zoo.
- Golden zoo plan in `testdata/`.
- Heuristic table: collection and item, nested resource, two producers,
  no producer, cycle.
- The order check rejects an `order` that is out of order or incomplete.
- The consistency check rejects an operation missing from the spec and a
  missing producer.

Decisions already made:
- `unsupported` in `contract` is an **error** in v0.
- Singularisation: a simple rule (`ies`→`y`, `s`→∅) plus a list of
  exceptions, no library.
- The id never depends on the `operationId`.
- The spec is parsed on every `run`, with no cache.

## 9. `run` (v0, smoke)

Steps:
1. Load the spec (`contract`) and the plan; check the `order`. If it is
   invalid, fail, name the operations that are out of order or missing,
   and suggest regenerating the plan (`plan reorder` comes later).
2. `executor.Do(req)`: `http.Client` with a timeout, base URL (default:
   the spec's `servers[0]`) and auth by flag, no retry.
3. `pool`: `map[string][]any` keyed by `opID|from`, with `Put` and
   `Pick(seed)`; extraction of `response.body.id` by dotted path.
4. Build the request: a value from the pool; else the schema's
   `example`/`default`; else a boundary or minimal valid value for the
   type. No value: skip with `missing_seed`.
5. v0 oracle: 5xx status, undocumented status, body outside the schema
   (type, required, enum, min/max) and echo for `POST`/`PUT`.
6. Report: JSON per operation (`ok | failed | skipped:<reason>`), findings
   and the status of each edge (`confirmed | refuted | untested`). Non-zero
   exit code if there is a finding. The `409` from `DELETE /feeds/{id}`
   does not count.
7. Loop:
   ```go
   for _, id := range selected { // already in topological order
       op := spec.Op(id)
       if budget.Exhausted() { break }
       if blocked, why := deps.Blocked(id); blocked { res.Skip(id, why); continue }
       req, err := build(op, plan.Ops[id], pool, rng)
       if err != nil { res.Skip(id, "missing_seed"); continue }
       resp, err := ex.Do(ctx, req)
       budget.Spend(1)
       res.Record(id, resp, oracle.Check(op, resp))
       if ok2xx(resp) { pool.Extract(id, resp) } else { deps.MarkFailed(id) }
   }
   ```
8. CLI: `--plan --spec --base-url --auth --budget --seed --no-deps
   --include-destructive`.

Decisions:
- Seed through `rand.New(rand.NewSource(seed))` passed along, no global
  `math/rand`.
- Edge confirmed: the consumer got a 2xx with the produced value. Refuted:
  a 404 or other 4xx with that value. Goes to the report only; does not
  change the plan.
- An operation with `enabled: false` stays in the `order`, `run` skips it,
  and consumers that depend only on it are `skipped`.
- Without `--include-destructive`, created resources stay in the
  environment; list their ids in the report.
- No concurrency in v0. Later, each group instance becomes a goroutine
  with its own pool.

## 10. Scope control (after v0)

In the **plan** (the API owner): `enabled: false`, `tags`, `max_depth`
(optional, to be defined).

In **run** (whoever executes): `--include/--exclude` filters by path
(glob), method or tag; `--profile smoke|standard|deep`; `--budget` and
`--max-depth`; `--changed-since <old plan>` for CI;
`--include-destructive`; operations in `seeds_needed` only run when the
seed value is supplied.

Rule: `run` can **restrict** the plan freely, but can only **widen** what
the plan allows (`enabled: false` is not re-enabled by a flag, except
`--force-enabled`).

## 11. After v0

- `standard`: type, enum and missing-field values, invalid values (bug
  006), `max_depth`.
- Concurrency per group instance (bugs 001 and 008).
- Differential oracles and several instances in order (bugs 003, 004,
  009).
- `deep`: adaptive mutation and a scheduler; the static plan is only the
  starting point.
- An LLM generator as another `Generator` implementation.
- `plan validate`, `plan reorder`, `empty`, JSON Schema, filters,
  `groups`/`uses`, Gitea and Swagger 2.0.

## Not verified

- The Gitea numbers are an upper bound: the prototype only checks for a
  `POST` on the parent collection with a plausible field, and the
  `owner`/`repo` and `GET /user` rules assume names.
- The estimate of which zoo bugs v0 reaches has not been measured.
- kin-openapi's Swagger 2.0 → 3.0 conversion package.
- Whether kin-openapi reads JSON and YAML through the same function.
- The real format of RESTler's `grammar.json`.

## Sources

- RESTler: https://github.com/microsoft/restler-fuzzer
- RESTler, improving coverage: https://github.com/microsoft/restler-fuzzer/blob/0beec440c801aed6191b2d67c0a846a5bf9088ca/docs/user-guide/ImprovingCoverage.md
- Schemathesis, stateful testing: https://schemathesis.readthedocs.io/en/latest/guides/stateful-testing/
- EvoMaster, dependencies: https://oda.oslomet.no/oda-xmlui/handle/11250/2991533
- LlamaRestTest: https://arxiv.org/html/2501.08598v2
