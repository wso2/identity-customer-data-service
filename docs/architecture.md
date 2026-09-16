# CDS Architecture

How the Customer Data Service is put together: the layers a request passes through, the
packages that own each one, and the conventions that keep them consistent.

The [concept docs](README.md) explain *what* the domain means. This one explains *where the
code lives* and *why it is arranged this way* — read it before adding a feature, and keep it
current when the shape of the service changes.

---

## 1. What CDS is

A single Go binary that stores unified customer profiles and serves them over HTTPS. It sits
alongside WSO2 Identity Server (IS), which is its identity source and its token authority:

```
        ┌────────────────────────────┐
        │  Applications / SDKs / UI  │
        └─────────────┬──────────────┘
                      │ HTTPS + Bearer token
                      ▼
   ┌──────────────────────────────────────────┐
   │                  CDS                     │
   │                                          │
   │   HTTP layer ──► domain ──► datasource   │
   │        │                                 │
   │        └──► queue ──► background workers │
   └───────┬──────────────────────┬───────────┘
           │ introspect, claims,  │
           │ SCIM, applications   │ profile / schema-sync messages
           ▼                      ▼
   ┌───────────────┐      ┌──────────────────────┐
   │ WSO2 Identity │      │ in-memory queue  or  │
   │    Server     │      │ ActiveMQ (STOMP)     │
   └───────┬───────┘      └──────────────────────┘
           │ lifecycle events (POST /profiles/sync)
           └──────────────────────────────► CDS
```

Two properties drive most of the design:

- **Everything is org-scoped.** Every request carries an org through the URL prefix, and every
  table carries an `org_handle` column. There is no global data.
- **Writes are cheap, unification is not.** A profile write returns as soon as it is persisted;
  deciding whether that profile is the same person as an existing one happens on a background
  worker, off the request path.

---

## 2. Request path

```
  HTTPS request
        │
        ▼
  cmd/server/main.go ──► enableCORS ──► ServeMux
        │
        ▼
  system/managers.ServiceManager
        │
        ├── "/cds/"     ──► root mux ──► health, ready   (no org)
        └── "/t/{org}/" ──► tenant dispatcher ──► shared routes mux
                                                       │
                                                       ▼
                                          system/services/*_service.go
                                                       │  route table
                                                       ▼
                                          <domain>/handler   HTTP concerns
                                                       │
                                                       ▼
                                          <domain>/provider  indirection seam
                                                       │
                                                       ▼
                                          <domain>/service   business rules
                                                       │
                                                       ▼
                                          <domain>/store     SQL
                                                       │
                                                       ▼
                                          system/database    client + dialects
```

### Org dispatch

`utils.MountTenantDispatcher` owns the `/t/` prefix. It lifts the segment after it, puts that in
the request context under `constants.TenantContextKey`, rewrites `r.URL.Path` to the remainder,
and forwards to a *single shared routes mux*.

That is why every service registers plain paths like `/cds/api/v1/profiles` — by the time the
routes mux sees the request, the org prefix is gone. Handlers read the org back with
`utils.ExtractOrgHandleFromPath(r)`, never by parsing the path themselves. A `/t/` path with no
remainder after the org segment is a 400; unknown paths 404 from the routes mux.

| Prefix | Identifier | Used by |
|---|---|---|
| `/t/{org-handle}` | tenant handle (e.g. `carbon.super`) | every org-scoped endpoint |
| `/cds/` | none | `/health`, `/ready` — mounted on their own root mux, since they must answer before any org exists |

### Routing

Routes use Go 1.22+ `ServeMux` method-and-wildcard patterns
(`"GET /cds/api/v1/profiles/{profileId}"`), so there is no router dependency and no manual path
matching. Path variables come from `r.PathValue("profileId")`.

`ApiBasePath` (`/cds/api`) lives in `system/constants`; services compose `base = ApiBasePath + "/v1"`.

---

## 3. Package layout

