# Bug zoo catalogue

Ground truth for specter's benchmarks. Each bug is planted on purpose, marked
in the code with its ID, and asserted by a test in this package.

## Status

The last column says whether `specter run` has found the bug. **Found** means
a run against the zoo reported a finding on the bug's endpoint that matches
its symptom. **Not found** says what specter still lacks. This is measured, not
estimated: update it after a run, and write the date and the commit.

Last checked: 2026-11-27, at commit `5839dd3` (one run of the zoo plan with both
oracles: 13 requests, one finding, a documented `422` from the baseline oracle,
and no bug).

**Found: 0 of 9.**

| ID | Endpoint | Trigger | Symptom | Class | Detectable by | Found by specter |
|---|---|---|---|---|---|---|
| BUG-001 | `DELETE /animals/{id}` | Two concurrent DELETEs of the same existing animal | Both return `204`; the second should return `404` | State / race (check-then-act) | Concurrent replay of the same request; response-status oracle | Not found. Needs concurrent replay, which `run` does not do yet, and `DELETE` is skipped without `--include-destructive` |
| BUG-002 | `PATCH /animals/{id}` | Body sets `age` to 128 or more (e.g. `130`) | `200`, but the returned and stored `age` is wrapped through an `int8`: `130` becomes `-126`, `255` becomes `-1`, `256` becomes `0` and `257` becomes `1`; `PUT` and `POST` keep the value | Boundary / integer overflow | Round-trip oracle: response value differs from the sent value (catches every wrapped value). A positive-age check on the response only catches the values that wrap to zero or below | Not found. `PATCH` runs, but with the minimal body (`{"age": 1}`). Needs boundary values (127 and 128) and an oracle that checks the response body against the schema |
| BUG-003 | `GET /animals` | `start` > 0 and an explicit `end` equal to the number of animals that match the filters (the total when there is no filter) | `200`, but the last animal is missing | Boundary / off-by-one | Compare a windowed list with the unpaginated list | Not found. `run` sends no query parameters, and nothing compares two lists |
| BUG-004 | `GET /animals` | List once (this fills the cache), `PUT` or `PATCH` an animal, list again with no create or delete in between | The list shows the old values while `GET /animals/{id}` shows the new ones | State / stale cache | Cross-endpoint consistency after a mutation (needs call ordering) | Not found. The plan order lists before it mutates, and there is no consistency oracle |
| BUG-005 | `POST /animals` | `name` longer than 64 bytes | `201`, but the stored name is cut to 64 bytes, possibly mid-character | Silent data corruption | Round-trip oracle: response name differs from the sent name | Not found. The body is the minimal one (`"name": "a"`). Needs a long string and an echo oracle |
| BUG-006 | `GET /feeds` | `diet` repeated, with an invalid value after the first (e.g. `?diet=herbivore&diet=pizzavore`) | `200`; only the first value is validated, the invalid one matches nothing. A single invalid `diet` correctly returns `400` | Input validation / repeated query parameter | Spec-driven enum check on every value of an array parameter; response-status oracle | Not found. `run` sends no query parameters and no invalid values |
| BUG-007 | `POST /feeds/{id}/restock` | Restock so that stock exceeds 1,000,000 | `200`, stock is past the cap that `POST` and `PUT` enforce; sending that feed back with `PUT` returns `400` | Boundary / missing invariant | Round-trip oracle: response body fails the request schema | Not found. `restock` runs with the minimal quantity. Needs a value at the maximum and a response-body oracle |
| BUG-008 | `POST /animals/{id}/feedings` | Concurrent feedings that together ask for more than the stock | More than one `201`; stock goes negative | State / race (check-then-act) | Concurrent replay; invariant stock >= 0 via `GET /feeds/{id}` | Not found. Needs concurrent replay and a body that uses a real `feedId` from the Pool (it gets `422` today) |
| BUG-009 | `DELETE /feeds/{id}` | Feed A used by an older feeding, then feed B used by a newer one; delete A | `204`, but should be `409`; the feeding of A now references a missing feed | Referential integrity (needs call ordering) | Sequence of feed, feed, feeding, feeding, delete; `GET /feeds/{feedId}` after listing feedings | Not found. Needs a sequence of calls, and `DELETE` is skipped without `--include-destructive` |

## Adding a bug

1. Pick the next ID (`BUG-010`, `BUG-011`, …) and add a row to the table
   above. Keep the trigger precise enough that someone else could reproduce
   it by hand, and write "Not found" in the last column until a run finds it.
2. Plant it in the code with a `// BUG-NNN:` comment explaining the flaw.
3. Add a test case that reproduces it.
4. The OpenAPI document describes the *intended* API, not the bug. Only
   change it when the bug needs a new endpoint.

IDs are never reused, even if a bug is removed.
