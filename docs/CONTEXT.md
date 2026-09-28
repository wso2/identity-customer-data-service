# CDS / CDM — Full Context

> **Version: `v0.3.65+71fb3bd`** — regenerated 2026-09-28. Previous snapshot: `v0.3.65+bea5aa4` (`archive/cds-context-v0.3.65+bea5aa4.md`).
>
> **Source of truth: the code.**
> Reconciled against [`wso2/identity-customer-data-service`](https://github.com/wso2/identity-customer-data-service) `main` @ `71fb3bd`.
>
> **On the version string.** `version.txt` at `71fb3bd` still reads `v0.3.65`, unchanged for the fourth snapshot running, so this one is again labelled with semver build metadata — `version.txt` value + HEAD short SHA — to keep it distinguishable.
>
> Two things to know about that number. First, as of commit `f4b1da6` `version.txt` is owned by the release builder and moves **only when a release is cut**, no longer on every merge to `main` (see §18) — so from here on it will lag `main` by design. Second, **`v0.3.65` was never released.** The repo's tags stop at **`v0.3.61`**; there is no `v0.3.65` tag. `a9bad2f` ("Version bump: v0.3.65") is a leftover of the *old* bump-on-merge workflow, which advanced the file on merges regardless of whether anything was released. So the current value is neither "what is on `main`" nor "the last release" — it is a stale artefact of the workflow that was just deleted, and the `v0.3.62`–`v0.3.65` numbers never corresponded to releases at all.
> Where the repo's own `docs/` or `api/customer-data-service.yaml` disagree with the Go source, the Go source wins.
> Product vision, use cases, and roadmap sections are carried over from internal requirements material — these are intent, not implementation, and are labelled as such.

---

## Table of Contents

1. [Product Vision & Strategic Context](#1-product-vision--strategic-context)
2. [Core Use Cases](#2-core-use-cases)
3. [Repository & Runtime Architecture](#3-repository--runtime-architecture)
4. [Routing & URL Structure](#4-routing--url-structure)
5. [Authentication, Authorization & Scopes](#5-authentication-authorization--scopes)
6. [Admin Config & Org Enablement](#6-admin-config--org-enablement)
7. [Applications & System Apps](#7-applications--system-apps)
8. [Data Model & Terminology](#8-data-model--terminology)
9. [Profiles](#9-profiles)
10. [Profile Schema](#10-profile-schema)
11. [Consent Management](#11-consent-management)
12. [Unification Rules](#12-unification-rules)
13. [How Profile Unification Works](#13-how-profile-unification-works)
14. [IS Sync — Identity Server Event Integration](#14-is-sync--identity-server-event-integration)
15. [Schema Sync](#15-schema-sync)
16. [Message Queue & Extending Providers](#16-message-queue--extending-providers)
17. [Database Schema](#17-database-schema)
18. [Deployment & Configuration](#18-deployment--configuration)
19. [Real-World Scenarios](#19-real-world-scenarios)
20. [MVP & Roadmap](#20-mvp--roadmap)

---

## 1. Product Vision & Strategic Context

> *Intent — from requirements material, not code.*

**We are NOT trying to enter the CDP market.** The goal is to incorporate the useful/possible parts of CDP needs into the WSO2 IAM product.

### Why build this?

- CDPs are bulky, expensive, and overbuilt for SMBs
- Existing CRM systems lack sufficient CDP capabilities
- Identity is the **foundation** of personalization — as a CIAM vendor, WSO2 is uniquely positioned to turn authentication and user context into real-time personalized experiences
- Enables "CIAM 3.0": connecting personalization and identity (the "digital double" story)
- Democratizes enterprise-grade personalization for SMBs without requiring a full CDP stack

### Who are we solving this for?

- **Developers** — want to integrate personalization quickly with minimal overhead
- **Product teams** — want engagement/conversion improvements without dedicated data teams
- **End users / customers** — expect modern, relevant product experiences from first interaction
- **Enterprises with siloed systems** — many systems holding fragmented customer data, wanting a Customer 360 view
- **Startups** — building new digital experiences, want to offload both IAM and customer data storage

### Positioning

> "WSO2's Customer Data Service brings customer context closer to where identity is already managed — at the point of authentication and access."

Key benefits:
- Unified, identity-first customer data management
- Omnichannel 360-degree customer data (web, mobile, in-person)
- Security, privacy, and consent-aware by design
- Developer-friendly APIs and SDKs for personalization
- Guest user profile management and anonymous-to-identified conversion tracking
- Cost-effective entry point (leverages existing IAM infrastructure)

### What CDS is NOT

- Not a full CDP (no segmentation engine, no campaign management OOTB)
- Not replacing IS — it works alongside IS
- Not handling merging of user **accounts** with different UserIDs (that's an IS responsibility) — and this is enforced in code: two permanent profiles with different `user_id` values are never merged

---

## 2. Core Use Cases

| Use Case | Description | Implemented? |
|---|---|---|
| **A1 — Identity Stitching** | Link anonymous user activity to registered accounts post-signup/login | Yes — cookie + `AUTHENTICATION_SUCCESS` / `POST_ADD_USER` |
| **A2 — Unifying Siloed Profiles** | Merge user profiles across devices, channels, applications | Yes — unification rules + worker |
| **B1 — Conversion Tracking** | Track anonymous-to-registered conversion | Partial — the lineage exists (`merged_from`), no analytics layer |
| **C1 — Master Data Management** | Single source of truth for customer profiles | Yes — profiles + schema + merge strategies |
| **D — Personalization** | APIs for app developers to use customer data to customize app behavior | Yes — consent-filtered profile reads, `/profiles/Me` |

---

## 3. Repository & Runtime Architecture

| Property | Value |
|---|---|
| Repo | `wso2/identity-customer-data-service` |
| Language | Go — `go 1.26.5` in `go.mod`; README states Go 1.26+ |
| License | Apache-2.0 |
| Version pinned here | `v0.3.65` (`version.txt`) — **not a released version.** The newest tag in the repo is `v0.3.61`; `v0.3.62`–`v0.3.65` exist only as `version.txt` values written by the bump-on-merge workflow deleted in `f4b1da6`. Since that commit the file moves only when the release builder cuts a release (§18), so it will now lag `main` deliberately rather than drifting from it accidentally. |
| Last actual release | `v0.3.61` (newest git tag) |
| Datastore | **Two options.** Inbuilt SQLite (`datasource.type: sqlite`, **the default when unset**) or PostgreSQL (`postgres`). `dbscripts/sqlite.sql` is embedded and applied by the server; `dbscripts/postgres.sql` is applied by the operator. Since `bea5aa4` the process holds **one shared connection pool per datasource** for its whole life (§3). |
| SQLite driver | `modernc.org/sqlite v1.53.0` (pure Go — no CGO, no external server) |
| HTTP | Go stdlib `net/http` + `ServeMux` with method+pattern routing (`"GET /path/{id}"`) |
| Build | `make all` → `target/cds-<version>.zip` |
| Test targets | `make integration-test` (PostgreSQL, needs Docker), `make integration-test-sqlite` (`CDS_TEST_DB=sqlite`, no Docker), `make unit-test` (**now `go test -race`**, because the process shares one connection pool across every request and every worker), `make mq-integration-test` |
| Container | `Dockerfile`; Helm chart at `install/helm` |

### Top-level layout

```
api/                    OpenAPI spec (not generated from code; endpoint list is authoritative in §4)
cmd/server/             main.go — entrypoint, blank-imports queue providers
config/repository/conf/ deployment.yaml
dbscripts/              postgres.sql (operator-applied), sqlite.sql (embedded via embed.go)
docs/                   concepts/ + guides/
install/helm/           Chart, values, templates (deployment, svc, hpa, pdb, ingress)
internal/               all application code
scripts/local-setup/    script.sh + templates/ — scripted CDS + Identity Server dev setup (§18)
test/                   integration, activemq_integration, setup + scenario guides
```

### Domain modules under `internal/`

Each follows the same layering: `handler/` (HTTP) → `provider/` (DI) → `service/` (business logic) → `store/` (SQL) with `model/` for types.

| Module | Purpose |
|---|---|
| `profile` | Profile CRUD, cookies, IS sync endpoint, per-profile consents |
| `profile_schema` | Schema attribute CRUD + IS schema sync endpoint |
| `unification_rules` | Rule CRUD, priority/name uniqueness |
| `consent` | Consent category CRUD |
| `admin_config` | Per-org `cds_enabled`, `initial_schema_sync_done`, `system_applications` |
| `application` | Maps `client_id` ↔ `app_id` per org |
| `identity_provider` | Models for IS responses (applications list, claims) |
| `health_check` | Liveness/readiness |

### `internal/system/` (cross-cutting)

`authn` (JWT parse / introspection), `authz` (scope check), `security` (middleware, `AuthnAndAuthz`, `AuthnWithAdminCredentials`), `cache`, `client` (Identity Server client), `config`, `constants`, `database` (+ `client`, `model`, `provider`, `scripts`), `errors`, `log`, `managers` (service registration), `pagination`, `queue` (+ `inmemory`, `activemq`), `services` (route registration), `utils`, `workers`.

### Datasource abstraction & query layer (`internal/system/database/`)

Reworked in v0.3.63 to support two dialects. **Stores are datasource-agnostic**; the client is the only place that picks a dialect.

| File / package | Role |
|---|---|
| `constants.go` | `TypePostgres` / `TypeSQLite`, `DefaultType = TypeSQLite`, `ResolveType` (empty → default, lowercased/trimmed), `IsSupportedType`, driver names, SQLite defaults, **and the PostgreSQL pool defaults** (`DefaultPostgresMaxOpenConns` 100, `MaxIdleConns` 25, `ConnMaxLifetime` 30m, `ConnMaxIdleTime` 5m, `ConnectTimeout` 10s) |
| `args.go` | `NormalizeSQLiteArgs` — binds `[]byte` args as text so marshalled JSON stays queryable by `json_extract` instead of landing as a BLOB |
| `model/dbquery.go` | `DBQuery{ID, Query, SQLiteQuery}` + `GetQuery(dbType)`, `Format(...)`, `Append(...)`, `WithSQL(...)` |
| `model/tx.go` | `Tx` wrapper carrying `dbType`, so statements inside a transaction resolve the same dialect as outside. **`ExecContext` / `QueryContext` are the live methods**; `Exec` / `Query` remain as `context.Background()` shims marked `Deprecated` |
| `client/dbclient.go` | `ExecuteQueryContext(ctx, model.DBQuery, ...)`, `BeginTxContext(ctx) (*model.Tx, error)`, `DBType()`, `Close()`. `ExecuteQuery` / `BeginTx` remain as `Deprecated` `context.Background()` shims. **`NewSharedDBClient` is now the only constructor** — `NewDBClient` and the `shared` flag are gone; `Close()` is unconditionally a no-op because the pool belongs to the process |
| `client/normalize.go` | Coerces SQLite scan results to the Go types `lib/pq` returns, keyed on declared column type: `JSONB` string→`[]byte`, `BOOLEAN` `0/1`→`bool`, `TIMESTAMP` text→`time.Time` |
| `provider/dbprovider.go` | Resolves type and returns the **process-owned pool** — one per datasource, opened on first use under `dbMu`, kept for the life of the process. `resolvePostgresPoolSettings` / `resolveSQLiteMaxOpenConns` decide every number; `CloseDB()` closes the pools at shutdown and makes every later request return `ErrDatabaseClosed`; `SetTestDB(db, dbType)` for tests |
| `provider/bootstrap.go` | `ValidateDataSource` (unsupported type; for `postgres`, missing hostname/username/password/name; **and `validateNumericSettings`**, which resolves the pool numbers with the same functions the pool uses, reading only the block that applies to the configured type) and `EnsureDatabase` (SQLite: file + dir + embedded schema in a transaction with statement-by-statement fallback. **PostgreSQL: now opens the shared pool**, so an unreachable host or a bad setting fails the start rather than the first request) |
| `scripts/query.go` | `newQuery(id, base, sqliteOverride...)`. `base` is the PostgreSQL text and the fallback for every other dialect |
| `scripts/queries.go` | All statements as `DBQuery` values (previously `map[string]string` keyed by db type) |
| `scripts/registry.go` | `AllQueries()` — hand-maintained map of all **68** statements, so tests can prepare/validate each one on both datasources. A statement omitted here goes unprepared and unchecked. |
| `scripts/dialect.go` | Runtime SQL fragments that differ per dialect: `LikeOperator`, `JSONEqCondition`, `JSONLikeCondition`, `KeysetCondition`, `EncodeStringArray` / `DecodeStringArray`, `ValidateJSONKey` |

**Statement ID convention:** `CDS-<DOMAIN>-<NN>` where DOMAIN ∈ `APP` (2), `SCH` (15), `UNR` (5), `PRF` (18), `CON` (15), `CKI` (8), `CFG` (4), `SYS` (1) — 68 total. IDs are permanent — new statements take the next unused number rather than renumbering. IDs appear in query error messages (`query CDS-PRF-07 failed: ...`).

**Migration note for readers of the old context:** the store idiom `scripts.X[provider.NewDBProvider().GetDBType()]` is gone. Stores now pass `scripts.X` (a `DBQuery`) straight to `dbClient.ExecuteQueryContext(ctx, …)` — the context-free `ExecuteQuery` survives only as a deprecated shim, and the one remaining non-test caller of a context-free method is `bootstrap.go`'s SQLite schema bootstrap (`tx.Exec`), which runs before any request exists. `dbClient.DBType()` is called only where bind *arguments* differ, not to select a statement.

**Dialect differences the layer papers over:**

| Concern | PostgreSQL | Inbuilt SQLite |
|---|---|---|
| Case-insensitive match | `ILIKE` | `LIKE` (ASCII-only case folding) |
| JSON equality | `col @> $n::jsonb` (containment, keeps GIN usable) | `json_extract(col, '$."key"') = $n` |
| JSON pattern match | `col -> 'a' ->> 'b' ILIKE $n` | `json_extract(col, '$."a"."b"') LIKE $n` |
| Keyset pagination | `(p.created_at, p.profile_id) < ($n::timestamptz, $m::text)` | same row-value comparison, no casts |
| String list column (`consent_categories.destinations`) | `TEXT[]` | JSON array text |

`ValidateJSONKey` restricts interpolated JSON keys to `constants.FilterRegex`, since filter keys are user-supplied and are not bindable as parameters. Before v0.3.63 these keys were `fmt.Sprintf`'d into the statement raw, so this is a SQL-injection hardening fix as well as a dialect helper. **It is also a user-visible behaviour change:** a `co` / `sw` / `eq` filter on a key that fails the regex now returns a `FILTER_PROFILE` error ("Invalid filter key: …") instead of reaching the database. Applies to profile filters (§9) and profile-schema attribute filters (§10).

`HealthCheckPing` (`CDS-SYS-01`) replaces the inline `"SELECT 1;"` previously in `health_check/service/health_check_service.go`, so the liveness probe now exercises the same query path as everything else.

### Connection pooling — reworked at `bea5aa4`

This is the single largest change in the range, and it reverses the previous model. **Before:** every store call opened a pool, used it, and closed it — `sql.Open` per request, so a burst of traffic opened a connection per call and PostgreSQL saw the instance as many short-lived clients. **Now:** the process owns one `*sql.DB` per datasource for its whole life, and every store, worker and probe shares it.

| Concern | How it works now |
|---|---|
| Ownership | `dbprovider.go` holds `postgresHandle` / `sqliteHandle` package vars behind `dbMu`. `GetDBClient()` returns a `DBClient` wrapping the shared handle |
| Client `Close()` | A no-op. A store's `defer dbClient.Close()` no longer closes anything — it is vestigial, and left in place throughout the stores |
| Publication order | A handle is published only after the datasource answers, so a pool no caller can reach is never cached and a failed attempt closes what it opened. PostgreSQL uses `PingContext` under the connect timeout; **SQLite uses a context-free `db.Ping()`** — it is a local file, and the DSN's `busy_timeout` bounds the lock wait — and publishes only after `initializeSQLiteSchema` also succeeds |
| Shutdown | `provider.CloseDB()` swaps the handles out, sets `closed`, and closes them. After that, `GetDBClient` returns `ErrDatabaseClosed` (`"the database is closed: the server has shut down"`) rather than opening a pool that would leak |
| Test override | `SetTestDB(db, dbType)` short-circuits both paths; the suite owns the handle, so `Close` must leave it open |

**PostgreSQL pool settings** — new `datasource.postgres` block, all per instance because one instance holds one pool:

| Key | Default | Meaning |
|---|---|---|
| `max_open_conns` | 100 | connections held, in use and idle together |
| `max_idle_conns` | 25 | unused connections kept open after a burst |
| `conn_max_lifetime_seconds` | 1800 (30m) | retire a connection at this age, so failover / DNS changes take effect |
| `conn_max_idle_time_seconds` | 300 (5m) | close a connection unused this long |
| `connect_timeout_seconds` | 10 | bounds **one connection attempt**, TCP dial through the PostgreSQL startup handshake |

Resolution rules, enforced once in `resolvePostgresPoolSettings` so the start-time check and the pool itself cannot disagree:

- **Zero means the default** — which is also what an omitted key gives.
- **A negative value is refused** and the server does not start.
- **`max_idle_conns` above `max_open_conns` is refused** — it reserves connections the pool can never hold. The comparison is against the limit the pool really uses, so an open limit left at zero is compared as 100, not as "no limit".
- Every problem is collected and reported in **one** error (`invalid datasource settings: …; …`), so an operator fixes a whole configuration in one pass rather than one mistake per restart.

`connect_timeout` reaches lib/pq as a DSN parameter, because a caller's context cannot interrupt every stage of the driver's startup/TLS/auth handshake. It is also the deadline of the verifying ping when the pool opens.

SQLite keeps its single handle and `DefaultSQLiteMaxOpenConns = 4`, now resolved through the same `poolResolution` helper (so a negative `max_open_conns` is refused there too) and opened without a deadline of its own — it is a local file, and the DSN's `busy_timeout` bounds the lock wait.

### Context propagation — new at `bea5aa4`

Every layer now carries a `context.Context` from the HTTP request down to the SQL call. Handlers take `ctx := r.Context()` as their first statement and thread it through service and store calls; roughly **61 store functions and 53 service functions** gained a leading `ctx` parameter, and the interfaces they satisfy changed with them. This is a breaking change for any out-of-tree caller of the `internal` packages.

What it buys: a client that disconnects, or a request that hits its deadline, now ends the database work it started — including the **wait for a free connection**, which is the state a bounded pool reaches under load. Previously a cancelled request left its query running to completion against a connection nobody was waiting for.

Two ends of the chain are worth noting:

- **Queue messages have no caller.** The worker is the context boundary for the work a message causes (see the worker lifecycle below), not the request that enqueued it.
- **`ServerError` deliberately does not implement `Unwrap`.** It was added in `163e252` and removed again in `dc579f6`: a store wraps its cause in a `ServerError` that carries the cause in the `Err` **field** rather than in the error chain. So `errors.Is(err, context.Canceled)` on a store result is **false**; a caller must `errors.As` to the `*ServerError` first and test `serverErr.Err`. Every layer below that field wraps with `%w`, so the chain under it is intact. The integration suite's `requireCauseIs` helper encodes the idiom.

**Transactions.** `BeginTxContext(ctx)` starts the transaction under the caller's context, so `database/sql` rolls it back and returns the connection when that context ends. Stores also roll back explicitly on every statement failure — previously several paths returned without a rollback, leaving the transaction to hold its connection until something else ended it.

### Background workers (`internal/system/workers/`)

| Worker | File | Trigger | Stop |
|---|---|---|---|
| Profile unification | `profile_worker.go` | Consumes `ProfileUnificationQueue` | `StopProfileWorker(ctx)` — drains in-flight unifications |
| Schema sync | `schema_sync_worker.go` | Consumes `SchemaSyncQueue` | `StopSchemaSyncWorker(ctx)` — drains in-flight sync jobs |
| Cookie cleanup | `cookie_cleanup_worker.go` | Timer — `cleanup.cookie.interval` (default 86400s), `batch_size` 500 | `StopCookieCleanupWorker(ctx)` — **cancels immediately**, because the sweep deletes in batches and the next start continues where it stopped |

**`jobLifecycle` (`workers/lifecycle.go`, new).** The two queue workers wrap each handler invocation in a lifecycle that counts active jobs:

- `run(work)` registers the job and passes it the worker's context. A job that arrives after stopping has begun is refused with `ErrWorkerStopping` and logged, not run.
- `stop(ctx, closeQueue)` sets `stopping` under a mutex (so no job can register after stop has decided to wait), waits for the active jobs, and closes the queue **last**.
- If `ctx` expires before the jobs finish, the job contexts are cancelled and `ErrShutdownIncomplete` is returned joined with any close error.

The cookie cleanup worker uses a plainer pattern — a cancel func plus a `done` channel closed when its goroutine returns — but the same contract: `Stop…(ctx)` returns `nil` only once the goroutine is gone, so on the happy path the caller may close the pool.

> **The deadline path is not a wait.** When `ctx` expires first, both `jobLifecycle.stop` (`lifecycle.go`) and `StopCookieCleanupWorker` return `ErrShutdownIncomplete` **immediately**, with work possibly still running. That is deliberate — the shutdown keeps its deadline and the pool closes under the remaining work rather than never closing (`cmd/server/shutdown.go` says so in as many words). So "the pool closes after the workers stop" holds only when the workers stopped *cleanly*.

### Graceful shutdown (`cmd/server/shutdown.go`, new)

`main.go` no longer inlines the shutdown. It resolves the grace period **at start** (so a refused value stops the server before anything runs, and the resolved value is logged), and on SIGINT/SIGTERM calls `shutdown(ctx, logger, server.Shutdown, workerList, provider.CloseDB)`.

The order is fixed and is the point of the change:

1. **HTTP intake stops first** (`server.Shutdown(ctx)`), so no new work reaches the workers.
2. **Every worker stops concurrently** — they do not depend on each other — each under the same `ctx`, results collected into a slice and joined.
3. **The shared database pool closes last**, and closes *even if* a worker overran, since the alternative is a pool that never closes.

`shutdown` returns nil only when the HTTP server and every worker stopped on their own; otherwise it joins the failures, and `errors.Is(err, workers.ErrShutdownIncomplete)` distinguishes "the deadline passed with work still running" from a plain error. `main` logs `"Shutdown completed with unfinished work."` and returns without the final `"Shutdown complete"` line in that case.

New `shutdown.grace_period_seconds` carries the deadline. Default `constants.DefaultShutdownGracePeriod = 25s`; zero means the default; a negative value is refused at start. It replaces the hard-coded 15s that previously bounded only `server.Shutdown`.

> **What it actually bounds.** `ctx` reaches `server.Shutdown` and every `worker.stop`, so HTTP drain, worker drain and the broker disconnect all live inside it. **The pool close does not** — `shutdown` takes `closeDB func() error` with no context, and `provider.CloseDB()` takes none either, so `sql.DB.Close()` runs unbounded after the deadline has already been spent. The shipped `deployment.yaml` comment claims the database pool shares the grace period; the code does not do that. In practice closing an idle pool is instant, but a pod sizing `terminationGracePeriodSeconds` on the assumption that 25s covers everything has no margin for it.

---

## 4. Routing & URL Structure

Routes are registered in `internal/system/services/*.go` against a shared mux, then mounted by `internal/system/managers/servicemanager.go`.

- `constants.ApiBasePath = "/cds/api"`; every service uses `base = ApiBasePath + "/v1"` → **`/cds/api/v1`**
- Tenanted routes are mounted behind a tenant dispatcher: **`/t/{orgHandle}` + `/cds/api/v1/...`**
  `MountTenantDispatcher` strips `/t/{orgHandle}`, injects the handle into request context under `TenantContextKey`, and delegates. Handlers read it via `utils.ExtractOrgHandleFromPath(r)`, falling back to `carbon.super`.
- Health/readiness are **non-tenanted**, served from a separate root mux under `/cds/`.

**Full path shape:** `https://{host}/t/{orgHandle}/cds/api/v1/{resource}`

Confirmed by the `Location` header built in `InitProfile`:
`{serverURL}/t/{orgHandle}/cds/api/v1/profiles/{profileId}`

### Registered endpoints (code-derived)

**Health** — `internal/system/services/health_check_service.go` (non-tenanted)

| Method | Path |
|---|---|
| GET | `/cds/api/v1/health` |
| GET | `/cds/api/v1/ready` |

**Profiles** — `profile_service.go`

| Method | Path | Handler |
|---|---|---|
| GET | `/profiles` | `GetAllProfiles` — cursor pagination, filters, attribute projection |
| POST | `/profiles` | `InitProfile` — creates anonymous profile + sets cookie |
| GET | `/profiles/Me` | `GetCurrentUserProfile` — resolved via `cds_profile` cookie |
| PATCH | `/profiles/Me` | `PatchCurrentUserProfile` |
| POST | `/profiles/sync` | `SyncProfile` — IS event webhook (Basic admin auth) |
| POST | `/profiles/link` | `LinkProfile` — attaches a `user_id` to an anonymous profile (scope `profile:link`) **← new** |
| GET | `/profiles/{profileId}` | `GetProfile` |
| PATCH | `/profiles/{profileId}` | `PatchProfile` |
| DELETE | `/profiles/{profileId}` | `DeleteProfile` |
| GET | `/profiles/{profileId}/consents` | `GetProfileConsents` |
| PUT | `/profiles/{profileId}/consents` | `UpdateProfileConsents` |

> There is a `UpdateProfile` method on the handler that is **not routed** — `PATCH` is the only update path.
> There is **no** `/profiles/merge` and **no** `/profiles/{id}/merge-history` endpoint. Merge lineage is exposed inline on the profile as `merged_from` / `merged_to`.

**Profile Schema** — `profile_schema_service.go`

| Method | Path | Notes |
|---|---|---|
| GET | `/profile-schema` | Whole schema (all scopes) |
| DELETE | `/profile-schema` | Deletes the org's whole schema → `204` |
| POST | `/profile-schema/sync` | IS schema event webhook (Basic admin auth) |
| POST | `/profile-schema/{scope}` | Add attributes to a scope → `201` |
| GET | `/profile-schema/{scope}` | List attributes in a scope |
| DELETE | `/profile-schema/{scope}` | Delete all attributes in a scope → `204` |
| GET | `/profile-schema/{scope}/{attrID}` | Single attribute |
| **PUT** | `/profile-schema/{scope}/{attrID}` | Update single attribute → `200` (handler is named `Patch…` but the verb is `PUT`) |
| DELETE | `/profile-schema/{scope}/{attrID}` | Delete single attribute → `204` |

**Unification Rules** — `unification_rules_service.go`

| Method | Path |
|---|---|
| POST | `/unification-rules` |
| GET | `/unification-rules` |
| GET | `/unification-rules/{ruleId}` |
| PATCH | `/unification-rules/{ruleId}` |
| DELETE | `/unification-rules/{ruleId}` |

**Consent Categories** — `consent_service.go`

| Method | Path |
|---|---|
| GET | `/consent-categories` |
| POST | `/consent-categories` |
| GET | `/consent-categories/{categoryId}` |
| PUT | `/consent-categories/{categoryId}` |
| DELETE | `/consent-categories/{categoryId}` |

**Admin Config** — `admin_config_service.go`

| Method | Path |
|---|---|
| GET | `/config` |
| PATCH | `/config` |

---

## 5. Authentication, Authorization & Scopes

Two distinct auth paths in `internal/system/security/middleware.go`:

| Path | Used by | Mechanism |
|---|---|---|
| `AuthnAndAuthz(r, "<scope-key>")` | All normal API endpoints | OAuth2 Bearer token |
| `AuthnWithAdminCredentials(r)` | `/profiles/sync`, `/profile-schema/sync` | HTTP Basic with configured admin credentials |

### Bearer token handling

`internal/system/authn/auth.go` supports **both** JWT and opaque tokens:
- JWT → parsed locally (`IsJWT`, `ParseJWTClaims`); claims of interest: `azp`, `client_id`, `active`, `org_handle`, `aud`, `exp`
- Opaque → introspected against IS via `introspection_client_id` / `introspection_client_secret`

`internal/system/authz/authorization.go` resolves the required scope strings from `auth_server.required_scopes` in `deployment.yaml`, so the scope mapping is **configuration, not hardcoded**.

### Scope map (from `config/repository/conf/deployment.yaml`)

| Scope key | Token scope |
|---|---|
| `profile:create` | `internal_cds_profile_create` |
| `profile:update` | `internal_cds_profile_update` |
| `profile:view` | `internal_cds_profile_view` |
| `profile:delete` | `internal_cds_profile_delete` |
| `profile:link` | `internal_cds_profile_link` **← new** |
| `unification_rules:view` | `internal_cds_unification_rule_view` |
| `unification_rules:create` | `internal_cds_unification_rule_create` |
| `unification_rules:update` | `internal_cds_unification_rule_update` |
| `unification_rules:delete` | `internal_cds_unification_rule_delete` |
| `profile_schema:view` | `internal_cds_profile_schema_view` |
| `profile_schema:create` | `internal_cds_profile_schema_create` |
| `profile_schema:update` | `internal_cds_profile_schema_update` |
| `profile_schema:delete` | `internal_cds_profile_schema_delete` |
| `admin_config:view` | `internal_cds_admin_config_view` |
| `admin_config:update` | `internal_cds_admin_config_update` |
| `consent_category:view` | `internal_cds_consent_category_view` |
| `consent_category:create` | `internal_cds_consent_category_create` |
| `consent_category:update` | `internal_cds_consent_category_update` |
| `consent_category:delete` | `internal_cds_consent_category_delete` |

### Setup prerequisites (from README)

1. Register an M2M application in the IS console, subscribed to Claim Management APIs, JWT access tokens, audience `iam-cds`
2. Create an admin user and add as console administrator
3. `conf/repository/config/dev.env` with `ENV`, `LOG_LEVEL`, `AUTH_SERVER_CLIENT_SECRET`, `AUTH_SERVER_ADMIN_PASSWORD`
4. Register the consuming applications and subscribe them to Profiles / Profile Schema / Unification Rules / Consent Management API resources
5. Obtain a client-credentials token with only the needed scopes

> **New in v0.3.65 — `scripts/local-setup/script.sh` automates all of the above**, and more besides. The README's five steps are *necessary but not sufficient* for a working IS↔CDS integration: the script also deploys the four extension bundles into IS `dropins` with their `[[event_handler]]` module config, exchanges TLS certificates in both directions, adds the `http://wso2.org/claims/cdsProfile` local claim and its SCIM2 mapping, sets `[console.extensions] cds_host`, relaxes the `/oauth2/introspect` policy, and flips `cds_enabled`. See §18. A setup built by following the README alone will authenticate but will not sync users.
>
> Two OAuth applications, not one: a **system app** for the calls CDS makes into IS, and a **client app** with `aud=iam-cds` for calling the CDS APIs. The Console is wired separately because it issues opaque tokens.
>
> The `internal_cds_*` scopes **cannot be created through the IS management API** — IS reserves the `internal_` prefix and rejects it over REST. The script seeds the CDS API resources and their scopes through `deployment.toml` instead.

---

## 6. Admin Config & Org Enablement

Model — `internal/admin_config/model`:

```go
type AdminConfig struct {
    OrgHandle             string   `json:"org_handle"`
    CDSEnabled            bool     `json:"cds_enabled"`
    InitialSchemaSyncDone bool     `json:"initial_schema_sync_done"`
    SystemApplications    []string `json:"system_applications"`
}
```

API surface is narrower than the internal model:
- `GET /config` → `AdminConfigAPI` = `{ cds_enabled, system_applications? }`
- `PATCH /config` → `AdminConfigUpdateAPI` = `{ cds_enabled *bool, system_applications? }` (pointer so absence ≠ false)

`initial_schema_sync_done` is system-managed and not exposed for write.

Persisted in the `cds_config` table as `(org_handle, config, value)` rows keyed by the constants `cds_enabled`, `initial_schema_sync_done`, `system_applications`.

### The `cds_enabled` gate

**Every** tenanted handler calls `isCDSEnabled(orgHandle)` before doing work. If false the request fails with `400` and error code `CDS_NOT_ENABLED`. This includes the IS sync webhook — sync events for a disabled org are rejected, not silently dropped.

---

## 7. Applications & System Apps

The `applications` table maps `(org_handle, client_id) → app_id`.

### Application identifier mode

`Config.ApplicationIdentifierType` (`application_identifier_type` in YAML):

| Value | Behaviour |
|---|---|
| `client_id` | **Default.** The OAuth `azp`/`client_id` claim *is* the application identifier |
| `app_id` | The claim is resolved through `ResolveAppIdentifierByClientID(orgHandle, clientID)` to an internal app ID |

`Config.UsesAppIDIdentifier()` is the helper that switches this.

### System app detection

1. `getCallerClientIDFromRequest(r)` — pull `azp`, falling back to `client_id`, from the JWT claims or introspection response
2. Resolve to an app identifier per the mode above
3. `isCallerSystemApplication(orgHandle, appId)` → `AdminConfigService.IsSystemApplication`, which checks membership in `system_applications`

> The `SystemApp` constant is an **outbound** `Authorization` scheme used by the Identity Server client (`internal/system/client/identity_client.go`), *not* an inbound request header. System-app status is never asserted by the caller — it is derived from the token.

### Application data filtering

`internal/profile/service/application_data_filter_service.go`, driven by query params:

| Param | Effect |
|---|---|
| `includeApplicationData=true` | **Required** — without it `application_data` is returned empty, for every caller |
| `application_identifier=a,b` or `*` | System apps only — selects which apps' data to include; `*` or omitted returns all |

| Caller | Result |
|---|---|
| System app | All app data, or the subset named in `application_identifier` |
| Regular app | Only its own `application_data[callerAppID]` bucket |

---

## 8. Data Model & Terminology

### Current model

```
UserID ←  0..1 : 1  → ProfileID
Application ← 1 : 1 → ApplicationContext
ProfileID ← 1 : n → ApplicationContext
ApplicationContext ← 1 : n → (name, value)
```

Internally, profile relationships are stored in the `profile_reference` table with a `profile_status` state machine, and projected onto `merged_from` / `merged_to` at the API boundary.

### Key terminology

| Term | Definition |
|---|---|
| **Temporary Profile** (anonymous) | A profile with no `user_id`. Created on first anonymous interaction. Linked to a browser cookie. |
| **Permanent Profile** (identified) | A profile with a `user_id`. Acts as the master record. |
| **Reference Profile** | Code-level term for a master profile — `profile_status = REFERENCE_PROFILE`, `is_reference_profile = true`. Child profiles carry `MERGED_TO`. |
| **ApplicationContext / application_data** | Data for a given application and profile, stored as key-value pairs. |
| **Identity Attributes** | Attributes sourced from IS claims (email, phone, name…). |
| **Traits** | Behavioural/preference data managed by the application. |
| **Profile Unification** | Recognising two profiles as the same person and merging them (CDP "identity resolution"). |
| **Merge Reason** | Either `system:user_id_match` or the `rule_name` of the rule that fired. |
| **Anonymous Profile Tracker** | The `cds_profile` cookie value, echoed back on profile-init/Me responses as `anonymous_profile_tracker`. |

### Profile status constants (`internal/system/constants`)

`REFERENCE_PROFILE`, `WAIT_ON_ADMIN`, `WAIT_ON_USER`, `MERGED_TO`

The `WAIT_ON_*` states and the `ProfileStatus.IsWaitingOnAdmin` / `IsWaitingOnUser` flags exist in the model for the planned approval-workflow feature but are **not yet driven by the unification worker**.

### Merge use-case / mode constants

Merge use cases: `TEMP_TEMP`, `TEMP_PERM`, `PERM_PERM`
Merge modes: `MERGE_BY_ADMIN`, `MERGE_BY_USER`, `MERGE_ON_TRIGGER`
Sync triggers: `SYNC_ON_SCHEDULE`, `SYNC_ON_UPDATE`

These back the `profile_unification_modes` / `profile_unification_triggers` tables and `unification_rules/model.DefaultConfig()` (all three merge types default to `MERGE_ON_TRIGGER`, trigger type `SYNC_ON_UPDATE`). Not yet surfaced through any API.

---

## 9. Profiles

A **profile** is the central entity — a person's identity attributes, behavioural traits, and per-application data, unified across interactions.

### Internal model (`internal/profile/model/profile.go`)

```go
type Profile struct {
    ProfileId          string
    UserId             string
    OrgHandle          string
    CreatedAt          time.Time
    UpdatedAt          time.Time
    Location           string
    IdentityAttributes map[string]interface{}
    Traits             map[string]interface{}
    ApplicationData    []ApplicationData
    ProfileStatus      *ProfileStatus
}

type ProfileStatus struct {
    ReferenceProfileId string
    IsReferenceProfile bool
    IsWaitingOnAdmin   bool
    IsWaitingOnUser    bool
    DeleteProfile      bool
    ListProfile        bool
    ReferenceReason    string
    References         []Reference
}

type Reference struct {
    ProfileId string
    Reason    string
}
```

### API projection (`internal/profile/model/profile_api.go`)

`ProfileResponse`:

| Field | Notes |
|---|---|
| `profile_id` | |
| `user_id` | omitted when empty (anonymous) |
| `anonymous_profile_tracker` | cookie ID; set only by `InitProfile` and `GetCurrentUserProfile` |
| `meta` | `{ created_at, updated_at, location }` |
| `identity_attributes` | |
| `traits` | |
| `application_data` | `map[appId]map[key]value` — **flattened**, unlike the internal array form |
| `merged_to` | `Reference` — set on child profiles |
| `merged_from` | `[]Reference` — set on master profiles |

`ProfileListResponse` is the same minus `anonymous_profile_tracker` and `merged_to`.

`ProfileRequest` (write): `{ user_id, identity_attributes?, traits?, application_data }`. `InitProfile` decodes with `DisallowUnknownFields()` — unknown keys are a `400`.

`ProfileListAPIResponse`: `{ pagination, profiles }`.

`ProfileLinkRequest` / `ProfileLinkResponse` (new) — both are the same flat pair, and both fields are required on the request:

```go
type ProfileLinkRequest struct {
    ProfileId string `json:"profile_id" bson:"profile_id"`
    UserId    string `json:"user_id"    bson:"user_id"`
}

type ProfileLinkResponse struct {
    ProfileId string `json:"profile_id" bson:"profile_id"`
    UserId    string `json:"user_id"    bson:"user_id"`
}
```

> Note the `bson` tags on both. They are inert — CDS has no Mongo datasource (§18 lists only SQLite and PostgreSQL) — but they match the convention used by the surrounding DTOs in the same file.

### Temporary vs permanent

**Temporary (anonymous)** — no `user_id`; linked to the `cds_profile` cookie via `profile_cookies`; accumulates behavioural data.
**Permanent (identified)** — has `user_id`; acts as master; may absorb temporary profiles over time.

### Lifecycle

```
Anonymous visit
      │
      ▼
POST /profiles → temporary profile created, cds_profile cookie issued
      │
      │  (login — AUTHENTICATION_SUCCESS, or signup — POST_ADD_USER)
      ▼
userId attached to the temp profile
      │
      ▼
Unification worker picks it up
      │
      ├── No existing permanent profile → temp profile promoted to permanent
      │
      └── Existing permanent profile found → temp merged in as child
                │
                ▼
            master updated with merged data; child gets merged_to
                │
                │  (SESSION_TERMINATE)
                ▼
            cookie deactivated (is_active = false)
            cookie cleanup worker batch-deletes after 24h
```

> As of `b38e9bd` the `userId attached to the temp profile` step has a **second entry point**: a caller can drive it directly with `POST /profiles/link` instead of waiting for the IS event. Everything downstream of that step — the unification worker, promotion, merge, cookie teardown — is identical either way. See below.

### Explicit linking — `POST /profiles/link` (new)

Until now the only way a `user_id` reached an anonymous profile was implicitly, through the IS event webhook (`POST /profiles/sync` on `AUTHENTICATION_SUCCESS` / `POST_ADD_USER`, §14). `POST /profiles/link` adds a **direct, caller-driven** path for the same transition — useful when the caller already knows both IDs and does not want to wait for, or depend on, an IS event.

Handler: `ProfileHandler.LinkProfile` in `internal/profile/handler/profile_handler.go`.

Request `{ "profile_id": "...", "user_id": "..." }` → response `{ "profile_id": "...", "user_id": "..." }` on `200`.

**Order of checks in the handler** — this ordering is worth knowing, because it decides which error a bad call gets back:

1. `security.AuthnAndAuthz(r, "profile:link")` — Bearer token, scope `profile:link`. **Authz runs before the org gate**, unlike nothing else in particular; it is simply first.
2. `isCDSEnabled(orgHandle)` — `400 CDS_NOT_ENABLED` if the org has not enabled CDS (§6).
3. JSON decode — `400` with a decode description. Note this handler uses a **plain `json.NewDecoder(r.Body).Decode`, *not* `DisallowUnknownFields()`** as `InitProfile` does, so unknown keys in a link request are silently ignored rather than rejected. That is an inconsistency with the rest of the profile write surface, not a documented choice.
4. `profile_id` empty after `TrimSpace` → `400`, description `profile_id is required to link a profile`.
5. `user_id` empty after `TrimSpace` → `400`, description `user_id is required to link a profile`.
6. `GetProfile(profileId)` → `404` if the profile does not exist.
7. Already-linked handling — see below.
8. Otherwise `UpdateProfile(profileId, orgHandle, profileRequest)` writes the `user_id`.

> Steps 4 and 5 both reuse the **`UPDATE_PROFILE` error code**, not a link-specific one — only the free-text `Description` distinguishes "missing profile_id" from "missing user_id". Clients cannot branch on the code.

**Already-linked semantics** (`existingProfile.UserId != ""`):

| Case | Result |
|---|---|
| Linked to a **different** `user_id` | `409 Conflict`, new error `PROFILE_ALREADY_LINKED` |
| Linked to the **same** `user_id` | `200` with the same response body — **idempotent, no write performed** |

This is the write-once `user_id` rule from §9 enforced at the API edge for the first time. The re-link rejection came in as its own commit (`fa910df`, "Reject linking a profile that belongs to a different user"), separately from the endpoint itself (`77c2dc3`).

**New error code** in `internal/system/errors/error_codes.go`:

```go
PROFILE_ALREADY_LINKED = ErrorMessage{
    Code:        errorPrefix + "11017",
    Message:     "Profile already linked.",
    Description: "The profile is already linked to a different user",
}
```

**The update is a full rewrite, not a patch.** `LinkProfile` builds a `model.ProfileRequest` carrying the existing profile's `IdentityAttributes`, `Traits` and `ApplicationData` back in alongside the new `UserId`, then calls `UpdateProfile`. Application data is passed through `profileService.WideAppDataMap(...)`, which only **widens the static type** — `map[string]map[string]interface{}` (what `ProfileResponse.ApplicationData` is) to `map[string]interface{}` (what `ProfileRequest.ApplicationData` is). The body is a plain key-by-key copy; no reshaping happens, and the internal `[]ApplicationData` array form on the `Profile` model is not involved at any point. So the endpoint reads the whole profile and writes the whole profile back — there is a read-modify-write window here, and no optimistic concurrency check guarding it.

**After the link, unification proceeds as normal.** Setting `user_id` is what the unification worker watches for, so linking a profile whose `user_id` already has a permanent profile elsewhere results in a merge on the next pass — covered by the integration test `Link_UserId_With_Existing_Profile_Unifies`.

**Route shape changed mid-series.** The endpoint landed under a path-variable route and was moved to the flat collection route `POST /profiles/link` by `d2a78ed` before any release. Only the flat form exists at HEAD. Anything written against the earlier shape needs updating.

**Coverage** — `test/integration/profile_link_test.go` (327 lines): `Link_Anonymous_Profile_Sets_UserId`, `Link_With_Missing_Identifier_Is_Rejected`, `Link_Unknown_Profile_Is_Not_Found`, `Relink_To_Different_User_Is_Rejected`, `Relink_To_Same_User_Is_Accepted`, `Link_UserId_With_Existing_Profile_Unifies`. The suite also asserts that `profile:link` is present in `config/repository/conf/deployment.yaml` and maps to exactly `["internal_cds_profile_link"]`, so dropping the scope from config fails the build rather than silently opening the endpoint.

### Cookies

`profile_cookies` maps `cookie_id` (the `cds_profile` value) → `profile_id`, with `is_active`.
- Created when an anonymous profile is issued (`CreateProfileCookie`)
- Reused if a valid cookie is presented to `POST /profiles` (`handleExistingCookie` short-circuits creation)
- Deactivated on `SESSION_TERMINATE`
- Batch-deleted by the cleanup worker after the configured interval (default 24h, batch 500)

Cookie domain comes from `auth_server.cookieDomain`.

### Listing, pagination, filtering, projection

`GET /profiles` uses **cursor pagination**, not offset:
- `limit` via `pagination.ParsePageSize` (default `DefaultLimit = 50`)
- `cursor` is a base64-encoded `ProfileCursor { created_at, profile_id, direction }`; direction is `next` (older) or `prev` (newer)
- Response `pagination` block: `{ count, page_size, next_cursor, previous_cursor }`
- `filter` params, repeatable and also splittable on ` and `. Filter keys validated against `FilterRegex = ^[a-zA-Z0-9._-]+$`; schema filter fields limited to `attribute_name`, `application_identifier`
- `attributes` query param selects which fields to project (`parseRequestedAttributes` / `buildProfileListResponse`)
- **No consent filtering** on list — it is an administrative operation

### Deletion

`DELETE /profiles/{id}` soft-deletes: `delete_profile = true`, `list_profile = false`.

---

## 10. Profile Schema

Defines the shape of data storable on a profile per organisation — names, types, mutability, and merge resolution.

### Scopes

| Scope | Description |
|---|---|
| `identity_attributes` | Sourced from IS claim dialects |
| `traits` | Behavioural / preference data |
| `application_data` | Per-application, namespaced by `application_identifier` |

Attribute names carry the scope prefix: `identity_attributes.email`, `traits.preferred_language`, `application_data.score`.

### Attribute model (`ProfileSchemaAttribute`)

| Field | Required | Description |
|---|---|---|
| `attribute_id` | auto | System-generated UUID |
| `attribute_name` | yes | Dot-notation path including scope prefix |
| `scope` | derived | |
| `display_name` | no | Human-readable label; max 50 chars, sanitised by `DisplayNameRegex`, camelCase auto-split |
| `value_type` | yes | See below |
| `merge_strategy` | yes | See below |
| `mutability` | yes | See below |
| `multi_valued` | no | Array of the declared type |
| `canonical_values` | no | `[{ value, label }]` |
| `sub_attributes` | no | `[{ attribute_id, attribute_name }]` when `value_type` is `complex` |
| `application_identifier` | no | Scopes the attribute to an application |
| `scim_dialect` | internal | Populated by schema sync; intended to be suppressed in responses |

### Value types

`string`, `integer`, `decimal`, `boolean`, `date`, `date_time`, `epoch`, `complex` — all present in `AllowedValueTypes`.

### Mutability

| Value | Meaning |
|---|---|
| `readWrite` | Freely read and updated |
| `readOnly` | System-managed — not updatable via API |
| `writeOnly` | Writable, not read back |
| `immutable` | Set at creation only |
| `writeOnce` | May start empty; once set, frozen |
| `computed` | **Declared as a constant but absent from `AllowedMutabilityValues`** — not accepted on writes |

### Merge strategies

| Strategy | Accepted on write? | Implemented in `MergeAttributeValue`? |
|---|---|---|
| `overwrite` | Yes | Yes — incoming replaces existing, unless incoming is empty |
| `combine` | Yes | Yes |
| `ignore` | **No** — rejected by `AllowedMergeStrategies` | Yes — keeps existing if non-nil |
| `latest` | No | No — falls through to default (returns incoming) |
| `oldest` | No | No |

`combine` semantics depend on `multi_valued`:
- `multi_valued: false` and both sides are maps → **deep merge** of the maps
- `multi_valued: true` → union by type: `string`/`text`, `integer`, `decimal`, `boolean`, `date_time` de-duplicate; `complex` **appends** without de-duplication

> `AllowedMergeStrategies` carries a `// todo: Remove later` against `overwrite`, so the accepted set is in flux.

### Core schema (built-in, not modifiable)

| Attribute | Type | Mutability |
|---|---|---|
| `profile_id` | `string` | `immutable` |
| `user_id` | `string` | `writeOnce` |
| `meta.created_at` | `date_time` | `readOnly` |
| `meta.updated_at` | `date_time` | `readOnly` |
| `meta.location` | `string` | `readOnly` |

### Write validation

`ValidateProfileAgainstSchema` enforces, per attribute: existence in schema, type correctness (incl. multi-valued and complex sub-attributes), canonical value membership, and mutability. Flattened sub-attribute keys are rejected outright (`rejectIfFlattenedSubAttribute`). Numeric equality is compared type-insensitively for mutability checks (`valuesEqualForMutability`).

---

## 11. Consent Management

Consent controls which profile attributes a caller may **read**. It does not gate writes.

### Key principles

| Principle | Detail |
|---|---|
| Category-level tracking | A profile consents to a whole category or not — no per-attribute granularity |
| Mandatory Identity Data | Every org has a built-in mandatory category, always enforced |
| System app bypass | System apps skip consent filtering entirely |
| Read-only gating | Applies to reads, not writes |
| No filtering on list | `GET /profiles` returns full profiles |

### Consent categories

An org-level definition declaring a name, a server-generated UUID `category_identifier`, a `purpose` (`profiling` | `personalization` | `destination`, per `AllowedConsentPurposes`), optional `destinations`, the covered attributes, and `is_mandatory`.

**Create/update request** (`ConsentCategoryRequest`):

```json
{
  "category_name": "Product Engagement",
  "purpose": "profiling",
  "destinations": ["segment"],
  "attributes": [
    { "attribute_name": "traits.engagement_score" },
    { "attribute_name": "traits.product_interests" },
    { "attribute_name": "application_data.events.event_name",
      "application_identifier": "i0QfDlYH6BIA8QygvmLRHikFrRIa" }
  ]
}
```

**Response** (`ConsentCategoryResponse`) adds `category_identifier`, `is_mandatory`, and a derived `scope` on each attribute. `org_handle` is deliberately omitted. `attribute_id` is internal and stripped by `ToResponse()`.

Validation:
- `category_identifier` always server-generated; caller values ignored
- Every `attribute_name` validated against the org's `profile_schema`; unknown → `400`
- `scope` derived from the name prefix, never supplied
- `application_data.*` attributes require `application_identifier`, and it must match the schema

**Error responses:**

| Status | Condition |
|---|---|
| 400 | Missing `category_name` or `purpose` |
| 400 | Invalid `purpose` |
| 400 | `attribute_name` not in the org's `profile_schema` |
| 400 | `applicationData` attribute missing or mismatched `application_identifier` |
| 403 | Attempt to modify or delete a mandatory category |
| 409 | Duplicate `category_name` |

### Attribute scopes (derived)

| Scope constant | Prefix | Example |
|---|---|---|
| `identityAttributes` | `identity_attributes.` | `identity_attributes.email` |
| `traits` | `traits.` | `traits.age` |
| `applicationData` | `application_data.` | `application_data.events.event_name` |

### applicationData matching quirk

`application_data` is a two-level map: `app_id → data_group_key → value`. `application_identifier` names the **outer** app ID, while `attribute_name` encodes the **inner** data-group key.

At filter time, matching is done on the **inner data-group key**, not the outer app ID. Consequence: if two apps both write under `"events"`, a single consent entry gates both.

### Complex attributes

Listing the parent `attribute_name` is sufficient — the whole object is returned. Sub-attributes need not be enumerated. Same for `applicationData`, since array entries cannot be partially filtered.

### Mandatory consent — Identity Data

- Auto-created when CDS is enabled; name `Identity Data`, purpose `profiling`, `is_mandatory: true`
- Covers all `identityAttributes`-scope fields
- Always applied — no `profile_consents` row needed, and it cannot be revoked
- Cannot be deleted or updated → `403`
- Stores **no rows** in `consent_category_attributes`; attributes are resolved live from `profile_schema WHERE scope = 'identity_attributes'` at filter time, so schema sync changes take effect automatically

### Per-profile consent records

```
profile_consents
  ├── profile_id
  ├── category_id     → consent_categories.category_identifier
  ├── consent_status  → true (consented) | false (revoked)
  └── consented_at
```
`UNIQUE (profile_id, category_id)`.

```
GET /t/{orgHandle}/cds/api/v1/profiles/{profileId}/consents
PUT /t/{orgHandle}/cds/api/v1/profiles/{profileId}/consents
```

`PUT` replaces the full non-mandatory consent set. Including a mandatory category → `403`.

Records are `ConsentRecord { category_identifier, is_consented, consented_at }`.

### Consent-scoped profile fetch

```
GET /t/{orgHandle}/cds/api/v1/profiles/{profileId}
GET .../profiles/{profileId}?consentCategoryId=<uuid>
GET .../profiles/{profileId}?consentCategoryId=<uuid1>&consentCategoryId=<uuid2>
```

`consentCategoryId` is parsed by `parseCommaSeparatedOrRepeated` — repeated params **and** comma-separated values both work.

| Caller | `consentCategoryId` | Response |
|---|---|---|
| System app | either | Full profile — filtering bypassed |
| Regular app | no | Mandatory identity data only |
| Regular app | yes, consented | Identity data + that category's attributes |
| Regular app | yes, not consented | Identity data only |
| Regular app | multiple | Identity data + **union** across consented categories |

Order of operations in `GetProfile`: fetch → `FilterApplicationData` (app scoping) → `FilterProfileByConsent` (only if not a system app).

### Data model

```
consent_categories
  ├── category_identifier  (UUID, server-generated, UNIQUE)
  ├── purpose              (profiling | personalization | destination)
  ├── is_mandatory
  └── consent_category_attributes
        ├── attribute_name
        ├── attribute_id   (FK → profile_schema.attribute_id ON DELETE CASCADE)
        ├── scope          (derived at write time)
        └── application_identifier

profile_consents
  ├── profile_id     (FK → profiles ON DELETE CASCADE)
  ├── category_id    (FK → consent_categories ON DELETE CASCADE)
  ├── consent_status
  └── consented_at
```

**Cascade behaviour:**

| Trigger | Effect |
|---|---|
| Delete `consent_categories` row | Removes its `consent_category_attributes` + `profile_consents` rows |
| Delete `profile_schema` attribute | Removes the `consent_category_attributes` row (category survives) |
| Delete `profiles` row | Removes all `profile_consents` for that profile |

---

## 12. Unification Rules

Rules tell CDS when two profiles represent the same person.

### Rule structure

| Field | Description |
|---|---|
| `rule_id` | System-generated UUID |
| `org_handle` | Owning organisation (internal; not in the API response) |
| `rule_name` | Human-readable; recorded as the merge `reason` |
| `property_name` | Attribute to match on, e.g. `identity_attributes.email` |
| `property_id` | FK → `profile_schema.attribute_id`, `ON DELETE CASCADE` |
| `priority` | Lower = evaluated first |
| `is_active` | Only active rules are evaluated |
| `created_at` / `updated_at` | Timestamps |

**API shapes:** `UnificationRuleAPIRequest` = `{ rule_name, property_name, priority, is_active }`. `UnificationRuleAPIResponse` adds `rule_id`. `UnificationRuleUpdateRequest` (PATCH) is `{ rule_name?, priority?, is_active? }` with pointers — **`property_name` cannot be changed after creation**.

**Uniqueness:** both `rule_name` and `priority` must be unique per org — `UNIFICATION_RULE_ALREADY_EXISTS` and `UNIFICATION_RULE_PRIORITY_EXISTS` are enforced on create and update.

### How rules are evaluated

1. Fetch all rules for the org, filter to active, sort by `priority` ascending
2. Fetch all existing master (reference) profiles for the org, excluding the incoming profile's own parent
3. For each rule, check whether any master profile shares the same value for `property_name`
4. First match triggers a merge — **only one rule fires per unification run**

Rules are evaluated **after** the system-level `userId` match.

### System merge reason

| Reason | Trigger |
|---|---|
| `system:user_id_match` | Two profiles share the same `userId` — automatic, cannot be disabled |

### Priority guidance

*Intent, not enforced by code.* Assign low numbers to high-confidence identifiers, higher numbers to weaker signals; leave gaps (10, 20, 30) so rules can be inserted without reordering. A typical retail ordering: userId (system, always first) → mobile → email → loyalty ID.

Setting `is_active: false` excludes a rule without deleting it. Existing merges are **not reversed**.

---

## 13. How Profile Unification Works

Runs asynchronously via `internal/system/workers/profile_worker.go`, fed by `ProfileUnificationQueue`.

### Overview

```
Profile created / updated
         │
         ▼
  EnqueueProfileForProcessing → ProfileUnificationQueue
         │
         ▼
  Worker picks up profile → unifyProfiles()
         │
         ├─ Step 1: userId match? ──yes──► mergeMatchedProfiles(system:user_id_match)
         │
         └─ Step 2: rule-based match?
                   │
                   └─ for each active rule (priority asc):
                         does any master profile share the same
                         value for rule.property_name?
                              │
                              yes ──► mergeMatchedProfiles(rule.rule_name) ──► stop
                              no  ──► try next rule
```

### Step 1 — System userId match

If the incoming profile has a non-empty `userId`, every existing master profile is checked (skipping the profile's own parent). First `userId` equality → immediate merge with reason `system:user_id_match`. Fires before rules; cannot be disabled.

### Step 2 — Rule-based matching

Active rules sorted ascending by `priority`. `doesProfileMatch` serialises the profile to JSON and walks `property_name` as a dot path (`getNestedJSONField`), collecting values; `checkForMatch` compares the two value sets. This means **multi-valued attributes match if any value overlaps**.

> Loop quirk: inside the rule loop, if the candidate master profile is the incoming profile's own parent, the code `return`s out of the whole function rather than `continue`-ing to the next candidate. Rule evaluation can therefore terminate early depending on candidate ordering.

### Merge cases (`mergeMatchedProfiles`)

Data is merged **first** (`MergeProfiles`), then master/child is resolved:

| Scenario | Result |
|---|---|
| Existing permanent + new temporary | Existing stays master; new becomes child |
| Existing temporary + new permanent | New becomes master; existing **and its children** re-parented to new |
| Both temporary | Existing becomes master, new becomes child |
| Both permanent, same userId | Existing becomes master, new becomes child |
| Both permanent, **different** userIds | **Not merged** — logged and returned |

The perm/temp split is handled by `mergePermanentAndTemporary`; the same-kind cases by `mergeSameKindProfiles`. `persistMergedProfileData` writes the result.

### Data merging

`MergeProfiles` walks the org's schema attributes and applies each attribute's `merge_strategy` via `MergeAttributeValue` across `identity_attributes`, `traits`, and `application_data` (the latter via `mergeAppData` + `buildApplicationSchemaRuleMap`, so app-scoped attributes resolve to their own schema entries). Nested paths are walked by `mergeByPath`.

See [§10](#10-profile-schema) for exact `combine` / `overwrite` semantics.

### Result on the profile

```jsonc
// Master
{
  "profile_id": "master-123",
  "user_id": "u-123",
  "merged_from": [
    { "profile_id": "anon-456", "reason": "system:user_id_match" },
    { "profile_id": "old-789",  "reason": "email_match" }
  ]
}

// Child
{
  "profile_id": "anon-456",
  "merged_to": { "profile_id": "master-123", "reason": "system:user_id_match" }
}
```

### Design decisions carried forward

*From requirements material.*

- **Merged profile ID** — Option A (chosen): reuse one existing profile ID as the master. Option B (create a fresh ID) was considered and not taken. Either way, old IDs remain resolvable as child references.
- **Transparency to applications** — apps need not know a merge occurred; they can keep using the profile ID they hold. Event-based notification remains a roadmap item.

---

## 14. IS Sync — Identity Server Event Integration

CDS receives lifecycle events from WSO2 Identity Server via a webhook-style endpoint.

### Endpoint

```
POST /t/{orgHandle}/cds/api/v1/profiles/sync
Authorization: Basic <admin credentials>
```

### Request body (`ProfileSync`)

```json
{
  "event": "POST_ADD_USER",
  "userId": "…",
  "profileCookie": "…",
  "profileId": "…",
  "orgHandle": "carbon.super",
  "claims": { "…": "…" }
}
```

> `orgHandle` is read from the **body**, not the path, and is **required** — an empty value is a `400`. There is a `todo` in the source questioning this. The org must also be CDS-enabled or the event is rejected with `CDS_NOT_ENABLED`.

### Events

#### `POST_ADD_USER`
New user registered in IS.

**With cookie (anonymous → registered):** resolve the anonymous profile from the active cookie → attach `userId`, merge claims into `identity_attributes` → save; the unification worker promotes it to permanent.
**Without cookie:** if no profile exists for the `userId`, create a new permanent profile with the claims as `identity_attributes`.

#### `AUTHENTICATION_SUCCESS`
Fired on every successful login.

| Scenario | Action |
|---|---|
| Cookie + anonymous profile + a different existing permanent profile | Attach `userId` to the anonymous profile; enqueue; worker merges via `system:user_id_match` |
| Cookie + anonymous profile + no permanent profile | Promote the anonymous profile by attaching `userId` |
| Cookie already resolves to the user's own profile | No-op |
| No cookie | No-op (logged) |

#### `POST_SET_USER_CLAIM_VALUE_WITH_ID` / `POST_SET_USER_CLAIM_VALUES_WITH_ID`
Look up the permanent profile by `userId` → translate IS claim URIs to CDS attribute paths (`extractClaimKeyFromLocalURI`, `setNestedMapValue`) → merge into `identity_attributes` → save.

#### `SESSION_TERMINATE`
Look up the permanent profile by `userId` → resolve the cookie record → verify it belongs to that profile or one of its merged children → set `is_active = false`. Cleanup worker removes deactivated records in batches after the configured interval.

#### `POST_DELETE_USER_WITH_ID`
Look up by `userId` → soft-delete (`delete_profile = true`, `list_profile = false`).

---

## 15. Schema Sync

Keeps the CDS schema aligned with IS claim changes. Asynchronous via `SchemaSyncQueue` and `schema_sync_worker.go`.

### Endpoint

```
POST /t/{orgHandle}/cds/api/v1/profile-schema/sync
Authorization: Basic <admin credentials>
```

### Payload (`ProfileSchemaSync`)

```json
{
  "event": "POST_ADD_EXTERNAL_CLAIM",
  "orgHandle": "carbon.super"
}
```

> The Go struct field is `OrgId` but its **JSON tag is `orgHandle`** — the wire key is `orgHandle`.

### Events

| Event constant | IS trigger |
|---|---|
| `POST_ADD_EXTERNAL_CLAIM` | New SCIM claim added |
| `POST_UPDATE_EXTERNAL_CLAIM` | SCIM claim updated |
| `POST_DELETE_EXTERNAL_CLAIM` | SCIM claim deleted |
| `POST_UPDATE_LOCAL_CLAIM` | Local claim updated |
| `POST_DELETE_LOCAL_CLAIM` | Local claim deleted |

All events trigger the same action: a full re-sync of the org's schema from IS.

### How it works

1. IS fires the event to the CDS sync endpoint
2. CDS enqueues a `ProfileSchemaSync` job onto `SchemaSyncQueue`
3. The worker calls `SyncProfileSchema(orgId)`
4. Claim dialects are fetched from IS via the Identity Client (`/api/server/v1/claim-dialects`) and reconciled with the stored schema
5. New attributes added, changed ones updated, removed ones deleted; `scim_dialect` is recorded on each attribute

Best-effort — failures are logged but don't affect the HTTP response to IS.

### Scheduled sync

Beyond event-driven sync, `sync.schema` in `deployment.yaml` enables a periodic re-sync (`enabled: true`, `interval: 3600` seconds by default).

### Initial sync

On first enablement (`cds_enabled`), an initial sync bootstraps the schema from IS; `initial_schema_sync_done` is set on success.

Schema sync affects **definitions only** — it does not rewrite existing profile data, though updated merge strategies and mutability apply to all subsequent writes and unification runs.

---

## 16. Message Queue & Extending Providers

Two interfaces in `internal/system/queue/queue.go`:

| Interface | Purpose |
|---|---|
| `ProfileUnificationQueue` | `Enqueue(profileModel.Profile) error`, `Start(func(profileModel.Profile)) error`, **`Close(ctx context.Context) error`** |
| `SchemaSyncQueue` | `Enqueue(schemaModel.ProfileSchemaSync) error`, `Start(func(schemaModel.ProfileSchemaSync)) error`, **`Close(ctx context.Context) error`** |

`Start` must launch its consumer loop in a goroutine and return immediately. `Close` must be idempotent.

> **Breaking change at `bea5aa4`: `Close()` became `Close(ctx context.Context)`.** A provider must now return **inside** `ctx`, ending its connection by force if a graceful close has not finished by then, so that shutdown keeps its deadline and leaves no goroutine behind. Any out-of-tree provider must be updated. `docs/guides/extending-queue-providers.md` still documents the old signature (§18, *Docs gaps*).

How the two built-ins satisfy it:

- **in-memory** holds no connection, so it ignores the context entirely (`Close(context.Context) error`) and just closes its channel.
- **ActiveMQ** now runs `conn.Disconnect()` in a goroutine and selects on `ctx.Done()`, because go-stomp waits longer for a disconnect receipt than the whole shutdown grace period. On expiry it closes the underlying `net.Conn` directly and returns an error naming the forced close. Supporting changes in `managedConn`: a `netConn` field holding the live socket; a `closed` flag checked under `mu` inside `dial()`, so a reconnect racing a shutdown cannot install a connection after the close; `closeOnce`/`closeErr` so repeated calls return the same answer; and `reconnectWithBackoff` now selects on the `done` channel instead of `time.Sleep`, so a backoff wait aborts on shutdown rather than running to term.

### Built-in providers

| Constant | Value | Notes |
|---|---|---|
| `queue.TypeMemory` | `"memory"` | Default; `internal/system/queue/inmemory`; `DefaultQueueSize = 1000`; single-instance only |
| `queue.TypeActiveMQ` | `"activemq"` | `internal/system/queue/activemq`, STOMP; durable, for multi-instance |

ActiveMQ locally:

```bash
docker run -d -p 61613:61613 --name activemq \
  -e ACTIVEMQ_ADMIN_LOGIN=admin -e ACTIVEMQ_ADMIN_PASSWORD=admin \
  rmohr/activemq
```

```yaml
message_queue:
  type: "activemq"
  broker:
    addr: "localhost:61613"
    username: "admin"
    password: "${BROKER_PASSWORD}"
    profile_queue_name: "/queue/cds-profile-unification"
    schema_sync_queue_name: "/queue/cds-schema-sync"
```

### Adding a provider

1. Create `internal/system/queue/myprovider/myprovider.go`
2. Implement both interfaces (3 methods each — note `Close` takes a `context.Context`)
3. Register via `init()` using `queue.RegisterProfileQueueProvider` / `queue.RegisterSchemaSyncQueueProvider` (factory in `queue/factory.go`)
4. Blank-import in `cmd/server/main.go` — as ActiveMQ already is
5. Set `message_queue.type: "myprovider"` in `deployment.yaml`
6. Optionally mirror `test/activemq_integration/`

```go
func init() {
    queue.RegisterProfileQueueProvider("myprovider",
        func(cfg config.ExternalBrokerConfig, tlsCfg config.TLSConfig) (queue.ProfileUnificationQueue, error) {
            return newProfileQueue(...)
        },
    )
    queue.RegisterSchemaSyncQueueProvider("myprovider",
        func(cfg config.ExternalBrokerConfig, tlsCfg config.TLSConfig) (queue.SchemaSyncQueue, error) {
            return newSchemaSyncQueue(...)
        },
    )
}
```

`ExternalBrokerConfig` carries `addr`, `username`, `password`, `profile_queue_name`, `schema_sync_queue_name`. Anything provider-specific goes in env vars or a new sub-section.

---

## 17. Database Schema

Two scripts ship, defining the same 13 tables:

- `dbscripts/postgres.sql` — applied by the operator (`psql < ...`). Uses `CREATE TABLE` (no `IF NOT EXISTS`), `TIMESTAMPTZ ... DEFAULT now()`, `TEXT[]`, `jsonb` defaults, and creates the `pg_trgm` extension plus GIN indexes.
- `dbscripts/sqlite.sql` — embedded in the binary (`dbscripts/embed.go`, `//go:embed`) and applied automatically on first start. Idempotent (`CREATE TABLE IF NOT EXISTS`), so re-applying is a no-op.

The table/column list below is common to both.

| Table | Purpose |
|---|---|
| `profiles` | `profile_id` PK, `user_id`, `org_handle`, `created_at`, `updated_at`, `location`, `origin_country`, `list_profile`, `delete_profile`, `traits` JSONB, `identity_attributes` JSONB |
| `profile_reference` | `profile_id` PK, `org_handle`, `profile_status`, `reference_profile_id`, `reference_profile_org_handle`, `reference_reason` — the merge lineage |
| `application_data` | `app_data_id` PK, `profile_id` FK CASCADE, `app_id`, `application_data` JSONB, `UNIQUE(profile_id, app_id)` |
| `applications` | `app_id` PK, `org_handle`, `client_id`, timestamps, `UNIQUE(org_handle, client_id)` |
| `profile_schema` | `attribute_id` PK, `scope`, `org_handle`, `attribute_name`, `display_name`, `value_type`, `merge_strategy`, `application_identifier`, `mutability`, `multi_valued`, `canonical_values` JSONB, `sub_attributes` JSONB, `scim_dialect` |
| `unification_rules` | `rule_id` PK, `org_handle`, `rule_name`, `property_name`, `property_id` FK → `profile_schema` CASCADE, `priority`, `is_active`, timestamps |
| `consent_categories` | `id` PK, `org_handle`, `category_name`, `category_identifier` UNIQUE, `purpose`, `destinations` TEXT[], `is_mandatory` |
| `consent_category_attributes` | `category_id` FK CASCADE, `scope`, `attribute_name`, `attribute_id` FK → `profile_schema` CASCADE, `application_identifier`, `UNIQUE(category_id, scope, attribute_name, application_identifier)` |
| `profile_consents` | `profile_id` FK CASCADE, `category_id` FK CASCADE, `consent_status`, `consented_at`, `UNIQUE(profile_id, category_id)` |
| `profile_cookies` | `cookie_id` PK, `profile_id` FK CASCADE, `is_active` |
| `cds_config` | `(org_handle, config)` PK, `value` — backs admin config |
| `profile_unification_modes` | `org_handle`, `merge_type`, `rule`, `UNIQUE(org_handle, merge_type, rule)` — for the planned approval workflow |
| `profile_unification_triggers` | `org_handle` UNIQUE, `trigger_type`, `last_trigger`, `duration` — for scheduled unification |

**Notable indexing (PostgreSQL):** composite `(org_handle, created_at, profile_id)` supporting cursor pagination; `(org_handle, user_id)`; `pg_trgm` GIN on `user_id` for contains-matching; GIN on `profiles.traits` and `profiles.identity_attributes`; GIN on `application_data->'app_specific_data'`; `(org_handle, is_active, priority)` on unification rules.

> `CREATE EXTENSION IF NOT EXISTS pg_trgm` means the DB user needs extension-creation rights on first init.

**Indexing on the inbuilt database:** the B-tree indexes carry over (`idx_profiles_org_created_profile`, `idx_profiles_org_user`, the three `profile_reference` indexes, both `application_data` indexes, both `profile_schema` indexes, both `unification_rules` indexes). The four GIN/trigram indexes have **no equivalent** — SQLite has neither `pg_trgm` nor GIN.

### Inbuilt-database type & behaviour differences

| Aspect | PostgreSQL | Inbuilt SQLite |
|---|---|---|
| Timestamps | `TIMESTAMPTZ DEFAULT now()` | `TIMESTAMP DEFAULT (strftime('%Y-%m-%d %H:%M:%f','now') \|\| '+00:00')` — sortable UTC text, which keyset pagination orders on |
| JSON columns | native `jsonb` | `JSONB` declared type over TEXT storage; read via `json_extract` |
| Booleans | native | stored as `0`/`1`, coerced back in `client/normalize.go` |
| `destinations` | `TEXT[]` | JSON array text — written by `scripts.EncodeStringArray(dbType, …)` (replaced the direct `pq.Array` calls), read by `DecodeStringArray`, which accepts either form |
| `VARCHAR(n)` | length enforced | **not enforced** |
| `co` / `sw` profile filters | trigram-accelerated | correct but **full table scan** |
| Case-insensitive match | `ILIKE`, locale-aware | `LIKE`, ASCII only |
| Concurrency | multi-instance | single file, single process — **not shared storage** |

**SQLite DSN defaults** (`database.DefaultSQLiteOptions`, all required):
`_pragma=foreign_keys(1)` (enforces the schema's `ON DELETE CASCADE`), `_pragma=journal_mode(WAL)` (readers alongside a writer), `_pragma=busy_timeout(5000)`, `_txlock=immediate` (take the write lock at transaction start), `_time_format=sqlite`, `_timezone=UTC`, `_texttotime=true`. Setting `datasource.sqlite.options` **replaces** the whole string, so any override must restate every option it still needs.

Default pool size is 4 (`DefaultSQLiteMaxOpenConns`) — SQLite serialises writers, so a small pool avoids lock contention. Since `bea5aa4` this is one pool for the process rather than one per store call (§3).

> **Fixed in v0.3.64:** `DeleteInactiveCookies` (`CDS-CKI-08`) targeted a non-existent `cookie_profiles` table — the table is `profile_cookies`, as every other cookie statement uses. `cookie_cleanup_worker.go` logs the failure via `logger.Debug`, so it failed silently once per `cleanup.cookie.interval` (86400s) in every deployment since the statement was introduced; inactive cookie rows have been accumulating. **Every statement in `AllQueries()` is now prepared against both datasources in tests**, which is what surfaced it.

---

## 18. Deployment & Configuration

### Datasource choice

`datasource.type` selects the store. **An unset value means `sqlite`** (`database.DefaultType`), though the shipped `deployment.yaml` sets `postgres` explicitly, so switching requires editing it.

| | Inbuilt (`sqlite`) | PostgreSQL (`postgres`) |
|---|---|---|
| Setup | none — file + schema created on first start | run a server, apply `dbscripts/postgres.sql` |
| Docker required | no | yes (or an existing server) |
| Intended for | local dev, demos, evaluation, single-instance | production, multi-instance |

Use PostgreSQL when you need: more than one CDS instance on the same data (the Helm chart defaults to **2 replicas**, so it requires PostgreSQL); trigram-accelerated `co`/`sw` filters; non-ASCII case-insensitive matching; or `VARCHAR(n)` length enforcement.

### Local run — Option A: inbuilt database (no Docker)

```yaml
# repository/conf/deployment.yaml
datasource:
  type: "sqlite"
```

```bash
make all
cd target && unzip cds-*.zip && cd cds-* && ./cds
```

The server creates `repository/database/cds.db` (relative to CDS home) and applies the embedded schema. Override the location with `datasource.sqlite.path` — a relative path resolves against `CDS_HOME`, an absolute path is used as-is. `.gitignore` now excludes `config/repository/database/`, `*.db`, `*.db-wal`, `*.db-shm`.

### Local run — Option B: PostgreSQL

```bash
docker run -d -p 5432:5432 --name postgres \
  -e POSTGRES_USER=cdsuser -e POSTGRES_PASSWORD=cdspwd -e POSTGRES_DB=cdsdb postgres

docker exec -i postgres psql -U cdsuser -d cdsdb < dbscripts/postgres.sql

make all
cd target && unzip cds-*.zip && cd cds-* && ./cds
```

Set `DB_PASSWORD=cdspwd` in `dev.env`. The server **refuses to start** if `type: postgres` and any of `hostname`, `username`, `password`, `name` is empty (`provider.ValidateDataSource`) — previously this only logged an error and continued.

Since `bea5aa4` the start is stricter in two further ways: a negative or contradictory `datasource.postgres.*` value is refused by `ValidateDataSource`, and `EnsureDatabase` now **opens the shared pool for PostgreSQL too** and pings it. An unreachable or misconfigured server therefore fails the boot within `connect_timeout_seconds` (10s by default) instead of letting the process come up and fail the first request.

> The README's PostgreSQL snippet and script filename were corrected upstream in v0.3.63; the note in the previous version of this context about that typo no longer applies.

### Local run — Option C: scripted setup (`scripts/local-setup/script.sh`) — new in v0.3.65

Options A and B start CDS alone. **A stock IS pack has no CDS configuration**, so a CDS started that way authenticates but receives no user events. `scripts/local-setup/script.sh` does the full wiring: builds CDS and the IS extension bundles, generates and exchanges the TLS certificates, writes both configurations, registers the OAuth applications, enables CDS for the organization, starts both servers, and runs smoke tests in both directions.

```bash
./scripts/local-setup/script.sh up                 # inbuilt SQLite, no external dependencies
./scripts/local-setup/script.sh up --db postgres   # CDS on PostgreSQL in a Docker container

./scripts/local-setup/script.sh start              # ~20s — reuses an existing provisioned setup
./scripts/local-setup/script.sh restart
./scripts/local-setup/script.sh down [--purge]
./scripts/local-setup/script.sh status
./scripts/local-setup/script.sh logs cds|is
```

`up` is idempotent and provisions; `start` builds nothing, clones nothing, runs no Maven and calls no management API. `up` records the datasource, ports, offset and tenant in the work directory, and `start`/`restart`/`down`/`status`/`logs` reuse them — command-line options still win. Re-run `up` to pick up a CDS code change: it rebuilds the binary, restarts CDS and leaves IS alone.

**Requirements:** `java` 21, `maven`, `go`, `jq`; `docker` only for `--db postgres`; plus `curl`, `git`, `unzip`, `openssl`, `lsof`, `keytool`. `product-is` builds with source and target 21, so an older JDK cannot compile it — the script prefers an installed JDK 21 over the machine default and stops if the resolved one is older; an exported `JAVA_HOME` wins.

**Work directory.** Everything lives in `.local-dev/` next to the repo (`--work-dir PATH` to move it); the repository itself is never modified, and `.gitignore` now excludes `.local-dev/`.

```text
<work-dir>/
  src/product-is/                          IS pack source checkout
  src/identity-customer-data-service-extensions/
  is/wso2is-<version>/                     extracted pack, used as IS_HOME
  bin/cds                                  the CDS binary
  cds-home/repository/conf/deployment.yaml generated CDS configuration
  cds-home/repository/database/cds.db      inbuilt database (--db sqlite)
  cds-home/etc/certs/                      CDS key pair + the IS certificate
  logs/{cds,is,is-build}.log
  run/{cds,is}.pid
  state.env                                client IDs, secrets, last-`up` settings — mode 0600
```

`state.env` holds the client secrets and lives outside the repository. The generated `deployment.yaml` carries **no** secrets: its `${NAME}` placeholders are left in place and CDS expands them from the environment at load time, which the script exports when it starts the binary.

**The IS pack.** The Console renders the **Customer Data** section only if the IS build knows the `cds_host` key, so the pack must be recent. By default the script shallow-clones [`wso2/product-is`](https://github.com/wso2/product-is) and runs `mvn clean install -Dmaven.test.skip=true`. First build 10–30 min; warm rebuild ~1 min; later runs reuse the extracted pack. `--is-zip PATH` skips the build, `--is-src PATH` builds an existing checkout, `--is-ref REF` picks the branch/tag (default `master`), `--is-rebuild` forces a rebuild, `--clean` re-extracts. Only `github.com` and the public WSO2 Maven repository are contacted.

**Ports.** CDS HTTPS `8900` (`--cds-port`); IS `9443`/`9763` (`--is-offset N` shifts both); PostgreSQL `5432` (`--pg-port`). `up` fails if a port is taken; `--force` kills the holder. A second concurrent setup needs its own `--work-dir`, `--is-offset`, `--cds-port` and `--pg-container`.

**What it configures** — each item is a thing CDS silently does not work without:

| | Why it matters |
|---|---|
| Four `org.wso2.identity.customer.data.service.*` bundles in `repository/components/dropins`, **with** their `[[event_handler]]` subscriptions | `AbstractEventHandler.canHandle()` returns false without a module config — a deployed bundle receives no events |
| CDS API resources and scopes, seeded via `deployment.toml` | IS reserves the `internal_` prefix and rejects `internal_cds_*` over REST |
| `[console.extensions] cds_host` | Makes the Console call CDS directly rather than building CDS URLs off the IS origin; those cross-origin calls rely on CDS `auth.cors_allowed_origins` |
| Certificate exchange both ways (CDS cert → IS `client-truststore.p12`, IS cert → CDS trust store) | CDS does not start with an unreadable `tls.trust_store`, and neither self-signed cert is in the system roots |
| `http://wso2.org/claims/cdsProfile` local claim + SCIM2 mapping | Without it, user events carry no profile cookie (§9 Cookies, §14) |
| Relaxed policy on `/oauth2/introspect` | Lets CDS introspect with its own client credentials instead of admin-user Basic auth |
| Two OAuth apps — system app (CDS→IS) and client app with `aud=iam-cds` (caller→CDS) | The Console is separate because it issues opaque tokens |
| `cds_enabled` for the organization | Every profile, schema and rule endpoint returns `400` until set (§6); setting it also runs the initial profile-schema sync and seeds the default consent category |

**Templates over script edits.** `script.sh` handles arguments, ordering and process control; the generated configuration lives in `templates/`, one file per artifact — `is-deployment.toml` (appended to the pack's `deployment.toml` between `# BEGIN/END cds-local-dev` markers, so a re-run replaces it), `cds-deployment.yaml` (copied to the CDS home, then the `required_scopes` block read out of `config/repository/conf/deployment.yaml` and one `datasource-*.yaml` fragment appended), `datasource-sqlite.yaml` / `datasource-postgres.yaml`, `openssl.cnf`. Change behaviour by editing a template, not the script. `is-deployment.toml` and `openssl.cnf` use `${NAME}` placeholders the script substitutes (an unfilled placeholder is an error); the CDS template's placeholders are deliberately left for CDS to expand from the environment.

> Because the CDS template is a **full configuration rather than an overlay**, a section added upstream to `config/repository/conf/deployment.yaml` would be silently missed. `up` compares the two and warns. Worth remembering whenever §18's `deployment.yaml` table gains a row.

**Smoke tests** (`up` runs them; `--skip-tests` to skip, `--tests` to add them to `start`):

1. `GET /cds/api/v1/ready` → 200 — CDS up, database reachable
2. a client-credentials token carries `aud=iam-cds`
3. the same token carries the expected `org_handle`
4. CDS reports the organization as enabled
5. `GET /profiles` → 200 — introspection and scope mapping work
6. `GET /profile-schema` → 200 — CDS reached the IS claim APIs
7. a user created in IS appears as a profile in CDS — dropins, event handlers and the IS→CDS sync path work

Test 7 creates and deletes an IS user; `--keep-test-user` keeps it.

**Scope limits.** Super tenant `carbon.super` only (`--tenant` exists but no other value is supported). Only the CDS datasource is configurable — IS keeps its default H2 databases. CDS runs with the in-memory queue (§16 for ActiveMQ). Certificates are self-signed and IS keeps its default administrator, so clients talk to both servers with verification off. This is a development setup, not a deployment reference.

Documented at `docs/guides/local-development.md`, linked from `docs/README.md`.

### `deployment.yaml` sections

| Section | Notable keys |
|---|---|
| `env`, `log` | `${ENV}`, `${LOG_LEVEL}` |
| `cache` | `enabled`, `default_cache_timeout: 15` |
| `auth` | `cors_allowed_origins` |
| `addr` | `host: 127.0.0.1`, `port: 8900` |
| `server_url` | Public base URL used to construct `Location` headers |
| `auth_server` | host/port, IS endpoints, `client_id`/`client_secret`, admin credentials, `introspection_client_id`/`_secret`, `isSystemAppGrantEnabled`, `cookieDomain`, `required_scopes` |
| `application_identifier_type` | `client_id` (default) or `app_id` |
| `sync.schema` | `enabled`, `interval` (3600s) |
| `cleanup.cookie` | `enabled`, `interval` (86400s), `batch_size` (500) |
| `message_queue` | `type` + `broker` block |
| `datasource` | `type`: `sqlite` (default when unset) or `postgres`. PostgreSQL keys — host/port/name/user/password, `sslmode` — read only when `type: postgres`. **New `datasource.postgres` pool block** — `max_open_conns` (100), `max_idle_conns` (25), `conn_max_lifetime_seconds` (1800), `conn_max_idle_time_seconds` (300), `connect_timeout_seconds` (10); zero means the default, negatives and an idle limit above the open limit are refused at start (§3). SQLite keys under `datasource.sqlite` — `path` (default `repository/database/cds.db`), `options` (DSN query string; replaces defaults wholesale), `max_open_conns` (default 4) — read only when `type: sqlite`. Both sub-blocks commented out in the shipped file. |
| `shutdown` | **New.** `grace_period_seconds` (default 25) bounds the HTTP drain, the worker drain and the broker disconnect. The shipped `deployment.yaml` sets it explicitly to `25`, tells operators to keep it below the pod's `terminationGracePeriodSeconds`, and **claims it also covers the database pool — it does not** (§3). Zero means the default; negative is refused at start |
| `tls` | `mtls_enabled`, `cert_dir`, `server_cert`, `server_key`, `client_cert`, `trust_store` |

Secrets are `${ENV_VAR}`-interpolated: `AUTH_SERVER_CLIENT_SECRET`, `AUTH_SERVER_ADMIN_PASSWORD`, `INTROSPECTION_CLIENT_SECRET`, `BROKER_PASSWORD`, `DB_PASSWORD`.

### Helm (`install/helm`)

Templates for deployment, service, config, env secret, HPA, PDB, and **two ingresses** — `cds-health-ingress` (non-tenanted `/cds/...`) and `cds-tenant-ingress` (`/t/{org}/...`), matching the routing split in §4. `values.yaml` sets `replicas: 2`, so a Helm deployment requires **both** PostgreSQL (the inbuilt database is not shared storage) and the ActiveMQ queue rather than the in-memory default.

New at `bea5aa4`. Note the split: two values render into the **application config** (`confs/deployment.yaml`), two are **pod-spec** fields in `templates/cds-deployment.yaml` — which is exactly the boundary the shutdown pairing straddles.

| Value | Rendered into | Default | Note |
|---|---|---|---|
| `config.shutdown.gracePeriodSeconds` | `confs/deployment.yaml` | 25 | the app's `shutdown` block |
| `config.datasource.postgres.*` | `confs/deployment.yaml` | 100 / 25 / 1800 / 300 / 10 | rendered unconditionally whatever the datasource type, which is why `validateNumericSettings` reads only the block matching the configured type |
| `terminationGracePeriodSeconds` | `templates/cds-deployment.yaml` (pod spec) | 30 | **must stay above** the app grace period, or Kubernetes SIGKILLs mid-drain. This is the pairing the whole shutdown change depends on — and it is split across two files, so a values override that moves one and not the other breaks it silently |
| `readinessProbe.timeoutSeconds` | `templates/cds-deployment.yaml` (probe) | 5 | **new, and load-bearing.** The readiness query runs under the probe request's context and has no timeout of its own, so this is its only bound. Kubernetes would otherwise default it to 1s |

> A `DefaultReadinessTimeout` constant was added in `67d5d45` to bound the readiness check in code, then removed again in `d9b5e3a` under review. The bound is deliberately the probe request's own deadline, not a second one inside the service — so a cluster that omits `readinessProbe.timeoutSeconds` gets Kubernetes' 1-second default, and nothing in CDS will correct it.

### Versioning & release process — reworked (new `docs/process/releasing.md`)

`.github/workflows/version-bump.yml` (151 lines) was **deleted**, and `.github/workflows/release.yml` absorbed its job. The semantic change matters for anyone reading `version.txt` to find out what `main` contains:

| | Before (≤ `a9bad2f`) | After (`f4b1da6`) |
|---|---|---|
| Who owns `version.txt` | the version-bump workflow | the release builder |
| When it moves | on merge to `main` | only when a release is cut |
| What it means | roughly "current `main`" | **the last released version** |
| Release dispatch input | `version`, required, must equal `version.txt` | `branch`, `version`, `bump_type`, `use_existing_version`, `commit_version_bump`, `prerelease` — all optional |

Version resolution precedence in the release workflow: explicit `version` input wins; else `use_existing_version: true` releases `version.txt` as-is with nothing committed; else `bump_type` is applied to `version.txt` (`patch` → z+1, `minor` → y+1.0, `major` → x+1.0.0).

> **The default bump type is `minor`** (`fa454ef`), and `minor` is also listed first in the choice options so it is what the dispatch UI preselects. The intended split, per the workflow's own header comment: a **scheduled** release is a `minor` bump off `main`; a **maintenance** release is a `patch` bump off the relevant release branch. Practically this means the next ordinary release off `main` is **`v0.4.0`, not `v0.3.66`** — a dispatcher who accepts the defaults will not produce the patch-level version the `v0.3.x` series might lead them to expect.

Other release-path changes:

- A release is **refused if the resolved tag already exists** (`git rev-parse "$VERSION"`) — the tag, not the file, is now the release identity.
- The version regex accepts an optional prerelease suffix: `^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9a-zA-Z.-]+)?$`.
- Releases can be built from **any branch** via the `branch` input, defaulting to `main` — this is what makes maintenance releases off a release branch possible.
- `version.txt` is always *written* so `make build` names the artifact correctly, but only *committed* when `commit_version_bump` is true.
- The workflow checks out with `secrets.CDS_RELEASE_PAT || secrets.GITHUB_TOKEN`; commits are authored as `wso2-iam-bot`.
- `update-deployment-versions.yml` no longer looks up the tag by the dispatched commit SHA — because the builder now commits the bump *and then* tags it, the dispatched SHA is not the tagged one. It resolves `git describe --tags --abbrev=0 --match 'v[0-9]*'` against the released branch tip instead.

Helm chart version: `install/helm/Chart.yaml` → **`0.0.34`** (`71fb3bd`, bumped from `0.0.33` at `4d50ab3`, which was itself `0.0.32` before). The chart version is tracked separately from the product version and is bumped by `wso2-iam-cloud-bot` independently of any code change — `71fb3bd` is a chart-version bump and nothing else. `appVersion` remains the stale placeholder `"0.0.1"`, so the chart still advertises a version that has never matched the product.

`docs/process/releasing.md` (134 lines, new) documents the model, inputs, scheduled/maintenance/major/prerelease runs, retrying a failed release, post-release steps, verification, Helm chart versioning and troubleshooting. `docs/README.md` gained a **Process** table pointing at it. Commit `3c56a87` ("Reword the release doc to avoid cutting a release") rewrote instruction text that, followed literally, would have triggered a real release.

### Tests

`test/integration`, `test/activemq_integration`, `test/setup`, plus `Integration_Test_Guide.MD` and `Unification_Scenarios.MD`.

The integration suite runs against either datasource: `CDS_TEST_DB=sqlite` selects the inbuilt database (`test/setup/sqlite_test_db.go`), anything else spins up PostgreSQL via testcontainers. Added in v0.3.63–64: `test/integration/profile_pagination_test.go`, `test/integration/queries_prepare_test.go` (prepares every statement in `scripts.AllQueries()` — this is what would have caught the `cookie_profiles` bug), `test/setup/query_prepare.go`. The old `TEST_MODE=true` signal for "don't close the pool" was replaced first by a `shared` flag on `DBClient` (v0.3.63) and then, at `bea5aa4`, by the shared pool itself — neither `TEST_MODE` nor `shared` exists in the tree any more; `provider.SetTestDB` is the only test hook. Added at `b38e9bd`: `test/integration/profile_link_test.go` (§9), which is also the first integration test to assert on the **contents of `deployment.yaml`** rather than on API behaviour alone.

CI (`.github/workflows/pr-builder.yml`) gained a `sqlite-test` job alongside the PostgreSQL one; it runs `make unit-test` **and** `make integration-test-sqlite`, and is the only job that runs the unit tests. `dependency-validation.yml` was hardened to pin the PR head SHA and fail if the head moved mid-run.

> **Correction to earlier snapshots of this document:** they reported a latent bug in `test/integration/main_test.go` — the `default:` branch for an unrecognised `CDS_TEST_DB` supposedly missing its `os.Exit(1)`. The `os.Exit(1)` is there, and was already there at `b38e9bd`. The claim was wrong and is withdrawn.

**Added at `bea5aa4` — 18 new files carrying 3,230 added lines, most of it test** (the range as a whole is +5,298 / −997 across 74 files). `make unit-test` now runs under `-race`, which is the check that matters for a shared pool.

| New test file | Covers |
|---|---|
| `cmd/server/shutdown_test.go` | The shutdown order — HTTP first, pool last, no worker still stopping when the pool closes; workers stopping concurrently (a barrier that a sequential implementation cannot pass); the configured grace period becoming the deadline; an overrunning worker or HTTP server still reaching the caller |
| `internal/system/config/shutdown_test.go` | `ResolveShutdownGracePeriod` — zero → default, negative refused |
| `internal/system/workers/lifecycle_test.go` | `jobLifecycle` — refusing late jobs, draining active ones, cancelling at the deadline |
| `internal/system/database/provider/lifecycle_test.go`, `concurrency_test.go`, `config_validation_test.go`, `connect_timeout_test.go`, `dbprovider_test.go` | Pool ownership across goroutines, `CloseDB` idempotence and `ErrDatabaseClosed`, every refused-settings case, the connect-timeout bound |
| `internal/system/database/client/ownership_test.go`, `transaction_test.go`, `dbclient_context_test.go` | `Close()` leaves the shared pool open; transactions under a context; cancellation reaching the query |
| `internal/system/database/model/tx_test.go` | `Tx.ExecContext` / `QueryContext` dialect handling |
| `internal/system/queue/activemq/close_test.go` | The forced close path — a disconnect that outlives the deadline, a reconnect racing shutdown |
| `test/integration/provider_test.go` | The **production** provider driven against the PostgreSQL container, using the new connection fields on `setup.TestPostgres` rather than the suite handle |
| `test/integration/context_cancellation_test.go` | A cancelled or expired context stopping a store call and a transaction — and the `requireCauseIs` idiom that `ServerError`'s missing `Unwrap` forces |
| `test/integration/request_deadline_test.go` | A deliberately saturated one-connection pool: proves a request's deadline ends the **wait for a connection**, not just the query |

`test/setup/postgres_test_containter.go` also gained a real readiness wait — `wait.ForLog("database system is ready to accept connections").WithOccurrence(2)` — because the image starts the server twice (once for `initdb`) and a port-only wait returned during the first start, so the first query could fail with "the database system is starting up".

### Docs gaps at `bea5aa4`

The repo's own `docs/` was not updated with this range. Flagged here, not edited:

- **`docs/guides/extending-queue-providers.md` is now wrong, not merely incomplete.** It documents the queue contract as `Close() error` in its method table (line 29) and in both `ProfileQueue` / `SchemaSyncQueue` code examples. The interface is `Close(ctx context.Context) error`. A provider written from this guide will not compile against `internal/system/queue/queue.go`, and the guide says nothing about the new obligation to return inside the deadline or end the connection by force.
- **No document covers the connection pool.** `datasource.postgres.*` exists only in the commented block of `config/repository/conf/deployment.yaml` and in `values.yaml`. There is no operator-facing note that the numbers are per instance, that 2 replicas × 100 connections is the real ceiling against the server's `max_connections`, or that a negative or contradictory value stops the server.
- **No document covers graceful shutdown.** `shutdown.grace_period_seconds` and its required pairing with the pod's `terminationGracePeriodSeconds` are described only in YAML comments. `docs/guides/local-development.md` and `docs/README.md` mention neither.
- **`api/customer-data-service.yaml` does not describe `/health` or `/ready` at all**, so the readiness probe's new dependence on the caller's request deadline (`readinessProbe.timeoutSeconds`) is undocumented on the API side. Unchanged by this range, but newly relevant.

---

## 19. Real-World Scenarios

> *Illustrative, drawn from requirements material and validated against current merge semantics.*

### Scenario A: Guest → Registered Shopper (Identity Stitching)

```
Before login (guest)
  [P_tmp_1] temporary
     ├─ profile_id = pid-abc
     ├─ user_id    = (none)
     ├─ traits: interest = shoes
     └─ application_data.web: cart_items = [A12, B44]

After login (stitched)
  [P_perm_U1] permanent / master
     ├─ user_id    = U1
     ├─ identity_attributes: email, mobile, name
     ├─ traits: interest = shoes
     ├─ application_data.web: cart_items = [A12, B44]
     └─ merged_from: [{ profile_id: pid-abc, reason: "system:user_id_match" }]
```

Schema needed:
- `application_data.web.cart_items` → `multi_valued: true`, `merge_strategy: combine`
- `identity_attributes.email` → `combine` (multi-valued) or `overwrite`
- `traits.interest` → `combine`

> `latest` is not a usable strategy — use `overwrite`.

### Scenario B: Multi-Channel Unification (Retail)

Four profiles:
- P1: web guest (`mobile=x`, `email=y`), `cart_items=[A12]`
- P2: web guest, later session (`mobile=x`), `cart_items=[B44]`
- P3: mobile app, logged in (`user_id=U1`, `email=y`), `wishlist=[W9]`
- P4: loyalty POS (`loyalty_id=L77`, `mobile=x`), `points=1200`

Rules by priority: userId (system) → mobile → email → loyalty ID.

Master (P3, the only permanent profile):
```
├─ user_id = U1
├─ identity_attributes: { email: y, mobile: x }
├─ traits: { interest: shoes }
├─ application_data:
│    ├─ web:     { cart_items: [A12, B44] }
│    ├─ mobile:  { wishlist: [W9] }
│    └─ loyalty: { loyalty_id: L77, points: 1200 }
└─ merged_from: [P1, P2, P4]
```

Because merging is pairwise and asynchronous, the profiles converge over several worker passes rather than in one shot — and only one rule fires per pass.

### Scenario C: Personalized Onboarding (SaaS signup funnel)

1. Visitor lands from a search for "AI agent access control" → `POST /profiles` creates a guest profile, cookie set
2. Context captured as traits (referrer, search keywords, device)
3. Browsing AI content updates `traits` with an interest tag
4. Newsletter signup writes `identity_attributes.email` onto the guest profile
5. Sign-up fires `POST_ADD_USER` with the cookie → `user_id` attached, prior preferences retained

Enables a personalized landing experience and filtered exports, e.g.
`?filter=identity_attributes.region eq China and traits.signupFor eq AI`

### Scenario D: Large Enterprise (Hotel Chain)

Three siloed systems: loyalty (keyed by personal email), booking (corporate email), on-site spa & dining (phone, unauthenticated).

CDS approach: **C1** as central master data authority ingesting from all three; **A2** unification rules merging across them without changing the legacy systems; **D** exposing the unified profile via consent-filtered APIs for personalized booking experiences.

---

## 20. MVP & Roadmap

### Implemented today

- **Profile management** — create/init anonymous profiles, cursor-paginated list with filters and attribute projection, get, patch, soft delete, `/Me` via cookie
- **Profile schema management** — per-scope attribute CRUD, typed validation, canonical values, complex sub-attributes
- **Unification rules** — CRUD with name and priority uniqueness
- **Profile unification** — async worker, `userId` invariant + priority-ordered rules, four merge cases, schema-driven attribute merging
- **Consent management** — categories, mandatory Identity Data, per-profile records, consent-filtered reads, system-app bypass
- **IS integration** — user lifecycle + session event sync, claim sync, event-driven and scheduled schema sync
- **Admin config** — per-org enablement and system-app registration
- **Ops** — health/readiness, cookie cleanup worker, pluggable queue, Helm chart with HPA/PDB, mTLS config
- **Zero-setup deployment** *(new in v0.3.63)* — inbuilt SQLite datasource, the default when `datasource.type` is unset. Schema embedded and applied on first start; no Docker, no external DB server. Dual-dialect query layer keeps the stores datasource-agnostic.

### Partially built / scaffolded

- Approval workflow for unification — `WAIT_ON_ADMIN` / `WAIT_ON_USER` statuses, `profile_unification_modes` and `profile_unification_triggers` tables, and `DefaultConfig()` exist, but nothing drives them and no API is exposed
- Merge strategies `ignore` / `latest` / `oldest` — partly implemented, not accepted by validation
- `computed` mutability — constant defined, not in the allowed set
- `origin_country` on `profiles` — column exists, no API path writes it

### Roadmap

- Probabilistic profile resolution (fuzzy matching)
- Synchronous profile resolution
- Profile unification approval workflow
- Data import and export
- Event ingestion to build profiles (profile enrichment from interactions)
- User segmentation (filter/export profiles by properties)
- Identity graph visualization
- MCP server for CDS
- Customer journey orchestration
- AI context support
- Verifiable Credential support

### Open items

- Authentication CDS → IS: system-app grant for org-specific tokens (`isSystemAppGrantEnabled` is present and defaults to `false`)
- Authorization is in-house — scope checking against JWT claims or introspection
- Guaranteed delivery on the message queue
- Console UI: editable profiles, account→profile linking, consent management screens
- Performance benchmarking
- Packaging: Go binary distribution alongside IS (cross-platform)
- Pricing model

---

*Reconciled against `wso2/identity-customer-data-service` `main` @ `a9bad2f` (`v0.3.65`, 2026-09-02). Sections 1, 2, 19 and the roadmap portion of 20 carry forward internal requirements material; everything else is derived from the source tree.*
