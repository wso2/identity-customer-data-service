# CDS — Agent Guide

Context for coding agents working in this repository. Start with
[docs/architecture.md](docs/architecture.md) for the full picture; this file is the short list
of things that are easy to get wrong here.

## Commands

```bash
make build                      # compile + package target/cds-<version>.zip
make lint                       # golangci-lint — CI blocks on this
make unit-test                  # go test ./internal/... ./dbscripts/...
make integration-test           # full suite on PostgreSQL (testcontainers — needs Docker)
make integration-test-sqlite    # the same suite on the inbuilt database (no Docker)
make mq-integration-test        # ActiveMQ suite
```

Add `test=TestName` to filter any test target. Prefer `make integration-test-sqlite` for a quick
loop — it needs no Docker and runs the same suite. Before proposing a change as done, run
`make lint` and both integration targets: CI runs the suite against **both** datasources, and a
dialect mistake passes on one and fails on the other.

## Layering — do not cross these

```
handler → provider → service → store → system/database
```

1. `handler` does HTTP only: authn/authz, decode, validate, status codes, encode. **No SQL, no
   business rules.**
2. `store` does SQL only. **No policy decisions.**
3. `service` is the only layer that may reach into another domain, and only through that
   domain's `provider` package — never its `service` package directly.
4. `system/*` is infrastructure. Only `services` (the route table), `workers` (the unification
   consumer) and `queue`/`client` (domain models as payloads) may import a domain — **do not
   widen that list**. Anything else needing domain behaviour belongs in the domain.
5. Nothing imports `cmd`.

## Database

**Every SQL statement is declared in `internal/system/database/scripts/queries.go`** via
`newQuery`, never inline in a store.

- ID format `CDS-<DOMAIN>-<NN>` (`APP`, `SCH`, `UNR`, `PRF`, `CON`, `CKI`, `CFG`, `SYS`).
  IDs are permanent — take the next unused number, never renumber.
- The first body is the PostgreSQL statement and the default for all datasources. Pass a second
  body **only** where SQLite genuinely differs (`now()`, `::text` casts, `ILIKE`).
- **Add the statement to `scripts.AllQueries()` in `registry.go`.** That list is hand-maintained;
  a statement missing from it is never checked by the suite.
- For filters assembled at runtime, use the helpers in `scripts/dialect.go`
  (`LikeOperator`, `JSONEqCondition`, `JSONLikeCondition`) instead of branching on `DBType()`
  in the store.
- Bind all values. Any JSON key that reaches SQL from user input must pass `ValidateJSONKey`.

**A schema change must be applied to all three files** — nothing in the build keeps them in step:

- `dbscripts/postgres.sql` — external install
- `dbscripts/sqlite.sql` — inbuilt datasource (embedded)
- `test/setup/schema.sql` — integration suites

Always `defer dbClient.Close()` in a store. It is a no-op on the inbuilt datasource, whose handle
is shared, but PostgreSQL opens a real pool per `GetDBClient()` call and leaks it otherwise.
Never open your own `sql.DB`.

## Unification

Runs on a background worker, not the request path. The mechanism is in
[docs/architecture.md §6](docs/architecture.md); the rule semantics are in
`docs/concepts/how-unification-works.md`. Four things to know before touching
`internal/system/workers/profile_worker.go`:

- **The queued message is a hint.** The worker re-reads the profile from the store by id and
  ignores the payload it was handed — that is what makes redelivery and an external broker safe.
  Do not widen the message to carry profile data.
- **Evaluation stops at the first match**, and candidates come back in row order, so
  `GetAllReferenceProfileExceptCurrent` ordering is behaviour. Its two dialect bodies must agree —
  that is why the SQLite one orders by `rowid`.
- **Matching happens in Go over profile JSON**, never in SQL.
- **Failures are logged and dropped** — no retry, no dead-letter — and `persistMergedProfileData`
  is not transactional. Do not assume a merge either fully happened or fully did not.

## Security

Every handler starts with `security.AuthnAndAuthz(r, "<operation>")`, then checks
`isCDSEnabled(orgHandle)`. The operation string is a key into `auth_server.required_scopes` in
`deployment.yaml` — **scopes are configuration, not constants.** A new operation must be added to:

- `config/repository/conf/deployment.yaml`
- `install/helm/confs/deployment.yaml`
- the scope table in `README.md`

Read the org with `utils.ExtractOrgHandleFromPath(r)`, never by parsing the path — the org prefix
has already been stripped by the dispatcher. Every query is org-scoped; a statement without an
`org_handle` predicate is a cross-tenant leak.

Never log tokens, secrets, or whole profiles.

## Errors

Return `errors2.NewClientError(msg, status)` when the caller is at fault and
`errors2.NewServerError(msg, cause)` when CDS is — the type decides whether details reach the
client. Both take an `ErrorMessage` from the catalogue in `internal/system/errors/error_codes.go`;
reuse an existing `CDS-nnnnn` code where one fits rather than minting a near-duplicate. Hand the
error to `utils.HandleError(w, err)`.

## Configuration

A new setting needs a `yaml`-tagged field in `internal/system/config/config.go`, a documented
default in `config/repository/conf/deployment.yaml`, **and** the key in
`install/helm/confs/deployment.yaml`. Read it via `config.GetCDSRuntime().Config`.

Do not assume a block in the shipped `deployment.yaml` is read — `sync:` and `cache:` have no
fields in `config.Config` and do nothing. Check the struct.

## Conventions

- Apache 2.0 licence header on every new `.go` file — copy one from a neighbouring file.
- Routes are registered in `internal/system/services/<domain>_service.go` using Go 1.22 patterns
  (`"GET " + base + "/things/{thingId}"`); read variables with `r.PathValue`.
- Log through `log.GetLogger()` with typed fields (`log.Error(err)`, `log.String(k, v)`).
- Comments explain *why*, not *what*. Match the density of the surrounding file.
- One-line commit messages, no body, no issue or PR references.

## Where to look

| Question | File |
|---|---|
| Which handler serves this path? | `internal/system/services/` |
| What SQL runs for this? | `internal/system/database/scripts/queries.go` |
| What config keys exist? | `internal/system/config/config.go` |
| What error codes exist? | `internal/system/errors/error_codes.go` |
| How does merging decide? | `docs/concepts/how-unification-works.md` |
| How does merging actually run? | `docs/architecture.md` §6 |
| How do IS events reach CDS? | `docs/guides/is-sync.md` |
| How is any of this wired? | `docs/architecture.md` |
