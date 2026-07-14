---
title: Identity Providers — WSO2 Identity Server vs. WSO2 Thunder
date: 2026-07-14
---

# 🔑 Identity Providers

CDS delegates all OAuth2/OIDC concerns — M2M token issuance, inbound token
validation, and application lookup — to an external identity provider. This
is selected via `auth_server.provider` and implemented behind the
`identity_provider.Client` interface (`internal/identity_provider/client.go`).

| `auth_server.provider` | Implementation | Status |
|---|---|---|
| `"wso2is"` (default, or unset) | `internal/identity_provider/wso2is` | Full support — the original, production integration. |
| `"thunder"` | `internal/identity_provider/thunder` | Early support for [WSO2 Thunder](https://github.com/thunder-id/thunderid). Verified against a real Thunder **v0.48.0** instance (see "Testing" below) — read the limitations here before using it beyond a spike/PoC. |

---

## Setting up Thunder

Quick start with Docker (Thunder ships an official prebuilt image with an
embedded SQLite database and a self-signed TLS cert — no external DB needed):

```bash
docker run -d --name thunder -p 8090:8090 \
  --entrypoint sh ghcr.io/thunder-id/thunderid:latest \
  -c "./setup.sh && ./start.sh"
```

`setup.sh` bootstraps a default super-admin user (`admin` / `admin`, override
via `ADMIN_USERNAME`/`ADMIN_PASSWORD` env vars) and default resources, then
`start.sh` runs the server at `https://localhost:8090` (Console UI at
`/console`). Confirm it's up:

```bash
curl -sk https://localhost:8090/health/readiness
```

### Registering CDS as a client_credentials application

Thunder's `/applications` management API and its Console UI both require an
already-authenticated admin bearer token to create an application — there is
no open self-registration endpoint suitable for a first-time bootstrap.
The practical way to provision CDS's application ahead of time is a
**bootstrap resource file**, applied by `setup.sh` before the server starts.
Drop a file like this into `/opt/thunderid/bootstrap/` (alongside Thunder's
own `01-default-resources.yaml`) before running `setup.sh`:

```yaml
# resource_type: application
id: <any-fresh-uuid>
name: CDS
ouId: 01900000-0000-7000-8000-000000000001   # Thunder's default OU
allowedUserTypes:
  - Person
inboundAuthConfig:
  - type: oauth2
    config:
      clientId: cds
      clientSecret: <a-strong-secret>
      grantTypes:
        - client_credentials
      tokenEndpointAuthMethod: client_secret_basic
      publicClient: false
```

Put `clientId`/`clientSecret` into `THUNDER_CLIENT_ID`/`THUNDER_CLIENT_SECRET`
(e.g. in `config/dev.env`) and set in `deployment.yaml` (or the Helm
equivalent under `identityServer`/`thunder` in `values.yaml`):

```yaml
auth_server:
  provider: "thunder"
  thunder:
    host: "localhost"
    port: "8090"
    client_id: "${THUNDER_CLIENT_ID}"
    client_secret: "${THUNDER_CLIENT_SECRET}"
    tokenEndpoint: "/oauth2/token"
    introspectionEndpoint: "/oauth2/introspect"
    jwksEndpoint: "/oauth2/jwks"
    applicationsEndpoint: "/applications"
```

Nothing else in CDS's own routing changes — the `/t/{orgHandle}/...` URL
scheme is CDS's own convention, not Thunder's.

`test/setup/thunder_test_container.go` contains a working, minimal
bootstrap-resource fixture used by the automated tests below — copy its
`thunderBootstrapYAML` constant as a starting point.

---

## Testing

`internal/identity_provider/thunder/client_test.go` unit-tests the client
against a mocked `httptest` server (fast, no Docker). In addition,
**`test/thunder_integration/`** runs the same client against a real,
ephemeral Thunder **v0.48.0** container (via testcontainers-go, image
`ghcr.io/thunder-id/thunderid:latest`) — this is what actually proved out
the behavior documented on this page, not just reading Thunder's API specs.
Run it with:

```bash
go test ./test/thunder_integration/...   # requires Docker
```

(If Docker runs via Colima rather than Docker Desktop, set `DOCKER_HOST` to
your Colima socket first, e.g. `export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock`.)

## What's verified working today (against real Thunder v0.48.0)

- **M2M token acquisition** — `client_credentials` against Thunder's
  `/oauth2/token`. Thunder has no `system_app_grant`/org-token-exchange
  grant, so `auth_server.isSystemAppGrantEnabled` is ignored in Thunder mode.
- **Inbound token validation** — RFC 7662 introspection against
  `/oauth2/introspect`, and local JWT signature verification via Thunder's
  standard JWKS endpoint (`thunder.Client.VerifyJWT` — exposed and tested,
  but not yet wired into the request-authentication hot path; see Phase 2
  below).
- Thunder's client_credentials **JWT access tokens carry `ouId`/`ouHandle`/
  `ouName` claims** (confirmed by decoding a real token) — but the
  **introspection response does not** include them. This asymmetry matters:
  today CDS only calls introspection (see `internal/system/authn/auth.go`),
  so it never sees OU info from Thunder even though it's sitting right there
  in the JWT. Phase 2's "wire VerifyJWT into the auth path" would also make
  this OU claim available to CDS — see the isolation caveat below for why
  that still wouldn't be a full fix.

## What's confirmed broken today (not a CDS bug — a Thunder v0.48.0 gap)

- **`FetchApplicationIdentifier` always fails with `403 Forbidden`.** Calling
  Thunder's `/applications` management API requires the caller's token to
  carry the "system" role, but **Thunder's role-assignment model only
  accepts `user` or `group` assignees — not `application`** (confirmed:
  attempting an `application`-type assignment during bootstrap fails with
  `ROL-1016: Invalid assignee type`). Since a `client_credentials` token is
  never attached to a user or group, it can never be granted this
  permission — regardless of the `scope` requested at the token endpoint.
  `test/thunder_integration` asserts this 403 explicitly so a future Thunder
  fix (or a discovered workaround) will be caught by a failing test rather
  than silently going unnoticed. Until then, any CDS schema attribute using
  `application_identifier` validation cannot be validated against Thunder.
- Thunder's `/applications` API also has no server-side filter (only
  `limit`/`offset` pagination) and no `issuer` field, unlike wso2is — moot
  today given the 403 above, but relevant if/when the role-assignment gap is
  fixed upstream.

