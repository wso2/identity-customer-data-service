# The System Application

CDS calls the Identity Server management APIs on behalf of each organisation it
serves: it reads the claim dialects to build the profile schema, resolves
applications, and looks up users over SCIM. Those calls need an access token, and
that token comes from a machine-to-machine application in the Identity Server.

This guide is about where that application comes from.

---

## Two deployment shapes

**Provisioned for.** Something outside CDS creates the application and hands CDS
its credentials as `AUTH_SERVER_CLIENT_ID` and `AUTH_SERVER_CLIENT_SECRET`. The
cloud control plane does this, and so does
[`scripts/local-setup/script.sh`](local-development.md). CDS reads the pair at
start and presents it on every token request.

**Provisioning itself.** CDS creates the application, as each organisation enables
CDS, and holds the credentials in its own database. This is the on-premise shape.

The first is the default. The second is what `auto_provision` turns on.

---

## Why one configured pair is not enough on-premise

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
4. It is subscribed to exactly the APIs CDS asks a token for, and nothing more:

   | API resource | Scopes |
   |---|---|
   | `/api/server/v1/applications` | `internal_application_mgt_view` |
   | `/api/server/v1/claim-dialects` | `internal_claim_meta_view` |
   | `/scim2/Users` | `internal_user_mgt_list`, `internal_user_mgt_view` |

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

`FetchToken` resolves the organisation's own credentials where a row exists, and
falls back to the configured `client_id` / `client_secret` where none does. A
deployment with `auto_provision` off does not read the table at all, so its token
path is unchanged.