```
cmd/server/            process lifecycle: config, logger, DB, workers, TLS server, shutdown
internal/
  <domain>/            one directory per bounded context
    handler/           HTTP: authn/authz, decode, validate, status codes, encode
    service/           business rules; the only layer that composes other domains
    store/             SQL execution and row→model mapping
    model/             domain structs and their JSON shape
    provider/          constructor seam returning the service interface
  system/              cross-cutting infrastructure (see below)
api/                   OpenAPI specification
config/repository/conf/ deployment.yaml shipped in the distribution
dbscripts/             postgres.sql (applied by the operator), sqlite.sql (embedded)
docs/                  this documentation
install/helm/          Helm chart
scripts/local-setup/   one-shot local IS + CDS environment
test/                  integration and scenario suites
```

### Domains

| Domain | Owns | Endpoints |
|---|---|---|
| `profile` | profiles, cookies, application data, consent records on a profile | `/profiles…` |
| `profile_schema` | attribute definitions, types, merge strategies, IS claim sync | `/profile-schema…` |
| `unification_rules` | the rules that decide when two profiles are one person | `/unification-rules…` |
| `consent` | consent categories and the attributes each one covers | `/consent-categories…` |
| `admin_config` | per-org settings: whether CDS is enabled, the org's system applications, initial-sync state | `/config` |
| `application` | resolving an OAuth `client_id` to an application identifier | — internal |
| `identity_provider` | IS response models | — internal |
| `health_check` | liveness and readiness | `/health`, `/ready` |

### `system/` — infrastructure

| Package | Responsibility |
|---|---|
| `managers` | Builds the mux: the root mux for health, the shared routes mux, and the tenant dispatcher over it |
| `services` | One file per domain; the **route table** — the place to look up which handler serves a path |
| `config` | `deployment.yaml` → `config.Config`; `GetCDSRuntime()` is the process-wide accessor |
| `authn` | Token validation — JWT claims (org, audience, expiry) plus introspection; opaque tokens are Console-only |
| `authz` | Scope check against `auth_server.required_scopes` from config |
| `security` | `AuthnAndAuthz(r, operation)` — the single entry point handlers call; also admin Basic auth |
| `client` | Outbound IS calls: token, introspection, claims, SCIM users, applications; TLS/mTLS setup |
| `database` | Type resolution, driver constants, SQLite value normalisation |
| `database/provider` | Connection pools and inbuilt-database bootstrap |
| `database/client` | `ExecuteQuery`, `BeginTx`; selects the dialect for a statement |
| `database/scripts` | Every SQL statement, plus the dialect fragments stores assemble at runtime |
| `queue` | Provider registry and the two queue interfaces |
| `workers` | Consumers: profile unification, schema sync, cookie cleanup |
| `errors` | `ClientError` / `ServerError` and the `CDS-nnnnn` code catalogue |
| `log` | `slog` wrapper with typed fields |
| `cache` | Small TTL cache — currently unused (see §11) |
| `pagination` | Cursor paging helpers |
| `constants`, `utils` | Shared values; the tenant dispatcher, `ExtractOrgHandleFromPath`, `HandleError`, `RespondJSON` |

### The `provider` seam

Every domain exposes a `provider` package whose only job is to hand back the service interface:

```go
profilesService := provider.NewProfilesProvider().GetProfilesService()
```

It looks like indirection for its own sake, and for a single implementation it nearly is. It
earns its place by giving cross-domain callers one import that does not pull in the other
domain's concrete types — the handler depends on `admin_config/provider`, not on
`admin_config/service` — and by giving tests one place to substitute an implementation.

**Rule:** cross-domain calls go through `provider`. Same-domain calls go straight to the
service.

### Layering rules

1. `handler` never touches `store`. It has no SQL and no business rules.
2. `store` never touches `handler`, and never decides policy — it executes statements and maps rows.
3. `service` is the only layer allowed to reach into another domain, and does so via `provider`.
4. `system/*` is infrastructure and mostly depends on no domain — `config`, `database`, `errors`,
   `log`, `authn`, `authz`, `security`, `utils`, `constants` import none. Four packages
   deliberately do, and the list should not grow:

   | Package | Imports | Why |
   |---|---|---|
   | `services` | every domain's `handler` | it *is* the route table |
   | `workers` | `profile`, `profile_schema`, `unification_rules` | the unification consumer drives the domain |
   | `queue`, `client` | domain `model` packages only | those models are the message and response payloads |

   Anything else that needs domain behaviour belongs in the domain, not in `system`.