## What's out of scope (by design, not a bug)

- **Profile schema auto-sync and SCIM.** Thunder has no claim-dialect or
  SCIM2 API — there's nothing to sync from. When `provider: thunder`:
  - The background schema-sync worker becomes a no-op for that deployment
    (`ProfileSchemaService.SyncProfileSchema` logs and returns immediately).
  - The IS-originated schema-sync webhook returns `501 Not Implemented`.
  - **You must create and maintain the profile schema manually** via CDS's
    own profile-schema CRUD API.
- **No real multi-tenant isolation.** Thunder has "Organization Units" (OUs)
  for grouping users/apps/roles, but they are **not** a tenant-isolation
  boundary: `client_id` is a single global namespace across all OUs, and
  there is no OU/org parameter on `/oauth2/authorize` or `/oauth2/token`.
  Practically, this means:
  - A valid, active Thunder token with the right scope from *any*
    registered client can be presented against *any* org's
    `/t/{orgHandle}/...` CDS path — nothing today cryptographically binds a
    Thunder token to one org/OU (the `ouHandle` JWT claim above is
    informational, not enforced by Thunder itself, and CDS doesn't check it
    yet either). CDS's `authz.ValidatePermission` scope check and the
    org-handle-derived DB routing still apply, but they are not a substitute
    for real tenant isolation.
  - **Never reuse the same Thunder `client_id` as a "system application"
    across two different CDS orgs** (`admin_config.system_applications`) —
    Thunder's global namespace makes that trust ambient across orgs.
  - If you map a CDS `org_handle` to a Thunder OU handle for your own
    administrative/organizational purposes, treat that mapping as a label,
    not a security boundary, until Thunder adds real per-OU isolation
    upstream.

