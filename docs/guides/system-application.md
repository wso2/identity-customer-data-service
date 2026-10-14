# The System Application

CDS calls the identity provider's management APIs on behalf of each organisation it
serves: it reads the claim dialects to build the profile schema, resolves
applications, and looks up users. Those calls need a credential, and on a WSO2
Identity Server that credential comes from a machine-to-machine application.

This guide is about where that application comes from, and how CDS decides what to do
for the identity provider it is pointed at.

---

## Which identity provider

```yaml
idp:
  type: "wso2is"   # or "asgardeo"
```

One deployment works with one identity provider. The setting decides how CDS
authenticates itself:

| | `wso2is` | `asgardeo` |
|---|---|---|
| How a credential is obtained | `client_credentials` against the organisation's token endpoint | `system_app_grant` exchanged at the super tenant |
| Authorization scheme | `Bearer` | `SystemApp` |
| Who creates the application | CDS, if `auto_provision` is on | the Asgardeo control plane |

`auth_server.isSystemAppGrantEnabled` is the deprecated spelling of the same choice.
The system application grant is an Asgardeo flow, so that flag only ever said which
provider you were on — one bit for a choice that now has more than two answers. It is
still honoured when `idp.type` says nothing, and the server warns at start; `idp.type`
wins wherever both are set.

Nothing above the credential source names a grant, a scheme, or an `internal_` scope.
CDS asks for a credential carrying the permissions it needs — `read:applications`,
`read:claims`, `read:users` — and puts the result in an `Authorization` header. A
provider that authenticates with a static key or a client certificate, rather than a
grant, answers without a round trip.

---

## Two deployment shapes

**Provisioned for.** Something outside CDS creates the application and hands CDS
its credentials as `AUTH_SERVER_CLIENT_ID` and `AUTH_SERVER_CLIENT_SECRET`. Asgardeo
does this from its control plane, and so does
[`scripts/local-setup/script.sh`](local-development.md). CDS reads the pair at
start and presents it on every token request. Provisioning does nothing on such a
deployment — the credential source declines it, rather than every caller testing
which provider it is talking to.

**Provisioning itself.** CDS creates the application, as each organisation enables
CDS, and holds the credentials in its own database. This is the on-premise shape.

The first is the default. The second is what `auto_provision` turns on.

---

## Why one configured pair is not enough on-premise (`wso2is`)

An OAuth client in the Identity Server belongs to one tenant, and can only
authenticate at the token endpoint of that tenant. CDS builds a token endpoint per
organisation — `https://<is>/t/<org>/oauth2/token` — so a single configured pair
works for exactly one organisation: the one its application was created in. Every
other organisation fails at the token call.

That is fine where a deployment serves one organisation, which is the cloud case.
An on-premise Identity Server with several tenants needs an application per tenant,
and something has to create them.

---

## Turning it on

```yaml
auth_server:
  admin_username: "admin"
  admin_password: "${AUTH_SERVER_ADMIN_PASSWORD}"
  system_application:
    auto_provision: true
    name_prefix: "CDS System App"
    encryption_key: "${SYSTEM_APP_ENCRYPTION_KEY}"
```

```bash
openssl rand -base64 32    # the value for SYSTEM_APP_ENCRYPTION_KEY
```

The server refuses to start when `auto_provision` is on and the key is missing or
unusable, and when the administrator credentials are not set. Both are failures a
deployment wants at start rather than at the first organisation that enables CDS.

---

## What happens on enable

`PATCH /cds/api/v1/config` with `cds_enabled: true` provisions before it does
anything else, because the initial profile-schema sync in the same call is itself a
management API call and needs the application to exist first.

1. An organisation whose credentials are already recorded is left alone.
2. An application named `<name_prefix> - <org handle>` already in the Identity
   Server is adopted. A restored database, or an earlier run that stopped half
   way, therefore does not produce a second application.
3. Otherwise a machine-to-machine application is created: `client_credentials`
   grant, JWT access tokens, management application.
4. It is subscribed to exactly the permissions CDS asks for, and nothing more. The
   mapping from a permission to the API resource and scopes that grant it is the
   identity provider's business, not CDS's:

   | CDS permission | API resource | Scopes |
   |---|---|---|
   | `read:applications` | `/api/server/v1/applications` | `internal_application_mgt_view` |
   | `read:claims` | `/api/server/v1/claim-dialects` | `internal_claim_meta_view` |
   | `read:users` | `/scim2/Users` | `internal_user_mgt_list`, `internal_user_mgt_view` |

5. The credentials the Identity Server issued are read back and stored.

Re-running is safe at every step.

---

## The administrator credentials

Provisioning runs as `admin_username` / `admin_password`, over HTTP Basic against
the management API. It has to: until provisioning has run, the organisation has no
application to get a token with.

This is the reason `auto_provision` is off by default. Turning it on grants CDS the
outbound use of an Identity Server administrator credential — before it, CDS uses
those credentials only to *verify* Basic authentication on its own admin surface.
A deployment that cannot accept that should provision its applications externally
and leave `auto_provision` off.

---

## How the secret is stored

Sealed, with AES-256-GCM, under `encryption_key`. Each row carries the nonce and
the ID of the key that sealed it, so a secret sealed under a key the deployment no
longer holds is reported as exactly that rather than as a corrupt row.

**It is not hashed, and cannot be.** CDS is the OAuth *client* here: it presents
this secret to the token endpoint on every token request. Hashing is for a secret
you verify, not one you send, and a hash cannot be presented. What CDS holds has to
be recoverable, so the protection is the key living outside the database — a
database dump on its own yields nothing usable against the Identity Server.

Rotating the key is not yet supported: there is no re-sealing pass, so a changed
key strands the rows sealed under the old one. Rotating a *client secret* is done
by deleting the organisation's row and re-enabling, which re-provisions.

---

## Where the credentials are used

The `wso2is` source resolves the organisation's own credentials where a row exists,
and falls back to the configured `client_id` / `client_secret` where none does. A
deployment with `auto_provision` off does not read the table at all, so its token path
is unchanged.

---

## Adding another identity provider

Implement `credential.Source` — `Token` and `Provision` — and register it from an
`init` function under the name `idp.type` should accept. The contract lives in
[`internal/system/idp/credential`](../../internal/system/idp/credential/credential.go)
and depends on nothing provider-specific; the implementations live beside the HTTP
machinery that talks to each provider.

A provider with no grant concept returns a credential it already holds. A provider
that issues CDS its credentials from outside returns `nil` from `Provision`. Neither
needs a change anywhere else.

Two things are **not** behind this contract yet, and a new provider will meet both:
inbound token validation (`internal/system/authn` still assumes JWT-or-introspection,
an `iam-cds` audience and an Identity Server Console client), and the `/t/{org}` URL
tenancy the management calls are built on.
