---
title: Identity Providers — WSO2 Identity Server vs. WSO2 Thunder
date: 2026-07-13
---

# 🔑 Identity Providers

CDS delegates all OAuth2/OIDC concerns — M2M token issuance, inbound token
validation, and application lookup — to an external identity provider. This
is selected via `auth_server.provider` and implemented behind the
`identity_provider.Client` interface (`internal/identity_provider/client.go`).

| `auth_server.provider` | Implementation | Status |
|---|---|---|
| `"wso2is"` (default, or unset) | `internal/identity_provider/wso2is` | Full support — the original, production integration. |
| `"thunder"` | `internal/identity_provider/thunder` | Early support for [WSO2 Thunder](https://github.com/thunder-id/thunderid). Read the limitations below before using it beyond a spike/PoC. |

---

## Using WSO2 Thunder

Set the following in `deployment.yaml` (or the Helm equivalent under
`identityServer`/`thunder` in `values.yaml`):

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

Register a `client_credentials` application in Thunder and put its
credentials in `THUNDER_CLIENT_ID` / `THUNDER_CLIENT_SECRET` (e.g. in
`config/dev.env`). Nothing else in CDS's own routing changes — the
`/t/{orgHandle}/...` URL scheme is CDS's own convention, not Thunder's.

## What works today

- **M2M token acquisition** — `client_credentials` against Thunder's
  `/oauth2/token`. Thunder has no `system_app_grant`/org-token-exchange
  grant, so `auth_server.isSystemAppGrantEnabled` is ignored in Thunder mode.
- **Inbound token validation** — RFC 7662 introspection against
  `/oauth2/introspect`, and local JWT signature verification via Thunder's
  standard JWKS endpoint (`thunder.Client.VerifyJWT` — exposed and unit
  tested, but not yet wired into the request-authentication hot path; see
  Phase 2 below).
- **Application lookup by clientId** — `FetchApplicationIdentifier`, used to
  validate `application_identifier` profile-schema attributes. Thunder's
  `/applications` API has no server-side filter, so this pages through
  results and matches `clientId` client-side (capped at 20 pages / 1000
  apps). Unlike wso2is, there is no `issuer` field to match against, so
  issuer-based application identifiers won't resolve under Thunder.

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
  there is no OU/org parameter on `/oauth2/authorize` or `/oauth2/token`, nor
  an OU claim in the introspection response. Practically, this means:
  - A valid, active Thunder token with the right scope from *any*
    registered client can be presented against *any* org's
    `/t/{orgHandle}/...` CDS path — nothing today cryptographically binds a
    Thunder token to one org/OU. CDS's `authz.ValidatePermission` scope
    check and the org-handle-derived DB routing still apply, but they are
    not a substitute for real tenant isolation.
  - **Never reuse the same Thunder `client_id` as a "system application"
    across two different CDS orgs** (`admin_config.system_applications`) —
    Thunder's global namespace makes that trust ambient across orgs.
  - If you map a CDS `org_handle` to a Thunder OU handle for your own
    administrative/organizational purposes, treat that mapping as a label,
    not a security boundary, until Thunder adds real per-OU isolation
    upstream.

## Phase 2 (not implemented yet)

- Wire `VerifyJWT`/JWKS into `internal/system/authn`'s request-authentication
  path (with a package-level JWKS cache — the current `identity_provider.Client`
  is constructed fresh per call site, so a struct-field cache would never be
  reused across requests).
- Revisit OU-based isolation if/when Thunder adds a real per-OU client_id
  namespace or token-scoping mechanism upstream.