## Anonymous-to-authenticated profile linking

CDS tracks a pre-login visitor purely internally: an "anonymous" profile is
just a database row with no `UserId`, keyed by a `cds_profile` cookie CDS
itself issues (`internal/profile/handler/profile_handler.go`,
`CreateProfileCookie`) — **no identity-provider call happens anywhere in
anonymous profile creation or its periodic cleanup**
(`internal/system/workers/cookie_cleanup_worker.go`), regardless of
`auth_server.provider`. This part needs no changes for Thunder.

Linking that anonymous profile to a real user *after* login is a different
story. Under wso2is, that link is driven by IS's **Action framework**: an
Action is configured in IS to call CDS's own `POST .../profiles/sync`
webhook (Basic admin auth) on `AuthenticationSuccess`, `AddUserEvent`, and
`SessionTermination` events, passing the cookie value + userId so CDS can
merge the anonymous and authenticated profiles
(`internal/profile/handler/profile_handler.go`, `SyncProfile`).

**Thunder has no equivalent global Action/event-subscription framework yet.**
What it does have:

- A flow/graph engine (login and registration are modeled as configurable
  node graphs) with a built-in `HTTPRequestExecutor` node that can call an
  arbitrary external HTTPS endpoint — including custom headers (so
  `Authorization: Basic ...` works) and a JSON body with placeholder
  resolution (e.g. `{{ctx(userId)}}`). An admin can manually drop this node
  into the tail of the Authentication flow (for `AuthenticationSuccess`) and
  the Registration flow (for `AddUserEvent`), configured to call CDS's
  `/profiles/sync` webhook the same way IS's Action would. This is
  per-flow, hand-authored configuration — not a one-time global
  subscription like IS Actions.
- **No logout/session-termination flow type exists at all** in Thunder
  today, so there is no way to notify CDS of `SessionTermination` — CDS's
  cookie-deactivation-on-logout path (`profile_handler.go`, the
  `SessionTermination` case in `SyncProfile`) simply cannot be triggered
  under Thunder. Anonymous-profile cookies will instead age out via the
  existing `cookie_cleanup_worker` on its normal inactivity timeout.
- A proper, generalized action/event framework is on Thunder's own roadmap
  (tracked upstream as thunder-id/thunderid#2118 and #2119) but not released
  as of v0.48.0.

**Net effect:** under `provider: thunder`, anonymous→authenticated profile
linking works only if you manually wire an `HTTPRequestExecutor` node into
Thunder's Authentication/Registration flows yourself (undocumented as a
turnkey CDS feature — no CDS-side code exists to automate this), and
logout-driven cookie deactivation doesn't work at all. This is a Phase 2 item.

## Phase 2 (not implemented yet)

- Wire `VerifyJWT`/JWKS into `internal/system/authn`'s request-authentication
  path (with a package-level JWKS cache — the current `identity_provider.Client`
  is constructed fresh per call site, so a struct-field cache would never be
  reused across requests).
- Revisit OU-based isolation if/when Thunder adds a real per-OU client_id
  namespace or token-scoping mechanism upstream.
- Revisit `FetchApplicationIdentifier` if/when Thunder's role-assignment
  model supports application assignees (or another authorization path for
  M2M callers) — track thunder-id/thunderid for this.
- Document/automate the Authentication-flow `HTTPRequestExecutor` webhook
  node setup for anonymous-profile linking, once Thunder's flow-export
  format stabilizes enough to ship a ready-made flow snippet.
