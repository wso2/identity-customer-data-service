---
name: add-endpoint
description: Add a new REST endpoint to CDS across every layer it touches — route, handler, service, store, SQL statement, scopes, schema, and tests. Use when adding or changing a CDS API operation, a new domain, or a persisted field behind an endpoint.
---

# Adding a CDS endpoint

A CDS endpoint is a vertical slice through seven places. Skipping one of them usually still
compiles, which is why this checklist exists. Read [docs/architecture.md](../../../docs/architecture.md)
first if you have not.

Work in this order — each step depends on the one above it.

## 1. Decide the domain

Endpoints live in an existing domain under `internal/` (`profile`, `profile_schema`,
`unification_rules`, `consent`, `admin_config`) unless the concept is genuinely new. A new
domain needs the full set: `handler/ service/ store/ model/ provider/`.

Find the closest existing endpoint and read its whole slice before writing anything. Match it.

## 2. Model

`internal/<domain>/model/` — the domain struct plus, where the wire shape differs from the stored
shape, a separate API struct (see `admin_config/model` for the `AdminConfig` / `AdminConfigAPI` /
`AdminConfigUpdateAPI` split: pointer fields on the update type distinguish "absent" from "false").

## 3. SQL

In `internal/system/database/scripts/queries.go`:

```go
// GetThingById fetches one thing for an org.
var GetThingById = newQuery("CDS-PRF-42",
    `SELECT thing_id, org_handle, payload::text FROM things
       WHERE org_handle = $1 AND thing_id = $2`,
    // SQLite has no ::text cast.
    `SELECT thing_id, org_handle, CAST(payload AS TEXT) AS payload FROM things
       WHERE org_handle = $1 AND thing_id = $2`)
```

- ID is `CDS-<DOMAIN>-<NN>` with the next unused number in that domain. Never renumber.
- Supply a SQLite override **only** where the dialects differ — `now()`, `::text`/`::jsonb`
  casts, `ILIKE`, `SERIAL`. Otherwise one body covers both.
- Every statement carries an `org_handle` predicate.
- **Add it to `AllQueries()` in `registry.go`.** Hand-maintained; omission means unchecked.

If the endpoint needs a new table or column, apply it to **all three** schema files:
`dbscripts/postgres.sql`, `dbscripts/sqlite.sql`, `test/setup/schema.sql`.

## 4. Store

`internal/<domain>/store/`. One function per statement:

```go
dbClient, err := provider.NewDBProvider().GetDBClient()
// ... wrap err in errors2.NewServerError with the domain's error code
defer dbClient.Close()   // no-op on the shared pool — correct and cheap

results, err := dbClient.ExecuteQuery(scripts.GetThingById, orgHandle, thingID)
```

Map rows to the model here. No policy decisions, no HTTP types. Column keys come back lowercased.

## 5. Service

`internal/<domain>/service/`. Add the method to the domain's `…ServiceInterface` and implement
it. This is where validation that is not about request *shape* belongs — uniqueness, referential
rules, cross-domain lookups.

Cross-domain calls go through the other domain's provider:

```go
schemaService := schemaProvider.NewProfileSchemaProvider().GetProfileSchemaService()
```

## 6. Handler

`internal/<domain>/handler/`. Every handler opens the same way:

```go
func (h *ThingHandler) GetThing(w http.ResponseWriter, r *http.Request) {
    if err := security.AuthnAndAuthz(r, "thing:view"); err != nil {
        utils.HandleError(w, err)
        return
    }
    orgHandle := utils.ExtractOrgHandleFromPath(r)
    if !isCDSEnabled(orgHandle) { /* CDS_NOT_ENABLED, 400 */ }

    thingID := r.PathValue("thingId")
    // ... decode, validate shape, call the service, respond
}
```

Use `utils.RespondJSON(w, status, payload, "thing")`. Return `ClientError` for caller mistakes and
`ServerError` for ours — the type decides what reaches the client. Add any new `CDS-nnnnn` code to
`internal/system/errors/error_codes.go` in the domain's number band.

## 7. Route

`internal/system/services/<domain>_service.go`:

```go
s.mux.HandleFunc("GET "+base+"/things/{thingId}", s.handler.GetThing)
```

Register static paths before wildcard ones, as the existing files do. A brand-new domain also
needs `services.NewThingService(routesMux)` added to `RegisterServices()` in
`internal/system/managers/servicemanager.go`.

Do **not** include `/t/{org}` in the pattern — the dispatcher has already stripped
the prefix.

## 8. Scopes

A new operation string needs its scope mapping in **all three**:

- `config/repository/conf/deployment.yaml` under `auth_server.required_scopes`
- `install/helm/confs/deployment.yaml`
- the scope table in `README.md`

An operation with no entry fails authorization for every caller — with no obvious clue why.

## 9. Spec and docs

Update `api/customer-data-service.yaml`. Note the spec has already drifted from the routes, so
check the route table rather than trusting it. If the endpoint changes a documented concept,
update the matching file under `docs/concepts/` or `docs/guides/`.

## 10. Tests

Add to `test/integration/` following the existing suites. `TestMain` starts a real datasource and
real workers, so integration tests exercise the true async path.

Then run, in order:

```bash
make lint
make unit-test
make integration-test-sqlite    # fast, no Docker
make integration-test           # PostgreSQL — catches dialect mistakes the above hides
```

Both datasource runs must pass. CI runs both and blocks the merge on either.

## Checklist

- [ ] Model (and API struct, if the wire shape differs)
- [ ] Statement in `queries.go` with a fresh `CDS-<DOMAIN>-<NN>` id
- [ ] Statement added to `AllQueries()`
- [ ] Schema change applied to all three `.sql` files
- [ ] Store function, org-scoped
- [ ] Service method on the interface and the implementation
- [ ] Handler with `AuthnAndAuthz` + `isCDSEnabled`
- [ ] Error codes added to the catalogue
- [ ] Route registered
- [ ] Scopes in both `deployment.yaml` files and the README
- [ ] OpenAPI spec updated
- [ ] Integration test
- [ ] Licence header on every new file
- [ ] `make lint` and both integration targets green