5. Nothing imports `cmd`.

---

## 4. Data layer

### One statement, two dialects

CDS runs on PostgreSQL or on an inbuilt SQLite datasource, selected by `datasource.type`. Stores
are written once and stay datasource-agnostic, because dialect selection happens in exactly two
places.

**Statements** are declared in `system/database/scripts/queries.go` via `newQuery`:

```go
var UpsertApplication = newQuery("CDS-APP-01",
    `INSERT INTO applications (...) VALUES ($1, $2, $3, now())
     ON CONFLICT (app_id) DO UPDATE SET ...`,
    // SQLite has no now(); strftime matches the format the schema defaults use.
    `INSERT INTO applications (...) VALUES ($1, $2, $3, strftime(...))
     ON CONFLICT (app_id) DO UPDATE SET ...`)
```

The first body is the PostgreSQL statement and the default for every datasource. The optional
second is the SQLite override, supplied only where the dialects actually differ. `DBClient.ExecuteQuery`
picks between them — the store just passes `scripts.UpsertApplication`.

Statement IDs follow `CDS-<DOMAIN>-<NN>` (`APP`, `SCH`, `UNR`, `PRF`, `CON`, `CKI`, `CFG`, `SYS`).
**An ID is permanent**: a new statement takes the next unused number in its domain; existing ones
are never renumbered.

Every statement must also be listed in `scripts.AllQueries()` in `registry.go`. That list is
maintained by hand and is what lets the test suite prepare and check each statement without
executing it — a statement left out of it is a statement nobody checks.

**Runtime-assembled fragments** — filters whose shape depends on user input — cannot be a fixed
pair of strings, so `scripts/dialect.go` supplies the per-dialect pieces: `LikeOperator`
(`ILIKE` vs `LIKE`), `JSONEqCondition`, `JSONLikeCondition`. These are the only cases where a
store asks the client for `DBType()`.

> JSON keys reach these helpers from user-supplied filters and are interpolated into SQL, so
> `ValidateJSONKey` restricts them to the filter character set. Values are always bound.

### Connection handling

The two datasources are handled differently, and the difference is worth knowing before you
write a store.

- **Inbuilt (SQLite):** opened **once** behind a `sync.Once`, schema applied on that first open,
  and shared for the process lifetime. `GetDBClient()` hands back a `SharedDBClient` over it
  whose `Close()` is a deliberate no-op — a single file written by a single process must not have
  its handle closed underneath a concurrent caller.
- **PostgreSQL:** `GetDBClient()` calls `sql.Open` and `Ping` **per call** and returns a
  `DBClient` whose `Close()` really closes.

So the `defer dbClient.Close()` that every store writes is load-bearing on PostgreSQL and a
no-op on SQLite. Write it either way — omitting it leaks a pool on PostgreSQL.

`database/sql` is itself a pool, so opening one per call means a profile write — dozens of
statements — pays a TCP, TLS and authentication round trip for each, and `datasource.max_open_conns`
has nothing to size. Sharing the PostgreSQL handle the way SQLite's is shared is the obvious
improvement; see §11.

### The three copies of the schema

| File | Used by |
|---|---|
| `dbscripts/postgres.sql` | external PostgreSQL install — applied by the operator |
| `dbscripts/sqlite.sql` | inbuilt datasource — embedded via `dbscripts/embed.go`, applied on first start |
| `test/setup/schema.sql` | integration suites |

Nothing in the build connects them, so a table added to one and not the others passes CI and
breaks on install. **Any schema change must be applied to all three.**

### Tables

| Table | Holds |
|---|---|
| `profiles` | the profile itself; `traits` and `identity_attributes` as JSON |
| `profile_reference` | merge graph — which profile was merged into which, and why |
| `application_data` | per-application data for a profile |
| `profile_cookies` | anonymous-visitor cookie → profile |
| `profile_consents` | consent decisions per profile |
| `profile_schema` | attribute definitions per org |
| `unification_rules` | matching rules per org |
| `profile_unification_modes`, `profile_unification_triggers` | per-org unification settings |
| `consent_categories`, `consent_category_attributes` | consent taxonomy |
| `applications` | `client_id` → app identifier |
| `cds_config` | per-org admin config |

