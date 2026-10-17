# Bug zoo catalogue

Ground truth for specter's benchmarks. Each bug is planted on purpose, marked
in the code with its ID, and asserted by a test in this package.

| ID | Endpoint | Trigger | Symptom | Class | Detectable by |
|---|---|---|---|---|---|
| BUG-001 | `DELETE /animals/{id}` | Two concurrent DELETEs of the same existing animal | Both return `204`; the second should return `404` | State / race (check-then-act) | Concurrent replay of the same request; response-status oracle |
| BUG-002 | `PATCH /animals/{id}` | Body sets `age` to 128 or more (e.g. `130`) | `200`, but the returned and stored `age` is wrapped through an `int8` (`130` becomes `-126`); `PUT` and `POST` keep the value | Boundary / integer overflow | Round-trip oracle: response value differs from the sent value; positive-age contract |
| BUG-003 | `GET /animals` | `start` > 0 and an explicit `end` equal to the total number of animals | `200`, but the last animal is missing | Boundary / off-by-one | Compare a windowed list with the unpaginated list |
| BUG-004 | `GET /animals` | List once, `PUT` or `PATCH` an animal, list again with no create or delete in between | The list shows the old values while `GET /animals/{id}` shows the new ones | State / stale cache | Cross-endpoint consistency after a mutation (needs call ordering) |
| BUG-005 | `POST /animals` | `name` longer than 64 bytes | `201`, but the stored name is cut to 64 bytes, possibly mid-character | Silent data corruption | Round-trip oracle: response name differs from the sent name |

## Adding a bug

1. Pick the next ID (`BUG-001`, `BUG-002`, …) and add a row above. Keep the
   trigger precise enough that someone else could reproduce it by hand.
2. Plant it in the code with a `// BUG-NNN:` comment explaining the flaw.
3. Add a test case that reproduces it.
4. The OpenAPI document describes the *intended* API, not the bug. Only
   change it when the bug needs a new endpoint.

IDs are never reused, even if a bug is removed.
| BUG-006 | `GET /feeds` | `diet` repeated, with an invalid value after the first (e.g. `?diet=herbivore&diet=pizzavore`) | `200`; only the first value is validated, the invalid one matches nothing. A single invalid `diet` correctly returns `400` | Input validation / repeated query parameter | Spec-driven enum check on every value of an array parameter; response-status oracle |
| BUG-007 | `POST /feeds/{id}/restock` | Restock so that stock exceeds 1,000,000 | `200`, stock is past the cap that `POST` and `PUT` enforce; sending that feed back with `PUT` returns `400` | Boundary / missing invariant | Round-trip oracle: response body fails the request schema |
| BUG-008 | `POST /animals/{id}/feedings` | Concurrent feedings that together ask for more than the stock | More than one `201`; stock goes negative | State / race (check-then-act) | Concurrent replay; invariant stock >= 0 via `GET /feeds/{id}` |
| BUG-009 | `DELETE /feeds/{id}` | Feed A used by an older feeding, then feed B used by a newer one; delete A | `204`, but should be `409`; the feeding of A now references a missing feed | Referential integrity (needs call ordering) | Sequence of feed, feed, feeding, feeding, delete; `GET /feeds/{feedId}` after listing feedings |