---

## 5. Asynchronous processing

### Queue providers

`system/queue` defines two interfaces — `ProfileUnificationQueue` and `SchemaSyncQueue` — and a
registry keyed by provider name. `message_queue.type` selects one:

- `memory` (default, and the value when the key is absent) — a buffered channel. Single instance only.
- `activemq` — STOMP. Registered by the blank import of the provider package in `main.go`, which
  runs its `init()`.

A new broker is added by implementing the interfaces and calling `RegisterProfileQueueProvider` /
`RegisterSchemaSyncQueueProvider` from an `init()`. See
[Extending Queue Providers](guides/extending-queue-providers.md).

### Workers

Started by `main` after the database is ready, stopped in reverse order on `SIGINT`/`SIGTERM`:

| Worker | Trigger | Does |
|---|---|---|
| Profile unification | a profile is created or updated | Re-reads the profile, evaluates unification rules against existing profiles, merges or records a reference |
| Schema sync | a claim-change event from IS on `POST /profile-schema/sync` | Pulls claims from IS and reconciles `profile_schema` |
| Cookie cleanup | `cleanup.cookie.interval` | Deletes expired cookie rows in batches |

The unification path is documented in [How Unification Works](concepts/how-unification-works.md).

### Coupling

`system/workers` is the one infrastructure package that reaches deep into domains — it calls
`profile/store` and `profile_schema/store` directly and pulls unification rules through
`unification_rules/provider`. That is why it appears on the exception list in §3 rather than
being a violation of it.

It also means the merge logic — `MergeProfiles`, `MergeAttributeValue`, the per-strategy value
handling — currently lives in `system/workers/profile_worker.go` rather than in the `profile`
domain. Treat that as the existing shape, not as a pattern to extend: new work should go into
the domain and be called from the worker.

---

## 6. Security

### Inbound

Every handler begins with one call:

```go
if err := security.AuthnAndAuthz(r, "profile:view"); err != nil {
    utils.HandleError(w, err)
    return
}
```

`AuthnAndAuthz` runs authentication then the scope check. The operation string
(`"profile:view"`) is a key into `auth_server.required_scopes` in `deployment.yaml`, which maps
it to the scopes a token must carry. **Scopes are configuration, not constants** — adding an
operation means adding it to the shipped `deployment.yaml`, the Helm chart values, and the README
scope table.

| Token | Accepted when |
|---|---|
| JWT | `org_handle` matches the request org, audience is `iam-cds`, not expired, and introspection reports it active |
| Opaque | introspection reports it active **and** `client_id` is the Console app |

The two IS-facing sync endpoints — `POST /profiles/sync` and `POST /profile-schema/sync` — use
admin Basic auth instead (`security.AuthnWithAdminCredentials`), compared in constant time
against `auth_server.admin_username` / `admin_password`. They are called by IS, not by an
application, so there is no user token to present.

Then: `isCDSEnabled(orgHandle)` — an org that has not enabled CDS gets a 400 regardless of scopes.

### Transport

TLS is mandatory; the server refuses to start without a certificate and key (default
`etc/certs`, `tls.cert_dir` to override). `tls.mtls_enabled` makes outbound IS calls present a
client certificate. CORS is an explicit allowlist — `auth.cors_allowed_origins` — and a
non-matching origin simply receives no CORS headers.

### Secrets

`deployment.yaml` holds `${ENV_VAR}` placeholders, resolved at load from the environment and from
`config/*.env`. Secrets are never committed; the Helm chart supplies them through
`cds-env-secret.yaml`.

---

## 7. Errors and logging

Two error types in `system/errors`, and the distinction is load-bearing:

| Type | Means | Response |
|---|---|---|
| `ClientError` | the caller did something wrong | its `StatusCode`, and the code/message/description are returned |
| `ServerError` | CDS did something wrong | 500 and a generic body; details are logged, never returned |

`utils.HandleError(w, err)` does the type switch. Both carry an `ErrorMessage` from the catalogue
in `errors/error_codes.go`, coded `CDS-<5 digits>` and grouped by domain (`151xx` profile schema,
and so on). Reuse an existing code where one fits rather than minting a near-duplicate.

Logging goes through `log.GetLogger()` with typed fields (`log.Error(err)`, `log.String(k, v)`).
`Debug` carries the diagnostic detail; `Info` marks completed operations. Never log a token, a
secret, or a full profile.

---

## 8. Configuration

`deployment.yaml` under `<CDS_HOME>/repository/conf/` is the single configuration file. CDS home
is resolved from `--cdsHome`, then `CDS_HOME`, then the working directory. `config.LoadConfig`
parses it into `config.Config` and `config.GetCDSRuntime().Config` serves it everywhere.

Adding a setting means: a field with a `yaml` tag in `system/config/config.go`, a documented
default in the shipped `config/repository/conf/deployment.yaml`, and the matching key in
`install/helm/confs/deployment.yaml`. A setting that exists in the struct but not in the shipped
file is a setting nobody will find.

| Section | Controls |
|---|---|
| `addr`, `server_url` | bind address; the base URL used for `Location` headers |
| `auth`, `auth_server` | CORS origins; IS endpoints, credentials, and the scope table |
| `datasource` | `postgres` or `sqlite`, plus the inbuilt database's path and pool bound |
| `message_queue` | queue provider and broker settings |
| `tls` | certificates, mTLS |
| `cleanup.cookie` | cookie cleanup interval and batch size |
| `application_identifier_type` | whether applications are keyed by `client_id` or `app_id` |

---

## 9. Build, test, run

```
make build                      # compile and package target/cds-<version>.zip
make lint                       # golangci-lint
make unit-test                  # go test ./internal/... ./dbscripts/...
make integration-test           # full suite on PostgreSQL (testcontainers — needs Docker)
make integration-test-sqlite    # the same suite on the inbuilt database (no Docker)
make mq-integration-test        # ActiveMQ suite
make all                        # clean, lint, build, integration-test
```

`test=TestName` filters any of the test targets.

The integration suite runs **twice in CI, once per datasource**, driven by `CDS_TEST_DB` in
`test/integration/main_test.go`. `TestMain` starts the datasource, installs it globally via
`provider.SetTestDB`, and starts the real workers — so integration tests exercise the true
async path, and a dialect mistake shows up in the run the other datasource would have hidden.

The PR builder runs build, lint, both integration runs, unit tests, and the MQ suite. Any of them
failing blocks the merge.

For a local IS + CDS environment, see [Local Development Setup](guides/local-development.md).

---

## 10. Deployment

The build produces a zip containing the `cds` binary, `repository/` (config and certs), and
`dbscripts/`. Unzip and run `./cds`.

The Helm chart in `install/helm/` deploys the same binary with `cds-config.yaml` for
`deployment.yaml`, `cds-env-secret.yaml` for secrets, separate ingresses for tenant traffic and
health, an HPA and a PDB. Its default of multiple replicas **requires PostgreSQL** — the inbuilt
datasource is a single file written by a single process, and the in-memory queue is not shared
between pods. A multi-replica deployment therefore needs `datasource.type: postgres` and an
external broker.

---

## 11. Known gaps

Kept honest on purpose; if you close one, delete the entry.

- **PostgreSQL opens a connection pool per `GetDBClient()` call** (§4), so pool settings are
  effectively inert on it and every statement pays a fresh connection. The inbuilt datasource
  already shares one handle; PostgreSQL should do the same.
- **`api/customer-data-service.yaml` has drifted.** It still describes `/events` and
  `/enrichment-rules`, which no service registers, and omits routes that exist. The route tables
  in `internal/system/services/` are the source of truth until the spec is regenerated.
- **`scripts.AllQueries()` is maintained by hand.** A new statement left out of it is silently unchecked.
- **The three schema files are kept in step by convention**, not by the build.
- **Handlers are thick.** `profile_handler.go` carries validation and composition that would sit
  more naturally in the service layer. New code should not follow it further in that direction.
- **The shipped `deployment.yaml` has unwired sections.** `sync:` and `cache:` have no matching
  fields in `config.Config`, so setting them does nothing — schema sync is event-driven and
  `system/cache` has no callers. Do not assume a block in the YAML is read; check the struct.
